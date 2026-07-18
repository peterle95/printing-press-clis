// Package reporting builds transparent ledger and unrealized-PnL reports.
// It never labels cash-flow arithmetic as time-weighted performance.
package reporting

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"trade-republic-pp-cli/internal/money"
	"trade-republic-pp-cli/internal/portfolio"
	"trade-republic-pp-cli/internal/transactions"
)

type LedgerTotals struct {
	Currency    string        `json:"currency"`
	Buys        money.Decimal `json:"buys"`
	Sells       money.Decimal `json:"sells"`
	Deposits    money.Decimal `json:"deposits"`
	Withdrawals money.Decimal `json:"withdrawals"`
	Dividends   money.Decimal `json:"dividends"`
	Interest    money.Decimal `json:"interest"`
	Fees        money.Decimal `json:"fees"`
	Taxes       money.Decimal `json:"taxes"`
	NetCashFlow money.Decimal `json:"net_cash_flow"`
	Count       int           `json:"count"`
}

type Bucket struct {
	Period string       `json:"period"`
	Totals LedgerTotals `json:"totals"`
}

type PnLReport struct {
	Currency      string         `json:"currency"`
	CostBasis     money.Decimal  `json:"cost_basis"`
	MarketValue   money.Decimal  `json:"market_value"`
	UnrealizedPnL money.Decimal  `json:"unrealized_pnl"`
	RealizedPnL   *money.Decimal `json:"realized_pnl"`
	Warnings      []string       `json:"warnings"`
}

func Totals(rows []transactions.Transaction) ([]LedgerTotals, error) {
	byCurrency := map[string]*LedgerTotals{}
	for _, row := range rows {
		currency := strings.ToUpper(row.Currency)
		if len(currency) != 3 {
			return nil, fmt.Errorf("transaction %s has invalid currency %q", row.ID, row.Currency)
		}
		total := byCurrency[currency]
		if total == nil {
			total = &LedgerTotals{Currency: currency}
			byCurrency[currency] = total
		}
		total.Count++
		total.NetCashFlow = total.NetCashFlow.Add(row.Amount).Sub(row.Fees.Abs()).Sub(row.Taxes.Abs())
		switch row.Kind {
		case transactions.Buy:
			total.Buys = total.Buys.Add(row.Amount.Abs())
		case transactions.Sell:
			total.Sells = total.Sells.Add(row.Amount.Abs())
		case transactions.Deposit, transactions.TransferIn:
			total.Deposits = total.Deposits.Add(row.Amount.Abs())
		case transactions.Withdrawal, transactions.TransferOut:
			total.Withdrawals = total.Withdrawals.Add(row.Amount.Abs())
		case transactions.Dividend:
			total.Dividends = total.Dividends.Add(row.Amount.Abs())
		case transactions.Interest:
			total.Interest = total.Interest.Add(row.Amount.Abs())
		case transactions.Fee:
			total.Fees = total.Fees.Add(row.Amount.Abs())
		case transactions.Tax:
			total.Taxes = total.Taxes.Add(row.Amount.Abs())
		}
		total.Fees = total.Fees.Add(row.Fees.Abs())
		total.Taxes = total.Taxes.Add(row.Taxes.Abs())
	}
	out := make([]LedgerTotals, 0, len(byCurrency))
	for _, value := range byCurrency {
		out = append(out, *value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Currency < out[j].Currency })
	return out, nil
}

func Buckets(rows []transactions.Transaction, period string, location *time.Location) ([]Bucket, error) {
	if location == nil {
		location = time.UTC
	}
	period = strings.ToLower(period)
	if period != "daily" && period != "monthly" {
		return nil, fmt.Errorf("unsupported report period %q", period)
	}
	grouped := map[string][]transactions.Transaction{}
	for _, row := range rows {
		format := "2006-01-02"
		if period == "monthly" {
			format = "2006-01"
		}
		key := row.OccurredAt.In(location).Format(format)
		grouped[key] = append(grouped[key], row)
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]Bucket, 0, len(keys))
	for _, key := range keys {
		totals, err := Totals(grouped[key])
		if err != nil {
			return nil, err
		}
		for _, total := range totals {
			out = append(out, Bucket{Period: key, Totals: total})
		}
	}
	return out, nil
}

func Filter(rows []transactions.Transaction, since, until time.Time, kinds ...transactions.Kind) []transactions.Transaction {
	allowed := map[transactions.Kind]bool{}
	for _, kind := range kinds {
		allowed[kind] = true
	}
	out := make([]transactions.Transaction, 0, len(rows))
	for _, row := range rows {
		if !since.IsZero() && row.OccurredAt.Before(since) {
			continue
		}
		if !until.IsZero() && !row.OccurredAt.Before(until) {
			continue
		}
		if len(allowed) > 0 && !allowed[row.Kind] {
			continue
		}
		out = append(out, row)
	}
	return out
}

func PnL(positions []portfolio.Position) (PnLReport, error) {
	summary, err := portfolio.Summarize(positions)
	if err != nil {
		return PnLReport{}, err
	}
	return PnLReport{
		Currency:      summary.Currency,
		CostBasis:     summary.CostBasis,
		MarketValue:   summary.MarketValue,
		UnrealizedPnL: summary.UnrealizedPnL,
		Warnings: []string{
			"Realized P&L is unavailable until lot matching, corporate actions, and FX history are complete.",
			"This report is not tax advice and is not a time-weighted performance calculation.",
		},
	}, nil
}
