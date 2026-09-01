package gmail

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"

	"mail-pp-cli/internal/accounts"
	ppmail "mail-pp-cli/internal/mail"
)

func TestRawMessageUsesRFCMessageIDForReplyHeaders(t *testing.T) {
	provider := NewProvider(accounts.Account{
		Name:    "gmail-main",
		Address: "user@example.com",
	}, "", 0)
	raw, err := provider.rawMessage(ppmail.OutboundMessage{
		From:           "user@example.com",
		To:             []string{"daniela@example.com"},
		Subject:        "Re: Einladung",
		Body:           "Hallo Daniela,\n\nDanke.\n",
		ReplyTo:        "gmail:gmail-main:abc123",
		ReplyMessageID: "<original@example.com>",
		References:     []string{"<older@example.com>"},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		t.Fatal(err)
	}
	message := string(data)
	if strings.Contains(message, "In-Reply-To: gmail:gmail-main:abc123") {
		t.Fatalf("raw message used provider ID in In-Reply-To:\n%s", message)
	}
	if !strings.Contains(message, "In-Reply-To: <original@example.com>") {
		t.Fatalf("raw message missing RFC In-Reply-To:\n%s", message)
	}
	if !strings.Contains(message, "References: <older@example.com> <original@example.com>") {
		t.Fatalf("raw message missing reference chain:\n%s", message)
	}
}

func TestSendDraftUsesDraftSendEndpointAndMarksDraftDeleted(t *testing.T) {
	provider := NewProvider(accounts.Account{Name: "gmail-main", Address: "user@example.com"}, "", 0)
	provider = provider.withScopes(ScopeSend)
	provider.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/gmail/v1/users/me/drafts/send" {
			t.Fatalf("request = %s %s, want POST /gmail/v1/users/me/drafts/send", req.Method, req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != `{"id":"draft-123"}` {
			t.Fatalf("request body = %q, want {\"id\":\"draft-123\"}", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"id":"sent-123","threadId":"thread-123"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	result, err := provider.sendDraft(context.Background(), "draft-123")
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != "gmail:gmail-main:sent-123" {
		t.Fatalf("id = %q, want gmail:gmail-main:sent-123", result.ID)
	}
	if result.ThreadID != "thread-123" {
		t.Fatalf("thread id = %q, want thread-123", result.ThreadID)
	}
	if !result.DraftDeleted {
		t.Fatal("draft_deleted = false, want true")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
