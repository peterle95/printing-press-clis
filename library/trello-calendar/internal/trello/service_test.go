package trello

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"trello-calendar-pp-cli/internal/client"
	"trello-calendar-pp-cli/internal/config"
	"trello-calendar-pp-cli/internal/scheduling"
)

func TestListOpenCardsPaginatesAndUsesAuthorizationHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != `OAuth oauth_consumer_key="key", oauth_token="token"` {
			t.Errorf("authorization header=%q", got)
		}
		if r.URL.Query().Get("key") != "" || r.URL.Query().Get("token") != "" {
			t.Error("credentials must not appear in query parameters")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("before") == "" {
			cards := make([]map[string]any, 1000)
			for i := range cards {
				cards[i] = map[string]any{"id": fmt.Sprintf("card-%04d", i), "name": "Card", "pos": i, "closed": false}
			}
			json.NewEncoder(w).Encode(cards)
			return
		}
		json.NewEncoder(w).Encode([]map[string]any{{"id": "last", "name": "Last", "pos": 1001, "closed": false}})
	}))
	defer server.Close()
	cfg := &config.Config{BaseURL: server.URL, TrelloAPIKey: "key", TrelloToken: "token"}
	c := client.New(cfg, 5*time.Second, 0)
	c.NoCache = true
	cards, err := New(c).ListOpenCards("list")
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1001 || cards[len(cards)-1].ID != "last" {
		t.Fatalf("pagination result len=%d last=%#v", len(cards), cards[len(cards)-1])
	}
}

func TestGetCardReturnsCard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != `OAuth oauth_consumer_key="key", oauth_token="token"` {
			t.Errorf("authorization header=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":     "card-1",
			"name":   "Test Card",
			"url":    "https://trello.com/c/card-1",
			"desc":   "A test card",
			"pos":    1.5,
			"labels": []map[string]string{{"name": "urgent", "color": "red"}},
			"closed": false,
		})
	}))
	defer server.Close()
	cfg := &config.Config{BaseURL: server.URL, TrelloAPIKey: "key", TrelloToken: "token"}
	c := client.New(cfg, 5*time.Second, 0)
	c.NoCache = true
	card, err := New(c).GetCard("card-1")
	if err != nil {
		t.Fatal(err)
	}
	if card == nil {
		t.Fatal("expected card, got nil")
	}
	if card.ID != "card-1" {
		t.Errorf("card id=%q", card.ID)
	}
	if card.Name != "Test Card" {
		t.Errorf("card name=%q", card.Name)
	}
	if card.URL != "https://trello.com/c/card-1" {
		t.Errorf("card url=%q", card.URL)
	}
	if card.Position != 1.5 {
		t.Errorf("card position=%f", card.Position)
	}
	if card.Closed {
		t.Error("card should not be closed")
	}
}

func TestGetCardReturnsNilOn404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}))
	defer server.Close()
	cfg := &config.Config{BaseURL: server.URL, TrelloAPIKey: "key", TrelloToken: "token"}
	c := client.New(cfg, 5*time.Second, 0)
	c.NoCache = true
	card, err := New(c).GetCard("card-1")
	if err != nil {
		t.Fatal(err)
	}
	if card != nil {
		t.Fatal("expected nil card for 404")
	}
}

func TestCreateCardSendsParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method=%q", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != `OAuth oauth_consumer_key="key", oauth_token="token"` {
			t.Errorf("authorization header=%q", got)
		}
		if got := r.URL.Query().Get("name"); got != "New Card" {
			t.Errorf("name param=%q", got)
		}
		if got := r.URL.Query().Get("idList"); got != "list-1" {
			t.Errorf("idList param=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":     "new-card",
			"name":   "New Card",
			"pos":    1.0,
			"closed": false,
		})
	}))
	defer server.Close()
	cfg := &config.Config{BaseURL: server.URL, TrelloAPIKey: "key", TrelloToken: "token"}
	c := client.New(cfg, 5*time.Second, 0)
	c.NoCache = true
	card, err := New(c).CreateCard("list-1", "New Card", "")
	if err != nil {
		t.Fatal(err)
	}
	if card == nil {
		t.Fatal("expected card, got nil")
	}
	if card.Name != "New Card" {
		t.Errorf("card name=%q", card.Name)
	}
}

func TestArchiveCardSendsClosedTrue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method=%q", r.Method)
		}
		if got := r.URL.Query().Get("closed"); got != "true" {
			t.Errorf("closed param=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":     "card-1",
			"name":   "Test Card",
			"closed": true,
		})
	}))
	defer server.Close()
	cfg := &config.Config{BaseURL: server.URL, TrelloAPIKey: "key", TrelloToken: "token"}
	c := client.New(cfg, 5*time.Second, 0)
	c.NoCache = true
	err := New(c).ArchiveCard("card-1")
	if err != nil {
		t.Fatal(err)
	}
}

func TestMoveCardSendsIdList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method=%q", r.Method)
		}
		if got := r.URL.Query().Get("idList"); got != "target-list" {
			t.Errorf("idList param=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":     "card-1",
			"name":   "Test Card",
			"closed": false,
		})
	}))
	defer server.Close()
	cfg := &config.Config{BaseURL: server.URL, TrelloAPIKey: "key", TrelloToken: "token"}
	c := client.New(cfg, 5*time.Second, 0)
	c.NoCache = true
	err := New(c).MoveCard("card-1", "target-list")
	if err != nil {
		t.Fatal(err)
	}
}

func TestListCardsWithFilter(t *testing.T) {
	var filtersSeen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != `OAuth oauth_consumer_key="key", oauth_token="token"` {
			t.Errorf("authorization header=%q", got)
		}
		filtersSeen = append(filtersSeen, r.URL.Query().Get("filter"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": "card-1", "name": "Open 1", "pos": 1.0, "closed": false},
			{"id": "card-2", "name": "Closed 1", "pos": 2.0, "closed": true},
			{"id": "card-3", "name": "Open 2", "pos": 3.0, "closed": false},
		})
	}))
	defer server.Close()
	cfg := &config.Config{BaseURL: server.URL, TrelloAPIKey: "key", TrelloToken: "token"}
	c := client.New(cfg, 5*time.Second, 0)
	c.NoCache = true
	svc := New(c)

	openCards, err := svc.ListCards("list", "open")
	if err != nil {
		t.Fatal(err)
	}
	if len(openCards) != 2 {
		t.Fatalf("open: expected 2 cards, got %d", len(openCards))
	}
	for _, card := range openCards {
		if card.Closed {
			t.Errorf("open: card %s should be open", card.ID)
		}
	}

	closedCards, err := svc.ListCards("list", "closed")
	if err != nil {
		t.Fatal(err)
	}
	if len(closedCards) != 1 {
		t.Fatalf("closed: expected 1 card, got %d", len(closedCards))
	}
	for _, card := range closedCards {
		if !card.Closed {
			t.Errorf("closed: card %s should be closed", card.ID)
		}
	}

	allCards, err := svc.ListCards("list", "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(allCards) != 3 {
		t.Fatalf("all: expected 3 cards, got %d", len(allCards))
	}

	expectedFilters := []string{"open", "closed", "all"}
	if len(filtersSeen) != 3 {
		t.Fatalf("expected 3 server calls, got %d", len(filtersSeen))
	}
	for i, f := range filtersSeen {
		if f != expectedFilters[i] {
			t.Errorf("call %d: filter=%q, want %q", i, f, expectedFilters[i])
		}
	}
}

func TestCreateCardRejectsEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()
	cfg := &config.Config{BaseURL: server.URL, TrelloAPIKey: "key", TrelloToken: "token"}
	c := client.New(cfg, 5*time.Second, 0)
	c.NoCache = true
	_, err := New(c).CreateCard("list-1", "Test", "")
	if err == nil {
		t.Fatal("expected error for empty response, got nil")
	}
	if !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("expected malformed response error, got: %v", err)
	}
}

func TestCustomFieldAndPluginDataExtraction(t *testing.T) {
	card := schedulingCard("card")
	applyCustomFields(&card, FieldMapping{
		Priority:   "priority-field",
		Time:       "time-field",
		Automation: "automation-field",
		Status:     "status-field",
		Options:    map[string]string{"opt-high": "High", "opt-auto": "auto-pick", "opt-ready": "ready"},
	}, []customFieldItemResponse{
		fieldItem("priority-field", "opt-high", "", ""),
		fieldItem("time-field", "", "90", ""),
		fieldItem("automation-field", "opt-auto", "", ""),
		fieldItem("status-field", "opt-ready", "", ""),
	})
	if card.Priority != "High" || card.EstimatedMinutes != 90 || card.Automation != "auto-pick" || card.Status != "ready" {
		t.Fatalf("native extraction failed: %#v", card)
	}

	pluginOnly := schedulingCard("plugin")
	applyPluginData(&pluginOnly, []pluginDataResponse{{Value: `{"fields":{"Priority":"Low","Time":"2h","Automation":"auto-pick","Status":"ready"}}`}})
	if pluginOnly.Priority != "Low" || pluginOnly.EstimatedMinutes != 120 || pluginOnly.Automation != "auto-pick" || pluginOnly.Status != "ready" {
		t.Fatalf("plugin extraction failed: %#v", pluginOnly)
	}

	applyPluginData(&card, []pluginDataResponse{{Value: `{"fields":{"Priority":"Low","Time":"30","Automation":"manual","Status":"blocked"}}`}})
	if card.Priority != "High" || card.EstimatedMinutes != 90 || card.Automation != "auto-pick" || card.Status != "ready" {
		t.Fatalf("plugin data overwrote native values: %#v", card)
	}
}

func TestDiscoverBoardMapsListsFieldsAndOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards/board/lists":
			json.NewEncoder(w).Encode([]map[string]any{{"id": "peter", "name": "Peter"}, {"id": "doing", "name": "Doing"}})
		case "/boards/board/labels":
			json.NewEncoder(w).Encode([]map[string]any{{"id": "label-auto", "name": "AUTO", "color": "green"}, {"id": "label-p1", "name": "P1 High", "color": "red"}})
		case "/boards/board/customFields":
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "f-priority", "name": "Priority", "options": []map[string]any{{"id": "o-high", "value": map[string]string{"text": "High"}}}},
				{"id": "f-time", "name": "Time"},
				{"id": "f-auto", "name": "Automation"},
				{"id": "f-status", "name": "Status"},
			})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()
	c := client.New(&config.Config{BaseURL: server.URL, TrelloAPIKey: "key", TrelloToken: "token"}, 5*time.Second, 0)
	c.NoCache = true
	discovery, err := New(c).DiscoverBoard("board")
	if err != nil {
		t.Fatal(err)
	}
	if len(discovery.Lists) != 2 || discovery.Lists[0].Name != "Peter" {
		t.Fatalf("lists=%#v", discovery.Lists)
	}
	if discovery.Fields.Priority != "f-priority" || discovery.Fields.Time != "f-time" || discovery.Fields.Automation != "f-auto" || discovery.Fields.Status != "f-status" {
		t.Fatalf("fields=%#v", discovery.Fields)
	}
	if discovery.Fields.Options["o-high"] != "High" {
		t.Fatalf("options=%#v", discovery.Fields.Options)
	}
	if discovery.Labels["AUTO"] != "label-auto" || discovery.Labels["P1 High"] != "label-p1" {
		t.Fatalf("labels=%#v", discovery.Labels)
	}
}

func schedulingCard(id string) scheduling.Card { return scheduling.Card{ID: id, Name: id} }

func fieldItem(fieldID, optionID, text, number string) customFieldItemResponse {
	var item customFieldItemResponse
	item.IDCustomField = fieldID
	item.IDValue = optionID
	item.Value.Text = text
	item.Value.Number = number
	return item
}
