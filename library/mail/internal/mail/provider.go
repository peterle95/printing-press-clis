package mail

import (
	"context"
	"fmt"
	"strings"
)

type MailProvider interface {
	AccountName() string
	ProviderName() string

	Inbox(ctx context.Context, limit int) ([]Message, error)
	Search(ctx context.Context, query string, limit int) ([]Message, error)
	Read(ctx context.Context, id string) (*Message, error)

	Draft(ctx context.Context, draft OutboundMessage) (*DraftResult, error)
	Send(ctx context.Context, msg OutboundMessage) (*SendResult, error)
}

type ArchiveProvider interface {
	Archive(ctx context.Context, id string) error
}

type LabelProvider interface {
	Label(ctx context.Context, id string, add, remove []string) (*LabelResult, error)
}

func ProviderMessageID(provider, account, rawID string) string {
	if strings.Count(rawID, ":") >= 2 && strings.HasPrefix(rawID, provider+":") {
		return rawID
	}
	return provider + ":" + account + ":" + rawID
}

func RawMessageID(provider, account, id string) (string, error) {
	parts := strings.SplitN(id, ":", 3)
	if len(parts) == 3 {
		if parts[0] != provider {
			return "", fmt.Errorf("message id provider %q does not match selected provider %q", parts[0], provider)
		}
		if parts[1] != account {
			return "", fmt.Errorf("message id account %q does not match selected account %q", parts[1], account)
		}
		return parts[2], nil
	}
	return id, nil
}
