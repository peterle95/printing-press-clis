package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"trade-republic-pp-cli/internal/instruments"
	"trade-republic-pp-cli/internal/money"
	"trade-republic-pp-cli/internal/portfolio"
	"trade-republic-pp-cli/internal/research"
	"trade-republic-pp-cli/internal/transactions"
)

type TransactionFilter struct {
	ISIN  string
	Kind  transactions.Kind
	Since *time.Time
	Until *time.Time
	Limit int
}

type MonetaryTotal struct {
	Currency string        `json:"currency"`
	Amount   money.Decimal `json:"amount"`
	Count    int           `json:"count"`
}

type PricePoint struct {
	ISIN      string        `json:"isin"`
	Price     money.Decimal `json:"price"`
	Currency  string        `json:"currency"`
	Venue     string        `json:"venue,omitempty"`
	AsOf      time.Time     `json:"as_of"`
	Source    string        `json:"source"`
	SourceURL string        `json:"source_url,omitempty"`
}

func (s *Store) Portfolio(ctx context.Context) ([]portfolio.Position, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.isin,
		CASE WHEN p.name='' THEN i.name ELSE p.name END,
		p.quantity_i, p.average_cost_i, p.price_i, p.market_value_i,
		p.currency, p.as_of, p.source
		FROM positions p JOIN instruments i ON i.isin=p.isin
		ORDER BY p.market_value_i DESC, p.isin`)
	if err != nil {
		return nil, fmt.Errorf("query portfolio: %w", err)
	}
	defer rows.Close()
	var positions []portfolio.Position
	for rows.Next() {
		item, err := scanPosition(rows)
		if err != nil {
			return nil, err
		}
		positions = append(positions, item)
	}
	return positions, rows.Err()
}

func (s *Store) Position(ctx context.Context, identifier string) (portfolio.Position, error) {
	isin, err := s.ResolveISIN(ctx, identifier)
	if err != nil {
		return portfolio.Position{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT p.isin,
		CASE WHEN p.name='' THEN i.name ELSE p.name END,
		p.quantity_i, p.average_cost_i, p.price_i, p.market_value_i,
		p.currency, p.as_of, p.source
		FROM positions p JOIN instruments i ON i.isin=p.isin WHERE p.isin=?`, isin)
	item, err := scanPosition(row)
	if errors.Is(err, sql.ErrNoRows) {
		return portfolio.Position{}, fmt.Errorf("%w: position %s", ErrNotFound, identifier)
	}
	return item, err
}

type rowScanner interface{ Scan(...any) error }

func scanPosition(row rowScanner) (portfolio.Position, error) {
	var item portfolio.Position
	var quantity, averageCost, price, marketValue int64
	var asOf string
	if err := row.Scan(&item.ISIN, &item.Name, &quantity, &averageCost, &price,
		&marketValue, &item.Currency, &asOf, &item.Source); err != nil {
		return item, err
	}
	item.Quantity = money.Decimal(quantity)
	item.AverageCost = money.Decimal(averageCost)
	item.Price = money.Decimal(price)
	item.MarketValue = money.Decimal(marketValue)
	parsed, err := parseTime(asOf)
	if err != nil {
		return item, fmt.Errorf("parse position as_of: %w", err)
	}
	item.AsOf = parsed
	return item, nil
}

func (s *Store) ResolveISIN(ctx context.Context, identifier string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	candidate := instruments.NormalizeISIN(identifier)
	if instruments.ValidateISIN(candidate) == nil {
		var exists string
		if err := s.db.QueryRowContext(ctx, `SELECT isin FROM instruments WHERE isin=?`, candidate).Scan(&exists); err == nil {
			return exists, nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("resolve ISIN: %w", err)
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT isin FROM instrument_aliases WHERE alias_key=? ORDER BY isin`, normalizeAlias(identifier))
	if err != nil {
		return "", fmt.Errorf("resolve instrument alias: %w", err)
	}
	defer rows.Close()
	var matches []string
	for rows.Next() {
		var isin string
		if err := rows.Scan(&isin); err != nil {
			return "", err
		}
		matches = append(matches, isin)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("%w: instrument %q", ErrNotFound, identifier)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("instrument alias %q is ambiguous across %s", identifier, strings.Join(matches, ", "))
	}
}

func (s *Store) SearchInstruments(ctx context.Context, query string, limit int) ([]instruments.Instrument, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	pattern := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT i.isin, i.name, i.kind, i.symbol,
		i.country, i.sector, i.domicile, i.base_currency, i.trading_currency, i.updated_at
		FROM instruments i LEFT JOIN instrument_aliases a ON a.isin=i.isin
		WHERE ?='' OR lower(i.isin) LIKE ? OR lower(i.name) LIKE ? OR lower(i.symbol) LIKE ? OR lower(a.alias) LIKE ?
		ORDER BY CASE WHEN lower(i.isin)=lower(?) OR lower(i.symbol)=lower(?) THEN 0 ELSE 1 END,
			i.name COLLATE NOCASE, i.isin LIMIT ?`, strings.Trim(pattern, "%"), pattern, pattern,
		pattern, pattern, query, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search instruments: %w", err)
	}
	defer rows.Close()
	var results []instruments.Instrument
	for rows.Next() {
		var item instruments.Instrument
		var updatedAt string
		if err := rows.Scan(&item.ISIN, &item.Name, &item.Kind, &item.Symbol,
			&item.Country, &item.Sector, &item.Domicile, &item.BaseCurrency,
			&item.TradingCurrency, &updatedAt); err != nil {
			return nil, err
		}
		item.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

func (s *Store) Allocation(ctx context.Context, group string) ([]portfolio.Allocation, error) {
	positions, err := s.Portfolio(ctx)
	if err != nil {
		return nil, err
	}
	group = strings.ToLower(strings.TrimSpace(group))
	if group == "" {
		group = "instrument"
	}
	instrumentByISIN := map[string]instruments.Instrument{}
	if group != "instrument" && group != "currency" {
		if group != "country" && group != "sector" && group != "kind" {
			return nil, fmt.Errorf("unsupported allocation group %q", group)
		}
		items, err := s.SearchInstruments(ctx, "", 500)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			instrumentByISIN[item.ISIN] = item
		}
	}
	values := map[string]money.Decimal{}
	var total money.Decimal
	for _, item := range positions {
		key := item.ISIN
		switch group {
		case "currency":
			key = item.Currency
		case "country":
			key = instrumentByISIN[item.ISIN].Country
		case "sector":
			key = instrumentByISIN[item.ISIN].Sector
		case "kind":
			key = instrumentByISIN[item.ISIN].Kind
		}
		if key == "" {
			key = "unknown"
		}
		values[key] = values[key].Add(item.MarketValue)
		total = total.Add(item.MarketValue)
	}
	result := make([]portfolio.Allocation, 0, len(values))
	for key, value := range values {
		result = append(result, portfolio.Allocation{
			Group: key, MarketValue: value, PercentageBP: basisPoints(value, total),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].MarketValue == result[j].MarketValue {
			return result[i].Group < result[j].Group
		}
		return result[i].MarketValue > result[j].MarketValue
	})
	return result, nil
}

func basisPoints(value, total money.Decimal) int64 {
	if total == 0 {
		return 0
	}
	numerator := new(big.Int).Mul(big.NewInt(int64(value)), big.NewInt(10_000))
	denominator := big.NewInt(int64(total))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if new(big.Int).Lsh(new(big.Int).Abs(remainder), 1).Cmp(new(big.Int).Abs(denominator)) >= 0 {
		if numerator.Sign() == denominator.Sign() {
			quotient.Add(quotient, big.NewInt(1))
		} else {
			quotient.Sub(quotient, big.NewInt(1))
		}
	}
	return quotient.Int64()
}

func (s *Store) Transactions(ctx context.Context, filter TransactionFilter) ([]transactions.Transaction, error) {
	clauses := []string{"1=1"}
	args := []any{}
	if filter.ISIN != "" {
		isin, err := s.ResolveISIN(ctx, filter.ISIN)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, "isin=?")
		args = append(args, isin)
	}
	if filter.Kind != "" {
		clauses = append(clauses, "kind=?")
		args = append(args, string(filter.Kind))
	}
	if filter.Since != nil {
		clauses = append(clauses, "occurred_at>=?")
		args = append(args, formatTime(*filter.Since))
	}
	if filter.Until != nil {
		clauses = append(clauses, "occurred_at<?")
		args = append(args, formatTime(*filter.Until))
	}
	limit := filter.Limit
	if limit <= 0 || limit > 10_000 {
		limit = 1_000
	}
	args = append(args, limit)
	query := `SELECT id, fingerprint, occurred_at, original_timestamp, timezone_assumption,
		kind, COALESCE(isin,''), quantity_i, amount_i, fees_i, taxes_i, currency,
		note, source, source_ref, raw_json FROM transactions WHERE ` +
		strings.Join(clauses, " AND ") + ` ORDER BY occurred_at DESC, id LIMIT ?`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query transactions: %w", err)
	}
	defer rows.Close()
	var results []transactions.Transaction
	for rows.Next() {
		var item transactions.Transaction
		var occurredAt, kind string
		var quantity, amount, fees, taxes int64
		if err := rows.Scan(&item.ID, &item.Fingerprint, &occurredAt, &item.OriginalTimestamp,
			&item.TimezoneAssumption, &kind, &item.ISIN, &quantity, &amount, &fees, &taxes,
			&item.Currency, &item.Note, &item.Source, &item.SourceRef, &item.RawJSON); err != nil {
			return nil, err
		}
		item.OccurredAt, err = parseTime(occurredAt)
		if err != nil {
			return nil, err
		}
		item.Kind = transactions.Kind(kind)
		item.Quantity = money.Decimal(quantity)
		item.Amount = money.Decimal(amount)
		item.Fees = money.Decimal(fees)
		item.Taxes = money.Decimal(taxes)
		results = append(results, item)
	}
	return results, rows.Err()
}

func (s *Store) DividendTotals(ctx context.Context, since, until *time.Time) ([]MonetaryTotal, error) {
	return s.activityTotals(ctx, "dividends", "net_i", since, until)
}

func (s *Store) FeeTotals(ctx context.Context, since, until *time.Time) ([]MonetaryTotal, error) {
	return s.activityTotals(ctx, "fees", "amount_i", since, until)
}

func (s *Store) TaxTotals(ctx context.Context, since, until *time.Time) ([]MonetaryTotal, error) {
	return s.activityTotals(ctx, "taxes", "amount_i", since, until)
}

func (s *Store) activityTotals(ctx context.Context, table, amountColumn string, since, until *time.Time) ([]MonetaryTotal, error) {
	allowed := map[string]string{"dividends": "net_i", "fees": "amount_i", "taxes": "amount_i"}
	if allowed[table] != amountColumn {
		return nil, fmt.Errorf("unsupported activity total")
	}
	clauses := []string{"1=1"}
	args := []any{}
	if since != nil {
		clauses = append(clauses, "occurred_at>=?")
		args = append(args, formatTime(*since))
	}
	if until != nil {
		clauses = append(clauses, "occurred_at<?")
		args = append(args, formatTime(*until))
	}
	query := `SELECT currency, COALESCE(SUM(` + amountColumn + `), 0), COUNT(*) FROM ` + table +
		` WHERE ` + strings.Join(clauses, " AND ") + ` GROUP BY currency ORDER BY currency`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query %s totals: %w", table, err)
	}
	defer rows.Close()
	var totals []MonetaryTotal
	for rows.Next() {
		var item MonetaryTotal
		var amount int64
		if err := rows.Scan(&item.Currency, &amount, &item.Count); err != nil {
			return nil, err
		}
		item.Amount = money.Decimal(amount)
		totals = append(totals, item)
	}
	return totals, rows.Err()
}

func (s *Store) CashBalance(ctx context.Context, currency string, maxAge time.Duration) (portfolio.CashBalance, error) {
	currency = normalizeCurrency(currency)
	var result portfolio.CashBalance
	var amount int64
	var asOf string
	err := s.db.QueryRowContext(ctx, `SELECT currency, amount_i, as_of, source FROM cash_balances WHERE currency=?`, currency).
		Scan(&result.Currency, &amount, &asOf, &result.Source)
	if errors.Is(err, sql.ErrNoRows) {
		return result, fmt.Errorf("%w: cash balance %s", ErrNotFound, currency)
	}
	if err != nil {
		return result, fmt.Errorf("query cash balance: %w", err)
	}
	result.Amount = money.Decimal(amount)
	result.AsOf, err = parseTime(asOf)
	if err != nil {
		return result, err
	}
	if maxAge > 0 && s.now().UTC().Sub(result.AsOf) > maxAge {
		return result, fmt.Errorf("%w: cash balance %s is from %s", ErrStale, currency, result.AsOf.Format(time.RFC3339))
	}
	return result, nil
}

func (s *Store) RecordPrice(ctx context.Context, point PricePoint) error {
	point.ISIN = instruments.NormalizeISIN(point.ISIN)
	if err := instruments.ValidateISIN(point.ISIN); err != nil {
		return err
	}
	point.Currency = normalizeCurrency(point.Currency)
	if point.Currency == "" || point.Source == "" || point.AsOf.IsZero() {
		return fmt.Errorf("price currency, source, and as_of are required")
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ensureInstrument(ctx, tx, point.ISIN, point.ISIN, point.AsOf, now); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO price_history(
		isin, price_i, currency, venue, as_of, source, source_url, created_at
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(isin, currency, venue, as_of, source) DO UPDATE SET
		price_i=excluded.price_i, source_url=excluded.source_url`, point.ISIN, int64(point.Price),
		point.Currency, point.Venue, formatTime(point.AsOf), point.Source, point.SourceURL, formatTime(now))
	if err != nil {
		return fmt.Errorf("record price: %w", err)
	}
	return tx.Commit()
}

func (s *Store) FreshPrice(ctx context.Context, identifier string, maxAge time.Duration) (PricePoint, error) {
	isin, err := s.ResolveISIN(ctx, identifier)
	if err != nil {
		return PricePoint{}, err
	}
	var result PricePoint
	var price int64
	var asOf string
	err = s.db.QueryRowContext(ctx, `SELECT isin, price_i, currency, venue, as_of, source, source_url
		FROM price_history WHERE isin=? ORDER BY as_of DESC, id DESC LIMIT 1`, isin).
		Scan(&result.ISIN, &price, &result.Currency, &result.Venue, &asOf, &result.Source, &result.SourceURL)
	if errors.Is(err, sql.ErrNoRows) {
		return result, fmt.Errorf("%w: price %s", ErrNotFound, identifier)
	}
	if err != nil {
		return result, fmt.Errorf("query latest price: %w", err)
	}
	result.Price = money.Decimal(price)
	result.AsOf, err = parseTime(asOf)
	if err != nil {
		return result, err
	}
	if maxAge > 0 && s.now().UTC().Sub(result.AsOf) > maxAge {
		return result, fmt.Errorf("%w: price %s is from %s", ErrStale, identifier, result.AsOf.Format(time.RFC3339))
	}
	return result, nil
}

func (s *Store) SaveResearchReport(ctx context.Context, report research.Report) (research.Report, error) {
	if report.Identifier == "" || report.Name == "" || report.Kind == "" || report.AsOf.IsZero() {
		return report, fmt.Errorf("research identifier, name, kind, and as_of are required")
	}
	if report.SchemaVersion == 0 {
		report.SchemaVersion = 1
	}
	if report.ISIN != "" {
		report.ISIN = instruments.NormalizeISIN(report.ISIN)
		if err := instruments.ValidateISIN(report.ISIN); err != nil {
			return report, err
		}
	}
	body, err := json.Marshal(report)
	if err != nil {
		return report, fmt.Errorf("encode research report: %w", err)
	}
	if report.ID == "" {
		digest := sha256.Sum256(body)
		report.ID = hex.EncodeToString(digest[:])
		body, err = json.Marshal(report)
		if err != nil {
			return report, err
		}
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return report, err
	}
	defer tx.Rollback()
	if report.ISIN != "" {
		if err := ensureInstrument(ctx, tx, report.ISIN, report.Name, report.AsOf, now); err != nil {
			return report, err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO research_reports(
		id, identifier, isin, symbol, name, kind, as_of, schema_version,
		report_json, created_at, updated_at
	) VALUES(?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET identifier=excluded.identifier, isin=excluded.isin,
		symbol=excluded.symbol, name=excluded.name, kind=excluded.kind, as_of=excluded.as_of,
		schema_version=excluded.schema_version, report_json=excluded.report_json,
		updated_at=excluded.updated_at`, report.ID, report.Identifier, report.ISIN, report.Symbol,
		report.Name, report.Kind, formatTime(report.AsOf), report.SchemaVersion, string(body),
		formatTime(now), formatTime(now))
	if err != nil {
		return report, fmt.Errorf("save research report: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return report, err
	}
	return report, nil
}

func (s *Store) ResearchReport(ctx context.Context, identifier string) (research.Report, error) {
	var body string
	err := s.db.QueryRowContext(ctx, `SELECT report_json FROM research_reports
		WHERE id=? OR lower(identifier)=lower(?) OR isin=? OR lower(symbol)=lower(?)
		ORDER BY as_of DESC LIMIT 1`, identifier, identifier, instruments.NormalizeISIN(identifier), identifier).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return research.Report{}, fmt.Errorf("%w: research report %q", ErrNotFound, identifier)
	}
	if err != nil {
		return research.Report{}, fmt.Errorf("query research report: %w", err)
	}
	var report research.Report
	if err := json.Unmarshal([]byte(body), &report); err != nil {
		return report, fmt.Errorf("decode research report: %w", err)
	}
	return report, nil
}
