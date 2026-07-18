package financejson

import (
	"strings"
	"testing"
	"time"
)

func TestDecodeNormalizesAndFilters(t *testing.T) {
	t.Parallel()
	raw := `{
  "schema":"trpp.finance/v1",
  "as_of":"2026-07-16T10:00:00Z",
  "instruments":[{"isin":"ie00b4l5y983","name":"Core MSCI World"}],
  "positions":[{"isin":"IE00B4L5Y983","name":"Core MSCI World","quantity":"2","average_cost":"90","price":"100","market_value":"200","currency":"EUR"}],
  "cash_balances":[{"currency":"eur","amount":"500"}],
  "transactions":[
    {"occurred_at":"2026-01-01T10:00:00Z","kind":"buy","isin":"IE00B4L5Y983","quantity":"1","amount":"-90","currency":"EUR"},
    {"occurred_at":"2026-07-01T10:00:00Z","kind":"buy","isin":"IE00B4L5Y983","quantity":"1","amount":"-95","currency":"EUR"}
  ],
  "documents":[], "research_reports":[]
}`
	bundle, err := Decode(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Instruments[0].ISIN != "IE00B4L5Y983" || bundle.CashBalances[0].Currency != "EUR" {
		t.Fatalf("normalization failed: %#v", bundle)
	}
	since := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	snapshot := bundle.Snapshot(&since)
	if len(snapshot.Transactions) != 1 || snapshot.Transactions[0].Fingerprint == "" {
		t.Fatalf("filtered snapshot = %#v", snapshot)
	}
}

func TestDecodeRejectsBadSchemaAndISIN(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		`{"schema":"v2","as_of":"2026-07-16T10:00:00Z"}`,
		`{"schema":"trpp.finance/v1","as_of":"2026-07-16T10:00:00Z","instruments":[{"isin":"BAD","name":"bad"}]}`,
	} {
		if _, err := Decode(strings.NewReader(raw)); err == nil {
			t.Fatalf("Decode(%s) unexpectedly succeeded", raw)
		}
	}
}
