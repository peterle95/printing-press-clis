package portfolio

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	"trade-republic-pp-cli/internal/instruments"
	"trade-republic-pp-cli/internal/money"
)

type Summary struct {
	Currency      string        `json:"currency"`
	MarketValue   money.Decimal `json:"market_value"`
	CostBasis     money.Decimal `json:"cost_basis"`
	UnrealizedPnL money.Decimal `json:"unrealized_pnl"`
	PositionCount int           `json:"position_count"`
}

func Summarize(positions []Position) (Summary, error) {
	var summary Summary
	for _, position := range positions {
		currency := strings.ToUpper(position.Currency)
		if summary.Currency == "" {
			summary.Currency = currency
		} else if currency != "" && summary.Currency != currency {
			return Summary{}, fmt.Errorf("cannot aggregate %s and %s without an FX rate", summary.Currency, currency)
		}
		summary.MarketValue = summary.MarketValue.Add(position.MarketValue)
		summary.CostBasis = summary.CostBasis.Add(position.Quantity.Mul(position.AverageCost))
		summary.PositionCount++
	}
	summary.UnrealizedPnL = summary.MarketValue.Sub(summary.CostBasis)
	return summary, nil
}

func Allocate(positions []Position, metadata map[string]instruments.Instrument, group string) ([]Allocation, error) {
	group = strings.ToLower(strings.TrimSpace(group))
	if group != "sector" && group != "country" && group != "currency" && group != "kind" {
		return nil, fmt.Errorf("unsupported allocation group %q (use sector, country, currency, or kind)", group)
	}
	summary, err := Summarize(positions)
	if err != nil {
		return nil, err
	}
	if summary.MarketValue <= 0 {
		return []Allocation{}, nil
	}
	values := map[string]money.Decimal{}
	for _, position := range positions {
		instrument := metadata[position.ISIN]
		var key string
		switch group {
		case "sector":
			key = instrument.Sector
		case "country":
			key = instrument.Country
		case "currency":
			key = position.Currency
		case "kind":
			key = instrument.Kind
		}
		if strings.TrimSpace(key) == "" {
			key = "Unclassified"
		}
		values[key] = values[key].Add(position.MarketValue)
	}
	allocations := make([]Allocation, 0, len(values))
	for key, value := range values {
		allocations = append(allocations, Allocation{Group: key, MarketValue: value, PercentageBP: basisPoints(value, summary.MarketValue)})
	}
	sort.Slice(allocations, func(i, j int) bool {
		if allocations[i].MarketValue == allocations[j].MarketValue {
			return allocations[i].Group < allocations[j].Group
		}
		return allocations[i].MarketValue > allocations[j].MarketValue
	})
	return allocations, nil
}

func basisPoints(value, total money.Decimal) int64 {
	if total == 0 {
		return 0
	}
	numerator := new(big.Int).Mul(big.NewInt(int64(value)), big.NewInt(10_000))
	return new(big.Int).Quo(numerator, big.NewInt(int64(total))).Int64()
}
