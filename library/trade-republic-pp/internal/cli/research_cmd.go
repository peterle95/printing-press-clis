package cli

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"trade-republic-pp-cli/internal/instruments"
	"trade-republic-pp-cli/internal/portfolio"
	"trade-republic-pp-cli/internal/research"
	"trade-republic-pp-cli/storage/financejson"
)

func searchCmd(f *flags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search locally synchronized instruments and aliases",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			database, _, closeStore, err := openStore(ctx, f)
			if err != nil {
				return err
			}
			defer closeStore()
			results, err := database.SearchInstruments(ctx, args[0], limit)
			if err != nil {
				return err
			}
			return emit(cmd, f, envelope{"version": 1, "query": args[0], "results": results}, formatSearch(results))
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum results")
	return cmd
}

func researchCmd(f *flags) *cobra.Command {
	var isin string
	cmd := &cobra.Command{
		Use:   "research [identifier]",
		Short: "Show cited research previously imported into SQLite",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			identifier := strings.TrimSpace(isin)
			if len(args) == 1 {
				if identifier != "" {
					return fmt.Errorf("provide an identifier argument or --isin, not both")
				}
				identifier = args[0]
			}
			if identifier == "" {
				return fmt.Errorf("research identifier or --isin is required")
			}
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			database, _, closeStore, err := openStore(ctx, f)
			if err != nil {
				return err
			}
			defer closeStore()
			report, err := database.ResearchReport(ctx, identifier)
			if err != nil {
				return err
			}
			positions, err := database.Portfolio(ctx)
			if err != nil {
				return err
			}
			metadata := loadPositionMetadata(ctx, database, positions)
			report, err = research.EnrichPortfolio(report, positions, metadata)
			if err != nil {
				return err
			}
			return emit(cmd, f, envelope{"version": 1, "research": report}, formatResearch(report))
		},
	}
	cmd.Flags().StringVar(&isin, "isin", "", "canonical instrument ISIN")
	cmd.AddCommand(researchImportCmd(f))
	return cmd
}

func researchImportCmd(f *flags) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "import <financejson-file>",
		Short: "Validate and cache citation-bearing FinanceJSON research",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bundle, err := financejson.Load(args[0])
			if err != nil {
				return err
			}
			if len(bundle.Research) == 0 {
				return fmt.Errorf("FinanceJSON contains no research_reports")
			}
			if dryRun {
				return emit(cmd, f, envelope{"version": 1, "dry_run": true, "research_reports": bundle.Research}, fmt.Sprintf("dry run: validated %d research reports", len(bundle.Research)))
			}
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			database, _, closeStore, err := openStore(ctx, f)
			if err != nil {
				return err
			}
			defer closeStore()
			saved := make([]research.Report, 0, len(bundle.Research))
			for _, report := range bundle.Research {
				stored, err := database.SaveResearchReport(ctx, report)
				if err != nil {
					return err
				}
				saved = append(saved, stored)
			}
			return emit(cmd, f, envelope{"version": 1, "research_reports": saved}, fmt.Sprintf("imported %d research reports", len(saved)))
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate without opening or writing SQLite")
	return cmd
}

func compareCmd(f *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "compare <identifier> <identifier> [identifier...]",
		Short: "Compare cached ETF or company research",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			database, _, closeStore, err := openStore(ctx, f)
			if err != nil {
				return err
			}
			defer closeStore()
			reports := make([]research.Report, 0, len(args))
			for _, identifier := range args {
				report, err := database.ResearchReport(ctx, identifier)
				if err != nil {
					return fmt.Errorf("research %s: %w", identifier, err)
				}
				reports = append(reports, report)
			}
			positions, err := database.Portfolio(ctx)
			if err != nil {
				return err
			}
			comparisons, err := research.Compare(reports, positions, loadPositionMetadata(ctx, database, positions))
			if err != nil {
				return err
			}
			return emit(cmd, f, envelope{"version": 1, "comparisons": comparisons}, formatComparisons(comparisons))
		},
	}
}

func newsCmd(f *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "news <identifier>",
		Short: "Show cited news cached with a research report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			database, _, closeStore, err := openStore(ctx, f)
			if err != nil {
				return err
			}
			defer closeStore()
			report, err := database.ResearchReport(ctx, args[0])
			if err != nil {
				return err
			}
			human := fmt.Sprintf("%d cached news items for %s", len(report.News), report.Name)
			if len(report.News) == 0 {
				human += "; import a cited FinanceJSON research update"
			}
			return emit(cmd, f, envelope{"version": 1, "identifier": args[0], "as_of": report.AsOf, "news": report.News, "citations": report.Citations}, human)
		},
	}
}

func loadPositionMetadata(ctx context.Context, database interface {
	SearchInstruments(context.Context, string, int) ([]instruments.Instrument, error)
}, positions []portfolio.Position) map[string]instruments.Instrument {
	metadata := make(map[string]instruments.Instrument, len(positions))
	for _, position := range positions {
		results, err := database.SearchInstruments(ctx, position.ISIN, 1)
		if err == nil && len(results) > 0 {
			metadata[position.ISIN] = results[0]
		}
	}
	return metadata
}

func formatSearch(results []instruments.Instrument) string {
	if len(results) == 0 {
		return "no local instruments matched"
	}
	var out strings.Builder
	writer := tabwriter.NewWriter(&out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "NAME\tISIN\tSYMBOL\tKIND\tCOUNTRY\tSECTOR")
	for _, item := range results {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n", item.Name, item.ISIN, item.Symbol, item.Kind, item.Country, item.Sector)
	}
	_ = writer.Flush()
	return strings.TrimSpace(out.String())
}

func formatResearch(report research.Report) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%s (%s)\nkind: %s\nas of: %s", report.Name, report.Identifier, report.Kind, report.AsOf.Format("2006-01-02"))
	if report.Summary != "" {
		fmt.Fprintf(&out, "\n\n%s", report.Summary)
	}
	if report.ETF != nil {
		etf := report.ETF
		fmt.Fprintf(&out, "\n\nISIN: %s\nindex: %s\nTER: %d bp\nfund size: %s %s\ndomicile: %s\nreplication: %s\ndistribution: %s\nbase currency: %s\ntrading currencies: %s\ntop holdings: %d\ncountry exposures: %d\nsector exposures: %d\nportfolio overlaps: %d", etf.ISIN, etf.IndexTracked, etf.TERBasisPoints, etf.FundSize, etf.FundSizeCurrency, etf.Domicile, etf.ReplicationMethod, etf.DistributionPolicy, etf.BaseCurrency, strings.Join(etf.TradingCurrencies, ", "), len(etf.TopHoldings), len(etf.CountryExposure), len(etf.SectorExposure), len(etf.PortfolioOverlap))
	}
	if report.Company != nil {
		company := report.Company
		fmt.Fprintf(&out, "\n\n%s\nrevenue periods: %d\nearnings periods: %d\ncompetitors: %s\nrecent filings: %d\nrisks: %d\ncatalysts: %d\nportfolio exposure: %s (%d bp)", company.BusinessSummary, len(company.RevenueTrend), len(company.EarningsTrend), strings.Join(company.MajorCompetitors, ", "), len(company.RecentFilings), len(company.Risks), len(company.Catalysts), company.PortfolioExposure, company.PortfolioExposureWeight)
	}
	fmt.Fprintf(&out, "\n\nsources: %d", len(report.Citations))
	for _, citation := range report.Citations {
		fmt.Fprintf(&out, "\n- %s: %s", citation.Title, citation.URL)
	}
	return out.String()
}

func formatComparisons(rows []research.Comparison) string {
	var out strings.Builder
	writer := tabwriter.NewWriter(&out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "IDENTIFIER\tNAME\tKIND\tAS OF\tTER BP\tFUND SIZE\tPORTFOLIO BP")
	for _, row := range rows {
		ter, size := "-", "-"
		if row.TERBasisPoints != nil {
			ter = fmt.Sprint(*row.TERBasisPoints)
		}
		if row.FundSize != nil {
			size = row.FundSize.String() + " " + row.FundSizeCurrency
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%d\n", row.Identifier, row.Name, row.Kind, row.AsOf, ter, size, row.PortfolioExposureWeight)
	}
	_ = writer.Flush()
	return strings.TrimSpace(out.String())
}
