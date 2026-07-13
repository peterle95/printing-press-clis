package trello

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
