package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"trade-republic-pp-cli/internal/reporting"
	store "trade-republic-pp-cli/storage/sqlite"
)

func reportCmd(f *flags) *cobra.Command {
	cmd := &cobra.Command{Use: "report", Short: "Build transparent local ledger and portfolio reports"}
	cmd.AddCommand(reportBucketsCmd(f, "daily"), reportBucketsCmd(f, "monthly"), reportPnLCmd(f), reportDividendsCmd(f), reportChargesCmd(f, "fees"), reportChargesCmd(f, "taxes"))
	return cmd
}

func reportBucketsCmd(f *flags, period string) *cobra.Command {
	var sinceValue string
	cmd := &cobra.Command{
		Use:   period,
		Short: "Aggregate " + period + " ledger activity (not time-weighted performance)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			database, cfg, closeStore, err := openStore(ctx, f)
			if err != nil {
				return err
			}
			defer closeStore()
			location, _ := time.LoadLocation(cfg.AccountTimezone)
			since, err := parseDate(sinceValue, location)
			if err != nil {
				return err
			}
			rows, err := database.Transactions(ctx, store.TransactionFilter{Since: since})
			if err != nil {
				return err
			}
			buckets, err := reporting.Buckets(rows, period, location)
			if err != nil {
				return err
			}
			return emit(cmd, f, envelope{"version": 1, "period": period, "definition": "normalized ledger cash flow, not investment performance", "buckets": buckets}, formatBuckets(buckets))
		},
	}
	cmd.Flags().StringVar(&sinceValue, "since", "", "inclusive start date")
	return cmd
}

func reportPnLCmd(f *flags) *cobra.Command {
	var period string
	cmd := &cobra.Command{
		Use:   "pnl",
		Short: "Show current unrealized P&L and period ledger context",
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
			pnl, err := reporting.PnL(positions)
			if err != nil {
				return err
			}
			location, _ := time.LoadLocation(cfg.AccountTimezone)
			start, err := periodStart(period, time.Now(), location)
			if err != nil {
				return err
			}
			var since *time.Time
			if !start.IsZero() {
				since = &start
			}
			rows, err := database.Transactions(ctx, store.TransactionFilter{Since: since})
			if err != nil {
				return err
			}
			ledger, err := reporting.Totals(rows)
			if err != nil {
				return err
			}
			return emit(cmd, f, envelope{"version": 1, "period": period, "pnl": pnl, "ledger_context": ledger}, fmt.Sprintf("current unrealized P&L: %s %s\nrealized P&L: unavailable\nperiod ledger rows: %d", pnl.UnrealizedPnL.Fixed(2), pnl.Currency, len(rows)))
		},
	}
	cmd.Flags().StringVar(&period, "period", "all", "all, ytd, mtd, 1y, or a start date")
	return cmd
}

func reportDividendsCmd(f *flags) *cobra.Command {
	var year int
	cmd := &cobra.Command{
		Use:   "dividends",
		Short: "Sum normalized dividends by currency",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			database, cfg, closeStore, err := openStore(ctx, f)
			if err != nil {
				return err
			}
			defer closeStore()
			location, _ := time.LoadLocation(cfg.AccountTimezone)
			if year == 0 {
				year = time.Now().In(location).Year()
			}
			start := time.Date(year, 1, 1, 0, 0, 0, 0, location)
			end := start.AddDate(1, 0, 0)
			totals, err := database.DividendTotals(ctx, &start, &end)
			if err != nil {
				return err
			}
			return emit(cmd, f, envelope{"version": 1, "year": year, "dividends": totals}, formatMonetaryTotals("dividends", totals))
		},
	}
	cmd.Flags().IntVar(&year, "year", 0, "calendar year (defaults to current year)")
	return cmd
}

func reportChargesCmd(f *flags, kind string) *cobra.Command {
	var period string
	cmd := &cobra.Command{
		Use:   kind,
		Short: "Sum normalized " + kind + " by currency",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			database, cfg, closeStore, err := openStore(ctx, f)
			if err != nil {
				return err
			}
			defer closeStore()
			location, _ := time.LoadLocation(cfg.AccountTimezone)
			start, err := periodStart(period, time.Now(), location)
			if err != nil {
				return err
			}
			var since *time.Time
			if !start.IsZero() {
				since = &start
			}
			var totals []store.MonetaryTotal
			if kind == "fees" {
				totals, err = database.FeeTotals(ctx, since, nil)
			} else {
				totals, err = database.TaxTotals(ctx, since, nil)
			}
			if err != nil {
				return err
			}
			return emit(cmd, f, envelope{"version": 1, "period": period, kind: totals}, formatMonetaryTotals(kind, totals))
		},
	}
	cmd.Flags().StringVar(&period, "period", "all", "all, ytd, mtd, 1y, or a start date")
	return cmd
}

func formatBuckets(buckets []reporting.Bucket) string {
	if len(buckets) == 0 {
		return "no ledger activity"
	}
	var out strings.Builder
	writer := tabwriter.NewWriter(&out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "PERIOD\tCCY\tNET CASH FLOW\tDIVIDENDS\tFEES\tTAXES\tCOUNT")
	for _, bucket := range buckets {
		total := bucket.Totals
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%d\n", bucket.Period, total.Currency, total.NetCashFlow.Fixed(2), total.Dividends.Fixed(2), total.Fees.Fixed(2), total.Taxes.Fixed(2), total.Count)
	}
	_ = writer.Flush()
	return strings.TrimSpace(out.String())
}

func formatMonetaryTotals(label string, totals []store.MonetaryTotal) string {
	if len(totals) == 0 {
		return "no " + label
	}
	var out strings.Builder
	for index, total := range totals {
		if index > 0 {
			out.WriteByte('\n')
		}
		fmt.Fprintf(&out, "%s: %s %s (%d records)", label, total.Amount.Fixed(2), total.Currency, total.Count)
	}
	return out.String()
}
