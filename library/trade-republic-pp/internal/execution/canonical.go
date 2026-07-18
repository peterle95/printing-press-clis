package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"trade-republic-pp-cli/internal/instruments"
)

const hashPrefix = "sha256:"

func normalizePolicy(policy Policy) Policy {
	policy.ID = strings.TrimSpace(policy.ID)
	policy.Currency = normalizeCurrency(policy.Currency)
	seen := make(map[string]struct{}, len(policy.AllowedISINs))
	allowed := make([]string, 0, len(policy.AllowedISINs))
	for _, value := range policy.AllowedISINs {
		isin := instruments.NormalizeISIN(value)
		if _, exists := seen[isin]; exists {
			continue
		}
		seen[isin] = struct{}{}
		allowed = append(allowed, isin)
	}
	sort.Strings(allowed)
	policy.AllowedISINs = allowed
	return policy
}

func normalizeIntent(intent OrderIntent) OrderIntent {
	intent.Side = Side(strings.ToLower(strings.TrimSpace(string(intent.Side))))
	intent.OrderType = OrderType(strings.ToLower(strings.TrimSpace(string(intent.OrderType))))
	intent.ISIN = instruments.NormalizeISIN(intent.ISIN)
	intent.Currency = normalizeCurrency(intent.Currency)
	return intent
}

func normalizeMarket(snapshot MarketSnapshot) MarketSnapshot {
	snapshot.ISIN = instruments.NormalizeISIN(snapshot.ISIN)
	snapshot.Currency = normalizeCurrency(snapshot.Currency)
	snapshot.ObservedAt = snapshot.ObservedAt.UTC()
	snapshot.Source = strings.TrimSpace(snapshot.Source)
	return snapshot
}

func normalizeBalance(snapshot BalanceSnapshot) BalanceSnapshot {
	snapshot.AccountID = strings.TrimSpace(snapshot.AccountID)
	snapshot.Currency = normalizeCurrency(snapshot.Currency)
	snapshot.ObservedAt = snapshot.ObservedAt.UTC()
	snapshot.Source = strings.TrimSpace(snapshot.Source)
	return snapshot
}

func normalizeCurrency(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func digest(value []byte) string {
	sum := sha256.Sum256(value)
	return hashPrefix + hex.EncodeToString(sum[:])
}

func digestString(value string) string { return digest([]byte(value)) }

func canonicalJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("canonical JSON: %w", err)
	}
	return encoded, nil
}

func canonicalPayloadHash(intent OrderIntent) (string, error) {
	value, err := canonicalJSON(struct {
		Schema string      `json:"schema"`
		Intent OrderIntent `json:"intent"`
	}{
		Schema: "trade-republic-pp/execution-intent/v1",
		Intent: normalizeIntent(intent),
	})
	if err != nil {
		return "", err
	}
	return digest(value), nil
}

func canonicalPolicyHash(policy Policy) (string, error) {
	value, err := canonicalJSON(struct {
		Schema string `json:"schema"`
		Policy Policy `json:"policy"`
	}{
		Schema: "trade-republic-pp/execution-policy/v1",
		Policy: normalizePolicy(policy),
	})
	if err != nil {
		return "", err
	}
	return digest(value), nil
}

func canonicalIdempotencyKey(
	accountID string,
	intent OrderIntent,
	policyHash string,
	createdAt time.Time,
	clientKey string,
) (string, error) {
	value, err := canonicalJSON(struct {
		Schema     string      `json:"schema"`
		AccountID  string      `json:"account_id"`
		Intent     OrderIntent `json:"intent"`
		PolicyHash string      `json:"policy_hash"`
		TradingDay string      `json:"trading_day"`
		ClientKey  string      `json:"client_key,omitempty"`
	}{
		Schema:     "trade-republic-pp/execution-idempotency/v2",
		AccountID:  strings.TrimSpace(accountID),
		Intent:     normalizeIntent(intent),
		PolicyHash: policyHash,
		TradingDay: createdAt.UTC().Format("2006-01-02"),
		ClientKey:  strings.TrimSpace(clientKey),
	})
	if err != nil {
		return "", err
	}
	return digest(value), nil
}

func canonicalPreviewBindingHash(preview Preview) (string, error) {
	riskState := preview.RiskState
	riskState.AuditHead = ""
	riskState.AuditSequence = 0
	value, err := canonicalJSON(struct {
		Schema               string          `json:"schema"`
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
	}{
		Schema:               "trade-republic-pp/execution-preview/v1",
		AccountID:            strings.TrimSpace(preview.AccountID),
		Intent:               normalizeIntent(preview.Intent),
		Policy:               normalizePolicy(preview.Policy),
		Market:               normalizeMarket(preview.Market),
		Balance:              normalizeBalance(preview.Balance),
		RiskState:            riskState,
		Decision:             normalizeDecision(preview.Decision),
		CreatedAt:            preview.CreatedAt.UTC(),
		ExpiresAt:            preview.ExpiresAt.UTC(),
		Nonce:                preview.Nonce,
		PayloadHash:          preview.PayloadHash,
		PolicyHash:           preview.PolicyHash,
		ClientIdempotencyKey: strings.TrimSpace(preview.ClientIdempotencyKey),
		IdempotencyKey:       preview.IdempotencyKey,
	})
	if err != nil {
		return "", err
	}
	return digest(value), nil
}

func normalizeDecision(decision RiskDecision) RiskDecision {
	decision.EvaluatedAt = decision.EvaluatedAt.UTC()
	if decision.Reasons == nil {
		decision.Reasons = []RiskReason{}
	}
	return decision
}

func previewID(bindingHash string) string {
	return "preview:" + strings.TrimPrefix(bindingHash, hashPrefix)
}

func approvalChallenge(preview Preview) string {
	return fmt.Sprintf(
		"APPROVE PAPER ORDER %s %s %s %s @ %s %s TOTAL %s EXPIRES %s NONCE %s",
		preview.ID,
		strings.ToUpper(string(preview.Intent.Side)),
		preview.Intent.ISIN,
		preview.Intent.Quantity.String(),
		preview.Intent.LimitPrice.String(),
		preview.Intent.Currency,
		preview.Intent.Amount.String(),
		preview.ExpiresAt.UTC().Format(time.RFC3339Nano),
		preview.Nonce,
	)
}

func canonicalApprovalHash(approval Approval) (string, error) {
	value, err := canonicalJSON(struct {
		Schema         string    `json:"schema"`
		PreviewID      string    `json:"preview_id"`
		PreviewHash    string    `json:"preview_hash"`
		IdempotencyKey string    `json:"idempotency_key"`
		Approver       string    `json:"approver"`
		ApprovedAt     time.Time `json:"approved_at"`
		ChallengeHash  string    `json:"challenge_hash"`
	}{
		Schema:         "trade-republic-pp/execution-approval/v1",
		PreviewID:      approval.PreviewID,
		PreviewHash:    approval.PreviewHash,
		IdempotencyKey: approval.IdempotencyKey,
		Approver:       strings.TrimSpace(approval.Approver),
		ApprovedAt:     approval.ApprovedAt.UTC(),
		ChallengeHash:  approval.ChallengeHash,
	})
	if err != nil {
		return "", err
	}
	return digest(value), nil
}

func verifyPreviewIntegrity(preview Preview) error {
	payloadHash, err := canonicalPayloadHash(preview.Intent)
	if err != nil || payloadHash != preview.PayloadHash {
		return ErrPreviewIntegrity
	}
	policyHash, err := canonicalPolicyHash(preview.Policy)
	if err != nil || policyHash != preview.PolicyHash {
		return ErrPreviewIntegrity
	}
	idempotencyKey, err := canonicalIdempotencyKey(
		preview.AccountID,
		preview.Intent,
		preview.PolicyHash,
		preview.CreatedAt,
		preview.ClientIdempotencyKey,
	)
	if err != nil || idempotencyKey != preview.IdempotencyKey {
		return ErrPreviewIntegrity
	}
	bindingHash, err := canonicalPreviewBindingHash(preview)
	if err != nil || bindingHash != preview.BindingHash {
		return ErrPreviewIntegrity
	}
	if preview.ID != previewID(bindingHash) || preview.ApprovalChallenge != approvalChallenge(preview) {
		return ErrPreviewIntegrity
	}
	return nil
}
