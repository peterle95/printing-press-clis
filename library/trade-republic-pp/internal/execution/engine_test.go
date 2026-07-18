package execution

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"trade-republic-pp-cli/internal/money"
)

const (
	testAccount = "paper-account"
	testISIN    = "IE00B4L5Y983"
	otherISIN   = "US0378331005"
)

func TestEngineCreatePreviewAllowsSafePaperOrder(t *testing.T) {
	repository := newMemoryRepository()
	engine := mustEngine(t, repository)
	request := safePreviewRequest(testTime())

	preview, err := engine.CreatePreview(context.Background(), request)
	if err != nil {
		t.Fatalf("CreatePreview() error = %v", err)
	}
	if !preview.Decision.Allowed {
		t.Fatalf("CreatePreview() denied safe order: %#v", preview.Decision.Reasons)
	}
	if len(preview.Decision.Reasons) != 0 {
		t.Fatalf("safe preview reasons = %#v, want none", preview.Decision.Reasons)
	}
	if preview.Decision.ReservedAfter != decimal("200") {
		t.Fatalf("ReservedAfter = %s, want 200", preview.Decision.ReservedAfter)
	}
	if !validDigest(preview.PayloadHash) || !validDigest(preview.PolicyHash) ||
		!validDigest(preview.IdempotencyKey) || !validDigest(preview.BindingHash) {
		t.Fatalf("preview hashes are not canonical digests: %#v", preview)
	}
	if err := verifyPreviewIntegrity(preview); err != nil {
		t.Fatalf("verifyPreviewIntegrity() error = %v", err)
	}
	if preview.ApprovalChallenge == "y" || !strings.Contains(preview.ApprovalChallenge, testISIN) {
		t.Fatalf("approval challenge is not an exact order-bound phrase: %q", preview.ApprovalChallenge)
	}
	if len(repository.previews) != 1 {
		t.Fatalf("persisted previews = %d, want 1", len(repository.previews))
	}
	if err := VerifyAuditChain(repository.events); err != nil {
		t.Fatalf("VerifyAuditChain() error = %v", err)
	}
}

func TestEngineCollectsEveryRiskReason(t *testing.T) {
	repository := newMemoryRepository()
	engine := mustEngine(t, repository)
	request := safePreviewRequest(testTime())
	request.Policy.KillSwitch = true
	request.Policy.PaperTrading = false
	request.Policy.AllowedISINs = []string{otherISIN}
	request.Intent.OrderType = OrderType("market")
	request.Intent.Amount = decimal("199")
	request.Market.ISIN = otherISIN
	request.Market.ObservedAt = request.Now.Add(-2 * time.Minute)
	request.Balance.AccountID = "another-account"
	request.Balance.AvailableCash = decimal("1")
	request.Balance.ObservedAt = request.Now.Add(-2 * time.Minute)

	preview, err := engine.CreatePreview(context.Background(), request)
	if err != nil {
		t.Fatalf("CreatePreview() error = %v", err)
	}
	if preview.Decision.Allowed {
		t.Fatal("CreatePreview() allowed an unsafe order")
	}
	for _, code := range []ReasonCode{
		ReasonKillSwitchActive,
		ReasonPaperTradingRequired,
		ReasonLimitOrderRequired,
		ReasonInstrumentNotWhitelisted,
		ReasonAmountMismatch,
		ReasonMarketISINMismatch,
		ReasonMarketSnapshotStale,
		ReasonBalanceAccountMismatch,
		ReasonBalanceSnapshotStale,
		ReasonInsufficientCash,
	} {
		if !preview.Decision.HasReason(code) {
			t.Errorf("risk decision missing reason %q: %#v", code, preview.Decision.Reasons)
		}
	}
	if len(repository.previews) != 0 {
		t.Fatalf("denied preview was persisted as a reservation")
	}
	if len(repository.events) != 1 {
		t.Fatalf("denied evaluation audit events = %d, want 1", len(repository.events))
	}
}

func TestEngineEnforcesValueLimits(t *testing.T) {
	t.Run("per order", func(t *testing.T) {
		repository := newMemoryRepository()
		engine := mustEngine(t, repository)
		request := safePreviewRequest(testTime())
		request.Intent.Quantity = decimal("12")
		request.Intent.Amount = decimal("1200")
		request.Balance.AvailableCash = decimal("10000")

		preview, err := engine.CreatePreview(context.Background(), request)
		if err != nil {
			t.Fatalf("CreatePreview() error = %v", err)
		}
		if !preview.Decision.HasReason(ReasonMaxOrderExceeded) {
			t.Fatalf("reasons = %#v, want max-order failure", preview.Decision.Reasons)
		}
	})

	t.Run("daily reserved", func(t *testing.T) {
		repository := newMemoryRepository()
		repository.baseState.ReservedExposure = decimal("4900")
		engine := mustEngine(t, repository)
		request := safePreviewRequest(testTime())

		preview, err := engine.CreatePreview(context.Background(), request)
		if err != nil {
			t.Fatalf("CreatePreview() error = %v", err)
		}
		if !preview.Decision.HasReason(ReasonMaxDailyReservedExceeded) {
			t.Fatalf("reasons = %#v, want daily-reserved failure", preview.Decision.Reasons)
		}
	})

	t.Run("limits are inclusive", func(t *testing.T) {
		repository := newMemoryRepository()
		repository.baseState.ReservedExposure = decimal("4900")
		engine := mustEngine(t, repository)
		request := safePreviewRequest(testTime())
		request.Policy.MaxOrderValue = decimal("200")
		request.Policy.MaxDailyReservedExposure = decimal("5100")

		preview, err := engine.CreatePreview(context.Background(), request)
		if err != nil {
			t.Fatalf("CreatePreview() error = %v", err)
		}
		if !preview.Decision.Allowed {
			t.Fatalf("order at exact limits denied: %#v", preview.Decision.Reasons)
		}
	})
}

func TestEngineRequiresExactISINWhitelist(t *testing.T) {
	repository := newMemoryRepository()
	engine := mustEngine(t, repository)
	request := safePreviewRequest(testTime())
	request.Policy.AllowedISINs = []string{"VWCE"}

	preview, err := engine.CreatePreview(context.Background(), request)
	if err != nil {
		t.Fatalf("CreatePreview() error = %v", err)
	}
	if !preview.Decision.HasReason(ReasonInvalidPolicy) ||
		!preview.Decision.HasReason(ReasonInstrumentNotWhitelisted) {
		t.Fatalf("ticker whitelist reasons = %#v", preview.Decision.Reasons)
	}
}

func TestEngineRejectsStaleSnapshots(t *testing.T) {
	repository := newMemoryRepository()
	engine := mustEngine(t, repository)
	request := safePreviewRequest(testTime())
	request.Policy.MaxMarketAge = 30 * time.Second
	request.Policy.MaxBalanceAge = 30 * time.Second
	request.Market.ObservedAt = request.Now.Add(-31 * time.Second)
	request.Balance.ObservedAt = request.Now.Add(-31 * time.Second)

	preview, err := engine.CreatePreview(context.Background(), request)
	if err != nil {
		t.Fatalf("CreatePreview() error = %v", err)
	}
	if !preview.Decision.HasReason(ReasonMarketSnapshotStale) ||
		!preview.Decision.HasReason(ReasonBalanceSnapshotStale) {
		t.Fatalf("stale snapshot reasons = %#v", preview.Decision.Reasons)
	}
}

func TestEngineRequiresLimitOrderAndConsistentAmount(t *testing.T) {
	repository := newMemoryRepository()
	engine := mustEngine(t, repository)
	request := safePreviewRequest(testTime())
	request.Intent.OrderType = OrderType("market")
	request.Intent.Amount = decimal("201")

	preview, err := engine.CreatePreview(context.Background(), request)
	if err != nil {
		t.Fatalf("CreatePreview() error = %v", err)
	}
	if !preview.Decision.HasReason(ReasonLimitOrderRequired) ||
		!preview.Decision.HasReason(ReasonAmountMismatch) {
		t.Fatalf("order shape reasons = %#v", preview.Decision.Reasons)
	}
}

func TestEngineEnforcesAvailableBuyBalance(t *testing.T) {
	repository := newMemoryRepository()
	repository.baseState.ReservedBuyCash = decimal("150")
	engine := mustEngine(t, repository)
	request := safePreviewRequest(testTime())
	request.Balance.AvailableCash = decimal("300")

	preview, err := engine.CreatePreview(context.Background(), request)
	if err != nil {
		t.Fatalf("CreatePreview() error = %v", err)
	}
	if !preview.Decision.HasReason(ReasonInsufficientCash) {
		t.Fatalf("reasons = %#v, want insufficient cash", preview.Decision.Reasons)
	}
}

func TestEngineRejectsDuplicateIdempotencyKey(t *testing.T) {
	repository := newMemoryRepository()
	engine := mustEngine(t, repository)
	request := safePreviewRequest(testTime())

	first, err := engine.CreatePreview(context.Background(), request)
	if err != nil {
		t.Fatalf("first CreatePreview() error = %v", err)
	}
	second, err := engine.CreatePreview(context.Background(), request)
	if err != nil {
		t.Fatalf("second CreatePreview() error = %v", err)
	}
	if first.IdempotencyKey != second.IdempotencyKey {
		t.Fatalf("same canonical request generated different idempotency keys")
	}
	if !second.Decision.HasReason(ReasonDuplicateIdempotency) {
		t.Fatalf("duplicate reasons = %#v", second.Decision.Reasons)
	}
	if len(repository.previews) != 1 {
		t.Fatalf("persisted previews = %d, want only original reservation", len(repository.previews))
	}
	if len(repository.events) != 2 {
		t.Fatalf("audit events = %d, want both attempts", len(repository.events))
	}
	if err := VerifyAuditChain(repository.events); err != nil {
		t.Fatalf("VerifyAuditChain() error = %v", err)
	}
}

func TestEngineApprovalChallengeExpiryAndDuplicate(t *testing.T) {
	t.Run("challenge must match exactly", func(t *testing.T) {
		repository := newMemoryRepository()
		engine := mustEngine(t, repository)
		request := safePreviewRequest(testTime())
		preview, err := engine.CreatePreview(context.Background(), request)
		if err != nil {
			t.Fatalf("CreatePreview() error = %v", err)
		}

		_, err = engine.Approve(context.Background(), ApprovalRequest{
			PreviewID:      preview.ID,
			TypedChallenge: preview.ApprovalChallenge + " ",
			Approver:       "operator",
			Now:            request.Now.Add(time.Second),
			Policy:         request.Policy,
		})
		if !errors.Is(err, ErrApprovalChallengeMismatch) {
			t.Fatalf("Approve() error = %v, want challenge mismatch", err)
		}
	})

	t.Run("expiry is exclusive", func(t *testing.T) {
		repository := newMemoryRepository()
		engine := mustEngine(t, repository)
		request := safePreviewRequest(testTime())
		preview, err := engine.CreatePreview(context.Background(), request)
		if err != nil {
			t.Fatalf("CreatePreview() error = %v", err)
		}

		_, err = engine.Approve(context.Background(), ApprovalRequest{
			PreviewID:      preview.ID,
			TypedChallenge: preview.ApprovalChallenge,
			Approver:       "operator",
			Now:            preview.ExpiresAt,
			Policy:         request.Policy,
		})
		if !errors.Is(err, ErrPreviewExpired) {
			t.Fatalf("Approve() error = %v, want expired", err)
		}
	})

	t.Run("successful approval is immutable and unique", func(t *testing.T) {
		repository := newMemoryRepository()
		engine := mustEngine(t, repository)
		request := safePreviewRequest(testTime())
		preview, err := engine.CreatePreview(context.Background(), request)
		if err != nil {
			t.Fatalf("CreatePreview() error = %v", err)
		}
		approvalRequest := ApprovalRequest{
			PreviewID:      preview.ID,
			TypedChallenge: preview.ApprovalChallenge,
			Approver:       "operator",
			Now:            request.Now.Add(time.Second),
			Policy:         request.Policy,
		}

		approval, err := engine.Approve(context.Background(), approvalRequest)
		if err != nil {
			t.Fatalf("Approve() error = %v", err)
		}
		if !validDigest(approval.Hash) || !validDigest(approval.ChallengeHash) {
			t.Fatalf("approval hashes are invalid: %#v", approval)
		}
		if approval.PreviewHash != preview.BindingHash || approval.IdempotencyKey != preview.IdempotencyKey {
			t.Fatalf("approval is not bound to preview: %#v", approval)
		}
		_, err = engine.Approve(context.Background(), approvalRequest)
		if !errors.Is(err, ErrApprovalAlreadyRecorded) {
			t.Fatalf("duplicate Approve() error = %v, want already recorded", err)
		}
		if len(repository.events) != 2 {
			t.Fatalf("audit events = %d, want preview and approval", len(repository.events))
		}
		if err := VerifyAuditChain(repository.events); err != nil {
			t.Fatalf("VerifyAuditChain() error = %v", err)
		}
	})
}

func safePreviewRequest(now time.Time) PreviewRequest {
	return PreviewRequest{
		AccountID: testAccount,
		Intent: OrderIntent{
			Side:       SideBuy,
			OrderType:  OrderTypeLimit,
			ISIN:       testISIN,
			Quantity:   decimal("2"),
			LimitPrice: decimal("100"),
			Amount:     decimal("200"),
			Currency:   "EUR",
		},
		Policy: Policy{
			ID:                       "paper-v1",
			PaperTrading:             true,
			Currency:                 "EUR",
			AllowedISINs:             []string{testISIN},
			MaxOrderValue:            decimal("1000"),
			MaxDailyReservedExposure: decimal("5000"),
			MaxMarketAge:             time.Minute,
			MaxBalanceAge:            time.Minute,
			PreviewTTL:               2 * time.Minute,
		},
		Market: MarketSnapshot{
			ISIN:       testISIN,
			Price:      decimal("99"),
			Currency:   "EUR",
			ObservedAt: now.Add(-5 * time.Second),
			Source:     "fixture",
		},
		Balance: BalanceSnapshot{
			AccountID:         testAccount,
			Currency:          "EUR",
			AvailableCash:     decimal("1000"),
			AvailableQuantity: decimal("10"),
			ObservedAt:        now.Add(-5 * time.Second),
			Source:            "fixture",
		},
		Now:   now,
		Nonce: "nonce-001",
	}
}

func testTime() time.Time {
	return time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
}

func decimal(value string) money.Decimal { return money.MustParse(value) }

func mustEngine(t *testing.T, repository Repository) *Engine {
	t.Helper()
	engine, err := NewEngine(repository)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	return engine
}

type memoryRepository struct {
	baseState RiskState
	previews  map[string]Preview
	approvals map[string]Approval
	events    []AuditEvent
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		previews:  make(map[string]Preview),
		approvals: make(map[string]Approval),
	}
}

func (repository *memoryRepository) LoadRiskState(_ context.Context, query RiskStateQuery) (RiskState, error) {
	state := repository.baseState
	state.AuditHead = GenesisAuditHash
	state.AuditSequence = uint64(len(repository.events))
	if len(repository.events) > 0 {
		state.AuditHead = repository.events[len(repository.events)-1].Hash
	}
	for _, preview := range repository.previews {
		if preview.AccountID != query.AccountID || preview.Intent.Currency != query.Currency {
			continue
		}
		if preview.IdempotencyKey == query.IdempotencyKey {
			state.IdempotencyExists = true
		}
		if preview.CreatedAt.Format("2006-01-02") != query.TradingDay || !query.AsOf.Before(preview.ExpiresAt) {
			continue
		}
		state.ReservedExposure = state.ReservedExposure.Add(preview.Intent.Amount)
		if preview.Intent.Side == SideBuy {
			state.ReservedBuyCash = state.ReservedBuyCash.Add(preview.Intent.Amount)
		}
		if preview.Intent.Side == SideSell && preview.Intent.ISIN == query.ISIN {
			state.ReservedSellQuantity = state.ReservedSellQuantity.Add(preview.Intent.Quantity)
		}
	}
	return state, nil
}

func (repository *memoryRepository) LoadPreview(_ context.Context, previewID string) (StoredPreview, error) {
	preview, ok := repository.previews[previewID]
	if !ok {
		return StoredPreview{}, ErrPreviewNotFound
	}
	stored := StoredPreview{
		Preview:       preview,
		AuditHead:     GenesisAuditHash,
		AuditSequence: uint64(len(repository.events)),
	}
	if len(repository.events) > 0 {
		stored.AuditHead = repository.events[len(repository.events)-1].Hash
	}
	if approval, exists := repository.approvals[previewID]; exists {
		copy := approval
		stored.Approval = &copy
	}
	return stored, nil
}

func (repository *memoryRepository) Commit(_ context.Context, request CommitRequest) error {
	currentHead := GenesisAuditHash
	if len(repository.events) > 0 {
		currentHead = repository.events[len(repository.events)-1].Hash
	}
	if request.ExpectedAuditHead != currentHead || request.ExpectedAuditSequence != uint64(len(repository.events)) {
		return ErrRepositoryConflict
	}
	if request.Audit.PreviousHash != currentHead || request.Audit.Sequence != uint64(len(repository.events)+1) {
		return ErrRepositoryConflict
	}
	if err := request.Audit.Verify(); err != nil {
		return err
	}
	if request.Preview != nil {
		if !request.Preview.Decision.Allowed {
			return ErrRepositoryConflict
		}
		if _, exists := repository.previews[request.Preview.ID]; exists {
			return ErrRepositoryConflict
		}
		for _, existing := range repository.previews {
			if existing.IdempotencyKey == request.Preview.IdempotencyKey {
				return ErrRepositoryConflict
			}
		}
	}
	if request.Approval != nil {
		if _, exists := repository.previews[request.Approval.PreviewID]; !exists {
			return ErrPreviewNotFound
		}
		if _, exists := repository.approvals[request.Approval.PreviewID]; exists {
			return ErrRepositoryConflict
		}
	}

	repository.events = append(repository.events, request.Audit)
	if request.Preview != nil {
		repository.previews[request.Preview.ID] = *request.Preview
	}
	if request.Approval != nil {
		repository.approvals[request.Approval.PreviewID] = *request.Approval
	}
	return nil
}
