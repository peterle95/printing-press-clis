// Package marketdata defines replaceable quote/search provider contracts.
package marketdata

import (
	"context"
	"time"

	"trade-republic-pp-cli/internal/instruments"
	"trade-republic-pp-cli/internal/money"
)

type Quote struct {
	ISIN, Currency, Venue string
	Price                 money.Decimal
	AsOf                  time.Time
	SourceURL             string
}

type Provider interface {
	Search(context.Context, string) ([]instruments.Instrument, error)
	Quote(context.Context, string) (Quote, error)
}
