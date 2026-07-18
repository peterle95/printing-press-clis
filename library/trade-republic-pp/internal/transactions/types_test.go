package transactions

import (
	"testing"
	"time"

	"trade-republic-pp-cli/internal/money"
)

func TestNormalizeKind(t *testing.T) {
	t.Parallel()
	tests := map[string]Kind{
		"Buy":            Buy,
		"Verkauf":        Sell,
		"Dividend":       Dividend,
		"TRANSFER-IN":    TransferIn,
		"not classified": Unknown,
	}
	for input, want := range tests {
		if got := NormalizeKind(input); got != want {
			t.Errorf("NormalizeKind(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFingerprintIgnoresProviderButNotEconomics(t *testing.T) {
	t.Parallel()
	base := Transaction{
		OccurredAt: time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC),
		Kind:       Buy,
		ISIN:       "IE00B4L5Y983",
		Quantity:   money.MustParse("1.25"),
		Amount:     money.MustParse("-100"),
		Currency:   "EUR",
		Source:     "pytr",
	}
	otherSource := base
	otherSource.Source = "statement"
	if Fingerprint(base) != Fingerprint(otherSource) {
		t.Fatal("equivalent provider and statement records did not deduplicate")
	}
	changed := base
	changed.Amount = money.MustParse("-101")
	if Fingerprint(base) == Fingerprint(changed) {
		t.Fatal("economically different records had the same fingerprint")
	}
}
