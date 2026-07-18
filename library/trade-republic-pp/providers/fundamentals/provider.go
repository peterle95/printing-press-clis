// Package fundamentals defines structured company and ETF data contracts.
package fundamentals

import (
	"context"

	"trade-republic-pp-cli/internal/research"
)

type Provider interface {
	ETF(context.Context, string) (research.ETFReport, error)
	Company(context.Context, string) (research.CompanyReport, error)
}
