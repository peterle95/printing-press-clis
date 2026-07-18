package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"trade-republic-pp-cli/config"
	"trade-republic-pp-cli/internal/execution"
	store "trade-republic-pp-cli/storage/sqlite"
)

func executionPolicy(cfg config.Config) execution.Policy {
	return execution.Policy{
		ID:                       "local-risk-v1",
		KillSwitch:               cfg.Risk.KillSwitch,
		PaperTrading:             cfg.Risk.PaperTrading,
		Currency:                 strings.ToUpper(cfg.BaseCurrency),
		AllowedISINs:             append([]string(nil), cfg.Risk.PermittedISINs...),
		MaxOrderValue:            cfg.Risk.MaxOrderValue,
		MaxDailyReservedExposure: cfg.Risk.MaxDailyValue,
		MaxMarketAge:             cfg.Risk.PriceMaxAge.Duration(),
		MaxBalanceAge:            cfg.Risk.BalanceMaxAge.Duration(),
		PreviewTTL:               cfg.Risk.PreviewValidity.Duration(),
	}
}

func localExecutionSnapshots(ctx context.Context, database *store.Store, accountID, isin, currency string) (execution.MarketSnapshot, execution.BalanceSnapshot) {
	market := execution.MarketSnapshot{ISIN: isin, Currency: currency}
	if point, err := database.FreshPrice(ctx, isin, 0); err == nil {
		market.Price, market.Currency, market.ObservedAt, market.Source = point.Price, point.Currency, point.AsOf, point.Source
	} else if position, positionErr := database.Position(ctx, isin); positionErr == nil {
		market.Price, market.Currency, market.ObservedAt, market.Source = position.Price, position.Currency, position.AsOf, position.Source
	}
	balance := execution.BalanceSnapshot{AccountID: accountID, Currency: currency}
	if cash, err := database.CashBalance(ctx, currency, 0); err == nil {
		balance.AvailableCash, balance.ObservedAt, balance.Source = cash.Amount, cash.AsOf, cash.Source
	}
	if position, err := database.Position(ctx, isin); err == nil {
		balance.AvailableQuantity = position.Quantity
		if balance.ObservedAt.IsZero() || position.AsOf.Before(balance.ObservedAt) {
			balance.ObservedAt = position.AsOf
		}
	}
	return market, balance
}

func randomNonce() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func formatPreview(preview execution.Preview) string {
	var out strings.Builder
	fmt.Fprintf(&out, "paper preview %s\nallowed: %t\nexposure: %s %s\nexpires: %s", preview.ID, preview.Decision.Allowed, preview.Decision.OrderExposure, preview.Intent.Currency, preview.ExpiresAt.Format(time.RFC3339))
	for _, reason := range preview.Decision.Reasons {
		fmt.Fprintf(&out, "\n- %s: %s", reason.Code, reason.Message)
	}
	if preview.Decision.Allowed {
		fmt.Fprintf(&out, "\napproval challenge: %s", preview.ApprovalChallenge)
	}
	out.WriteString("\nlive submission: not implemented")
	return out.String()
}
