package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"trade-republic-pp-cli/internal/execution"
	"trade-republic-pp-cli/internal/money"
)

var _ execution.Repository = (*Store)(nil)

func (s *Store) LoadRiskState(ctx context.Context, query execution.RiskStateQuery) (execution.RiskState, error) {
	var state execution.RiskState
	var exposure, buyCash, sellQuantity int64
	err := s.db.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(amount_i), 0),
		COALESCE(SUM(CASE WHEN side='buy' THEN amount_i ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN side='sell' AND isin=? THEN quantity_i ELSE 0 END), 0)
		FROM execution_previews
		WHERE account_id=? AND currency=? AND substr(created_at, 1, 10)=?
			AND status IN ('pending', 'approved') AND (status='approved' OR expires_at>?)`, query.ISIN,
		query.AccountID, normalizeCurrency(query.Currency), query.TradingDay, formatTime(query.AsOf)).
		Scan(&exposure, &buyCash, &sellQuantity)
	if err != nil {
		return state, fmt.Errorf("load execution reservations: %w", err)
	}
	state.ReservedExposure = money.Decimal(exposure)
	state.ReservedBuyCash = money.Decimal(buyCash)
	state.ReservedSellQuantity = money.Decimal(sellQuantity)
	var idempotencyCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM execution_previews WHERE idempotency_key=?`,
		query.IdempotencyKey).Scan(&idempotencyCount); err != nil {
		return state, fmt.Errorf("load execution idempotency: %w", err)
	}
	state.IdempotencyExists = idempotencyCount != 0
	state.AuditHead, state.AuditSequence, err = loadAuditHead(ctx, s.db)
	if err != nil {
		return state, err
	}
	return state, nil
}

func (s *Store) LoadPreview(ctx context.Context, previewID string) (execution.StoredPreview, error) {
	var stored execution.StoredPreview
	var previewJSON string
	err := s.db.QueryRowContext(ctx, `SELECT payload_json FROM execution_previews WHERE id=?`, previewID).Scan(&previewJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return stored, execution.ErrPreviewNotFound
	}
	if err != nil {
		return stored, fmt.Errorf("load execution preview: %w", err)
	}
	if err := json.Unmarshal([]byte(previewJSON), &stored.Preview); err != nil {
		return stored, fmt.Errorf("decode execution preview: %w", err)
	}
	var approvalJSON string
	err = s.db.QueryRowContext(ctx, `SELECT approval_json FROM execution_approvals WHERE preview_id=?`, previewID).Scan(&approvalJSON)
	if err == nil {
		var approval execution.Approval
		if err := json.Unmarshal([]byte(approvalJSON), &approval); err != nil {
			return stored, fmt.Errorf("decode execution approval: %w", err)
		}
		stored.Approval = &approval
	} else if !errors.Is(err, sql.ErrNoRows) {
		return stored, fmt.Errorf("load execution approval: %w", err)
	}
	stored.AuditHead, stored.AuditSequence, err = loadAuditHead(ctx, s.db)
	if err != nil {
		return stored, err
	}
	return stored, nil
}

func (s *Store) Commit(ctx context.Context, request execution.CommitRequest) error {
	if request.Preview != nil && request.Approval != nil {
		return execution.ErrRepositoryConflict
	}
	if request.Audit.Type == execution.AuditPreviewEvaluated && request.Evaluation == nil {
		return execution.ErrRepositoryConflict
	}
	if err := request.Audit.Verify(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin execution commit: %w", err)
	}
	defer tx.Rollback()

	currentHead, currentSequence, err := loadAuditHead(ctx, tx)
	if err != nil {
		return err
	}
	if request.ExpectedAuditHead != currentHead || request.ExpectedAuditSequence != currentSequence ||
		request.Audit.PreviousHash != currentHead || request.Audit.Sequence != currentSequence+1 {
		return execution.ErrRepositoryConflict
	}
	if request.Preview != nil {
		if err := insertRepositoryPreview(ctx, tx, *request.Preview); err != nil {
			return err
		}
	}
	if request.Approval != nil {
		if err := insertRepositoryApproval(ctx, tx, *request.Approval); err != nil {
			return err
		}
	}
	if err := insertRepositoryAudit(ctx, tx, request); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		if isConstraintError(err) {
			return execution.ErrRepositoryConflict
		}
		return fmt.Errorf("commit execution repository: %w", err)
	}
	return nil
}

type auditHeadQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadAuditHead(ctx context.Context, queryer auditHeadQuerier) (string, uint64, error) {
	var hash string
	var sequence uint64
	err := queryer.QueryRowContext(ctx, `SELECT entry_hash, sequence FROM execution_audit_log ORDER BY sequence DESC LIMIT 1`).
		Scan(&hash, &sequence)
	if errors.Is(err, sql.ErrNoRows) {
		return execution.GenesisAuditHash, 0, nil
	}
	if err != nil {
		return "", 0, fmt.Errorf("load execution audit head: %w", err)
	}
	return hash, sequence, nil
}

func insertRepositoryPreview(ctx context.Context, tx *sql.Tx, preview execution.Preview) error {
	if !preview.Decision.Allowed {
		return execution.ErrRepositoryConflict
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM execution_previews WHERE id=? OR idempotency_key=?`,
		preview.ID, preview.IdempotencyKey).Scan(&count); err != nil {
		return fmt.Errorf("check execution preview uniqueness: %w", err)
	}
	if count != 0 {
		return execution.ErrRepositoryConflict
	}
	if err := ensureInstrument(ctx, tx, preview.Intent.ISIN, preview.Intent.ISIN, preview.CreatedAt, preview.CreatedAt); err != nil {
		return err
	}
	body, err := json.Marshal(preview)
	if err != nil {
		return fmt.Errorf("encode execution preview: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO execution_previews(
		id, idempotency_key, isin, side, quantity_i, amount_i, limit_price_i,
		currency, mode, price_as_of, balance_as_of, expires_at, payload_json,
		status, created_at, updated_at, account_id, binding_hash
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?, 'paper', ?, ?, ?, ?, 'pending', ?, ?, ?, ?)`,
		preview.ID, preview.IdempotencyKey, preview.Intent.ISIN, string(preview.Intent.Side),
		int64(preview.Intent.Quantity), int64(preview.Intent.Amount), int64(preview.Intent.LimitPrice),
		preview.Intent.Currency, formatTime(preview.Market.ObservedAt), formatTime(preview.Balance.ObservedAt),
		formatTime(preview.ExpiresAt), string(body), formatTime(preview.CreatedAt),
		formatTime(preview.CreatedAt), preview.AccountID, preview.BindingHash)
	if err != nil {
		if isConstraintError(err) {
			return execution.ErrRepositoryConflict
		}
		return fmt.Errorf("insert execution preview: %w", err)
	}
	return nil
}

func insertRepositoryApproval(ctx context.Context, tx *sql.Tx, approval execution.Approval) error {
	var previewExpiry string
	if err := tx.QueryRowContext(ctx, `SELECT expires_at FROM execution_previews WHERE id=?`, approval.PreviewID).
		Scan(&previewExpiry); errors.Is(err, sql.ErrNoRows) {
		return execution.ErrPreviewNotFound
	} else if err != nil {
		return fmt.Errorf("load approval preview: %w", err)
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM execution_approvals WHERE preview_id=?`, approval.PreviewID).
		Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return execution.ErrRepositoryConflict
	}
	body, err := json.Marshal(approval)
	if err != nil {
		return fmt.Errorf("encode execution approval: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO execution_approvals(
		id, preview_id, challenge_hash, approved_by, approved_at, expires_at,
		metadata_json, approval_json
	) VALUES(?, ?, ?, ?, ?, ?, '{}', ?)`, approval.ID, approval.PreviewID,
		approval.ChallengeHash, approval.Approver, formatTime(approval.ApprovedAt), previewExpiry, string(body))
	if err != nil {
		if isConstraintError(err) {
			return execution.ErrRepositoryConflict
		}
		return fmt.Errorf("insert execution approval: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE execution_previews SET status='approved', updated_at=? WHERE id=? AND status='pending'`,
		formatTime(approval.ApprovedAt), approval.PreviewID); err != nil {
		return fmt.Errorf("mark execution preview approved: %w", err)
	}
	return nil
}

func insertRepositoryAudit(ctx context.Context, tx *sql.Tx, request execution.CommitRequest) error {
	event := request.Audit
	if request.Evaluation != nil {
		if event.SubjectID != request.Evaluation.ID || event.DataHash != request.Evaluation.BindingHash ||
			event.Type != execution.AuditPreviewEvaluated {
			return execution.ErrRepositoryConflict
		}
	}
	if request.Preview != nil {
		if request.Evaluation == nil || request.Evaluation.ID != request.Preview.ID || !request.Preview.Decision.Allowed {
			return execution.ErrRepositoryConflict
		}
	}
	if request.Approval != nil {
		if event.SubjectID != request.Approval.ID || event.DataHash != request.Approval.Hash ||
			event.Type != execution.AuditApprovalRecorded {
			return execution.ErrRepositoryConflict
		}
	}
	body, err := json.Marshal(event)
	if request.Evaluation != nil {
		body, err = json.Marshal(struct {
			Event      execution.AuditEvent `json:"event"`
			Evaluation execution.Preview    `json:"evaluation"`
		}{Event: event, Evaluation: *request.Evaluation})
	}
	if err != nil {
		return fmt.Errorf("encode execution audit event: %w", err)
	}
	previewID := ""
	if request.Preview != nil {
		previewID = request.Preview.ID
	} else if request.Evaluation != nil {
		previewID = request.Evaluation.ID
	} else if request.Approval != nil {
		previewID = request.Approval.PreviewID
	}
	id := "audit:" + strings.TrimPrefix(event.Hash, "sha256:")
	_, err = tx.ExecContext(ctx, `INSERT INTO execution_audit_log(
		sequence, id, preview_id, event_type, payload_json, previous_hash, entry_hash,
		created_at, account_id, subject_id, data_hash
	) VALUES(?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?)`, event.Sequence, id,
		previewID, string(event.Type), string(body), event.PreviousHash, event.Hash,
		formatTime(event.OccurredAt), event.AccountID, event.SubjectID, event.DataHash)
	if err != nil {
		if isConstraintError(err) {
			return execution.ErrRepositoryConflict
		}
		return fmt.Errorf("insert execution audit event: %w", err)
	}
	return nil
}

func isConstraintError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "constraint") || strings.Contains(message, "unique")
}
