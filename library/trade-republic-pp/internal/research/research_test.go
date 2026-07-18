package research

import (
	"testing"
	"time"

	"trade-republic-pp-cli/internal/instruments"
	"trade-republic-pp-cli/internal/money"
	"trade-republic-pp-cli/internal/portfolio"
)

func TestEnrichPortfolioCompanyAndETFOverlap(t *testing.T) {
	t.Parallel()
	positions := []portfolio.Position{
		{ISIN: "US5949181045", Name: "Microsoft", MarketValue: money.MustParse("100"), Currency: "EUR"},
		{ISIN: "IE00B4L5Y983", Name: "World ETF", MarketValue: money.MustParse("300"), Currency: "EUR"},
	}
	metadata := map[string]instruments.Instrument{"US5949181045": {ISIN: "US5949181045", Symbol: "MSFT"}}
	report := Report{
		SchemaVersion: 1, Identifier: "WORLD", ISIN: "IE00B4L5Y983", Name: "World ETF", Kind: "etf", AsOf: time.Now(),
		ETF:       &ETFReport{FundName: "World ETF", ISIN: "IE00B4L5Y983", TopHoldings: []Holding{{Name: "Microsoft", ISIN: "US5949181045", WeightBP: 400}}},
		Citations: []Citation{{Title: "factsheet", URL: "https://example.invalid/factsheet"}},
	}
	enriched, err := EnrichPortfolio(report, positions, metadata)
	if err != nil {
		t.Fatal(err)
	}
	if len(enriched.ETF.PortfolioOverlap) != 2 {
		t.Fatalf("overlap = %#v", enriched.ETF.PortfolioOverlap)
	}
}

func TestValidateRequiresCitations(t *testing.T) {
	t.Parallel()
	report := Report{SchemaVersion: 1, Identifier: "ASML", Name: "ASML", Kind: "company", AsOf: time.Now(), Company: &CompanyReport{}}
	if err := Validate(report); err == nil {
		t.Fatal("report without citations unexpectedly validated")
	}
}
