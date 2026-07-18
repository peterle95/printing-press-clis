package traderepublic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPortfolioParserRejectsInvalidISINAndLocalizedDecimal(t *testing.T) {
	for name, row := range map[string]string{
		"invalid ISIN":      "Synthetic;INVALID;1;2;3;4\n",
		"localized decimal": "Synthetic;IE00B4L5Y983;1,25;2;3;4\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "portfolio.csv")
			data := "Name;ISIN;quantity;price;avgCost;netValue\n" + row
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := parsePortfolioCSV(path, "EUR", time.Now()); err == nil {
				t.Fatal("expected parser error")
			}
		})
	}
}

func TestTransactionParserRejectsOversizedJSONLLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transactions.json")
	line := `{"Date":"2026-07-16T10:00:00","Type":"DEPOSIT","Value":1,"Note":"` + strings.Repeat("x", maxTransactionLineBytes) + `"}`
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseTransactionJSONL(path, "EUR", location, nil); err == nil {
		t.Fatal("expected bounded-line parser error")
	}
}

func TestExpandDecimalExponentIsExact(t *testing.T) {
	for input, want := range map[string]string{
		"1.23456789e2":  "123.456789",
		"-1.25e-3":      "-0.00125",
		"7e3":           "7000",
		"0.00000001e-1": "0.000000001",
	} {
		got, err := expandDecimalExponent(input)
		if err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if got != want {
			t.Fatalf("expand(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestConservativeLastDays(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	since := now.Add(-28 * time.Hour)
	if got := conservativeLastDays(now, &since); got != 4 {
		t.Fatalf("last days = %d, want 4", got)
	}
	if got := conservativeLastDays(now, nil); got != 0 {
		t.Fatalf("unbounded last days = %d, want 0", got)
	}
}
