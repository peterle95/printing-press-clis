package gmail

import (
	"context"
	"net/http"
	"strings"

	ppmail "mail-pp-cli/internal/mail"
)

func (p *Provider) Send(ctx context.Context, msg ppmail.OutboundMessage) (*ppmail.SendResult, error) {
	raw, err := p.rawMessage(msg)
	if err != nil {
		return nil, err
	}
	body := map[string]string{"raw": raw}
	if strings.TrimSpace(msg.ReplyThreadID) != "" {
		body["threadId"] = strings.TrimSpace(msg.ReplyThreadID)
	}
	var result struct {
		ID       string `json:"id"`
		ThreadID string `json:"threadId"`
	}
	if err := p.withScopes(ScopeSend).do(ctx, http.MethodPost, "/users/me/messages/send", nil, body, &result); err != nil {
		return nil, err
	}
	return &ppmail.SendResult{ID: ppmail.ProviderMessageID("gmail", p.account.Name, result.ID), ThreadID: result.ThreadID, Account: p.account.Name, Provider: "gmail"}, nil
}
