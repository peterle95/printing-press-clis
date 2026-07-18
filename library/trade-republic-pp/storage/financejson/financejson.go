// Package financejson implements the versioned, offline interchange format.
// SQLite remains the source of truth; FinanceJSON is only an import/export and
// deterministic fixture format.
package financejson

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"trade-republic-pp-cli/internal/instruments"
	"trade-republic-pp-cli/internal/portfolio"
	"trade-republic-pp-cli/internal/research"
	"trade-republic-pp-cli/internal/transactions"
	"trade-republic-pp-cli/providers/traderepublic"
)

const (
	Schema      = "trpp.finance/v1"
	maxFileSize = 50 << 20
)

type Bundle struct {
	Schema       string                     `json:"schema"`
	GeneratedAt  time.Time                  `json:"generated_at"`
	AsOf         time.Time                  `json:"as_of"`
	Provider     string                     `json:"provider,omitempty"`
	Instruments  []instruments.Instrument   `json:"instruments"`
	Positions    []portfolio.Position       `json:"positions"`
	CashBalances []portfolio.CashBalance    `json:"cash_balances"`
	Transactions []transactions.Transaction `json:"transactions"`
	Documents    []transactions.Document    `json:"documents"`
	Research     []research.Report          `json:"research_reports"`
	Warnings     []string                   `json:"warnings,omitempty"`
}

func Load(path string) (Bundle, error) {
	file, err := os.Open(path)
	if err != nil {
		return Bundle{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Bundle{}, err
	}
	if !info.Mode().IsRegular() {
		return Bundle{}, fmt.Errorf("FinanceJSON input must be a regular file")
	}
	if info.Size() > maxFileSize {
		return Bundle{}, fmt.Errorf("FinanceJSON input exceeds %d bytes", maxFileSize)
	}
	return Decode(io.LimitReader(file, maxFileSize+1))
}

func Decode(reader io.Reader) (Bundle, error) {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	var bundle Bundle
	if err := decoder.Decode(&bundle); err != nil {
		return Bundle{}, fmt.Errorf("decode FinanceJSON: %w", err)
	}
	if err := ensureEOF(decoder); err != nil {
		return Bundle{}, err
	}
	if err := bundle.NormalizeAndValidate(); err != nil {
		return Bundle{}, err
	}
	return bundle, nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode trailing FinanceJSON: %w", err)
	}
	return fmt.Errorf("FinanceJSON contains more than one document")
}

func (b *Bundle) NormalizeAndValidate() error {
	if b.Schema != Schema {
		return fmt.Errorf("unsupported FinanceJSON schema %q (want %q)", b.Schema, Schema)
	}
	if b.AsOf.IsZero() {
		return fmt.Errorf("FinanceJSON as_of is required")
	}
	if b.GeneratedAt.IsZero() {
		b.GeneratedAt = b.AsOf
	}
	if b.Provider == "" {
		b.Provider = "financejson"
	}
	known := make(map[string]int, len(b.Instruments))
	for index := range b.Instruments {
		item := &b.Instruments[index]
		item.ISIN = instruments.NormalizeISIN(item.ISIN)
		if err := instruments.ValidateISIN(item.ISIN); err != nil {
			return fmt.Errorf("instrument %d: %w", index, err)
		}
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("instrument %s has no name", item.ISIN)
		}
		if item.UpdatedAt.IsZero() {
			item.UpdatedAt = b.AsOf
		}
		if previous, duplicate := known[item.ISIN]; duplicate {
			return fmt.Errorf("instruments %d and %d duplicate ISIN %s", previous, index, item.ISIN)
		}
		known[item.ISIN] = index
	}
	for index := range b.Positions {
		position := &b.Positions[index]
		position.ISIN = instruments.NormalizeISIN(position.ISIN)
		if err := instruments.ValidateISIN(position.ISIN); err != nil {
			return fmt.Errorf("position %d: %w", index, err)
		}
		if position.Quantity < 0 || position.Price < 0 || position.MarketValue < 0 {
			return fmt.Errorf("position %s contains a negative quantity, price, or market value", position.ISIN)
		}
		if position.AsOf.IsZero() {
			position.AsOf = b.AsOf
		}
		if position.Source == "" {
			position.Source = b.Provider
		}
		if _, exists := known[position.ISIN]; !exists {
			if strings.TrimSpace(position.Name) == "" {
				return fmt.Errorf("position %s has no matching instrument or name", position.ISIN)
			}
			b.Instruments = append(b.Instruments, instruments.Instrument{ISIN: position.ISIN, Name: position.Name, TradingCurrency: position.Currency, UpdatedAt: position.AsOf})
			known[position.ISIN] = len(b.Instruments) - 1
		}
	}
	for index := range b.CashBalances {
		balance := &b.CashBalances[index]
		balance.Currency = strings.ToUpper(strings.TrimSpace(balance.Currency))
		if len(balance.Currency) != 3 {
			return fmt.Errorf("cash balance %d has invalid currency %q", index, balance.Currency)
		}
		if balance.AsOf.IsZero() {
			balance.AsOf = b.AsOf
		}
		if balance.Source == "" {
			balance.Source = b.Provider
		}
	}
	for index := range b.Transactions {
		activity := &b.Transactions[index]
		if activity.OccurredAt.IsZero() {
			return fmt.Errorf("transaction %d has no occurred_at", index)
		}
		if activity.ISIN != "" {
			activity.ISIN = instruments.NormalizeISIN(activity.ISIN)
			if err := instruments.ValidateISIN(activity.ISIN); err != nil {
				return fmt.Errorf("transaction %d: %w", index, err)
			}
		}
		activity.Currency = strings.ToUpper(strings.TrimSpace(activity.Currency))
		if len(activity.Currency) != 3 {
			return fmt.Errorf("transaction %d has invalid currency %q", index, activity.Currency)
		}
		if activity.Kind == "" {
			activity.Kind = transactions.Unknown
		}
		if activity.Source == "" {
			activity.Source = b.Provider
		}
		transactions.EnsureIdentity(activity)
	}
	for index := range b.Research {
		report := &b.Research[index]
		if report.SchemaVersion == 0 {
			report.SchemaVersion = 1
		}
		if report.AsOf.IsZero() {
			return fmt.Errorf("research report %d has no as_of", index)
		}
		if report.ID == "" {
			raw, _ := json.Marshal(report)
			hash := sha256.Sum256(raw)
			report.ID = hex.EncodeToString(hash[:])
		}
		if err := research.Validate(*report); err != nil {
			return fmt.Errorf("research report %d: %w", index, err)
		}
	}
	return nil
}

func (b Bundle) Snapshot(since *time.Time) traderepublic.Snapshot {
	activities := b.Transactions
	if since != nil {
		activities = make([]transactions.Transaction, 0, len(b.Transactions))
		for _, activity := range b.Transactions {
			if !activity.OccurredAt.Before(*since) {
				activities = append(activities, activity)
			}
		}
	}
	return traderepublic.Snapshot{
		Provider:       b.Provider,
		Adapter:        "financejson",
		AdapterVersion: "v1",
		AsOf:           b.AsOf,
		Instruments:    b.Instruments,
		Positions:      b.Positions,
		CashBalances:   b.CashBalances,
		Transactions:   activities,
		Documents:      b.Documents,
		Warnings:       b.Warnings,
	}
}

func Encode(writer io.Writer, bundle Bundle) error {
	if err := bundle.NormalizeAndValidate(); err != nil {
		return err
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(bundle)
}
