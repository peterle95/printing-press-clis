package research

import (
	"fmt"
	"math/big"
	"net/url"
	"sort"
	"strings"

	"trade-republic-pp-cli/internal/instruments"
	"trade-republic-pp-cli/internal/money"
	"trade-republic-pp-cli/internal/portfolio"
)

type Comparison struct {
	Identifier              string                   `json:"identifier"`
	Name                    string                   `json:"name"`
	Kind                    string                   `json:"kind"`
	AsOf                    string                   `json:"as_of"`
	TERBasisPoints          *int64                   `json:"ter_basis_points,omitempty"`
	FundSize                *money.Decimal           `json:"fund_size,omitempty"`
	FundSizeCurrency        string                   `json:"fund_size_currency,omitempty"`
	TrackingDifferenceBP    *int64                   `json:"tracking_difference_basis_points,omitempty"`
	ValuationRatios         map[string]money.Decimal `json:"valuation_ratios,omitempty"`
	PortfolioExposure       money.Decimal            `json:"portfolio_exposure"`
	PortfolioExposureWeight int64                    `json:"portfolio_exposure_basis_points"`
	Warnings                []string                 `json:"warnings,omitempty"`
}

func Validate(report Report) error {
	if report.SchemaVersion != 1 {
		return fmt.Errorf("research report %q has unsupported schema version %d", report.Identifier, report.SchemaVersion)
	}
	if strings.TrimSpace(report.Identifier) == "" || strings.TrimSpace(report.Name) == "" {
		return fmt.Errorf("research report identifier and name are required")
	}
	if report.AsOf.IsZero() {
		return fmt.Errorf("research report %q has no as_of", report.Identifier)
	}
	switch strings.ToLower(report.Kind) {
	case "etf":
		if report.ETF == nil {
			return fmt.Errorf("ETF report %q has no etf section", report.Identifier)
		}
	case "company":
		if report.Company == nil {
			return fmt.Errorf("company report %q has no company section", report.Identifier)
		}
	default:
		return fmt.Errorf("research report %q has unsupported kind %q", report.Identifier, report.Kind)
	}
	if len(report.Citations) == 0 {
		return fmt.Errorf("research report %q has no source citations", report.Identifier)
	}
	for index, citation := range report.Citations {
		parsed, err := url.Parse(citation.URL)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
			return fmt.Errorf("research report %q citation %d has invalid URL", report.Identifier, index)
		}
	}
	return nil
}

func EnrichPortfolio(report Report, positions []portfolio.Position, metadata map[string]instruments.Instrument) (Report, error) {
	if err := Validate(report); err != nil {
		return Report{}, err
	}
	summary, err := portfolio.Summarize(positions)
	if err != nil {
		return Report{}, err
	}
	copyReport := report
	var direct money.Decimal
	for _, position := range positions {
		instrument := metadata[position.ISIN]
		if sameInstrument(report, position.ISIN, instrument.Symbol) {
			direct = direct.Add(position.MarketValue)
		}
	}
	weight := ratioBP(direct, summary.MarketValue)
	if copyReport.Company != nil {
		company := *copyReport.Company
		company.PortfolioExposure = direct
		company.PortfolioExposureWeight = weight
		copyReport.Company = &company
	}
	if copyReport.ETF != nil {
		etf := *copyReport.ETF
		etf.PortfolioOverlap = overlap(etf, positions, metadata)
		if direct > 0 {
			etf.PortfolioOverlap = append(etf.PortfolioOverlap, PortfolioOverlap{
				Identifier: report.Identifier,
				Name:       report.Name,
				WeightBP:   weight,
				Reason:     "the fund itself is already held in the portfolio",
			})
		}
		sort.Slice(etf.PortfolioOverlap, func(i, j int) bool {
			return etf.PortfolioOverlap[i].WeightBP > etf.PortfolioOverlap[j].WeightBP
		})
		copyReport.ETF = &etf
	}
	return copyReport, nil
}

func sameInstrument(report Report, isin, symbol string) bool {
	if report.ISIN != "" && strings.EqualFold(report.ISIN, isin) {
		return true
	}
	return report.Symbol != "" && strings.EqualFold(report.Symbol, symbol)
}

func overlap(etf ETFReport, positions []portfolio.Position, metadata map[string]instruments.Instrument) []PortfolioOverlap {
	var result []PortfolioOverlap
	seen := map[string]bool{}
	for _, holding := range etf.TopHoldings {
		for _, position := range positions {
			instrument := metadata[position.ISIN]
			matches := holding.ISIN != "" && strings.EqualFold(holding.ISIN, position.ISIN)
			matches = matches || holding.Symbol != "" && strings.EqualFold(holding.Symbol, instrument.Symbol)
			if !matches || seen[position.ISIN] {
				continue
			}
			seen[position.ISIN] = true
			result = append(result, PortfolioOverlap{
				Identifier: position.ISIN,
				Name:       position.Name,
				WeightBP:   holding.WeightBP,
				Reason:     "a top holding of this fund is held directly",
			})
		}
	}
	return result
}

func Compare(reports []Report, positions []portfolio.Position, metadata map[string]instruments.Instrument) ([]Comparison, error) {
	comparisons := make([]Comparison, 0, len(reports))
	for _, report := range reports {
		enriched, err := EnrichPortfolio(report, positions, metadata)
		if err != nil {
			return nil, err
		}
		comparison := Comparison{Identifier: report.Identifier, Name: report.Name, Kind: report.Kind, AsOf: report.AsOf.Format("2006-01-02"), Warnings: report.Warnings}
		if enriched.ETF != nil {
			ter := enriched.ETF.TERBasisPoints
			size := enriched.ETF.FundSize
			comparison.TERBasisPoints = &ter
			comparison.FundSize = &size
			comparison.FundSizeCurrency = enriched.ETF.FundSizeCurrency
			comparison.TrackingDifferenceBP = enriched.ETF.TrackingDifferenceBP
		}
		if enriched.Company != nil {
			comparison.ValuationRatios = enriched.Company.ValuationRatios
			comparison.PortfolioExposure = enriched.Company.PortfolioExposure
			comparison.PortfolioExposureWeight = enriched.Company.PortfolioExposureWeight
		}
		comparisons = append(comparisons, comparison)
	}
	return comparisons, nil
}

func ratioBP(value, total money.Decimal) int64 {
	if total <= 0 || value <= 0 {
		return 0
	}
	numerator := new(big.Int).Mul(big.NewInt(int64(value)), big.NewInt(10_000))
	return new(big.Int).Quo(numerator, big.NewInt(int64(total))).Int64()
}
