package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"trade-republic-pp-cli/internal/execution"
	"trade-republic-pp-cli/internal/money"
)

func orderPreviewCmd(f *flags) *cobra.Command {
	var buy, sell, amountValue, quantityValue, limitValue, currency, accountID, nonce, idempotencyKey string
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Evaluate a paper order through the deterministic risk engine",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if (buy == "") == (sell == "") {
				return fmt.Errorf("provide exactly one of --buy or --sell")
			}
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			database, cfg, closeStore, err := openStore(ctx, f)
			if err != nil {
				return err
			}
			defer closeStore()
			identifier, side := buy, execution.SideBuy
			if sell != "" {
				identifier, side = sell, execution.SideSell
			}
			isin, err := database.ResolveISIN(ctx, identifier)
			if err != nil {
				return err
			}
			amount, err := money.Parse(amountValue)
			if err != nil {
				return fmt.Errorf("--amount: %w", err)
			}
			quantity, err := money.Parse(quantityValue)
			if err != nil {
				return fmt.Errorf("--quantity: %w", err)
			}
			limitPrice, err := money.Parse(limitValue)
			if err != nil {
				return fmt.Errorf("--limit-price: %w", err)
			}
			if amount.IsZero() && quantity > 0 && limitPrice > 0 {
				amount = quantity.Mul(limitPrice)
			}
			if quantity.IsZero() && amount > 0 && limitPrice > 0 {
				quantity, err = amount.Div(limitPrice)
				if err != nil {
					return err
				}
			}
			if currency == "" {
				currency = cfg.BaseCurrency
			}
			currency = strings.ToUpper(currency)
			now := time.Now().UTC()
			market, balance := localExecutionSnapshots(ctx, database, accountID, isin, currency)
			if nonce == "" {
				nonce, err = randomNonce()
				if err != nil {
					return err
				}
			}
			engine, err := execution.NewEngine(database)
			if err != nil {
				return err
			}
			preview, err := engine.CreatePreview(ctx, execution.PreviewRequest{
				AccountID: accountID,
				Intent: execution.OrderIntent{
					Side: side, OrderType: execution.OrderTypeLimit, ISIN: isin,
					Quantity: quantity, LimitPrice: limitPrice, Amount: amount, Currency: currency,
				},
				Policy: executionPolicy(cfg), Market: market, Balance: balance, Now: now, Nonce: nonce, ClientIdempotencyKey: idempotencyKey,
			})
			if err != nil {
				return err
			}
			return emit(cmd, f, envelope{
				"version": 1, "paper_only": true, "live_submission_supported": false, "preview": preview,
			}, formatPreview(preview))
		},
	}
	cmd.Flags().StringVar(&buy, "buy", "", "ISIN or locally known alias to buy")
	cmd.Flags().StringVar(&sell, "sell", "", "ISIN or locally known alias to sell")
	cmd.Flags().StringVar(&amountValue, "amount", "", "positive order notional as an exact decimal")
	cmd.Flags().StringVar(&quantityValue, "quantity", "", "positive quantity as an exact decimal")
	cmd.Flags().StringVar(&limitValue, "limit-price", "", "required limit price as an exact decimal")
	cmd.Flags().StringVar(&currency, "currency", "", "ISO 4217 currency (defaults to configured base currency)")
	cmd.Flags().StringVar(&accountID, "account", "default", "opaque local account identifier")
	cmd.Flags().StringVar(&nonce, "nonce", "", "unique request nonce (generated when omitted)")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "optional stable idempotency key for this paper preview")
	return cmd
}
