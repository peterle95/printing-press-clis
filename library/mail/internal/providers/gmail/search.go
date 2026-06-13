package gmail

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"mail-pp-cli/internal/mail"
)

func (p *Provider) Inbox(ctx context.Context, limit int) ([]mail.Message, error) {
	return p.withScopes(ScopeReadonly).list(ctx, "in:inbox", limit)
}

func (p *Provider) Search(ctx context.Context, query string, limit int) ([]mail.Message, error) {
	return p.withScopes(ScopeReadonly).list(ctx, query, limit)
}

func (p *Provider) list(ctx context.Context, query string, limit int) ([]mail.Message, error) {
	if limit <= 0 {
		limit = 10
	}
	params := url.Values{}
	params.Set("maxResults", strconv.Itoa(limit))
	if query != "" {
		params.Set("q", query)
	}
	var list gmailListResponse
	if err := p.do(ctx, http.MethodGet, "/users/me/messages", params, nil, &list); err != nil {
		return nil, err
	}
	out := make([]mail.Message, 0, len(list.Messages))
	for _, item := range list.Messages {
		msg, err := p.getMetadata(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, *msg)
	}
	return out, nil
}

func (p *Provider) getMetadata(ctx context.Context, rawID string) (*mail.Message, error) {
	params := url.Values{}
	params.Set("format", "metadata")
	for _, h := range []string{"From", "To", "Cc", "Bcc", "Subject", "Date", "Message-ID", "References"} {
		params.Add("metadataHeaders", h)
	}
	var apiMsg gmailMessage
	if err := p.do(ctx, http.MethodGet, "/users/me/messages/"+url.PathEscape(rawID), params, nil, &apiMsg); err != nil {
		return nil, err
	}
	msg := decodeMessage(p.account.Name, apiMsg, false)
	return &msg, nil
}

type gmailListResponse struct {
	Messages []struct {
		ID       string `json:"id"`
		ThreadID string `json:"threadId"`
	} `json:"messages"`
	NextPageToken string `json:"nextPageToken"`
}
