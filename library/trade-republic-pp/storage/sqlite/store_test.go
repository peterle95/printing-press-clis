package sqlite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestOpenAppliesMigrationsAndPrivateSettings(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(dir, "trade-republic.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if info, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("directory mode = %o, want 700", got)
	}
	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("database mode = %o, want 600", got)
	}

	var journalMode string
	if err := store.db.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
	var foreignKeys, busyTimeout int
	if err := store.db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 || busyTimeout != 5000 {
		t.Fatalf("foreign_keys=%d busy_timeout=%d", foreignKeys, busyTimeout)
	}

	migrations, err := store.Migrations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 4 {
		t.Fatalf("migration count = %d, want 4", len(migrations))
	}
	for index, migration := range migrations {
		if migration.Version != index+1 || len(migration.Checksum) != 64 || migration.AppliedAt.IsZero() {
			t.Fatalf("invalid migration record: %#v", migration)
		}
	}

	wantTables := []string{
		"cash_balances", "cash_movements", "dividends", "documents",
		"execution_approvals", "execution_audit_log", "execution_idempotency",
		"execution_previews", "fees", "instrument_aliases", "instruments",
		"positions", "price_history", "research_reports", "schema_migrations",
		"sync_runs", "taxes", "transactions",
	}
	rows, err := store.db.QueryContext(ctx, `SELECT name FROM sqlite_schema
		WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	var gotTables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		gotTables = append(gotTables, name)
	}
	rows.Close()
	if fmt.Sprint(gotTables) != fmt.Sprint(wantTables) {
		t.Fatalf("tables = %v, want %v", gotTables, wantTables)
	}
	for _, table := range gotTables {
		columnRows, err := store.db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info("%s")`, table))
		if err != nil {
			t.Fatal(err)
		}
		for columnRows.Next() {
			var cid, notNull, pk int
			var name, columnType string
			var defaultValue any
			if err := columnRows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
				t.Fatal(err)
			}
			if columnType == "REAL" {
				t.Fatalf("%s.%s uses forbidden REAL storage", table, name)
			}
		}
		columnRows.Close()
	}
}

func TestMigrationsAreIdempotentAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "data", "trade-republic.db")
	first, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	firstRecords, err := first.Migrations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	secondRecords, err := second.Migrations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstRecords) != len(secondRecords) {
		t.Fatalf("migration records changed: %d -> %d", len(firstRecords), len(secondRecords))
	}
	for index := range firstRecords {
		if firstRecords[index].Version != secondRecords[index].Version ||
			firstRecords[index].Name != secondRecords[index].Name ||
			firstRecords[index].Checksum != secondRecords[index].Checksum {
			t.Fatalf("migration changed on reopen: %#v != %#v", firstRecords[index], secondRecords[index])
		}
	}
}

func testStore(t *testing.T, now time.Time) *Store {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "private", "data.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	store.now = func() time.Time { return now }
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}
