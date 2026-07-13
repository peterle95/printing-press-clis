// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCardGetDisplaysCard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/cards/test-card-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "test-card-123", "name": "Test Card", "url": "https://trello.com/c/test",
			"desc": "A test card", "pos": 1.0, "closed": false,
		})
	}))
	defer server.Close()

	t.Setenv("TRELLO_API_KEY", "key")
	t.Setenv("TRELLO_TOKEN", "token")
	t.Setenv("TRELLO_CALENDAR_BASE_URL", server.URL)

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"card", "get", "test-card-123", "--agent", "--config", "/dev/null"})

	if err := root.Execute(); err != nil {
		t.Fatalf("card get failed: %v stderr=%s", err, stderr.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout=%s", err, stdout.String())
	}
	if result["name"] != "Test Card" {
		t.Fatalf("expected card name 'Test Card', got %v", result["name"])
	}
	if result["id"] != "test-card-123" {
		t.Fatalf("expected card id 'test-card-123', got %v", result["id"])
	}
}

func TestCardGetNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()

	t.Setenv("TRELLO_API_KEY", "key")
	t.Setenv("TRELLO_TOKEN", "token")
	t.Setenv("TRELLO_CALENDAR_BASE_URL", server.URL)

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"card", "get", "nonexistent", "--agent", "--config", "/dev/null"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for card not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got: %v", err)
	}
}

func TestCardCreateDryRun(t *testing.T) {
	var serverCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusTeapot)
	}))
	defer server.Close()

	t.Setenv("TRELLO_API_KEY", "key")
	t.Setenv("TRELLO_TOKEN", "token")
	t.Setenv("TRELLO_CALENDAR_BASE_URL", server.URL)

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"card", "create", "--name", "Test", "--list-id", "list1", "--dry-run", "--agent", "--config", "/dev/null"})

	if err := root.Execute(); err != nil {
		t.Fatalf("dry-run failed: %v stderr=%s", err, stderr.String())
	}
	if serverCalled {
		t.Fatal("dry-run made an API call")
	}
	if !strings.Contains(stderr.String(), "dry-run") {
		t.Fatalf("expected dry-run message in stderr, got: %s", stderr.String())
	}
}

func TestCardCreateSendsRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/cards" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		gotName := r.URL.Query().Get("name")
		if gotName != "New Card" {
			t.Errorf("expected name 'New Card', got %q", gotName)
		}
		gotList := r.URL.Query().Get("idList")
		if gotList != "list1" {
			t.Errorf("expected idList 'list1', got %q", gotList)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "new-card-456", "name": "New Card", "url": "https://trello.com/c/new-card-456",
			"desc": "", "pos": 1.0, "closed": false,
		})
	}))
	defer server.Close()

	t.Setenv("TRELLO_API_KEY", "key")
	t.Setenv("TRELLO_TOKEN", "token")
	t.Setenv("TRELLO_CALENDAR_BASE_URL", server.URL)

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"card", "create", "--name", "New Card", "--list-id", "list1", "--yes", "--agent", "--config", "/dev/null"})

	if err := root.Execute(); err != nil {
		t.Fatalf("card create failed: %v stderr=%s", err, stderr.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout=%s", err, stdout.String())
	}
	if result["id"] != "new-card-456" {
		t.Fatalf("expected card id 'new-card-456', got %v", result["id"])
	}
	if result["name"] != "New Card" {
		t.Fatalf("expected card name 'New Card', got %v", result["name"])
	}
}

func TestCardArchiveRequiresYesInJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("API call should not be made without --yes")
	}))
	defer server.Close()

	t.Setenv("TRELLO_API_KEY", "key")
	t.Setenv("TRELLO_TOKEN", "token")
	t.Setenv("TRELLO_CALENDAR_BASE_URL", server.URL)

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	// Use --json + --no-input without --yes to trigger the requirement
	root.SetArgs([]string{"card", "archive", "test-card-id", "--json", "--no-input", "--config", "/dev/null"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error requiring --yes")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected error mentioning --yes, got: %v", err)
	}
}

func TestCardArchiveWithYes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/cards/test-card-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		closed := r.URL.Query().Get("closed")
		if closed != "true" {
			t.Errorf("expected closed=true, got %q", closed)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": "test-card-123", "closed": true})
	}))
	defer server.Close()

	t.Setenv("TRELLO_API_KEY", "key")
	t.Setenv("TRELLO_TOKEN", "token")
	t.Setenv("TRELLO_CALENDAR_BASE_URL", server.URL)

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"card", "archive", "test-card-123", "--yes", "--agent", "--config", "/dev/null"})

	if err := root.Execute(); err != nil {
		t.Fatalf("archive failed: %v stderr=%s", err, stderr.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout=%s", err, stdout.String())
	}
	if result["status"] != "archived" {
		t.Fatalf("expected status 'archived', got %v", result["status"])
	}
	if result["card_id"] != "test-card-123" {
		t.Fatalf("expected card_id 'test-card-123', got %v", result["card_id"])
	}
}

func TestCardMoveRequiresListID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("API call should not be made when --list-id is missing")
	}))
	defer server.Close()

	t.Setenv("TRELLO_API_KEY", "key")
	t.Setenv("TRELLO_TOKEN", "token")
	t.Setenv("TRELLO_CALENDAR_BASE_URL", server.URL)

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"card", "move", "test-card-id", "--yes", "--agent", "--config", "/dev/null"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error requiring --list-id")
	}
	if !strings.Contains(err.Error(), "--list-id") {
		t.Fatalf("expected error mentioning --list-id, got: %v", err)
	}
}

func TestCardMoveSendsRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/cards/test-card-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		idList := r.URL.Query().Get("idList")
		if idList != "target-list" {
			t.Errorf("expected idList 'target-list', got %q", idList)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "test-card-123", "idList": "target-list",
		})
	}))
	defer server.Close()

	t.Setenv("TRELLO_API_KEY", "key")
	t.Setenv("TRELLO_TOKEN", "token")
	t.Setenv("TRELLO_CALENDAR_BASE_URL", server.URL)

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"card", "move", "test-card-123", "--list-id", "target-list", "--yes", "--agent", "--config", "/dev/null"})

	if err := root.Execute(); err != nil {
		t.Fatalf("move failed: %v stderr=%s", err, stderr.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout=%s", err, stdout.String())
	}
	if result["status"] != "moved" {
		t.Fatalf("expected status 'moved', got %v", result["status"])
	}
	if result["card_id"] != "test-card-123" {
		t.Fatalf("expected card_id 'test-card-123', got %v", result["card_id"])
	}
	if result["target_list_id"] != "target-list" {
		t.Fatalf("expected target_list_id 'target-list', got %v", result["target_list_id"])
	}
}

func TestCardCreateRequiresYesInJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("API call should not be made without --yes in JSON mode")
	}))
	defer server.Close()

	t.Setenv("TRELLO_API_KEY", "key")
	t.Setenv("TRELLO_TOKEN", "token")
	t.Setenv("TRELLO_CALENDAR_BASE_URL", server.URL)

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"card", "create", "--name", "Test", "--list-id", "list1", "--json", "--no-input", "--config", "/dev/null"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error requiring --yes")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected error mentioning --yes, got: %v", err)
	}
}

func TestCardCreateCancelledOnNo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("API call should not be made when user cancels")
	}))
	defer server.Close()

	t.Setenv("TRELLO_API_KEY", "key")
	t.Setenv("TRELLO_TOKEN", "token")
	t.Setenv("TRELLO_CALENDAR_BASE_URL", server.URL)

	var flags rootFlags
	root := newRootCmd(&flags)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	// Simulate user typing "n"
	root.SetIn(strings.NewReader("n\n"))
	root.SetArgs([]string{"card", "create", "--name", "Test", "--list-id", "list1", "--config", "/dev/null"})

	err := root.Execute()
	if err != nil {
		t.Fatal(err)
	}
	// Should print "Create cancelled."
	output := buf.String()
	if !strings.Contains(output, "Create cancelled") {
		t.Fatalf("expected cancellation message, got: %s", output)
	}
}

func TestCardCreateWithDesc(t *testing.T) {
	expectedDesc := "A test description"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/cards" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		gotDesc := r.URL.Query().Get("desc")
		if gotDesc != expectedDesc {
			t.Errorf("expected desc %q, got %q", expectedDesc, gotDesc)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "card-with-desc", "name": "Card with Desc",
		})
	}))
	defer server.Close()

	t.Setenv("TRELLO_API_KEY", "key")
	t.Setenv("TRELLO_TOKEN", "token")
	t.Setenv("TRELLO_CALENDAR_BASE_URL", server.URL)

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"card", "create", "--name", "Card with Desc", "--list-id", "list1", "--desc", expectedDesc, "--yes", "--agent", "--config", "/dev/null"})

	if err := root.Execute(); err != nil {
		t.Fatalf("card create with desc failed: %v stderr=%s", err, stderr.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout=%s", err, stdout.String())
	}
	if result["name"] != "Card with Desc" {
		t.Fatalf("expected name 'Card with Desc', got %v", result["name"])
	}
}

func TestCardGetDryRunOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("API call should not be made during dry-run")
	}))
	defer server.Close()

	t.Setenv("TRELLO_API_KEY", "key")
	t.Setenv("TRELLO_TOKEN", "token")
	t.Setenv("TRELLO_CALENDAR_BASE_URL", server.URL)

	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"card", "get", "test-card-123", "--dry-run", "--config", "/dev/null"})

	if err := root.Execute(); err != nil {
		t.Fatalf("dry-run failed: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "dry-run") {
		t.Fatalf("expected dry-run message in stderr, got: %s", stderr.String())
	}
}
