package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"trade-republic-pp-cli/providers/traderepublic"
	"trade-republic-pp-cli/storage/financejson"
	store "trade-republic-pp-cli/storage/sqlite"
)

func syncCmd(f *flags) *cobra.Command {
	var providerName, input, sinceValue string
	var includeDocuments, dryRun bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize a normalized read-only snapshot into SQLite",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, _, err := loadConfig(f)
			if err != nil {
				return err
			}
			location, err := time.LoadLocation(cfg.AccountTimezone)
			if err != nil {
				return err
			}
			since, err := parseDate(sinceValue, location)
			if err != nil {
				return err
			}
			request := traderepublic.SyncRequest{Since: since, IncludeDocuments: includeDocuments, DocumentsDir: cfg.DocumentsDirectory}
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()

			var snapshot traderepublic.Snapshot
			var bundle financejson.Bundle
			switch strings.ToLower(providerName) {
			case "financejson", "fixture":
				if input == "" {
					return fmt.Errorf("--input is required with --provider financejson")
				}
				bundle, err = financejson.Load(input)
				if err != nil {
					return err
				}
				snapshot = bundle.Snapshot(since)
			case "pytr":
				if input != "" {
					return fmt.Errorf("--input is only valid with --provider financejson")
				}
				adapter, adapterErr := newTradeRepublicAdapter(cfg, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
				if adapterErr != nil {
					return adapterErr
				}
				snapshot, err = adapter.Sync(ctx, request)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported provider %q (use pytr or financejson)", providerName)
			}
			if dryRun {
				result := envelope{
					"version": 1, "dry_run": true, "provider": snapshot.Provider,
					"as_of": snapshot.AsOf, "instruments": len(snapshot.Instruments),
					"positions": len(snapshot.Positions), "cash_balances": len(snapshot.CashBalances),
					"transactions": len(snapshot.Transactions), "documents": len(snapshot.Documents),
					"research_reports": len(bundle.Research), "warnings": snapshot.Warnings,
				}
				return emit(cmd, f, result, fmt.Sprintf("dry run: %d positions, %d transactions, %d documents", len(snapshot.Positions), len(snapshot.Transactions), len(snapshot.Documents)))
			}
			database, _, closeStore, err := openStore(ctx, f)
			if err != nil {
				return err
			}
			defer closeStore()
			result, err := database.ApplySync(ctx, snapshot, store.ApplySyncOptions{
				Request:  request,
				Metadata: map[string]string{"adapter": snapshot.Adapter, "adapter_version": snapshot.AdapterVersion},
			})
			if err != nil {
				return err
			}
			for _, report := range bundle.Research {
				if _, err := database.SaveResearchReport(ctx, report); err != nil {
					return fmt.Errorf("save research report %s after sync %s: %w", report.Identifier, result.RunID, err)
				}
			}
			return emit(cmd, f, envelope{"version": 1, "sync": result, "research_reports": len(bundle.Research)}, fmt.Sprintf("sync %s: %d positions, %d transactions, %d documents", result.Status, result.Positions, result.Transactions, result.Documents))
		},
	}
	cmd.Flags().StringVar(&providerName, "provider", "pytr", "sync provider: pytr or financejson")
	cmd.Flags().StringVar(&input, "input", "", "FinanceJSON input file")
	cmd.Flags().StringVar(&sinceValue, "since", "", "inclusive date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().BoolVar(&includeDocuments, "documents", false, "download and import statement documents")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate and normalize without opening or writing SQLite")
	return cmd
}
