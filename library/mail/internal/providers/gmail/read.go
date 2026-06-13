package gmail

import (
	"context"
	"encoding/base64"
	"html"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	ppmail "mail-pp-cli/internal/mail"
)

func (p *Provider) Read(ctx context.Context, id string) (*ppmail.Message, error) {
	rawID, err := p.rawID(id)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("format", "full")
	var apiMsg gmailMessage
	if err := p.withScopes(ScopeReadonly).do(ctx, http.MethodGet, "/users/me/messages/"+url.PathEscape(rawID), params, nil, &apiMsg); err != nil {
		return nil, err
	}
	msg := decodeMessage(p.account.Name, apiMsg, true)
	return &msg, nil
}

func decodeMessage(account string, apiMsg gmailMessage, includeBody bool) ppmail.Message {
	headers := apiMsg.Payload.Headers
	date, _ := mail.ParseDate(headerValue(headers, "Date"))
	body := ""
	if includeBody {
		body = strings.TrimSpace(extractBody(apiMsg.Payload))
	}
	return ppmail.Message{
		ID:          ppmail.ProviderMessageID("gmail", account, apiMsg.ID),
		Provider:    "gmail",
		Account:     account,
		ThreadID:    apiMsg.ThreadID,
		MessageID:   headerValue(headers, "Message-ID"),
		References:  parseReferences(headerValue(headers, "References")),
		From:        firstAddress(headerValue(headers, "From")),
		To:          parseAddressListLossy(headerValue(headers, "To")),
		Cc:          parseAddressListLossy(headerValue(headers, "Cc")),
		Bcc:         parseAddressListLossy(headerValue(headers, "Bcc")),
		Subject:     headerValue(headers, "Subject"),
		Date:        date,
		Snippet:     apiMsg.Snippet,
		Body:        body,
		Labels:      apiMsg.LabelIDs,
		Attachments: extractAttachments(apiMsg.Payload),
	}
}

func extractBody(payload gmailPayload) string {
	var plain []string
	var htmlParts []string
	walkPayload(payload, func(part gmailPayload) {
		if part.Filename != "" {
			return
		}
		if part.Body.Data == "" {
			return
		}
		decoded := decodeBodyData(part.Body.Data)
		switch strings.ToLower(part.MimeType) {
		case "text/plain":
			plain = append(plain, decoded)
		case "text/html":
			htmlParts = append(htmlParts, htmlToText(decoded))
		}
	})
	if len(plain) > 0 {
		return strings.Join(plain, "\n\n")
	}
	return strings.Join(htmlParts, "\n\n")
}

func extractAttachments(payload gmailPayload) []ppmail.Attachment {
	var out []ppmail.Attachment
	walkPayload(payload, func(part gmailPayload) {
		if part.Filename == "" {
			return
		}
		out = append(out, ppmail.Attachment{
			ID:       part.Body.AttachmentID,
			Filename: part.Filename,
			MimeType: part.MimeType,
			Size:     part.Body.Size,
		})
	})
	return out
}

func walkPayload(part gmailPayload, fn func(gmailPayload)) {
	fn(part)
	for _, child := range part.Parts {
		walkPayload(child, fn)
	}
}

func decodeBodyData(data string) string {
	b, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		b, err = base64.URLEncoding.DecodeString(data)
	}
	if err != nil {
		return ""
	}
	return string(b)
}

func firstAddress(value string) string {
	parsed, err := mail.ParseAddress(value)
	if err != nil {
		return strings.TrimSpace(value)
	}
	return parsed.String()
}

func parseAddressList(value string) ([]string, error) {
	list, err := mail.ParseAddressList(value)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list))
	for _, addr := range list {
		out = append(out, addr.String())
	}
	return out, nil
}

func parseAddressListLossy(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := parseAddressList(value)
	if err != nil {
		return splitCSV(value)
	}
	return parsed
}

func parseReferences(value string) []string {
	var out []string
	for _, part := range strings.Fields(value) {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func htmlToText(value string) string {
	value = htmlTagPattern.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	return strings.Join(strings.Fields(value), " ")
}

var htmlTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)

type gmailMessage struct {
	ID           string       `json:"id"`
	ThreadID     string       `json:"threadId"`
	LabelIDs     []string     `json:"labelIds"`
	Snippet      string       `json:"snippet"`
	InternalDate string       `json:"internalDate"`
	Payload      gmailPayload `json:"payload"`
}

type gmailPayload struct {
	PartID   string         `json:"partId"`
	MimeType string         `json:"mimeType"`
	Filename string         `json:"filename"`
	Headers  []gmailHeader  `json:"headers"`
	Body     gmailBody      `json:"body"`
	Parts    []gmailPayload `json:"parts"`
}

type gmailHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type gmailBody struct {
	AttachmentID string `json:"attachmentId"`
	Size         int64  `json:"size"`
	Data         string `json:"data"`
}

func unixMillis(value string) time.Time {
	return time.Time{}
}
