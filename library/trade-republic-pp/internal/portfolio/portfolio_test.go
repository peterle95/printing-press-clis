package portfolio

import (
	"testing"

	"trade-republic-pp-cli/internal/instruments"
	"trade-republic-pp-cli/internal/money"
)

func TestSummaryAndAllocation(t *testing.T) {
	t.Parallel()
	positions := []Position{
		{ISIN: "IE00B4L5Y983", Quantity: money.MustParse("2"), AverageCost: money.MustParse("80"), MarketValue: money.MustParse("200"), Currency: "EUR"},
		{ISIN: "US5949181045", Quantity: money.MustParse("1"), AverageCost: money.MustParse("50"), MarketValue: money.MustParse("100"), Currency: "EUR"},
	}
	summary, err := Summarize(positions)
	if err != nil {
		t.Fatal(err)
	}
	if summary.CostBasis.String() != "210" || summary.UnrealizedPnL.String() != "90" {
		t.Fatalf("summary = %#v", summary)
	}
	rows, err := Allocate(positions, map[string]instruments.Instrument{
		"IE00B4L5Y983": {ISIN: "IE00B4L5Y983", Sector: "Diversified"},
		"US5949181045": {ISIN: "US5949181045", Sector: "Technology"},
	}, "sector")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Group != "Diversified" || rows[0].PercentageBP != 6666 {
		t.Fatalf("allocations = %#v", rows)
	}
}

func TestSummaryRejectsMixedCurrencies(t *testing.T) {
	t.Parallel()
	_, err := Summarize([]Position{{Currency: "EUR"}, {Currency: "USD"}})
	if err == nil {
		t.Fatal("mixed currencies unexpectedly aggregated")
	}
}
