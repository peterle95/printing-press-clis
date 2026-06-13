package gmail

import (
	"encoding/base64"
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
