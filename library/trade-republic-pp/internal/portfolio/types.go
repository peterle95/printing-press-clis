// Package portfolio owns normalized positions, balances, and allocations.
package portfolio

import (
	"time"

	"trade-republic-pp-cli/internal/money"
)

type Position struct {
	ISIN        string        `json:"isin"`
	Name        string        `json:"name"`
	Quantity    money.Decimal `json:"quantity"`
	AverageCost money.Decimal `json:"average_cost"`
	Price       money.Decimal `json:"price"`
	MarketValue money.Decimal `json:"market_value"`
	Currency    string        `json:"currency"`
	AsOf        time.Time     `json:"as_of"`
	Source      string        `json:"source"`
}

type CashBalance struct {
	Currency string        `json:"currency"`
	Amount   money.Decimal `json:"amount"`
	AsOf     time.Time     `json:"as_of"`
	Source   string        `json:"source"`
}

type Allocation struct {
	Group        string        `json:"group"`
	MarketValue  money.Decimal `json:"market_value"`
	PercentageBP int64         `json:"percentage_basis_points"`
}
