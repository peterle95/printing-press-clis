package reporting

import (
	"testing"
	"time"

	"trade-republic-pp-cli/internal/money"
	"trade-republic-pp-cli/internal/transactions"
)

func TestTotalsSeparateCurrenciesAndCharges(t *testing.T) {
	t.Parallel()
	rows := []transactions.Transaction{
		{Kind: transactions.Buy, Amount: money.MustParse("-100"), Fees: money.MustParse("1"), Currency: "EUR"},
		{Kind: transactions.Dividend, Amount: money.MustParse("10"), Taxes: money.MustParse("2"), Currency: "EUR"},
		{Kind: transactions.Deposit, Amount: money.MustParse("50"), Currency: "USD"},
	}
	totals, err := Totals(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) != 2 || totals[0].Currency != "EUR" || totals[0].Fees.String() != "1" || totals[0].Taxes.String() != "2" {
		t.Fatalf("totals = %#v", totals)
	}
}

func TestMonthlyBuckets(t *testing.T) {
	t.Parallel()
	rows := []transactions.Transaction{
		{OccurredAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Kind: transactions.Deposit, Amount: money.MustParse("1"), Currency: "EUR"},
		{OccurredAt: time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC), Kind: transactions.Deposit, Amount: money.MustParse("1"), Currency: "EUR"},
	}
	buckets, err := Buckets(rows, "monthly", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 2 || buckets[1].Period != "2026-02" {
		t.Fatalf("buckets = %#v", buckets)
	}
}
