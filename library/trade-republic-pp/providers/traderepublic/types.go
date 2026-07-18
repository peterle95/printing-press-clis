// Package traderepublic defines the normalized, read-only broker boundary.
package traderepublic

import (
	"context"
	"time"

	"trade-republic-pp-cli/internal/instruments"
	"trade-republic-pp-cli/internal/portfolio"
	"trade-republic-pp-cli/internal/transactions"
)

type SyncRequest struct {
	Since            *time.Time
	IncludeDocuments bool
	DocumentsDir     string
	DryRun           bool
}

type Snapshot struct {
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
	// PositionsComplete marks Positions as a complete point-in-time portfolio
	// snapshot. Stores may reconcile positions missing from such a snapshot.
	PositionsComplete bool
}

type Status struct {
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

type Provider interface {
	Status(context.Context) Status
	Login(context.Context) error
	Sync(context.Context, SyncRequest) (Snapshot, error)
}
