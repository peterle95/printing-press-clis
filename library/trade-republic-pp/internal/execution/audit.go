package execution

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const GenesisAuditHash = hashPrefix + "0000000000000000000000000000000000000000000000000000000000000000"

type AuditEventType string

const (
	AuditPreviewEvaluated AuditEventType = "preview_evaluated"
	AuditApprovalRecorded AuditEventType = "approval_recorded"
)

// AuditEvent is immutable. Hash commits to every preceding field, including
// PreviousHash, which makes an ordered stream tamper-evident.
type AuditEvent struct {
	Sequence     uint64         `json:"sequence"`
	Type         AuditEventType `json:"type"`
	OccurredAt   time.Time      `json:"occurred_at"`
	AccountID    string         `json:"account_id"`
	SubjectID    string         `json:"subject_id"`
	DataHash     string         `json:"data_hash"`
	PreviousHash string         `json:"previous_hash"`
	Hash         string         `json:"hash"`
}

func NewAuditEvent(
	sequence uint64,
	eventType AuditEventType,
	occurredAt time.Time,
	accountID string,
	subjectID string,
	dataHash string,
	previousHash string,
) (AuditEvent, error) {
	if sequence == 0 {
		return AuditEvent{}, fmt.Errorf("audit sequence must be positive")
	}
	if occurredAt.IsZero() {
		return AuditEvent{}, fmt.Errorf("audit occurrence time is required")
	}
	accountID = strings.TrimSpace(accountID)
	subjectID = strings.TrimSpace(subjectID)
	if accountID == "" || subjectID == "" {
		return AuditEvent{}, fmt.Errorf("audit account and subject are required")
	}
	if eventType != AuditPreviewEvaluated && eventType != AuditApprovalRecorded {
		return AuditEvent{}, fmt.Errorf("unsupported audit event type %q", eventType)
	}
	if !validDigest(dataHash) {
		return AuditEvent{}, fmt.Errorf("invalid audit data hash")
	}
	if previousHash == "" {
		previousHash = GenesisAuditHash
	}
	if !validDigest(previousHash) {
		return AuditEvent{}, fmt.Errorf("invalid previous audit hash")
	}
	if sequence == 1 && previousHash != GenesisAuditHash {
		return AuditEvent{}, fmt.Errorf("first audit event must follow genesis")
	}
	if sequence > 1 && previousHash == GenesisAuditHash {
		return AuditEvent{}, fmt.Errorf("non-first audit event cannot follow genesis")
	}
	event := AuditEvent{
		Sequence:     sequence,
		Type:         eventType,
		OccurredAt:   occurredAt.UTC(),
		AccountID:    accountID,
		SubjectID:    subjectID,
		DataHash:     dataHash,
		PreviousHash: previousHash,
	}
	hash, err := canonicalAuditHash(event)
	if err != nil {
		return AuditEvent{}, err
	}
	event.Hash = hash
	return event, nil
}

func canonicalAuditHash(event AuditEvent) (string, error) {
	value, err := canonicalJSON(struct {
		Schema       string         `json:"schema"`
		Sequence     uint64         `json:"sequence"`
		Type         AuditEventType `json:"type"`
		OccurredAt   time.Time      `json:"occurred_at"`
		AccountID    string         `json:"account_id"`
		SubjectID    string         `json:"subject_id"`
		DataHash     string         `json:"data_hash"`
		PreviousHash string         `json:"previous_hash"`
	}{
		Schema:       "trade-republic-pp/execution-audit/v1",
		Sequence:     event.Sequence,
		Type:         event.Type,
		OccurredAt:   event.OccurredAt.UTC(),
		AccountID:    strings.TrimSpace(event.AccountID),
		SubjectID:    strings.TrimSpace(event.SubjectID),
		DataHash:     event.DataHash,
		PreviousHash: event.PreviousHash,
	})
	if err != nil {
		return "", err
	}
	return digest(value), nil
}

func (event AuditEvent) Verify() error {
	if !validDigest(event.Hash) || !validDigest(event.PreviousHash) || !validDigest(event.DataHash) {
		return fmt.Errorf("audit event %d contains an invalid hash", event.Sequence)
	}
	expected, err := canonicalAuditHash(event)
	if err != nil {
		return err
	}
	if expected != event.Hash {
		return fmt.Errorf("audit event %d hash mismatch", event.Sequence)
	}
	return nil
}

// VerifyAuditChain verifies order, continuity, and every canonical event hash.
func VerifyAuditChain(events []AuditEvent) error {
	previous := GenesisAuditHash
	for index, event := range events {
		expectedSequence := uint64(index + 1)
		if event.Sequence != expectedSequence {
			return fmt.Errorf("audit sequence mismatch at index %d: got %d, want %d", index, event.Sequence, expectedSequence)
		}
		if event.PreviousHash != previous {
			return fmt.Errorf("audit previous hash mismatch at sequence %d", event.Sequence)
		}
		if err := event.Verify(); err != nil {
			return err
		}
		previous = event.Hash
	}
	return nil
}

func validDigest(value string) bool {
	if !strings.HasPrefix(value, hashPrefix) {
		return false
	}
	raw := strings.TrimPrefix(value, hashPrefix)
	if len(raw) != 64 {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}
