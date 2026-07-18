package sqlite

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"trade-republic-pp-cli/internal/instruments"
	"trade-republic-pp-cli/internal/money"
	"trade-republic-pp-cli/internal/portfolio"
	"trade-republic-pp-cli/internal/transactions"
	"trade-republic-pp-cli/providers/traderepublic"
)

const testISIN = "IE00B4L5Y983"

func TestApplySyncIsIdempotentAndDeduplicates(t *testing.T) {
	ctx := context.Background()
	now := testNow()
	store := testStore(t, now)
	snapshot := testSnapshot(now)
	options := ApplySyncOptions{IdempotencyKey: "sync-fixture", Metadata: map[string]string{"caller": "test"}}

	first, err := store.ApplySync(ctx, snapshot, options)
	if err != nil {
		t.Fatalf("first ApplySync() error = %v", err)
	}
	if first.Status != "success" || first.Duplicate {
		t.Fatalf("first result = %#v", first)
	}
	second, err := store.ApplySync(ctx, snapshot, options)
	if err != nil {
		t.Fatalf("second ApplySync() error = %v", err)
	}
	if !second.Duplicate || second.RunID != first.RunID {
		t.Fatalf("second result = %#v, first = %#v", second, first)
	}

	wantCounts := map[string]int{
		"sync_runs": 1, "instruments": 1, "instrument_aliases": 2,
		"positions": 1, "cash_balances": 1, "transactions": 2,
		"cash_movements": 2, "dividends": 1, "fees": 1, "taxes": 1,
		"documents": 1,
	}
	for table, want := range wantCounts {
		var got int
		if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != want {
			t.Fatalf("%s count = %d, want %d", table, got, want)
		}
	}
	var quantity int64
	if err := store.db.QueryRowContext(ctx, `SELECT quantity_i FROM positions WHERE isin=?`, testISIN).Scan(&quantity); err != nil {
		t.Fatal(err)
	}
	if quantity != int64(money.MustParse("2")) {
		t.Fatalf("quantity_i = %d, want %d", quantity, money.MustParse("2"))
	}

	changed := snapshot
	changed.Positions = append([]portfolio.Position(nil), snapshot.Positions...)
	changed.Positions[0].Quantity = money.MustParse("3")
	if _, err := store.ApplySync(ctx, changed, options); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed request error = %v, want idempotency conflict", err)
	}
}

func TestApplySyncRollsBackDataAndRetainsFailedRun(t *testing.T) {
	ctx := context.Background()
	now := testNow()
	store := testStore(t, now)
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER force_position_failure
		BEFORE INSERT ON positions BEGIN SELECT RAISE(ABORT, 'forced position failure'); END`); err != nil {
		t.Fatal(err)
	}
	options := ApplySyncOptions{IdempotencyKey: "rollback-fixture"}
	if _, err := store.ApplySync(ctx, testSnapshot(now), options); err == nil || !strings.Contains(err.Error(), "forced position failure") {
		t.Fatalf("ApplySync() error = %v, want forced failure", err)
	}
	for _, table := range []string{"instruments", "positions", "transactions", "documents"} {
		var count int
		if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s count after rollback = %d", table, count)
		}
	}
	var status, errorText string
	if err := store.db.QueryRowContext(ctx, `SELECT status, error_text FROM sync_runs`).Scan(&status, &errorText); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || !strings.Contains(errorText, "forced position failure") {
		t.Fatalf("failed run status=%q error=%q", status, errorText)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TRIGGER force_position_failure`); err != nil {
		t.Fatal(err)
	}
	result, err := store.ApplySync(ctx, testSnapshot(now), options)
	if err != nil {
		t.Fatalf("retry ApplySync() error = %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("retry result = %#v", result)
	}
	var runCount int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_runs`).Scan(&runCount); err != nil {
		t.Fatal(err)
	}
	if runCount != 1 {
		t.Fatalf("sync run count after retry = %d, want 1", runCount)
	}
}

func TestApplySyncDryRunDoesNotWrite(t *testing.T) {
	ctx := context.Background()
	now := testNow()
	store := testStore(t, now)
	result, err := store.ApplySync(ctx, testSnapshot(now), ApplySyncOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "dry_run" || !result.DryRun {
		t.Fatalf("dry-run result = %#v", result)
	}
	for _, table := range []string{"sync_runs", "instruments", "positions", "transactions", "documents"} {
		var count int
		if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s count after dry run = %d", table, count)
		}
	}
}

func testSnapshot(now time.Time) traderepublic.Snapshot {
	instrument := instruments.Instrument{
		ISIN: testISIN, Name: "iShares Core MSCI World", Kind: "etf", Symbol: "EUNL",
		Country: "IE", Sector: "diversified", Domicile: "IE", BaseCurrency: "USD",
		TradingCurrency: "EUR", UpdatedAt: now,
	}
	buy := transactions.Transaction{
		ID: "buy-1", OccurredAt: now.Add(-48 * time.Hour), Kind: transactions.Buy,
		ISIN: testISIN, Quantity: money.MustParse("2"), Amount: money.MustParse("-181"),
		Fees: money.MustParse("1"), Currency: "EUR", Source: "pytr", SourceRef: "timeline:buy-1",
	}
	dividend := transactions.Transaction{
		ID: "dividend-1", OccurredAt: now.Add(-24 * time.Hour), Kind: transactions.Dividend,
		ISIN: testISIN, Amount: money.MustParse("10"), Taxes: money.MustParse("1"),
		Currency: "EUR", Source: "pytr", SourceRef: "timeline:dividend-1",
	}
	document := transactions.Document{
		ID: "document-1", SHA256: strings.Repeat("a", 64), Path: "/private/document.pdf",
		Filename: "document.pdf", DocumentType: "statement", OccurredAt: now.Add(-24 * time.Hour),
		ISIN: testISIN, Source: "pytr", ImportedAt: now, ParserVersion: "fixture-v1",
	}
	return traderepublic.Snapshot{
		Provider: "trade-republic", Adapter: "pytr", AdapterVersion: "fixture", AsOf: now,
		Instruments: []instruments.Instrument{instrument, instrument},
		Positions: []portfolio.Position{{
			ISIN: testISIN, Name: instrument.Name, Quantity: money.MustParse("2"),
			AverageCost: money.MustParse("90"), Price: money.MustParse("100"),
			MarketValue: money.MustParse("200"), Currency: "EUR", AsOf: now, Source: "pytr",
		}},
		CashBalances: []portfolio.CashBalance{{Currency: "EUR", Amount: money.MustParse("1000"), AsOf: now, Source: "pytr"}},
		Transactions: []transactions.Transaction{buy, dividend, buy},
		Documents:    []transactions.Document{document, document},
		Warnings:     []string{"fixture warning"},
	}
}

func testNow() time.Time { return time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC) }
