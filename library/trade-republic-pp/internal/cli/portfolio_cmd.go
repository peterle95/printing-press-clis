package cli

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	portfolioDomain "trade-republic-pp-cli/internal/portfolio"
	store "trade-republic-pp-cli/storage/sqlite"
)

func portfolioCmd(f *flags) *cobra.Command {
	var includeCash bool
	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Show the latest locally synchronized portfolio",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			database, cfg, closeStore, err := openStore(ctx, f)
			if err != nil {
				return err
			}
			defer closeStore()
			positions, err := database.Portfolio(ctx)
			if err != nil {
				return err
			}
			summaries, warnings := summariesByCurrency(positions)
			var cash any
			if includeCash {
				balance, balanceErr := database.CashBalance(ctx, cfg.BaseCurrency, 0)
				if balanceErr == nil {
					cash = balance
				} else if !errors.Is(balanceErr, store.ErrNotFound) {
					warnings = append(warnings, balanceErr.Error())
				}
			}
			value := envelope{"version": 1, "positions": positions, "summaries": summaries, "cash_balance": cash, "warnings": warnings}
			return emit(cmd, f, value, formatPortfolio(positions, summaries, cash))
		},
	}
	cmd.Flags().BoolVar(&includeCash, "cash", true, "include the latest imported cash balance when available")
	return cmd
}

func positionCmd(f *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "position <ISIN-or-alias>",
		Short: "Show one position, resolving symbols as aliases to ISIN",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			database, _, closeStore, err := openStore(ctx, f)
			if err != nil {
				return err
			}
			defer closeStore()
			isin, err := database.ResolveISIN(ctx, args[0])
			if err != nil {
				return err
			}
			position, err := database.Position(ctx, isin)
			if err != nil {
				return err
			}
			cost := position.Quantity.Mul(position.AverageCost)
			pnl := position.MarketValue.Sub(cost)
			return emit(cmd, f, envelope{"version": 1, "position": position, "cost_basis": cost, "unrealized_pnl": pnl}, fmt.Sprintf("%s (%s)\nquantity: %s\naverage cost: %s %s\nprice: %s %s\nmarket value: %s %s\nunrealized P&L: %s %s", position.Name, position.ISIN, position.Quantity, position.AverageCost.Fixed(2), position.Currency, position.Price.Fixed(4), position.Currency, position.MarketValue.Fixed(2), position.Currency, pnl.Fixed(2), position.Currency))
		},
	}
}

func allocationCmd(f *flags) *cobra.Command {
	var group string
	cmd := &cobra.Command{
		Use:   "allocation",
		Short: "Group latest market value by instrument metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			database, _, closeStore, err := openStore(ctx, f)
			if err != nil {
				return err
			}
			defer closeStore()
			rows, err := database.Allocation(ctx, group)
			if err != nil {
				return err
			}
			return emit(cmd, f, envelope{"version": 1, "group": group, "allocation": rows}, formatAllocation(rows))
		},
	}
	cmd.Flags().StringVar(&group, "group", "instrument", "instrument, sector, country, currency, or kind")
	return cmd
}

func summariesByCurrency(positions []portfolioDomain.Position) ([]portfolioDomain.Summary, []string) {
	groups := map[string][]portfolioDomain.Position{}
	for _, position := range positions {
		groups[strings.ToUpper(position.Currency)] = append(groups[strings.ToUpper(position.Currency)], position)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var summaries []portfolioDomain.Summary
	var warnings []string
	for _, key := range keys {
		summary, err := portfolioDomain.Summarize(groups[key])
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		summaries = append(summaries, summary)
	}
	return summaries, warnings
}

func formatPortfolio(positions []portfolioDomain.Position, summaries []portfolioDomain.Summary, cash any) string {
	if len(positions) == 0 {
		return "portfolio is empty; run tr sync first"
	}
	var out strings.Builder
	writer := tabwriter.NewWriter(&out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "NAME\tISIN\tQUANTITY\tPRICE\tVALUE\tAS OF")
	for _, position := range positions {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s %s\t%s %s\t%s\n", position.Name, position.ISIN, position.Quantity, position.Price.Fixed(4), position.Currency, position.MarketValue.Fixed(2), position.Currency, position.AsOf.Format("2006-01-02 15:04"))
	}
	_ = writer.Flush()
	for _, summary := range summaries {
		fmt.Fprintf(&out, "\n%s total %s; cost basis %s; unrealized P&L %s", summary.Currency, summary.MarketValue.Fixed(2), summary.CostBasis.Fixed(2), summary.UnrealizedPnL.Fixed(2))
	}
	if cash != nil {
		fmt.Fprint(&out, "\ncash balance available in JSON output")
	}
	return strings.TrimSpace(out.String())
}

func formatAllocation(rows []portfolioDomain.Allocation) string {
	if len(rows) == 0 {
		return "no allocation data"
	}
	var out strings.Builder
	writer := tabwriter.NewWriter(&out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "GROUP\tMARKET VALUE\tWEIGHT")
	for _, row := range rows {
		fmt.Fprintf(writer, "%s\t%s\t%d.%02d%%\n", row.Group, row.MarketValue.Fixed(2), row.PercentageBP/100, row.PercentageBP%100)
	}
	_ = writer.Flush()
	return strings.TrimSpace(out.String())
}
