package googlecalendar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"trello-calendar-pp-cli/internal/scheduling"
)

func testLocation(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestListEventsPaginatesAndAcceptsCancelledTombstone(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageToken") == "" {
			json.NewEncoder(w).Encode(map[string]any{
				"nextPageToken": "next",
				"items": []any{
					map[string]any{"id": "timed", "status": "confirmed", "start": map[string]any{"dateTime": "2026-07-13T10:00:00+02:00"}, "end": map[string]any{"dateTime": "2026-07-13T11:00:00+02:00"}},
					map[string]any{"id": "deleted", "status": "cancelled"},
				},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "all", "status": "confirmed", "start": map[string]any{"date": "2026-07-14"}, "end": map[string]any{"date": "2026-07-15"}}}})
	}))
	defer server.Close()
	c := NewClient(server.URL, "primary", testLocation(t), server.Client())
	events, err := c.ListEvents(context.Background(), time.Date(2026, 7, 13, 0, 0, 0, 0, testLocation(t)), time.Date(2026, 7, 20, 0, 0, 0, 0, testLocation(t)))
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || len(events) != 3 {
		t.Fatalf("calls=%d events=%#v", calls.Load(), events)
	}
	if events[2].ID != "deleted" && events[1].ID != "deleted" && events[0].ID != "deleted" {
		t.Fatalf("cancelled tombstone missing")
	}
}

func TestRetryAndDeterministicTombstoneDuplicate(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "tcc") {
			w.WriteHeader(http.StatusGone)
			return
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()
	c := NewClient(server.URL, "primary", testLocation(t), server.Client())
	c.Sleep = func(time.Duration) {}
	start := time.Date(2026, 7, 13, 0, 0, 0, 0, testLocation(t))
	if _, err := c.ListEvents(context.Background(), start, start.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("retry calls=%d", calls.Load())
	}
	exists, err := c.FindCard(context.Background(), "board", "card")
	if err != nil || exists {
		t.Fatalf("deleted deterministic event must be retried: exists=%v err=%v", exists, err)
	}
}

func TestCreateEventExtendedProperties(t *testing.T) {
	var received apiEvent
	var postCalls atomic.Int32
	var eventID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			postCalls.Add(1)
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Errorf("decode request: %v", err)
			}
			w.Write([]byte(`{"id":"wrong-response-id"}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": eventID, "status": "confirmed", "extendedProperties": map[string]any{"private": map[string]string{"trelloCardId": "card", "trelloBoardId": "board"}}})
	}))
	defer server.Close()
	loc := testLocation(t)
	c := NewClient(server.URL, "primary", loc, server.Client())
	c.confirmWait = func(context.Context, time.Duration) error { return nil }
	start := time.Date(2026, 7, 13, 10, 0, 0, 0, loc)
	assignment := scheduling.Assignment{Card: scheduling.Card{ID: "card", Name: "Finish auth"}, Date: "2026-07-13", Start: start, End: start.Add(time.Hour)}
	eventID = scheduling.DeterministicEventID("primary", "board", "card")
	id, created, err := c.CreateEvent(context.Background(), "board", assignment, "[Trello] ", "description", "11")
	if err != nil || !created || id != eventID || postCalls.Load() != 1 {
		t.Fatalf("create result id=%q created=%v err=%v", id, created, err)
	}
	props := received.ExtendedProperties.Private
	if props["trelloCardId"] != "card" || props["trelloBoardId"] != "board" || props["source"] != scheduling.Source {
		t.Fatalf("extended properties=%#v", props)
	}
	if received.Summary != "[Trello] Finish auth" {
		t.Fatalf("summary=%q", received.Summary)
	}
	if received.ColorID != "11" {
		t.Fatalf("color=%q", received.ColorID)
	}
}

func TestCreateEventConfirmationAndReconciliation(t *testing.T) {
	tests := []struct {
		name       string
		postStatus int
		getStatus  []int
		metadata   map[string]string
		wantCreate bool
		wantError  bool
	}{
		{name: "delayed visibility", postStatus: http.StatusOK, getStatus: []int{http.StatusNotFound, http.StatusInternalServerError, http.StatusOK}, wantCreate: true},
		{name: "confirmation exhaustion", postStatus: http.StatusOK, getStatus: []int{http.StatusNotFound, http.StatusNotFound, http.StatusNotFound, http.StatusNotFound}, wantError: true},
		{name: "strict card metadata", postStatus: http.StatusOK, getStatus: []int{http.StatusOK}, metadata: map[string]string{"trelloCardId": "other", "trelloBoardId": "board"}, wantError: true},
		{name: "conflict reconciliation", postStatus: http.StatusConflict, getStatus: []int{http.StatusOK}},
		{name: "ambiguous server reconciliation", postStatus: http.StatusServiceUnavailable, getStatus: []int{http.StatusNotFound, http.StatusOK}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var posts atomic.Int32
			var gets atomic.Int32
			id := scheduling.DeterministicEventID("primary", "board", "card")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPost {
					posts.Add(1)
					w.WriteHeader(test.postStatus)
					if test.postStatus >= 200 && test.postStatus < 300 {
						w.Write([]byte(`{}`))
					}
					return
				}
				index := int(gets.Add(1)) - 1
				status := test.getStatus[index]
				w.WriteHeader(status)
				if status == http.StatusOK {
					metadata := test.metadata
					if metadata == nil {
						metadata = map[string]string{"trelloCardId": "card", "trelloBoardId": "board"}
					}
					json.NewEncoder(w).Encode(map[string]any{"id": id, "status": "confirmed", "extendedProperties": map[string]any{"private": metadata}})
				}
			}))
			defer server.Close()
			loc := testLocation(t)
			client := NewClient(server.URL, "primary", loc, server.Client())
			client.confirmWait = func(context.Context, time.Duration) error { return nil }
			start := time.Date(2026, 7, 13, 10, 0, 0, 0, loc)
			assignment := scheduling.Assignment{Card: scheduling.Card{ID: "card", Name: "Card"}, Start: start, End: start.Add(time.Hour)}
			gotID, created, err := client.CreateEvent(context.Background(), "board", assignment, "", "", "")
			if posts.Load() != 1 {
				t.Fatalf("POST calls=%d", posts.Load())
			}
			if test.wantError {
				if err == nil || gotID != "" || created {
					t.Fatalf("result id=%q created=%v err=%v", gotID, created, err)
				}
				return
			}
			if err != nil || gotID != id || created != test.wantCreate {
				t.Fatalf("result id=%q created=%v err=%v", gotID, created, err)
			}
		})
	}
}

func TestFindCardEventFallbackAndReschedule(t *testing.T) {
	var patched bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/events/tcc"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/events"):
			w.Write([]byte(`{"items":[{"id":"legacy-event","status":"confirmed","start":{"dateTime":"2026-07-10T10:00:00+02:00"},"end":{"dateTime":"2026-07-10T11:00:00+02:00"},"extendedProperties":{"private":{"trelloCardId":"card"}}}]}`))
		case r.Method == http.MethodPatch:
			var body map[string]apiEventTime
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode patch: %v", err)
			}
			patched = body["start"].DateTime == "2026-07-13T10:00:00+02:00" && body["end"].DateTime == "2026-07-13T11:00:00+02:00"
			w.Write([]byte(`{}`))
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()
	loc := testLocation(t)
	c := NewClient(server.URL, "primary", loc, server.Client())
	event, found, err := c.FindCardEvent(context.Background(), "board", "card")
	if err != nil || !found || event.ID != "legacy-event" {
		t.Fatalf("event=%#v found=%v err=%v", event, found, err)
	}
	start := time.Date(2026, 7, 13, 10, 0, 0, 0, loc)
	if err := c.RescheduleEvent(context.Background(), event.ID, start, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if !patched {
		t.Fatal("reschedule patch did not contain the requested time")
	}
}
