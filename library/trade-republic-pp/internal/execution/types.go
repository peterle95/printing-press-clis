// Package execution implements deterministic, paper-only order previews,
// approvals, and audit records. It deliberately contains no broker submission
// interface.
package execution

import (
	"time"

	"trade-republic-pp-cli/internal/money"
)

type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

type OrderType string

const (
	OrderTypeLimit OrderType = "limit"
)

// Policy is the complete deterministic policy applied to an order preview.
// PaperTrading must be true; this module never permits a live-trading policy.
type Policy struct {
	ID                       string        `json:"id"`
	KillSwitch               bool          `json:"kill_switch"`
	PaperTrading             bool          `json:"paper_trading"`
	Currency                 string        `json:"currency"`
	AllowedISINs             []string      `json:"allowed_isins"`
	MaxOrderValue            money.Decimal `json:"max_order_value"`
	MaxDailyReservedExposure money.Decimal `json:"max_daily_reserved_exposure"`
	MaxMarketAge             time.Duration `json:"max_market_age"`
	MaxBalanceAge            time.Duration `json:"max_balance_age"`
	PreviewTTL               time.Duration `json:"preview_ttl"`
}

// OrderIntent is an immutable paper-order proposal. Amount must exactly equal
// Quantity multiplied by LimitPrice at money.Decimal precision.
type OrderIntent struct {
	Side       Side          `json:"side"`
	OrderType  OrderType     `json:"order_type"`
	ISIN       string        `json:"isin"`
	Quantity   money.Decimal `json:"quantity"`
	LimitPrice money.Decimal `json:"limit_price"`
	Amount     money.Decimal `json:"amount"`
	Currency   string        `json:"currency"`
}

// MarketSnapshot is the price observation used by the deterministic checks.
type MarketSnapshot struct {
	ISIN       string        `json:"isin"`
	Price      money.Decimal `json:"price"`
	Currency   string        `json:"currency"`
	ObservedAt time.Time     `json:"observed_at"`
	Source     string        `json:"source,omitempty"`
}

// BalanceSnapshot contains the cash and instrument quantity available before
// accounting for other locally reserved paper previews.
type BalanceSnapshot struct {
	AccountID         string        `json:"account_id"`
	Currency          string        `json:"currency"`
	AvailableCash     money.Decimal `json:"available_cash"`
	AvailableQuantity money.Decimal `json:"available_quantity"`
	ObservedAt        time.Time     `json:"observed_at"`
	Source            string        `json:"source,omitempty"`
}

type ReasonCode string

const (
	ReasonInvalidAccount           ReasonCode = "invalid_account"
	ReasonKillSwitchActive         ReasonCode = "kill_switch_active"
	ReasonPaperTradingRequired     ReasonCode = "paper_trading_required"
	ReasonInvalidPolicy            ReasonCode = "invalid_policy"
	ReasonInvalidSide              ReasonCode = "invalid_side"
	ReasonLimitOrderRequired       ReasonCode = "limit_order_required"
	ReasonInvalidISIN              ReasonCode = "invalid_isin"
	ReasonInstrumentNotWhitelisted ReasonCode = "instrument_not_whitelisted"
	ReasonInvalidQuantity          ReasonCode = "invalid_quantity"
	ReasonInvalidLimitPrice        ReasonCode = "invalid_limit_price"
	ReasonInvalidAmount            ReasonCode = "invalid_amount"
	ReasonInvalidCurrency          ReasonCode = "invalid_currency"
	ReasonAmountMismatch           ReasonCode = "amount_mismatch"
	ReasonDecimalOverflow          ReasonCode = "decimal_overflow"
	ReasonMaxOrderExceeded         ReasonCode = "max_order_exceeded"
	ReasonMaxDailyReservedExceeded ReasonCode = "max_daily_reserved_exceeded"
	ReasonInvalidReservedState     ReasonCode = "invalid_reserved_state"
	ReasonMarketISINMismatch       ReasonCode = "market_isin_mismatch"
	ReasonMarketCurrencyMismatch   ReasonCode = "market_currency_mismatch"
	ReasonInvalidMarketPrice       ReasonCode = "invalid_market_price"
	ReasonMarketSnapshotMissing    ReasonCode = "market_snapshot_missing"
	ReasonMarketSnapshotInFuture   ReasonCode = "market_snapshot_in_future"
	ReasonMarketSnapshotStale      ReasonCode = "market_snapshot_stale"
	ReasonBalanceAccountMismatch   ReasonCode = "balance_account_mismatch"
	ReasonBalanceCurrencyMismatch  ReasonCode = "balance_currency_mismatch"
	ReasonInvalidBalance           ReasonCode = "invalid_balance"
	ReasonBalanceSnapshotMissing   ReasonCode = "balance_snapshot_missing"
	ReasonBalanceSnapshotInFuture  ReasonCode = "balance_snapshot_in_future"
	ReasonBalanceSnapshotStale     ReasonCode = "balance_snapshot_stale"
	ReasonInsufficientCash         ReasonCode = "insufficient_cash"
	ReasonInsufficientQuantity     ReasonCode = "insufficient_quantity"
	ReasonDuplicateIdempotency     ReasonCode = "duplicate_idempotency"
	ReasonInvalidIdempotencyKey    ReasonCode = "invalid_idempotency_key"
	ReasonInvalidNonce             ReasonCode = "invalid_nonce"
	ReasonInvalidEvaluationTime    ReasonCode = "invalid_evaluation_time"
)

type RiskReason struct {
	Code    ReasonCode `json:"code"`
	Field   string     `json:"field,omitempty"`
	Message string     `json:"message"`
}

// RiskDecision contains every failed check in deterministic evaluation order.
// Reasons is empty exactly when Allowed is true.
type RiskDecision struct {
	Allowed         bool          `json:"allowed"`
	Reasons         []RiskReason  `json:"reasons"`
	OrderExposure   money.Decimal `json:"order_exposure"`
	ReservedBefore  money.Decimal `json:"reserved_before"`
	ReservedAfter   money.Decimal `json:"reserved_after"`
	BuyCashRequired money.Decimal `json:"buy_cash_required"`
	EvaluatedAt     time.Time     `json:"evaluated_at"`
}

func (d RiskDecision) HasReason(code ReasonCode) bool {
	for _, reason := range d.Reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

// Preview binds the normalized payload, policy, observations, repository risk
// state, expiry, and nonce to cryptographic hashes and an exact challenge.
type Preview struct {
	ID                   string          `json:"id"`
	AccountID            string          `json:"account_id"`
	Intent               OrderIntent     `json:"intent"`
	Policy               Policy          `json:"policy"`
	Market               MarketSnapshot  `json:"market"`
	Balance              BalanceSnapshot `json:"balance"`
	RiskState            RiskState       `json:"risk_state"`
	Decision             RiskDecision    `json:"decision"`
	CreatedAt            time.Time       `json:"created_at"`
	ExpiresAt            time.Time       `json:"expires_at"`
	Nonce                string          `json:"nonce"`
	PayloadHash          string          `json:"payload_hash"`
	PolicyHash           string          `json:"policy_hash"`
	ClientIdempotencyKey string          `json:"client_idempotency_key,omitempty"`
	IdempotencyKey       string          `json:"idempotency_key"`
	BindingHash          string          `json:"binding_hash"`
	ApprovalChallenge    string          `json:"approval_challenge"`
}

type Approval struct {
	ID             string    `json:"id"`
	PreviewID      string    `json:"preview_id"`
	PreviewHash    string    `json:"preview_hash"`
	IdempotencyKey string    `json:"idempotency_key"`
	Approver       string    `json:"approver"`
	ApprovedAt     time.Time `json:"approved_at"`
	ChallengeHash  string    `json:"challenge_hash"`
	Hash           string    `json:"hash"`
}
