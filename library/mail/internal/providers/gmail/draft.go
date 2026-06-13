package gmail

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	ppmail "mail-pp-cli/internal/mail"
)

func (p *Provider) Draft(ctx context.Context, draft ppmail.OutboundMessage) (*ppmail.DraftResult, error) {
	raw, err := p.rawMessage(draft)
	if err != nil {
		return nil, err
	}
	message := map[string]string{"raw": raw}
	if strings.TrimSpace(draft.ReplyThreadID) != "" {
		message["threadId"] = strings.TrimSpace(draft.ReplyThreadID)
	}
	body := map[string]any{"message": message}
	var result struct {
		ID      string `json:"id"`
		Message struct {
			ID       string `json:"id"`
			ThreadID string `json:"threadId"`
		} `json:"message"`
	}
	if err := p.withScopes(ScopeCompose).do(ctx, http.MethodPost, "/users/me/drafts", nil, body, &result); err != nil {
		return nil, err
	}
	return &ppmail.DraftResult{ID: result.ID, MessageID: result.Message.ID, ThreadID: result.Message.ThreadID, Account: p.account.Name, Provider: "gmail"}, nil
}

func (p *Provider) rawMessage(msg ppmail.OutboundMessage) (string, error) {
	if len(msg.To) == 0 && len(msg.Cc) == 0 && len(msg.Bcc) == 0 {
		return "", fmt.Errorf("at least one recipient is required")
	}
	from := msg.From
	if strings.TrimSpace(from) == "" {
		from = p.account.Address
	}
	if _, err := mail.ParseAddress(from); err != nil {
		from = fmt.Sprintf("<%s>", from)
	}
	var b bytes.Buffer
	writeHeader(&b, "From", from)
	writeHeader(&b, "To", strings.Join(msg.To, ", "))
	writeHeader(&b, "Cc", strings.Join(msg.Cc, ", "))
	writeHeader(&b, "Bcc", strings.Join(msg.Bcc, ", "))
	writeHeader(&b, "Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	writeHeader(&b, "Date", time.Now().Format(time.RFC1123Z))
	if replyMessageID := replyHeaderMessageID(msg); replyMessageID != "" {
		writeHeader(&b, "In-Reply-To", replyMessageID)
		writeHeader(&b, "References", strings.Join(replyReferences(msg, replyMessageID), " "))
	}
	writeHeader(&b, "MIME-Version", "1.0")
	writeHeader(&b, "Content-Type", `text/plain; charset="UTF-8"`)
	writeHeader(&b, "Content-Transfer-Encoding", "8bit")
	b.WriteString("\r\n")
	body := strings.ReplaceAll(msg.Body, "\n", "\r\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\r\n") {
		b.WriteString("\r\n")
	}
	return base64.RawURLEncoding.EncodeToString(b.Bytes()), nil
}

func replyHeaderMessageID(msg ppmail.OutboundMessage) string {
	if strings.TrimSpace(msg.ReplyMessageID) != "" {
		return strings.TrimSpace(msg.ReplyMessageID)
	}
	if looksLikeRFCMessageID(msg.ReplyTo) {
		return strings.TrimSpace(msg.ReplyTo)
	}
	return ""
}

func replyReferences(msg ppmail.OutboundMessage, replyMessageID string) []string {
	seen := map[string]bool{}
	var out []string
	for _, ref := range msg.References {
		ref = strings.TrimSpace(ref)
		if ref != "" && !seen[ref] {
			seen[ref] = true
			out = append(out, ref)
		}
	}
	if replyMessageID != "" && !seen[replyMessageID] {
		out = append(out, replyMessageID)
	}
	return out
}

func looksLikeRFCMessageID(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">") && strings.Contains(value, "@")
}

func writeHeader(b *bytes.Buffer, name, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	fmt.Fprintf(b, "%s: %s\r\n", name, value)
}

func (p *Provider) Archive(ctx context.Context, id string) error {
	rawID, err := p.rawID(id)
	if err != nil {
		return err
	}
	body := map[string]any{"removeLabelIds": []string{"INBOX"}}
	return p.withScopes(ScopeModify).do(ctx, http.MethodPost, "/users/me/messages/"+url.PathEscape(rawID)+"/modify", nil, body, nil)
}

func (p *Provider) Label(ctx context.Context, id string, add, remove []string) (*ppmail.LabelResult, error) {
	rawID, err := p.rawID(id)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"addLabelIds": add, "removeLabelIds": remove}
	if err := p.withScopes(ScopeModify).do(ctx, http.MethodPost, "/users/me/messages/"+url.PathEscape(rawID)+"/modify", nil, body, nil); err != nil {
		return nil, err
	}
	return &ppmail.LabelResult{ID: ppmail.ProviderMessageID("gmail", p.account.Name, rawID), Added: add, Removed: remove}, nil
}
