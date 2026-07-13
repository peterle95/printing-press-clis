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
