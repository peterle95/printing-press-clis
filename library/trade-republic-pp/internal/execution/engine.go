package execution

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"trade-republic-pp-cli/internal/instruments"
	"trade-republic-pp-cli/internal/money"
)

var (
	ErrPreviewIntegrity          = errors.New("execution preview integrity check failed")
	ErrPreviewDenied             = errors.New("execution preview was denied by risk policy")
	ErrPreviewExpired            = errors.New("execution preview has expired")
	ErrApprovalTooEarly          = errors.New("approval predates execution preview")
	ErrApprovalChallengeMismatch = errors.New("typed approval challenge does not match exactly")
	ErrApproverRequired          = errors.New("approval requires an approver identity")
	ErrApprovalAlreadyRecorded   = errors.New("execution preview already has an approval")
	ErrApprovalTimeRequired      = errors.New("approval time is required")
	ErrCurrentPolicyChanged      = errors.New("current execution policy does not match the preview policy")
	ErrApprovalIntegrity         = errors.New("execution approval integrity check failed")
)

type Engine struct {
	repository Repository
}

func NewEngine(repository Repository) (*Engine, error) {
	if repository == nil {
		return nil, ErrRepositoryRequired
	}
	return &Engine{repository: repository}, nil
}

type PreviewRequest struct {
	AccountID            string
	Intent               OrderIntent
	Policy               Policy
	Market               MarketSnapshot
	Balance              BalanceSnapshot
	Now                  time.Time
	Nonce                string
	ClientIdempotencyKey string
}

type ApprovalRequest struct {
	PreviewID      string
	TypedChallenge string
	Approver       string
	Now            time.Time
	Policy         Policy
}

// CreatePreview normalizes and hashes the request, loads persisted reservation
// state, evaluates every risk check, and appends one audit event. Only an
// allowed preview is persisted as a reservation; denied evaluations are audit
// records only.
func (engine *Engine) CreatePreview(ctx context.Context, request PreviewRequest) (Preview, error) {
	accountID := strings.TrimSpace(request.AccountID)
	intent := normalizeIntent(request.Intent)
	policy := normalizePolicy(request.Policy)
	market := normalizeMarket(request.Market)
	balance := normalizeBalance(request.Balance)
	createdAt := request.Now.UTC()
	expiresAt := createdAt
	if policy.PreviewTTL > 0 {
		expiresAt = createdAt.Add(policy.PreviewTTL)
	}

	payloadHash, err := canonicalPayloadHash(intent)
	if err != nil {
		return Preview{}, err
	}
	policyHash, err := canonicalPolicyHash(policy)
	if err != nil {
		return Preview{}, err
	}
	clientKey := strings.TrimSpace(request.ClientIdempotencyKey)
	idempotencyKey, err := canonicalIdempotencyKey(
		accountID,
		intent,
		policyHash,
		createdAt,
		clientKey,
	)
	if err != nil {
		return Preview{}, err
	}

	state, err := engine.repository.LoadRiskState(ctx, RiskStateQuery{
		AccountID:      accountID,
		TradingDay:     createdAt.Format("2006-01-02"),
		Currency:       intent.Currency,
		ISIN:           intent.ISIN,
		AsOf:           createdAt,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return Preview{}, err
	}

	decision := evaluateRisk(riskInput{
		AccountID:            accountID,
		Intent:               intent,
		Policy:               policy,
		Market:               market,
		Balance:              balance,
		State:                state,
		Now:                  createdAt,
		Nonce:                request.Nonce,
		ClientIdempotencyKey: clientKey,
	})
	preview := Preview{
		AccountID:            accountID,
		Intent:               intent,
		Policy:               policy,
		Market:               market,
		Balance:              balance,
		RiskState:            state,
		Decision:             decision,
		CreatedAt:            createdAt,
		ExpiresAt:            expiresAt,
		Nonce:                request.Nonce,
		PayloadHash:          payloadHash,
		PolicyHash:           policyHash,
		ClientIdempotencyKey: clientKey,
		IdempotencyKey:       idempotencyKey,
	}
	preview.BindingHash, err = canonicalPreviewBindingHash(preview)
	if err != nil {
		return Preview{}, err
	}
	preview.ID = previewID(preview.BindingHash)
	preview.ApprovalChallenge = approvalChallenge(preview)

	event, err := NewAuditEvent(
		state.AuditSequence+1,
		AuditPreviewEvaluated,
		createdAt,
		accountID,
		preview.ID,
		preview.BindingHash,
		state.AuditHead,
	)
	if err != nil {
		return preview, err
	}
	commit := CommitRequest{
		ExpectedAuditHead:     normalizedAuditHead(state.AuditHead),
		ExpectedAuditSequence: state.AuditSequence,
		Evaluation:            &preview,
		Audit:                 event,
	}
	if preview.Decision.Allowed {
		commit.Preview = &preview
	}
	if err := engine.repository.Commit(ctx, commit); err != nil {
		return preview, err
	}
	return preview, nil
}

// Approve loads the persisted preview and requires the exact challenge. It
// creates an immutable paper approval only while the bound preview is allowed,
// intact, and unexpired.
func (engine *Engine) Approve(ctx context.Context, request ApprovalRequest) (Approval, error) {
	stored, err := engine.repository.LoadPreview(ctx, strings.TrimSpace(request.PreviewID))
	if err != nil {
		return Approval{}, err
	}
	if err := verifyPreviewIntegrity(stored.Preview); err != nil {
		return Approval{}, err
	}
	if stored.Approval != nil {
		return Approval{}, ErrApprovalAlreadyRecorded
	}
	if err := verifyCurrentPolicy(stored.Preview, request.Policy); err != nil {
		return Approval{}, err
	}
	if !stored.Preview.Decision.Allowed {
		return Approval{}, ErrPreviewDenied
	}
	now := request.Now.UTC()
	if now.IsZero() {
		return Approval{}, ErrApprovalTimeRequired
	}
	if now.Before(stored.Preview.CreatedAt) {
		return Approval{}, ErrApprovalTooEarly
	}
	if !now.Before(stored.Preview.ExpiresAt) {
		return Approval{}, ErrPreviewExpired
	}
	expectedChallenge := stored.Preview.ApprovalChallenge
	if subtle.ConstantTimeCompare([]byte(request.TypedChallenge), []byte(expectedChallenge)) != 1 {
		return Approval{}, ErrApprovalChallengeMismatch
	}
	approver := strings.TrimSpace(request.Approver)
	if approver == "" {
		return Approval{}, ErrApproverRequired
	}

	approval := Approval{
		PreviewID:      stored.Preview.ID,
		PreviewHash:    stored.Preview.BindingHash,
		IdempotencyKey: stored.Preview.IdempotencyKey,
		Approver:       approver,
		ApprovedAt:     now,
		ChallengeHash:  digestString(expectedChallenge),
	}
	approval.Hash, err = canonicalApprovalHash(approval)
	if err != nil {
		return Approval{}, err
	}
	approval.ID = "approval:" + strings.TrimPrefix(approval.Hash, hashPrefix)

	event, err := NewAuditEvent(
		stored.AuditSequence+1,
		AuditApprovalRecorded,
		now,
		stored.Preview.AccountID,
		approval.ID,
		approval.Hash,
		stored.AuditHead,
	)
	if err != nil {
		return Approval{}, err
	}
	if err := engine.repository.Commit(ctx, CommitRequest{
		ExpectedAuditHead:     normalizedAuditHead(stored.AuditHead),
		ExpectedAuditSequence: stored.AuditSequence,
		Approval:              &approval,
		Audit:                 event,
	}); err != nil {
		return Approval{}, err
	}
	return approval, nil
}

type riskInput struct {
	AccountID            string
	Intent               OrderIntent
	Policy               Policy
	Market               MarketSnapshot
	Balance              BalanceSnapshot
	State                RiskState
	Now                  time.Time
	Nonce                string
	ClientIdempotencyKey string
}

func evaluateRisk(input riskInput) RiskDecision {
	decision := RiskDecision{
		Reasons:        []RiskReason{},
		EvaluatedAt:    input.Now.UTC(),
		ReservedBefore: input.State.ReservedExposure,
	}
	add := func(code ReasonCode, field, message string) {
		decision.Reasons = append(decision.Reasons, RiskReason{Code: code, Field: field, Message: message})
	}

	if input.AccountID == "" {
		add(ReasonInvalidAccount, "account_id", "account ID is required")
	}
	if input.Now.IsZero() {
		add(ReasonInvalidEvaluationTime, "now", "evaluation time is required")
	}
	if !validNonce(input.Nonce) {
		add(ReasonInvalidNonce, "nonce", "nonce must contain 1-128 safe ASCII characters")
	}
	if input.ClientIdempotencyKey != "" && !validNonce(input.ClientIdempotencyKey) {
		add(ReasonInvalidIdempotencyKey, "idempotency_key", "idempotency key must contain 1-128 safe ASCII characters")
	}
	if input.Policy.KillSwitch {
		add(ReasonKillSwitchActive, "policy.kill_switch", "global kill switch is active")
	}
	if !input.Policy.PaperTrading {
		add(ReasonPaperTradingRequired, "policy.paper_trading", "execution domain permits paper trading only")
	}
	if input.Policy.ID == "" {
		add(ReasonInvalidPolicy, "policy.id", "policy ID is required")
	}
	if !validCurrency(input.Policy.Currency) {
		add(ReasonInvalidPolicy, "policy.currency", "policy currency must be a three-letter ISO code")
	}
	if input.Policy.MaxOrderValue <= 0 {
		add(ReasonInvalidPolicy, "policy.max_order_value", "maximum order value must be positive")
	}
	if input.Policy.MaxDailyReservedExposure <= 0 {
		add(ReasonInvalidPolicy, "policy.max_daily_reserved_exposure", "maximum daily reserved exposure must be positive")
	}
	if input.Policy.MaxMarketAge <= 0 {
		add(ReasonInvalidPolicy, "policy.max_market_age", "maximum market age must be positive")
	}
	if input.Policy.MaxBalanceAge <= 0 {
		add(ReasonInvalidPolicy, "policy.max_balance_age", "maximum balance age must be positive")
	}
	if input.Policy.PreviewTTL <= 0 {
		add(ReasonInvalidPolicy, "policy.preview_ttl", "preview TTL must be positive")
	}
	if len(input.Policy.AllowedISINs) == 0 {
		add(ReasonInvalidPolicy, "policy.allowed_isins", "at least one exact ISIN must be allowed")
	}
	for index, isin := range input.Policy.AllowedISINs {
		if err := instruments.ValidateISIN(isin); err != nil {
			add(ReasonInvalidPolicy, fmt.Sprintf("policy.allowed_isins[%d]", index), err.Error())
		}
	}

	if input.Intent.Side != SideBuy && input.Intent.Side != SideSell {
		add(ReasonInvalidSide, "intent.side", "side must be buy or sell")
	}
	if input.Intent.OrderType != OrderTypeLimit {
		add(ReasonLimitOrderRequired, "intent.order_type", "only limit orders are permitted")
	}
	if err := instruments.ValidateISIN(input.Intent.ISIN); err != nil {
		add(ReasonInvalidISIN, "intent.isin", err.Error())
	}
	if !containsExact(input.Policy.AllowedISINs, input.Intent.ISIN) {
		add(ReasonInstrumentNotWhitelisted, "intent.isin", "ISIN is not in the exact policy whitelist")
	}
	if input.Intent.Quantity <= 0 {
		add(ReasonInvalidQuantity, "intent.quantity", "quantity must be positive")
	}
	if input.Intent.LimitPrice <= 0 {
		add(ReasonInvalidLimitPrice, "intent.limit_price", "limit price must be positive")
	}
	if input.Intent.Amount <= 0 {
		add(ReasonInvalidAmount, "intent.amount", "amount must be positive")
	} else {
		decision.OrderExposure = input.Intent.Amount
	}
	if !validCurrency(input.Intent.Currency) {
		add(ReasonInvalidCurrency, "intent.currency", "currency must be a three-letter ISO code")
	} else if input.Intent.Currency != input.Policy.Currency {
		add(ReasonInvalidCurrency, "intent.currency", "intent currency does not match policy currency")
	}
	if input.Intent.Quantity > 0 && input.Intent.LimitPrice > 0 {
		expected, multiplicationOK := checkedMultiply(input.Intent.Quantity, input.Intent.LimitPrice)
		if !multiplicationOK {
			add(ReasonDecimalOverflow, "intent.amount", "quantity multiplied by limit price overflows money.Decimal")
		} else if expected != input.Intent.Amount {
			add(ReasonAmountMismatch, "intent.amount", fmt.Sprintf("amount must equal quantity times limit price (%s)", expected.String()))
		}
	}

	if input.State.ReservedExposure < 0 {
		add(ReasonInvalidReservedState, "risk_state.reserved_exposure", "reserved exposure cannot be negative")
	}
	if input.State.ReservedBuyCash < 0 {
		add(ReasonInvalidReservedState, "risk_state.reserved_buy_cash", "reserved buy cash cannot be negative")
	}
	if input.State.ReservedSellQuantity < 0 {
		add(ReasonInvalidReservedState, "risk_state.reserved_sell_quantity", "reserved sell quantity cannot be negative")
	}
	if input.Intent.Amount > 0 && input.Policy.MaxOrderValue > 0 && input.Intent.Amount > input.Policy.MaxOrderValue {
		add(ReasonMaxOrderExceeded, "intent.amount", "order exposure exceeds the per-order maximum")
	}
	if input.Intent.Amount > 0 && input.State.ReservedExposure >= 0 {
		reservedAfter, ok := checkedAdd(input.State.ReservedExposure, input.Intent.Amount)
		if !ok {
			add(ReasonDecimalOverflow, "risk_state.reserved_exposure", "daily reserved exposure overflows money.Decimal")
		} else {
			decision.ReservedAfter = reservedAfter
			if input.Policy.MaxDailyReservedExposure > 0 && reservedAfter > input.Policy.MaxDailyReservedExposure {
				add(ReasonMaxDailyReservedExceeded, "intent.amount", "order plus existing reservations exceeds the daily maximum")
			}
		}
	} else {
		decision.ReservedAfter = input.State.ReservedExposure
	}

	if input.Market.ISIN != input.Intent.ISIN {
		add(ReasonMarketISINMismatch, "market.isin", "market snapshot ISIN does not match the order")
	}
	if input.Market.Currency != input.Intent.Currency {
		add(ReasonMarketCurrencyMismatch, "market.currency", "market snapshot currency does not match the order")
	}
	if input.Market.Price <= 0 {
		add(ReasonInvalidMarketPrice, "market.price", "market price must be positive")
	}
	checkFreshness(
		input.Now,
		input.Market.ObservedAt,
		input.Policy.MaxMarketAge,
		ReasonMarketSnapshotMissing,
		ReasonMarketSnapshotInFuture,
		ReasonMarketSnapshotStale,
		"market.observed_at",
		add,
	)

	if input.Balance.AccountID != input.AccountID {
		add(ReasonBalanceAccountMismatch, "balance.account_id", "balance snapshot account does not match the order")
	}
	if input.Balance.Currency != input.Intent.Currency {
		add(ReasonBalanceCurrencyMismatch, "balance.currency", "balance snapshot currency does not match the order")
	}
	if input.Balance.AvailableCash < 0 {
		add(ReasonInvalidBalance, "balance.available_cash", "available cash cannot be negative")
	}
	if input.Balance.AvailableQuantity < 0 {
		add(ReasonInvalidBalance, "balance.available_quantity", "available quantity cannot be negative")
	}
	checkFreshness(
		input.Now,
		input.Balance.ObservedAt,
		input.Policy.MaxBalanceAge,
		ReasonBalanceSnapshotMissing,
		ReasonBalanceSnapshotInFuture,
		ReasonBalanceSnapshotStale,
		"balance.observed_at",
		add,
	)

	if input.Intent.Side == SideBuy && input.Intent.Amount > 0 && input.State.ReservedBuyCash >= 0 {
		cashRequired, ok := checkedAdd(input.State.ReservedBuyCash, input.Intent.Amount)
		if !ok {
			add(ReasonDecimalOverflow, "risk_state.reserved_buy_cash", "required buy cash overflows money.Decimal")
		} else {
			decision.BuyCashRequired = cashRequired
			if cashRequired > input.Balance.AvailableCash {
				add(ReasonInsufficientCash, "balance.available_cash", "available cash does not cover this order and existing buy reservations")
			}
		}
	}
	if input.Intent.Side == SideSell && input.Intent.Quantity > 0 && input.State.ReservedSellQuantity >= 0 {
		quantityRequired, ok := checkedAdd(input.State.ReservedSellQuantity, input.Intent.Quantity)
		if !ok {
			add(ReasonDecimalOverflow, "risk_state.reserved_sell_quantity", "required sell quantity overflows money.Decimal")
		} else if quantityRequired > input.Balance.AvailableQuantity {
			add(ReasonInsufficientQuantity, "balance.available_quantity", "available quantity does not cover this order and existing sell reservations")
		}
	}
	if input.State.IdempotencyExists {
		add(ReasonDuplicateIdempotency, "idempotency_key", "an allowed preview already reserves this idempotency key")
	}

	decision.Allowed = len(decision.Reasons) == 0
	return decision
}

func verifyCurrentPolicy(preview Preview, policy Policy) error {
	policyHash, err := canonicalPolicyHash(policy)
	if err != nil {
		return err
	}
	if policyHash != preview.PolicyHash {
		return ErrCurrentPolicyChanged
	}
	policy = normalizePolicy(policy)
	if policy.KillSwitch || !policy.PaperTrading {
		return ErrPreviewDenied
	}
	return nil
}

func VerifyApprovedPreviewForExport(stored StoredPreview, policy Policy, now time.Time) error {
	if err := verifyPreviewIntegrity(stored.Preview); err != nil {
		return err
	}
	if stored.Approval == nil {
		return ErrApprovalIntegrity
	}
	if err := verifyApprovalIntegrity(stored.Preview, *stored.Approval); err != nil {
		return err
	}
	if err := verifyCurrentPolicy(stored.Preview, policy); err != nil {
		return err
	}
	now = now.UTC()
	if now.IsZero() || !now.Before(stored.Preview.ExpiresAt) {
		return ErrPreviewExpired
	}
	return nil
}

func verifyApprovalIntegrity(preview Preview, approval Approval) error {
	if approval.PreviewID != preview.ID || approval.PreviewHash != preview.BindingHash || approval.IdempotencyKey != preview.IdempotencyKey {
		return ErrApprovalIntegrity
	}
	if approval.ChallengeHash != digestString(preview.ApprovalChallenge) {
		return ErrApprovalIntegrity
	}
	expected, err := canonicalApprovalHash(approval)
	if err != nil || expected != approval.Hash {
		return ErrApprovalIntegrity
	}
	if approval.ApprovedAt.Before(preview.CreatedAt) || !approval.ApprovedAt.Before(preview.ExpiresAt) {
		return ErrApprovalIntegrity
	}
	return nil
}

func checkFreshness(
	now time.Time,
	observedAt time.Time,
	maximumAge time.Duration,
	missingCode ReasonCode,
	futureCode ReasonCode,
	staleCode ReasonCode,
	field string,
	add func(ReasonCode, string, string),
) {
	if observedAt.IsZero() {
		add(missingCode, field, "snapshot observation time is required")
		return
	}
	if now.IsZero() {
		return
	}
	if observedAt.After(now) {
		add(futureCode, field, "snapshot observation time is in the future")
		return
	}
	if maximumAge > 0 && now.Sub(observedAt) > maximumAge {
		add(staleCode, field, "snapshot is older than policy permits")
	}
}

func checkedAdd(left, right money.Decimal) (money.Decimal, bool) {
	sum := new(big.Int).Add(big.NewInt(int64(left)), big.NewInt(int64(right)))
	if !sum.IsInt64() {
		return 0, false
	}
	return money.Decimal(sum.Int64()), true
}

func checkedMultiply(left, right money.Decimal) (money.Decimal, bool) {
	product := new(big.Int).Mul(big.NewInt(int64(left)), big.NewInt(int64(right)))
	scale := big.NewInt(money.Scale)
	quotient := new(big.Int)
	remainder := new(big.Int)
	quotient.QuoRem(product, scale, remainder)
	absRemainder := new(big.Int).Abs(remainder)
	if new(big.Int).Lsh(absRemainder, 1).Cmp(scale) >= 0 {
		if product.Sign() < 0 {
			quotient.Sub(quotient, big.NewInt(1))
		} else {
			quotient.Add(quotient, big.NewInt(1))
		}
	}
	if !quotient.IsInt64() {
		return 0, false
	}
	return money.Decimal(quotient.Int64()), true
}

func validCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validNonce(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '.' || character == '_' || character == ':' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func containsExact(values []string, target string) bool {
	index := sortSearchStrings(values, target)
	return index < len(values) && values[index] == target
}

func sortSearchStrings(values []string, target string) int {
	low, high := 0, len(values)
	for low < high {
		middle := int(uint(low+high) >> 1)
		if values[middle] < target {
			low = middle + 1
		} else {
			high = middle
		}
	}
	return low
}

func normalizedAuditHead(value string) string {
	if value == "" {
		return GenesisAuditHash
	}
	return value
}
