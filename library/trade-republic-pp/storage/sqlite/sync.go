package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"trade-republic-pp-cli/internal/instruments"
	"trade-republic-pp-cli/internal/money"
	"trade-republic-pp-cli/internal/portfolio"
	"trade-republic-pp-cli/internal/transactions"
	"trade-republic-pp-cli/providers/traderepublic"
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type ApplySyncOptions struct {
	Request        traderepublic.SyncRequest
	DryRun         bool
	IdempotencyKey string
	Metadata       map[string]string
}

type SyncResult struct {
	RunID        string    `json:"run_id"`
	RequestKey   string    `json:"request_key"`
	Status       string    `json:"status"`
	DryRun       bool      `json:"dry_run"`
	Duplicate    bool      `json:"duplicate"`
	Instruments  int       `json:"instruments"`
	Positions    int       `json:"positions"`
	CashBalances int       `json:"cash_balances"`
	Transactions int       `json:"transactions"`
	Documents    int       `json:"documents"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	Warnings     []string  `json:"warnings,omitempty"`
}

type normalizedSnapshot struct {
	Provider       string
	Adapter        string
	AdapterVersion string
	AsOf           time.Time
	Instruments    []instruments.Instrument
	Positions      []portfolio.Position
	CashBalances   []portfolio.CashBalance
	Transactions   []transactions.Transaction
	Documents      []transactions.Document
	Warnings       []string
}

// ApplySync validates and atomically applies a normalized broker snapshot.
// Identical successful requests are returned as duplicate successes. Failed
// imports roll back all portfolio data while retaining a failed sync-run row.
func (s *Store) ApplySync(ctx context.Context, snapshot traderepublic.Snapshot, options ApplySyncOptions) (SyncResult, error) {
	started := s.now().UTC()
	normalized, validationErr := normalizeSnapshot(snapshot, started)
	requestHash, hashErr := syncRequestHash(normalized, options)
	if hashErr != nil {
		return SyncResult{}, fmt.Errorf("hash sync request: %w", hashErr)
	}
	requestKey := strings.TrimSpace(options.IdempotencyKey)
	if requestKey == "" {
		requestKey = "auto:" + requestHash
	}
	runID := "sync_" + requestHash[:32]
	result := SyncResult{
		RunID:        runID,
		RequestKey:   requestKey,
		Status:       "running",
		DryRun:       options.DryRun,
		Instruments:  len(normalized.Instruments),
		Positions:    len(normalized.Positions),
		CashBalances: len(normalized.CashBalances),
		Transactions: len(normalized.Transactions),
		Documents:    len(normalized.Documents),
		StartedAt:    started,
		Warnings:     append([]string(nil), normalized.Warnings...),
	}
	if validationErr != nil {
		result.Status = "failed"
		result.CompletedAt = s.now().UTC()
		if !options.DryRun {
			if err := s.recordFailedSync(ctx, normalized, options, result, requestHash, validationErr); err != nil {
				return result, errors.Join(validationErr, err)
			}
		}
		return result, validationErr
	}
	if options.DryRun {
		result.Status = "dry_run"
		result.CompletedAt = s.now().UTC()
		return result, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin sync: %w", err)
	}
	defer tx.Rollback()

	existing, found, err := syncResultByKey(ctx, tx, requestKey)
	if err != nil {
		return result, err
	}
	if found {
		if existing.requestHash != requestHash {
			return result, fmt.Errorf("%w: %q", ErrIdempotencyConflict, requestKey)
		}
		if existing.result.Status == "success" {
			existing.result.Duplicate = true
			return existing.result, nil
		}
		runID = existing.result.RunID
		result.RunID = runID
	}

	metadataJSON, err := encodeStringMap(options.Metadata)
	if err != nil {
		return result, err
	}
	warningsJSON, err := json.Marshal(normalized.Warnings)
	if err != nil {
		return result, fmt.Errorf("encode sync warnings: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sync_runs(
		id, request_key, request_hash, provider, adapter, adapter_version, snapshot_as_of,
		requested_since, include_documents, dry_run, request_metadata_json, warnings_json,
		status, started_at, error_text
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, 'running', ?, '')
	ON CONFLICT(request_key) DO UPDATE SET
		request_hash=excluded.request_hash, provider=excluded.provider, adapter=excluded.adapter,
		adapter_version=excluded.adapter_version, snapshot_as_of=excluded.snapshot_as_of,
		requested_since=excluded.requested_since, include_documents=excluded.include_documents,
		request_metadata_json=excluded.request_metadata_json, warnings_json=excluded.warnings_json,
		status='running', started_at=excluded.started_at, completed_at=NULL, error_text=''`,
		runID, requestKey, requestHash, normalized.Provider, normalized.Adapter,
		normalized.AdapterVersion, formatTime(normalized.AsOf), nullableTime(options.Request.Since),
		boolInt(options.Request.IncludeDocuments), metadataJSON, string(warningsJSON), formatTime(started),
	)
	if err != nil {
		return result, fmt.Errorf("start sync run: %w", err)
	}

	if err := applyNormalizedSnapshot(ctx, tx, normalized, runID, started); err != nil {
		tx.Rollback()
		result.Status = "failed"
		result.CompletedAt = s.now().UTC()
		if recordErr := s.recordFailedSync(ctx, normalized, options, result, requestHash, err); recordErr != nil {
			return result, errors.Join(err, recordErr)
		}
		return result, err
	}
	completed := s.now().UTC()
	_, err = tx.ExecContext(ctx, `UPDATE sync_runs SET
		status='success', completed_at=?, error_text='', instruments_count=?, positions_count=?,
		cash_balances_count=?, transactions_count=?, documents_count=? WHERE id=?`,
		formatTime(completed), result.Instruments, result.Positions, result.CashBalances,
		result.Transactions, result.Documents, runID,
	)
	if err != nil {
		return result, fmt.Errorf("complete sync run: %w", err)
	}
	if err := tx.Commit(); err != nil {
		result.Status = "failed"
		result.CompletedAt = s.now().UTC()
		if recordErr := s.recordFailedSync(ctx, normalized, options, result, requestHash, err); recordErr != nil {
			return result, errors.Join(err, recordErr)
		}
		return result, fmt.Errorf("commit sync: %w", err)
	}
	result.Status = "success"
	result.CompletedAt = completed
	return result, nil
}

func normalizeSnapshot(snapshot traderepublic.Snapshot, now time.Time) (normalizedSnapshot, error) {
	out := normalizedSnapshot{
		Provider:       strings.TrimSpace(snapshot.Provider),
		Adapter:        strings.TrimSpace(snapshot.Adapter),
		AdapterVersion: strings.TrimSpace(snapshot.AdapterVersion),
		AsOf:           snapshot.AsOf.UTC(),
		Warnings:       append([]string(nil), snapshot.Warnings...),
	}
	var problems []error
	if out.Provider == "" {
		problems = append(problems, fmt.Errorf("snapshot provider is required"))
	}
	if snapshot.AsOf.IsZero() {
		problems = append(problems, fmt.Errorf("snapshot as_of is required"))
	}

	instrumentMap := map[string]instruments.Instrument{}
	for _, item := range snapshot.Instruments {
		item.ISIN = instruments.NormalizeISIN(item.ISIN)
		if err := instruments.ValidateISIN(item.ISIN); err != nil {
			problems = append(problems, err)
			continue
		}
		item.Name = strings.TrimSpace(item.Name)
		if item.Name == "" {
			item.Name = item.ISIN
		}
		item.Symbol = strings.ToUpper(strings.TrimSpace(item.Symbol))
		item.BaseCurrency = normalizeCurrency(item.BaseCurrency)
		item.TradingCurrency = normalizeCurrency(item.TradingCurrency)
		if item.UpdatedAt.IsZero() {
			item.UpdatedAt = out.AsOf
		}
		instrumentMap[item.ISIN] = item
	}
	for _, item := range instrumentMap {
		out.Instruments = append(out.Instruments, item)
	}
	sort.Slice(out.Instruments, func(i, j int) bool { return out.Instruments[i].ISIN < out.Instruments[j].ISIN })

	positionMap := map[string]portfolio.Position{}
	for _, item := range snapshot.Positions {
		item.ISIN = instruments.NormalizeISIN(item.ISIN)
		if err := instruments.ValidateISIN(item.ISIN); err != nil {
			problems = append(problems, err)
			continue
		}
		item.Currency = normalizeCurrency(item.Currency)
		if item.Currency == "" {
			problems = append(problems, fmt.Errorf("position %s currency is required", item.ISIN))
			continue
		}
		if item.AsOf.IsZero() {
			item.AsOf = out.AsOf
		}
		if item.Source == "" {
			item.Source = out.Provider
		}
		positionMap[item.ISIN] = item
	}
	for _, item := range positionMap {
		out.Positions = append(out.Positions, item)
	}
	sort.Slice(out.Positions, func(i, j int) bool { return out.Positions[i].ISIN < out.Positions[j].ISIN })

	cashMap := map[string]portfolio.CashBalance{}
	for _, item := range snapshot.CashBalances {
		item.Currency = normalizeCurrency(item.Currency)
		if item.Currency == "" {
			problems = append(problems, fmt.Errorf("cash balance currency is required"))
			continue
		}
		if item.AsOf.IsZero() {
			item.AsOf = out.AsOf
		}
		if item.Source == "" {
			item.Source = out.Provider
		}
		cashMap[item.Currency] = item
	}
	for _, item := range cashMap {
		out.CashBalances = append(out.CashBalances, item)
	}
	sort.Slice(out.CashBalances, func(i, j int) bool { return out.CashBalances[i].Currency < out.CashBalances[j].Currency })

	transactionMap := map[string]transactions.Transaction{}
	for _, item := range snapshot.Transactions {
		if item.OccurredAt.IsZero() {
			problems = append(problems, fmt.Errorf("transaction occurred_at is required"))
			continue
		}
		item.ISIN = instruments.NormalizeISIN(item.ISIN)
		if item.ISIN != "" {
			if err := instruments.ValidateISIN(item.ISIN); err != nil {
				problems = append(problems, err)
				continue
			}
		}
		item.Currency = normalizeCurrency(item.Currency)
		if item.Currency == "" {
			problems = append(problems, fmt.Errorf("transaction currency is required"))
			continue
		}
		if item.Kind == "" {
			item.Kind = transactions.Unknown
		}
		if item.Source == "" {
			item.Source = out.Provider
		}
		expectedFingerprint := transactions.Fingerprint(item)
		if item.Fingerprint != "" && item.Fingerprint != expectedFingerprint {
			problems = append(problems, fmt.Errorf("transaction %q fingerprint does not match normalized fields", item.ID))
			continue
		}
		item.Fingerprint = expectedFingerprint
		if item.ID == "" {
			item.ID = expectedFingerprint
		}
		transactionMap[item.Fingerprint] = item
	}
	for _, item := range transactionMap {
		out.Transactions = append(out.Transactions, item)
	}
	sort.Slice(out.Transactions, func(i, j int) bool { return out.Transactions[i].Fingerprint < out.Transactions[j].Fingerprint })

	documentMap := map[string]transactions.Document{}
	for _, item := range snapshot.Documents {
		item.SHA256 = strings.ToLower(strings.TrimSpace(item.SHA256))
		if !sha256Pattern.MatchString(item.SHA256) {
			problems = append(problems, fmt.Errorf("document %q has invalid sha256", item.Filename))
			continue
		}
		item.ISIN = instruments.NormalizeISIN(item.ISIN)
		if item.ISIN != "" {
			if err := instruments.ValidateISIN(item.ISIN); err != nil {
				problems = append(problems, err)
				continue
			}
		}
		if item.ID == "" {
			item.ID = item.SHA256
		}
		if item.Source == "" {
			item.Source = out.Provider
		}
		if item.ImportedAt.IsZero() {
			item.ImportedAt = now
		}
		documentMap[item.SHA256] = item
	}
	for _, item := range documentMap {
		out.Documents = append(out.Documents, item)
	}
	sort.Slice(out.Documents, func(i, j int) bool { return out.Documents[i].SHA256 < out.Documents[j].SHA256 })
	sort.Strings(out.Warnings)
	return out, errors.Join(problems...)
}

func syncRequestHash(snapshot normalizedSnapshot, options ApplySyncOptions) (string, error) {
	payload := struct {
		Snapshot normalizedSnapshot
		Request  traderepublic.SyncRequest
		Metadata map[string]string
	}{snapshot, options.Request, options.Metadata}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func applyNormalizedSnapshot(ctx context.Context, tx *sql.Tx, snapshot normalizedSnapshot, runID string, now time.Time) error {
	for _, item := range snapshot.Instruments {
		if err := upsertInstrument(ctx, tx, item, now); err != nil {
			return err
		}
	}
	for _, item := range snapshot.Positions {
		if err := ensureInstrument(ctx, tx, item.ISIN, item.Name, item.AsOf, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO positions(
			isin, name, quantity_i, average_cost_i, price_i, market_value_i, currency,
			as_of, source, sync_run_id, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(isin) DO UPDATE SET name=excluded.name, quantity_i=excluded.quantity_i,
			average_cost_i=excluded.average_cost_i, price_i=excluded.price_i,
			market_value_i=excluded.market_value_i, currency=excluded.currency,
			as_of=excluded.as_of, source=excluded.source, sync_run_id=excluded.sync_run_id,
			updated_at=excluded.updated_at`, item.ISIN, item.Name, int64(item.Quantity),
			int64(item.AverageCost), int64(item.Price), int64(item.MarketValue), item.Currency,
			formatTime(item.AsOf), item.Source, runID, formatTime(now)); err != nil {
			return fmt.Errorf("upsert position %s: %w", item.ISIN, err)
		}
		if item.Price != 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO price_history(
				isin, price_i, currency, venue, as_of, source, source_url, created_at
			) VALUES(?, ?, ?, '', ?, ?, '', ?)
			ON CONFLICT(isin, currency, venue, as_of, source) DO UPDATE SET price_i=excluded.price_i`,
				item.ISIN, int64(item.Price), item.Currency, formatTime(item.AsOf),
				"position:"+item.Source, formatTime(now)); err != nil {
				return fmt.Errorf("store position price %s: %w", item.ISIN, err)
			}
		}
	}
	for _, item := range snapshot.CashBalances {
		if _, err := tx.ExecContext(ctx, `INSERT INTO cash_balances(
			currency, amount_i, as_of, source, sync_run_id, updated_at
		) VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(currency) DO UPDATE SET amount_i=excluded.amount_i, as_of=excluded.as_of,
			source=excluded.source, sync_run_id=excluded.sync_run_id, updated_at=excluded.updated_at`,
			item.Currency, int64(item.Amount), formatTime(item.AsOf), item.Source, runID, formatTime(now)); err != nil {
			return fmt.Errorf("upsert cash balance %s: %w", item.Currency, err)
		}
	}
	for _, item := range snapshot.Transactions {
		if item.ISIN != "" {
			if err := ensureInstrument(ctx, tx, item.ISIN, item.ISIN, item.OccurredAt, now); err != nil {
				return err
			}
		}
		storedID, err := upsertTransaction(ctx, tx, item, runID, now)
		if err != nil {
			return err
		}
		if err := upsertDerivedLedger(ctx, tx, item, storedID, runID); err != nil {
			return err
		}
	}
	for _, item := range snapshot.Documents {
		if item.ISIN != "" {
			if err := ensureInstrument(ctx, tx, item.ISIN, item.ISIN, snapshot.AsOf, now); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO documents(
			id, sha256, path, filename, document_type, occurred_at, isin, source,
			imported_at, parser_version, sync_run_id, created_at
		) VALUES(?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)
		ON CONFLICT(sha256) DO UPDATE SET path=excluded.path, filename=excluded.filename,
			document_type=excluded.document_type, occurred_at=excluded.occurred_at,
			isin=excluded.isin, source=excluded.source, imported_at=excluded.imported_at,
			parser_version=excluded.parser_version, sync_run_id=excluded.sync_run_id`,
			item.ID, item.SHA256, item.Path, item.Filename, item.DocumentType,
			nullableTimeValue(item.OccurredAt), item.ISIN, item.Source, formatTime(item.ImportedAt),
			item.ParserVersion, runID, formatTime(now)); err != nil {
			return fmt.Errorf("upsert document %s: %w", item.ID, err)
		}
	}
	return nil
}

func upsertInstrument(ctx context.Context, tx *sql.Tx, item instruments.Instrument, now time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO instruments(
		isin, name, kind, symbol, country, sector, domicile, base_currency,
		trading_currency, updated_at, created_at
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(isin) DO UPDATE SET
		name=CASE WHEN excluded.name='' THEN instruments.name ELSE excluded.name END,
		kind=CASE WHEN excluded.kind='' THEN instruments.kind ELSE excluded.kind END,
		symbol=CASE WHEN excluded.symbol='' THEN instruments.symbol ELSE excluded.symbol END,
		country=CASE WHEN excluded.country='' THEN instruments.country ELSE excluded.country END,
		sector=CASE WHEN excluded.sector='' THEN instruments.sector ELSE excluded.sector END,
		domicile=CASE WHEN excluded.domicile='' THEN instruments.domicile ELSE excluded.domicile END,
		base_currency=CASE WHEN excluded.base_currency='' THEN instruments.base_currency ELSE excluded.base_currency END,
		trading_currency=CASE WHEN excluded.trading_currency='' THEN instruments.trading_currency ELSE excluded.trading_currency END,
		updated_at=excluded.updated_at`, item.ISIN, item.Name, item.Kind, item.Symbol,
		item.Country, item.Sector, item.Domicile, item.BaseCurrency, item.TradingCurrency,
		formatTime(item.UpdatedAt), formatTime(now))
	if err != nil {
		return fmt.Errorf("upsert instrument %s: %w", item.ISIN, err)
	}
	for _, alias := range []struct{ value, kind string }{{item.Symbol, "symbol"}, {item.Name, "name"}} {
		if strings.TrimSpace(alias.value) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO instrument_aliases(alias_key, alias, isin, kind, created_at)
			VALUES(?, ?, ?, ?, ?) ON CONFLICT(alias_key, isin, kind) DO UPDATE SET alias=excluded.alias`,
			normalizeAlias(alias.value), alias.value, item.ISIN, alias.kind, formatTime(now)); err != nil {
			return fmt.Errorf("upsert alias %q: %w", alias.value, err)
		}
	}
	return nil
}

func ensureInstrument(ctx context.Context, tx *sql.Tx, isin, name string, asOf, now time.Time) error {
	if name == "" {
		name = isin
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO instruments(isin, name, updated_at, created_at)
		VALUES(?, ?, ?, ?) ON CONFLICT(isin) DO NOTHING`, isin, name, formatTime(asOf), formatTime(now))
	if err != nil {
		return fmt.Errorf("ensure instrument %s: %w", isin, err)
	}
	return nil
}

func upsertTransaction(ctx context.Context, tx *sql.Tx, item transactions.Transaction, runID string, now time.Time) (string, error) {
	storedID := item.ID
	var existingID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM transactions WHERE fingerprint=?`, item.Fingerprint).Scan(&existingID); err == nil {
		storedID = existingID
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("resolve transaction fingerprint: %w", err)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO transactions(
		id, fingerprint, occurred_at, original_timestamp, timezone_assumption, kind, isin,
		quantity_i, amount_i, fees_i, taxes_i, currency, note, source, source_ref,
		raw_json, sync_run_id, created_at, updated_at
	) VALUES(?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET fingerprint=excluded.fingerprint, occurred_at=excluded.occurred_at,
		original_timestamp=excluded.original_timestamp, timezone_assumption=excluded.timezone_assumption,
		kind=excluded.kind, isin=excluded.isin, quantity_i=excluded.quantity_i,
		amount_i=excluded.amount_i, fees_i=excluded.fees_i, taxes_i=excluded.taxes_i,
		currency=excluded.currency, note=excluded.note, source=excluded.source,
		source_ref=excluded.source_ref, raw_json=excluded.raw_json,
		sync_run_id=excluded.sync_run_id, updated_at=excluded.updated_at`,
		storedID, item.Fingerprint, formatTime(item.OccurredAt), item.OriginalTimestamp,
		item.TimezoneAssumption, string(item.Kind), item.ISIN, int64(item.Quantity),
		int64(item.Amount), int64(item.Fees), int64(item.Taxes), item.Currency,
		item.Note, item.Source, item.SourceRef, item.RawJSON, runID, formatTime(now), formatTime(now))
	if err != nil {
		return "", fmt.Errorf("upsert transaction %s: %w", storedID, err)
	}
	return storedID, nil
}

func upsertDerivedLedger(ctx context.Context, tx *sql.Tx, item transactions.Transaction, transactionID, runID string) error {
	if item.Amount != 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO cash_movements(
			id, transaction_id, occurred_at, kind, amount_i, currency, sync_run_id
		) VALUES(?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(transaction_id) DO UPDATE SET occurred_at=excluded.occurred_at,
			kind=excluded.kind, amount_i=excluded.amount_i, currency=excluded.currency,
			sync_run_id=excluded.sync_run_id`, "cash:"+transactionID, transactionID,
			formatTime(item.OccurredAt), string(item.Kind), int64(item.Amount), item.Currency, runID); err != nil {
			return fmt.Errorf("upsert cash movement %s: %w", transactionID, err)
		}
	}
	if item.Kind == transactions.Dividend {
		taxes := item.Taxes.Abs()
		net := item.Amount.Sub(taxes).Sub(item.Fees.Abs())
		if _, err := tx.ExecContext(ctx, `INSERT INTO dividends(
			id, transaction_id, isin, occurred_at, gross_i, net_i, taxes_i, currency, sync_run_id
		) VALUES(?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?)
		ON CONFLICT(transaction_id) DO UPDATE SET isin=excluded.isin,
			occurred_at=excluded.occurred_at, gross_i=excluded.gross_i, net_i=excluded.net_i,
			taxes_i=excluded.taxes_i, currency=excluded.currency, sync_run_id=excluded.sync_run_id`,
			"dividend:"+transactionID, transactionID, item.ISIN, formatTime(item.OccurredAt),
			int64(item.Amount), int64(net), int64(taxes), item.Currency, runID); err != nil {
			return fmt.Errorf("upsert dividend %s: %w", transactionID, err)
		}
	}
	fee := item.Fees.Abs()
	if fee == 0 && item.Kind == transactions.Fee {
		fee = item.Amount.Abs()
	}
	if fee != 0 {
		if err := upsertCharge(ctx, tx, "fees", "fee:"+transactionID, transactionID, item, fee, runID); err != nil {
			return err
		}
	}
	tax := item.Taxes.Abs()
	if tax == 0 && item.Kind == transactions.Tax {
		tax = item.Amount.Abs()
	}
	if tax != 0 {
		if err := upsertCharge(ctx, tx, "taxes", "tax:"+transactionID, transactionID, item, tax, runID); err != nil {
			return err
		}
	}
	return nil
}

func upsertCharge(ctx context.Context, tx *sql.Tx, table, id, transactionID string, item transactions.Transaction, amount money.Decimal, runID string) error {
	query := `INSERT INTO ` + table + `(
		id, transaction_id, isin, occurred_at, amount_i, currency, sync_run_id
	) VALUES(?, ?, NULLIF(?, ''), ?, ?, ?, ?)
	ON CONFLICT(transaction_id) DO UPDATE SET isin=excluded.isin, occurred_at=excluded.occurred_at,
		amount_i=excluded.amount_i, currency=excluded.currency, sync_run_id=excluded.sync_run_id`
	if _, err := tx.ExecContext(ctx, query, id, transactionID, item.ISIN,
		formatTime(item.OccurredAt), int64(amount), item.Currency, runID); err != nil {
		return fmt.Errorf("upsert %s for %s: %w", table, transactionID, err)
	}
	return nil
}

type storedSyncResult struct {
	result      SyncResult
	requestHash string
}

func syncResultByKey(ctx context.Context, tx *sql.Tx, key string) (storedSyncResult, bool, error) {
	var stored storedSyncResult
	var startedAt string
	var completed sql.NullString
	var warningsJSON string
	err := tx.QueryRowContext(ctx, `SELECT id, request_key, request_hash, status, dry_run,
		instruments_count, positions_count, cash_balances_count, transactions_count,
		documents_count, started_at, completed_at, warnings_json
		FROM sync_runs WHERE request_key=?`, key).Scan(
		&stored.result.RunID, &stored.result.RequestKey, &stored.requestHash, &stored.result.Status,
		&stored.result.DryRun, &stored.result.Instruments, &stored.result.Positions,
		&stored.result.CashBalances, &stored.result.Transactions, &stored.result.Documents,
		&startedAt, &completed, &warningsJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return stored, false, nil
	}
	if err != nil {
		return stored, false, fmt.Errorf("query sync idempotency: %w", err)
	}
	stored.result.StartedAt, err = parseTime(startedAt)
	if err != nil {
		return stored, false, err
	}
	if completed.Valid {
		stored.result.CompletedAt, err = parseTime(completed.String)
		if err != nil {
			return stored, false, err
		}
	}
	if err := json.Unmarshal([]byte(warningsJSON), &stored.result.Warnings); err != nil {
		return stored, false, fmt.Errorf("decode sync warnings: %w", err)
	}
	return stored, true, nil
}

func (s *Store) recordFailedSync(ctx context.Context, snapshot normalizedSnapshot, options ApplySyncOptions, result SyncResult, requestHash string, syncErr error) error {
	metadataJSON, err := encodeStringMap(options.Metadata)
	if err != nil {
		return err
	}
	warningsJSON, err := json.Marshal(snapshot.Warnings)
	if err != nil {
		return fmt.Errorf("encode failed sync warnings: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO sync_runs(
		id, request_key, request_hash, provider, adapter, adapter_version, snapshot_as_of,
		requested_since, include_documents, dry_run, request_metadata_json, warnings_json,
		status, instruments_count, positions_count, cash_balances_count, transactions_count,
		documents_count, started_at, completed_at, error_text
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, 'failed', ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(request_key) DO UPDATE SET status='failed', completed_at=excluded.completed_at,
		error_text=excluded.error_text, warnings_json=excluded.warnings_json`,
		result.RunID, result.RequestKey, requestHash, snapshot.Provider, snapshot.Adapter,
		snapshot.AdapterVersion, formatTime(snapshot.AsOf), nullableTime(options.Request.Since),
		boolInt(options.Request.IncludeDocuments), metadataJSON, string(warningsJSON),
		result.Instruments, result.Positions, result.CashBalances, result.Transactions,
		result.Documents, formatTime(result.StartedAt), formatTime(result.CompletedAt), syncErr.Error())
	if err != nil {
		return fmt.Errorf("record failed sync: %w", err)
	}
	return nil
}

func encodeStringMap(value map[string]string) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode sync metadata: %w", err)
	}
	if string(data) == "null" {
		return "{}", nil
	}
	return string(data), nil
}

func normalizeAlias(value string) string {
	return strings.ToUpper(strings.Join(strings.Fields(value), " "))
}

func normalizeCurrency(value string) string { return strings.ToUpper(strings.TrimSpace(value)) }

func nullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return formatTime(*value)
}

func nullableTimeValue(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return formatTime(value)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
