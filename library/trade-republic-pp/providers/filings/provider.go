// Package filings defines a read-only filings/news provider contract.
package filings

import (
	"context"

	"trade-republic-pp-cli/internal/research"
)

type Provider interface {
	RecentFilings(context.Context, string) ([]research.Filing, error)
	News(context.Context, string) ([]research.NewsItem, error)
}
