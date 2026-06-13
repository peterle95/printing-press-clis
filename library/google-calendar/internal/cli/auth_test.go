// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestCodeFromRedirectURL(t *testing.T) {
	code, err := codeFromRedirectURL("http://localhost:8085/callback?state=abc&code=secret-code", "abc")
	if err != nil {
		t.Fatalf("codeFromRedirectURL returned error: %v", err)
	}
	if code != "secret-code" {
		t.Fatalf("code = %q, want %q", code, "secret-code")
	}
}

func TestCodeFromRedirectURLRejectsWrongState(t *testing.T) {
	if _, err := codeFromRedirectURL("http://localhost:8085/callback?state=wrong&code=secret-code", "abc"); err == nil {
		t.Fatal("codeFromRedirectURL accepted a mismatched state")
	}
}

func TestResolveTokenExpiry(t *testing.T) {
	expiry, err := resolveTokenExpiry("2026-05-17T12:00:00Z", 0)
	if err != nil {
		t.Fatalf("resolveTokenExpiry returned error: %v", err)
	}
	if got := expiry.Format(time.RFC3339); got != "2026-05-17T12:00:00Z" {
		t.Fatalf("expiry = %s", got)
	}
}

func TestLoadOAuthDesktopCredentials(t *testing.T) {
	path := t.TempDir() + "/credentials.json"
	data := `{"installed":{"client_id":"id","client_secret":"secret","auth_uri":"https://auth.example","token_uri":"https://token.example"}}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	creds, err := loadOAuthDesktopCredentials(path)
	if err != nil {
		t.Fatalf("loadOAuthDesktopCredentials returned error: %v", err)
	}
	got := strings.Join([]string{creds.ClientID, creds.ClientSecret, creds.AuthorizationURL, creds.TokenURL}, "|")
	if got != "id|secret|https://auth.example|https://token.example" {
		t.Fatalf("credentials = %s", got)
	}
}
