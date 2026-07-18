// Package research owns evidence-backed, structured investment research.
// It intentionally has no dependency on the execution package.
package research

import (
	"time"

	"trade-republic-pp-cli/internal/money"
)

type Citation struct {
	Title       string    `json:"title" yaml:"title"`
	URL         string    `json:"url" yaml:"url"`
	Publisher   string    `json:"publisher,omitempty" yaml:"publisher,omitempty"`
	PublishedAt time.Time `json:"published_at,omitempty" yaml:"published_at,omitempty"`
	RetrievedAt time.Time `json:"retrieved_at,omitempty" yaml:"retrieved_at,omitempty"`
}

type Holding struct {
	Name     string `json:"name" yaml:"name"`
	ISIN     string `json:"isin,omitempty" yaml:"isin,omitempty"`
	Symbol   string `json:"symbol,omitempty" yaml:"symbol,omitempty"`
	WeightBP int64  `json:"weight_basis_points" yaml:"weight_basis_points"`
}

type Exposure struct {
	Name     string `json:"name" yaml:"name"`
	WeightBP int64  `json:"weight_basis_points" yaml:"weight_basis_points"`
}

type Liquidity struct {
	AverageDailyVolume money.Decimal `json:"average_daily_volume" yaml:"average_daily_volume"`
	SpreadBP           int64         `json:"spread_basis_points" yaml:"spread_basis_points"`
	Venue              string        `json:"venue,omitempty" yaml:"venue,omitempty"`
	AsOf               time.Time     `json:"as_of,omitempty" yaml:"as_of,omitempty"`
}

type PortfolioOverlap struct {
	Identifier string `json:"identifier" yaml:"identifier"`
	Name       string `json:"name" yaml:"name"`
	WeightBP   int64  `json:"weight_basis_points" yaml:"weight_basis_points"`
	Reason     string `json:"reason" yaml:"reason"`
}

type ETFReport struct {
	FundName             string             `json:"fund_name" yaml:"fund_name"`
	ISIN                 string             `json:"isin" yaml:"isin"`
	IndexTracked         string             `json:"index_tracked" yaml:"index_tracked"`
	TERBasisPoints       int64              `json:"ter_basis_points" yaml:"ter_basis_points"`
	FundSize             money.Decimal      `json:"fund_size" yaml:"fund_size"`
	FundSizeCurrency     string             `json:"fund_size_currency" yaml:"fund_size_currency"`
	Domicile             string             `json:"domicile" yaml:"domicile"`
	ReplicationMethod    string             `json:"replication_method" yaml:"replication_method"`
	DistributionPolicy   string             `json:"distribution_policy" yaml:"distribution_policy"`
	BaseCurrency         string             `json:"base_currency" yaml:"base_currency"`
	TradingCurrencies    []string           `json:"trading_currencies" yaml:"trading_currencies"`
	TopHoldings          []Holding          `json:"top_holdings" yaml:"top_holdings"`
	CountryExposure      []Exposure         `json:"country_exposure" yaml:"country_exposure"`
	SectorExposure       []Exposure         `json:"sector_exposure" yaml:"sector_exposure"`
	TrackingDifferenceBP *int64             `json:"tracking_difference_basis_points,omitempty" yaml:"tracking_difference_basis_points,omitempty"`
	Liquidity            Liquidity          `json:"liquidity" yaml:"liquidity"`
	PortfolioOverlap     []PortfolioOverlap `json:"portfolio_overlap" yaml:"portfolio_overlap"`
}

type PeriodValue struct {
	Period string        `json:"period" yaml:"period"`
	Value  money.Decimal `json:"value" yaml:"value"`
	Unit   string        `json:"unit,omitempty" yaml:"unit,omitempty"`
}

type Filing struct {
	Title       string    `json:"title" yaml:"title"`
	Kind        string    `json:"kind,omitempty" yaml:"kind,omitempty"`
	FiledAt     time.Time `json:"filed_at,omitempty" yaml:"filed_at,omitempty"`
	URL         string    `json:"url" yaml:"url"`
	Summary     string    `json:"summary,omitempty" yaml:"summary,omitempty"`
	Source      string    `json:"source,omitempty" yaml:"source,omitempty"`
	RetrievedAt time.Time `json:"retrieved_at,omitempty" yaml:"retrieved_at,omitempty"`
}

type ActivityItem struct {
	Description string    `json:"description" yaml:"description"`
	Actor       string    `json:"actor,omitempty" yaml:"actor,omitempty"`
	OccurredAt  time.Time `json:"occurred_at,omitempty" yaml:"occurred_at,omitempty"`
	URL         string    `json:"url,omitempty" yaml:"url,omitempty"`
}

type CompanyReport struct {
	BusinessSummary         string                   `json:"business_summary" yaml:"business_summary"`
	RevenueTrend            []PeriodValue            `json:"revenue_trend" yaml:"revenue_trend"`
	EarningsTrend           []PeriodValue            `json:"earnings_trend" yaml:"earnings_trend"`
	Margins                 map[string]money.Decimal `json:"margins" yaml:"margins"`
	CashFlow                map[string]money.Decimal `json:"cash_flow" yaml:"cash_flow"`
	Debt                    map[string]money.Decimal `json:"debt" yaml:"debt"`
	ValuationRatios         map[string]money.Decimal `json:"valuation_ratios" yaml:"valuation_ratios"`
	MajorCompetitors        []string                 `json:"major_competitors" yaml:"major_competitors"`
	RecentFilings           []Filing                 `json:"recent_filings" yaml:"recent_filings"`
	RecentEarnings          []PeriodValue            `json:"recent_earnings" yaml:"recent_earnings"`
	InsiderActivity         []ActivityItem           `json:"insider_activity" yaml:"insider_activity"`
	InstitutionalActivity   []ActivityItem           `json:"institutional_activity" yaml:"institutional_activity"`
	Risks                   []string                 `json:"risks" yaml:"risks"`
	Catalysts               []string                 `json:"catalysts" yaml:"catalysts"`
	PortfolioExposure       money.Decimal            `json:"portfolio_exposure" yaml:"portfolio_exposure"`
	PortfolioExposureWeight int64                    `json:"portfolio_exposure_basis_points" yaml:"portfolio_exposure_basis_points"`
}

type NewsItem struct {
	Headline    string    `json:"headline" yaml:"headline"`
	Summary     string    `json:"summary,omitempty" yaml:"summary,omitempty"`
	Publisher   string    `json:"publisher,omitempty" yaml:"publisher,omitempty"`
	URL         string    `json:"url" yaml:"url"`
	PublishedAt time.Time `json:"published_at,omitempty" yaml:"published_at,omitempty"`
	RetrievedAt time.Time `json:"retrieved_at,omitempty" yaml:"retrieved_at,omitempty"`
}

type Report struct {
	SchemaVersion int            `json:"schema_version" yaml:"schema_version"`
	ID            string         `json:"id" yaml:"id"`
	Identifier    string         `json:"identifier" yaml:"identifier"`
	ISIN          string         `json:"isin,omitempty" yaml:"isin,omitempty"`
	Symbol        string         `json:"symbol,omitempty" yaml:"symbol,omitempty"`
	Name          string         `json:"name" yaml:"name"`
	Kind          string         `json:"kind" yaml:"kind"`
	AsOf          time.Time      `json:"as_of" yaml:"as_of"`
	Summary       string         `json:"summary,omitempty" yaml:"summary,omitempty"`
	ETF           *ETFReport     `json:"etf,omitempty" yaml:"etf,omitempty"`
	Company       *CompanyReport `json:"company,omitempty" yaml:"company,omitempty"`
	News          []NewsItem     `json:"news" yaml:"news"`
	Citations     []Citation     `json:"citations" yaml:"citations"`
	Warnings      []string       `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}
