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
	if err != nil || !exists {
		t.Fatalf("cancelled deterministic event should be duplicate: exists=%v err=%v", exists, err)
	}
}

func TestCreateEventExtendedProperties(t *testing.T) {
	var received apiEvent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"created"}`))
	}))
	defer server.Close()
	loc := testLocation(t)
	c := NewClient(server.URL, "primary", loc, server.Client())
	start := time.Date(2026, 7, 13, 10, 0, 0, 0, loc)
	assignment := scheduling.Assignment{Card: scheduling.Card{ID: "card", Name: "Finish auth"}, Date: "2026-07-13", Start: start, End: start.Add(time.Hour)}
	id, created, err := c.CreateEvent(context.Background(), "board", assignment, "[Trello] ", "description", "11")
	if err != nil || !created || id != "created" {
		t.Fatalf("create result id=%q created=%v err=%v", id, created, err)
	}
	props := received.ExtendedProperties.Private
	if props["trelloCardId"] != "card" || props["trelloBoardId"] != "board" || props["source"] != scheduling.Source {
		t.Fatalf("extended properties=%#v", props)
	}
	if received.Summary != "[Trello] Finish auth" {
		t.Fatalf("summary=%q", received.Summary)
	}
	if received.ColorID != "11" { t.Fatalf("color=%q", received.ColorID) }
}
