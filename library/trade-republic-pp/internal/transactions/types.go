// Package transactions owns normalized broker and statement activity.
package transactions

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"trade-republic-pp-cli/internal/money"
)

type Kind string

const (
	Buy         Kind = "buy"
	Sell        Kind = "sell"
	Dividend    Kind = "dividend"
	Deposit     Kind = "deposit"
	Withdrawal  Kind = "withdrawal"
	Interest    Kind = "interest"
	Fee         Kind = "fee"
	Tax         Kind = "tax"
	TransferIn  Kind = "transfer_in"
	TransferOut Kind = "transfer_out"
	Unknown     Kind = "unknown"
)

type Transaction struct {
	ID                 string        `json:"id"`
	Fingerprint        string        `json:"fingerprint"`
	OccurredAt         time.Time     `json:"occurred_at"`
	OriginalTimestamp  string        `json:"original_timestamp,omitempty"`
	TimezoneAssumption string        `json:"timezone_assumption,omitempty"`
	Kind               Kind          `json:"kind"`
	ISIN               string        `json:"isin,omitempty"`
	Quantity           money.Decimal `json:"quantity"`
	Amount             money.Decimal `json:"amount"`
	Fees               money.Decimal `json:"fees"`
	Taxes              money.Decimal `json:"taxes"`
	Currency           string        `json:"currency"`
	Note               string        `json:"note,omitempty"`
	Source             string        `json:"source"`
	SourceRef          string        `json:"source_ref,omitempty"`
	RawJSON            string        `json:"-"`
}

type CashMovement struct {
	ID, TransactionID string
	OccurredAt        time.Time
	Kind              Kind
	Amount            money.Decimal
	Currency          string
}

type DividendRecord struct {
	ID, TransactionID, ISIN string
	OccurredAt              time.Time
	Gross, Net, Taxes       money.Decimal
	Currency                string
}

type Charge struct {
	ID, TransactionID, ISIN string
	OccurredAt              time.Time
	Amount                  money.Decimal
	Currency                string
}

type Document struct {
	ID            string    `json:"id"`
	SHA256        string    `json:"sha256"`
	Path          string    `json:"path"`
	Filename      string    `json:"filename"`
	DocumentType  string    `json:"document_type"`
	OccurredAt    time.Time `json:"occurred_at,omitempty"`
	ISIN          string    `json:"isin,omitempty"`
	Source        string    `json:"source"`
	ImportedAt    time.Time `json:"imported_at"`
	ParserVersion string    `json:"parser_version,omitempty"`
}

func NormalizeKind(value string) Kind {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("-", "_", " ", "_").Replace(value)
	switch value {
	case "buy", "purchase", "kauf":
		return Buy
	case "sell", "sale", "verkauf":
		return Sell
	case "dividend", "dividende", "distribution":
		return Dividend
	case "deposit", "cash_deposit", "einzahlung":
		return Deposit
	case "withdrawal", "cash_withdrawal", "auszahlung":
		return Withdrawal
	case "interest", "zinsen":
		return Interest
	case "fee", "fees", "gebühr", "gebuehr":
		return Fee
	case "tax", "taxes", "steuer", "steuern":
		return Tax
	case "transfer_in":
		return TransferIn
	case "transfer_out":
		return TransferOut
	default:
		return Unknown
	}
}

func Fingerprint(t Transaction) string {
	parts := []string{
		t.OccurredAt.UTC().Format(time.RFC3339Nano), string(t.Kind),
		strings.ToUpper(t.ISIN), t.Quantity.String(), t.Amount.String(),
		t.Fees.Abs().String(), t.Taxes.Abs().String(), strings.ToUpper(t.Currency),
		strings.TrimSpace(t.Note),
	}
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(hash[:])
}

func EnsureIdentity(t *Transaction) {
	if t.Fingerprint == "" {
		t.Fingerprint = Fingerprint(*t)
	}
	if t.ID == "" {
		t.ID = t.Fingerprint
	}
}
