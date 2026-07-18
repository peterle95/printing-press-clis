package execution

import (
	"testing"
	"time"
)

func TestAuditChainDetectsTamperingAndReordering(t *testing.T) {
	start := testTime()
	first := mustAuditEvent(t, 1, start, "preview:one", digestString("one"), GenesisAuditHash)
	second := mustAuditEvent(t, 2, start.Add(time.Second), "approval:two", digestString("two"), first.Hash)
	third := mustAuditEvent(t, 3, start.Add(2*time.Second), "preview:three", digestString("three"), second.Hash)
	chain := []AuditEvent{first, second, third}

	if err := VerifyAuditChain(chain); err != nil {
		t.Fatalf("VerifyAuditChain() error = %v", err)
	}

	t.Run("canonical payload tampering", func(t *testing.T) {
		tampered := append([]AuditEvent(nil), chain...)
		tampered[1].SubjectID = "approval:tampered"
		if err := VerifyAuditChain(tampered); err == nil {
			t.Fatal("VerifyAuditChain() accepted a tampered event")
		}
	})

	t.Run("reordering", func(t *testing.T) {
		reordered := []AuditEvent{first, third, second}
		if err := VerifyAuditChain(reordered); err == nil {
			t.Fatal("VerifyAuditChain() accepted reordered events")
		}
	})

	t.Run("removed event", func(t *testing.T) {
		removed := []AuditEvent{first, third}
		if err := VerifyAuditChain(removed); err == nil {
			t.Fatal("VerifyAuditChain() accepted a chain with a removed event")
		}
	})
}

func TestAuditEventRequiresCanonicalGenesis(t *testing.T) {
	_, err := NewAuditEvent(
		1,
		AuditPreviewEvaluated,
		testTime(),
		testAccount,
		"preview:one",
		digestString("one"),
		digestString("not-genesis"),
	)
	if err == nil {
		t.Fatal("NewAuditEvent() accepted a non-genesis first event")
	}
}

func mustAuditEvent(
	t *testing.T,
	sequence uint64,
	at time.Time,
	subjectID string,
	dataHash string,
	previousHash string,
) AuditEvent {
	t.Helper()
	eventType := AuditPreviewEvaluated
	if sequence == 2 {
		eventType = AuditApprovalRecorded
	}
	event, err := NewAuditEvent(
		sequence,
		eventType,
		at,
		testAccount,
		subjectID,
		dataHash,
		previousHash,
	)
	if err != nil {
		t.Fatalf("NewAuditEvent() error = %v", err)
	}
	return event
}
