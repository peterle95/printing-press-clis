package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"trade-republic-pp-cli/providers/traderepublic"
	store "trade-republic-pp-cli/storage/sqlite"
)

func importCmd(f *flags) *cobra.Command {
	cmd := &cobra.Command{Use: "import", Short: "Import offline broker documents"}
	cmd.AddCommand(importDocumentsCmd(f))
	return cmd
}

func importDocumentsCmd(f *flags) *cobra.Command {
	var destination, sinceValue string
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "documents <directory>",
		Aliases: []string{"statements"},
		Short:   "Hash and inspect Trade Republic statement PDFs without network access",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			if destination == "" {
				destination = cfg.DocumentsDirectory
			}
			adapter, err := newTradeRepublicAdapter(cfg, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			result, err := adapter.ImportDocuments(ctx, traderepublic.DocumentImportRequest{SourceDirectory: args[0], DestinationDirectory: destination, Since: since})
			if err != nil {
				return err
			}
			if dryRun {
				return emit(cmd, f, envelope{"version": 1, "dry_run": true, "documents": result.Documents, "warnings": result.Warnings}, fmt.Sprintf("dry run: recognized %d documents", len(result.Documents)))
			}
			snapshot := traderepublic.Snapshot{Provider: "document-import", Adapter: "statement-pdf", AdapterVersion: "v1", AsOf: time.Now().UTC(), Documents: result.Documents, Warnings: result.Warnings}
			database, _, closeStore, err := openStore(ctx, f)
			if err != nil {
				return err
			}
			defer closeStore()
			syncResult, err := database.ApplySync(ctx, snapshot, store.ApplySyncOptions{Request: traderepublic.SyncRequest{Since: since, IncludeDocuments: true, DocumentsDir: destination}})
			if err != nil {
				return err
			}
			return emit(cmd, f, envelope{"version": 1, "import": syncResult, "documents": result.Documents, "warnings": result.Warnings}, fmt.Sprintf("imported %d documents", syncResult.Documents))
		},
	}
	cmd.Flags().StringVar(&destination, "destination", "", "private document destination (defaults to configuration)")
	cmd.Flags().StringVar(&sinceValue, "since", "", "ignore documents older than this date")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "inspect without opening or writing SQLite")
	return cmd
}
