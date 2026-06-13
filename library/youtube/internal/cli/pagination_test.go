package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

type fakePaginatedClient struct {
	responses []string
	calls     []map[string]string
}

func (f *fakePaginatedClient) GetWithHeaders(_ string, params map[string]string, _ map[string]string) (json.RawMessage, error) {
	copied := map[string]string{}
	for k, v := range params {
		copied[k] = v
	}
	f.calls = append(f.calls, copied)
	idx := len(f.calls) - 1
	if idx >= len(f.responses) {
		idx = len(f.responses) - 1
	}
	return json.RawMessage(f.responses[idx]), nil
}

func TestPaginatedGetPreservesCanonicalCursorCase(t *testing.T) {
	client := &fakePaginatedClient{
		responses: []string{
			`{"items":[{"id":"first"}],"nextPageToken":"NEXT"}`,
			`{"items":[{"id":"second"}]}`,
		},
	}

	raw, err := paginatedGet(client, "/youtube/v3/subscriptions", map[string]string{
		"pageToken":  "",
		"maxResults": "50",
	}, nil, true, "pagetoken", "nextPageToken", "")
	if err != nil {
		t.Fatalf("paginatedGet returned error: %v", err)
	}

	var items []map[string]string
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if got := client.calls[1]["pageToken"]; got != "NEXT" {
		t.Fatalf("second request pageToken = %q, want NEXT; params=%v", got, client.calls[1])
	}
	if _, ok := client.calls[1]["pagetoken"]; ok {
		t.Fatalf("second request used lowercased pagetoken: %v", client.calls[1])
	}
}

func TestPaginatedGetStopsOnStickyCursor(t *testing.T) {
	client := &fakePaginatedClient{
		responses: []string{
			`{"items":[{"id":"first"}],"nextPageToken":"SAME"}`,
			`{"items":[{"id":"first-again"}],"nextPageToken":"SAME"}`,
		},
	}

	_, err := paginatedGet(client, "/youtube/v3/subscriptions", map[string]string{
		"pageToken":  "",
		"maxResults": "50",
	}, nil, true, "pageToken", "nextPageToken", "")
	if err == nil {
		t.Fatal("expected sticky cursor error")
	}
	if !strings.Contains(err.Error(), "pagination cursor did not advance") {
		t.Fatalf("unexpected error: %v", err)
	}
}
