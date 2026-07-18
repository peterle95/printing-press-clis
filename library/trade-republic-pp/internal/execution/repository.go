package execution

import (
	"context"
	"errors"
	"time"

	"trade-republic-pp-cli/internal/money"
)

var (
	ErrRepositoryRequired = errors.New("execution repository is required")
	ErrRepositoryConflict = errors.New("execution repository conflict")
	ErrPreviewNotFound    = errors.New("execution preview not found")
)

// RiskStateQuery asks the repository for reservations and duplicate state at
// one deterministic instant. TradingDay is always the UTC YYYY-MM-DD date.
type RiskStateQuery struct {
	AccountID      string
	TradingDay     string
	Currency       string
	ISIN           string
	AsOf           time.Time
	IdempotencyKey string
}

// RiskState is the complete persisted state used by one risk evaluation.
// AuditHead and AuditSequence are persistence metadata and are intentionally
// excluded from JSON and preview binding hashes.
type RiskState struct {
	ReservedExposure     money.Decimal `json:"reserved_exposure"`
	ReservedBuyCash      money.Decimal `json:"reserved_buy_cash"`
	ReservedSellQuantity money.Decimal `json:"reserved_sell_quantity"`
	IdempotencyExists    bool          `json:"idempotency_exists"`
	AuditHead            string        `json:"-"`
	AuditSequence        uint64        `json:"-"`
}

// StoredPreview includes the current account audit head so approval storage
// can atomically extend the same append-only chain.
type StoredPreview struct {
	Preview       Preview
	Approval      *Approval
	AuditHead     string
	AuditSequence uint64
}

// CommitRequest is an atomic append. Evaluation is present for every preview
// attempt, including denied attempts, so risk decisions remain reconstructable.
// Preview is present only for an allowed reservation.
type CommitRequest struct {
	ExpectedAuditHead     string
	ExpectedAuditSequence uint64
	Evaluation            *Preview
	Preview               *Preview
	Approval              *Approval
	Audit                 AuditEvent
}

// Repository is the persistence seam for a future SQLite adapter. Commit must
// atomically reject stale audit heads, duplicate risk-approved idempotency
// keys, duplicate preview IDs, and duplicate approvals with
// ErrRepositoryConflict.
type Repository interface {
	LoadRiskState(context.Context, RiskStateQuery) (RiskState, error)
	LoadPreview(context.Context, string) (StoredPreview, error)
	Commit(context.Context, CommitRequest) error
}
