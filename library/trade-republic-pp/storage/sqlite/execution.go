package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"trade-republic-pp-cli/internal/instruments"
	"trade-republic-pp-cli/internal/money"
)

var disableLegacyExecutionWrites = true

type ExecutionPreview struct {
	ID             string        `json:"id"`
	IdempotencyKey string        `json:"idempotency_key"`
	ISIN           string        `json:"isin"`
	Side           string        `json:"side"`
	Quantity       money.Decimal `json:"quantity"`
	Amount         money.Decimal `json:"amount"`
	LimitPrice     money.Decimal `json:"limit_price"`
	Currency       string        `json:"currency"`
	Mode           string        `json:"mode"`
	PriceAsOf      time.Time     `json:"price_as_of"`
	BalanceAsOf    time.Time     `json:"balance_as_of"`
	ExpiresAt      time.Time     `json:"expires_at"`
	PayloadJSON    string        `json:"payload_json"`
	Status         string        `json:"status"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

type ExecutionApproval struct {
	ID            string    `json:"id"`
	PreviewID     string    `json:"preview_id"`
	ChallengeHash string    `json:"challenge_hash"`
	ApprovedBy    string    `json:"approved_by"`
	ApprovedAt    time.Time `json:"approved_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	MetadataJSON  string    `json:"metadata_json"`
}

type ExecutionAuditEntry struct {
	Sequence     int64     `json:"sequence"`
	ID           string    `json:"id"`
	PreviewID    string    `json:"preview_id,omitempty"`
	EventType    string    `json:"event_type"`
	PayloadJSON  string    `json:"payload_json"`
	PreviousHash string    `json:"previous_hash"`
	EntryHash    string    `json:"entry_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

type ExecutionIdempotency struct {
	Key         string    `json:"key"`
	Operation   string    `json:"operation"`
	RequestHash string    `json:"request_hash"`
	Status      string    `json:"status"`
	ResultJSON  string    `json:"result_json,omitempty"`
	ErrorText   string    `json:"error_text,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (s *Store) CreateExecutionPreview(ctx context.Context, preview ExecutionPreview) (ExecutionPreview, bool, error) {
	if disableLegacyExecutionWrites {
		return preview, false, fmt.Errorf("legacy execution preview writer is disabled; use internal/execution.Engine")
	}
	preview.ISIN = instruments.NormalizeISIN(preview.ISIN)
	if err := instruments.ValidateISIN(preview.ISIN); err != nil {
		return preview, false, err
	}
	preview.Side = strings.ToLower(strings.TrimSpace(preview.Side))
	if preview.Side != "buy" && preview.Side != "sell" {
		return preview, false, fmt.Errorf("execution side must be buy or sell")
	}
	preview.Mode = strings.ToLower(strings.TrimSpace(preview.Mode))
	if preview.Mode != "paper" && preview.Mode != "export" {
		return preview, false, fmt.Errorf("execution mode must be paper or export")
	}
	preview.Currency = normalizeCurrency(preview.Currency)
	if preview.IdempotencyKey == "" || preview.Currency == "" || preview.LimitPrice <= 0 ||
		preview.PriceAsOf.IsZero() || preview.BalanceAsOf.IsZero() || preview.ExpiresAt.IsZero() {
		return preview, false, fmt.Errorf("preview idempotency key, currency, positive limit price, freshness timestamps, and expiry are required")
	}
	if preview.Amount <= 0 && preview.Quantity <= 0 {
		return preview, false, fmt.Errorf("preview requires a positive amount or quantity")
	}
	now := s.now().UTC()
	if !preview.ExpiresAt.After(now) {
		return preview, false, fmt.Errorf("preview expiry must be in the future")
	}
	if preview.ID == "" {
		digest := sha256.Sum256([]byte(preview.IdempotencyKey + "\x1f" + preview.PayloadJSON))
		preview.ID = "preview_" + hex.EncodeToString(digest[:16])
	}
	if preview.Status == "" {
		preview.Status = "pending"
	}
	preview.CreatedAt = now
	preview.UpdatedAt = now

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return preview, false, err
	}
	defer tx.Rollback()
	existing, found, err := executionPreviewByKey(ctx, tx, preview.IdempotencyKey)
	if err != nil {
		return preview, false, err
	}
	if found {
		if !samePreviewRequest(existing, preview) {
			return preview, false, fmt.Errorf("%w: execution preview %q", ErrIdempotencyConflict, preview.IdempotencyKey)
		}
		return existing, true, nil
	}
	if err := ensureInstrument(ctx, tx, preview.ISIN, preview.ISIN, now, now); err != nil {
		return preview, false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO execution_previews(
		id, idempotency_key, isin, side, quantity_i, amount_i, limit_price_i,
		currency, mode, price_as_of, balance_as_of, expires_at, payload_json,
		status, created_at, updated_at
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, preview.ID,
		preview.IdempotencyKey, preview.ISIN, preview.Side, int64(preview.Quantity),
		int64(preview.Amount), int64(preview.LimitPrice), preview.Currency, preview.Mode,
		formatTime(preview.PriceAsOf), formatTime(preview.BalanceAsOf), formatTime(preview.ExpiresAt),
		preview.PayloadJSON, preview.Status, formatTime(now), formatTime(now))
	if err != nil {
		return preview, false, fmt.Errorf("create execution preview: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return preview, false, err
	}
	return preview, false, nil
}

func executionPreviewByKey(ctx context.Context, tx *sql.Tx, key string) (ExecutionPreview, bool, error) {
	var item ExecutionPreview
	var quantity, amount, limitPrice int64
	var priceAsOf, balanceAsOf, expiresAt, createdAt, updatedAt string
	err := tx.QueryRowContext(ctx, `SELECT id, idempotency_key, isin, side, quantity_i,
		amount_i, limit_price_i, currency, mode, price_as_of, balance_as_of, expires_at,
		payload_json, status, created_at, updated_at FROM execution_previews WHERE idempotency_key=?`, key).
		Scan(&item.ID, &item.IdempotencyKey, &item.ISIN, &item.Side, &quantity, &amount,
			&limitPrice, &item.Currency, &item.Mode, &priceAsOf, &balanceAsOf, &expiresAt,
			&item.PayloadJSON, &item.Status, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return item, false, nil
	}
	if err != nil {
		return item, false, fmt.Errorf("query execution preview: %w", err)
	}
	item.Quantity = money.Decimal(quantity)
	item.Amount = money.Decimal(amount)
	item.LimitPrice = money.Decimal(limitPrice)
	for value, target := range map[string]*time.Time{
		priceAsOf: &item.PriceAsOf, balanceAsOf: &item.BalanceAsOf,
		expiresAt: &item.ExpiresAt, createdAt: &item.CreatedAt, updatedAt: &item.UpdatedAt,
	} {
		parsed, parseErr := parseTime(value)
		if parseErr != nil {
			return item, false, parseErr
		}
		*target = parsed
	}
	return item, true, nil
}

func samePreviewRequest(left, right ExecutionPreview) bool {
	return left.ISIN == right.ISIN && left.Side == right.Side && left.Quantity == right.Quantity &&
		left.Amount == right.Amount && left.LimitPrice == right.LimitPrice &&
		left.Currency == right.Currency && left.Mode == right.Mode &&
		left.PriceAsOf.Equal(right.PriceAsOf) && left.BalanceAsOf.Equal(right.BalanceAsOf) &&
		left.ExpiresAt.Equal(right.ExpiresAt) && left.PayloadJSON == right.PayloadJSON
}

func (s *Store) SaveExecutionApproval(ctx context.Context, approval ExecutionApproval) error {
	if disableLegacyExecutionWrites {
		return fmt.Errorf("legacy execution approval writer is disabled; use internal/execution.Engine")
	}
	if approval.PreviewID == "" || approval.ChallengeHash == "" || approval.ApprovedBy == "" ||
		approval.ApprovedAt.IsZero() || approval.ExpiresAt.IsZero() {
		return fmt.Errorf("approval preview, challenge hash, approver, approval time, and expiry are required")
	}
	if approval.ID == "" {
		digest := sha256.Sum256([]byte(approval.PreviewID + "\x1f" + approval.ChallengeHash))
		approval.ID = "approval_" + hex.EncodeToString(digest[:16])
	}
	if approval.MetadataJSON == "" {
		approval.MetadataJSON = "{}"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var previewExpiry string
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT expires_at, status FROM execution_previews WHERE id=?`, approval.PreviewID).
		Scan(&previewExpiry, &status); errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: execution preview %s", ErrNotFound, approval.PreviewID)
	} else if err != nil {
		return err
	}
	expires, err := parseTime(previewExpiry)
	if err != nil {
		return err
	}
	if status != "pending" || !expires.After(approval.ApprovedAt) || !approval.ExpiresAt.After(approval.ApprovedAt) {
		return fmt.Errorf("execution preview is not eligible for approval")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO execution_approvals(
		id, preview_id, challenge_hash, approved_by, approved_at, expires_at, metadata_json
	) VALUES(?, ?, ?, ?, ?, ?, ?)`, approval.ID, approval.PreviewID, approval.ChallengeHash,
		approval.ApprovedBy, formatTime(approval.ApprovedAt), formatTime(approval.ExpiresAt),
		approval.MetadataJSON); err != nil {
		return fmt.Errorf("save execution approval: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE execution_previews SET status='approved', updated_at=? WHERE id=?`,
		formatTime(s.now().UTC()), approval.PreviewID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AppendExecutionAudit(ctx context.Context, entry ExecutionAuditEntry) (ExecutionAuditEntry, error) {
	if disableLegacyExecutionWrites {
		return entry, fmt.Errorf("legacy execution audit writer is disabled; use internal/execution.Engine")
	}
	if entry.EventType == "" || entry.PayloadJSON == "" {
		return entry, fmt.Errorf("audit event type and payload are required")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = s.now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return entry, err
	}
	defer tx.Rollback()
	var previous string
	err = tx.QueryRowContext(ctx, `SELECT entry_hash FROM execution_audit_log ORDER BY sequence DESC LIMIT 1`).Scan(&previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return entry, err
	}
	entry.PreviousHash = previous
	digest := sha256.Sum256([]byte(strings.Join([]string{
		entry.PreviousHash, entry.PreviewID, entry.EventType, entry.PayloadJSON, formatTime(entry.CreatedAt),
	}, "\x1f")))
	entry.EntryHash = hex.EncodeToString(digest[:])
	if entry.ID == "" {
		entry.ID = "audit_" + entry.EntryHash[:32]
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO execution_audit_log(
		id, preview_id, event_type, payload_json, previous_hash, entry_hash, created_at
	) VALUES(?, NULLIF(?, ''), ?, ?, ?, ?, ?)`, entry.ID, entry.PreviewID, entry.EventType,
		entry.PayloadJSON, entry.PreviousHash, entry.EntryHash, formatTime(entry.CreatedAt))
	if err != nil {
		return entry, fmt.Errorf("append execution audit: %w", err)
	}
	entry.Sequence, err = result.LastInsertId()
	if err != nil {
		return entry, err
	}
	if err := tx.Commit(); err != nil {
		return entry, err
	}
	return entry, nil
}

func (s *Store) ReserveExecutionIdempotency(ctx context.Context, record ExecutionIdempotency) (ExecutionIdempotency, bool, error) {
	if disableLegacyExecutionWrites {
		return record, false, fmt.Errorf("legacy execution idempotency writer is disabled; use internal/execution.Engine")
	}
	if record.Key == "" || record.Operation == "" || record.RequestHash == "" || record.ExpiresAt.IsZero() {
		return record, false, fmt.Errorf("idempotency key, operation, request hash, and expiry are required")
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return record, false, err
	}
	defer tx.Rollback()
	existing, found, err := executionIdempotencyByKey(ctx, tx, record.Key)
	if err != nil {
		return record, false, err
	}
	if found && existing.ExpiresAt.After(now) {
		if existing.Operation != record.Operation || existing.RequestHash != record.RequestHash {
			return record, false, fmt.Errorf("%w: execution operation %q", ErrIdempotencyConflict, record.Key)
		}
		return existing, true, nil
	}
	record.Status = "reserved"
	record.CreatedAt = now
	record.UpdatedAt = now
	_, err = tx.ExecContext(ctx, `INSERT INTO execution_idempotency(
		idempotency_key, operation, request_hash, status, result_json, error_text,
		created_at, updated_at, expires_at
	) VALUES(?, ?, ?, 'reserved', '', '', ?, ?, ?)
	ON CONFLICT(idempotency_key) DO UPDATE SET operation=excluded.operation,
		request_hash=excluded.request_hash, status='reserved', result_json='', error_text='',
		created_at=excluded.created_at, updated_at=excluded.updated_at, expires_at=excluded.expires_at`,
		record.Key, record.Operation, record.RequestHash, formatTime(now), formatTime(now),
		formatTime(record.ExpiresAt))
	if err != nil {
		return record, false, fmt.Errorf("reserve execution idempotency: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return record, false, err
	}
	return record, false, nil
}

func executionIdempotencyByKey(ctx context.Context, tx *sql.Tx, key string) (ExecutionIdempotency, bool, error) {
	var item ExecutionIdempotency
	var createdAt, updatedAt, expiresAt string
	err := tx.QueryRowContext(ctx, `SELECT idempotency_key, operation, request_hash, status,
		result_json, error_text, created_at, updated_at, expires_at
		FROM execution_idempotency WHERE idempotency_key=?`, key).Scan(&item.Key,
		&item.Operation, &item.RequestHash, &item.Status, &item.ResultJSON, &item.ErrorText,
		&createdAt, &updatedAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return item, false, nil
	}
	if err != nil {
		return item, false, err
	}
	item.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return item, false, err
	}
	item.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return item, false, err
	}
	item.ExpiresAt, err = parseTime(expiresAt)
	return item, true, err
}

func (s *Store) CompleteExecutionIdempotency(ctx context.Context, key, status, resultJSON, errorText string) error {
	if disableLegacyExecutionWrites {
		return fmt.Errorf("legacy execution idempotency writer is disabled; use internal/execution.Engine")
	}
	if status != "completed" && status != "failed" {
		return fmt.Errorf("idempotency completion status must be completed or failed")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE execution_idempotency SET
		status=?, result_json=?, error_text=?, updated_at=? WHERE idempotency_key=?`,
		status, resultJSON, errorText, formatTime(s.now().UTC()), key)
	if err != nil {
		return fmt.Errorf("complete execution idempotency: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: execution idempotency %q", ErrNotFound, key)
	}
	return nil
}
