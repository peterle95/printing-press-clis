// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.
package weather

import "math"

const (
	SourceWindy = "windy.com"

	RiskUnknown = "unknown"
	RiskLow     = "low"
	RiskMedium  = "medium"
	RiskHigh    = "high"

	ConfidenceLow    = "low"
	ConfidenceMedium = "medium"
	ConfidenceHigh   = "high"
)

type Location struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
}

type ErrorItem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Current struct {
	Summary          string   `json:"summary"`
	TemperatureC     *float64 `json:"temperature_c"`
	RainRisk         string   `json:"rain_risk"`
	ThunderstormRisk string   `json:"thunderstorm_risk"`
	WindSpeedKmh     *float64 `json:"wind_speed_kmh"`
	CloudCoverPct    *int     `json:"cloud_cover_pct,omitempty"`
	CloudCover       string   `json:"cloud_cover,omitempty"`
	IsDay            *bool    `json:"is_day,omitempty"`
	HumidityPct      *int     `json:"humidity_pct,omitempty"`
	Confidence       string   `json:"confidence"`
}

type ForecastPoint struct {
	Time                      string   `json:"time"`
	Summary                   string   `json:"summary"`
	TemperatureC              *float64 `json:"temperature_c"`
	PrecipitationMm           *float64 `json:"precipitation_mm"`
	ConvectivePrecipitationMm *float64 `json:"convective_precipitation_mm"`
	RainRisk                  string   `json:"rain_risk"`
	ThunderstormRisk          string   `json:"thunderstorm_risk"`
	WindSpeedKmh              *float64 `json:"wind_speed_kmh"`
	CloudCoverPct             *int     `json:"cloud_cover_pct,omitempty"`
	CloudCover                string   `json:"cloud_cover,omitempty"`
	IsDay                     *bool    `json:"is_day,omitempty"`
	HumidityPct               *int     `json:"humidity_pct,omitempty"`
	Confidence                string   `json:"confidence"`
}

type RawObservations struct {
	NetworkEndpointsUsed []string `json:"network_endpoints_used"`
	Model                *string  `json:"model"`
	Layer                string   `json:"layer"`
	Notes                []string `json:"notes"`
	ModelAgreementPct    *float64 `json:"model_agreement_pct,omitempty"`
	ModelDivergence      []string `json:"model_divergence,omitempty"`
}

type Observation struct {
	Source          string          `json:"source"`
	Location        Location        `json:"location"`
	CheckedAt       string          `json:"checked_at"`
	Current         Current         `json:"current"`
	Forecast        []ForecastPoint `json:"forecast,omitempty"`
	Confidence      string          `json:"confidence"`
	RawObservations RawObservations `json:"raw_observations"`
	Errors          []ErrorItem     `json:"errors"`
}

type NowOutput struct {
	Source          string          `json:"source"`
	Location        Location        `json:"location"`
	CheckedAt       string          `json:"checked_at"`
	Current         Current         `json:"current"`
	Confidence      string          `json:"confidence"`
	RawObservations RawObservations `json:"raw_observations"`
	Errors          []ErrorItem     `json:"errors"`
}

type RainOutput struct {
	Source           string          `json:"source"`
	Location         Location        `json:"location"`
	CheckedAt        string          `json:"checked_at"`
	PeriodHours      int             `json:"period_hours"`
	Summary          string          `json:"summary"`
	RainRisk         string          `json:"rain_risk"`
	ThunderstormRisk string          `json:"thunderstorm_risk"`
	Confidence       string          `json:"confidence"`
	Forecast         []ForecastPoint `json:"forecast"`
	RawObservations  RawObservations `json:"raw_observations"`
	Errors           []ErrorItem     `json:"errors"`
}

type DayForecastOutput struct {
	Source           string          `json:"source"`
	Location         Location        `json:"location"`
	CheckedAt        string          `json:"checked_at"`
	Day              string          `json:"day"`
	Date             string          `json:"date"`
	Summary          string          `json:"summary"`
	RainRisk         string          `json:"rain_risk"`
	ThunderstormRisk string          `json:"thunderstorm_risk"`
	TemperatureMinC  *float64        `json:"temperature_min_c"`
	TemperatureMaxC  *float64        `json:"temperature_max_c"`
	WindSpeedMaxKmh  *float64        `json:"wind_speed_max_kmh"`
	Confidence       string          `json:"confidence"`
	Forecast         []ForecastPoint `json:"forecast"`
	RawObservations  RawObservations `json:"raw_observations"`
	Errors           []ErrorItem     `json:"errors"`
}

type WeekDaySummary struct {
	DayName          string   `json:"day_name"`
	Date             string   `json:"date"`
	Summary          string   `json:"summary"`
	RainRisk         string   `json:"rain_risk"`
	ThunderstormRisk string   `json:"thunderstorm_risk"`
	TemperatureMinC  *float64 `json:"temperature_min_c"`
	TemperatureMaxC  *float64 `json:"temperature_max_c"`
	WindSpeedMaxKmh  *float64 `json:"wind_speed_max_kmh"`
	MaxPrecipMm      *float64 `json:"max_precip_mm"`
	CloudCover       string   `json:"cloud_cover"`
	CloudCoverPct    int      `json:"cloud_cover_pct"`
	Score            int      `json:"score"`
}

type BestDay struct {
	DayName string `json:"day_name"`
	Date    string `json:"date"`
	Reason  string `json:"reason"`
}

type WeekForecastOutput struct {
	Source          string            `json:"source"`
	Location        Location          `json:"location"`
	CheckedAt       string            `json:"checked_at"`
	Days            []WeekDaySummary  `json:"days"`
	BestDays        []BestDay         `json:"best_days"`
	Confidence      string            `json:"confidence"`
	RawObservations RawObservations   `json:"raw_observations"`
	Errors          []ErrorItem       `json:"errors"`
}

type AdviceWeather struct {
	RainRisk         string   `json:"rain_risk"`
	ThunderstormRisk string   `json:"thunderstorm_risk"`
	TemperatureC     *float64 `json:"temperature_c"`
	WindSpeedKmh     *float64 `json:"wind_speed_kmh"`
}

type CalendarAdvice struct {
	ShouldCreateEvent bool          `json:"should_create_event"`
	EventType         string        `json:"event_type"`
	Title             string        `json:"title"`
	SuggestedTime     *string       `json:"suggested_time"`
	Reason            string        `json:"reason"`
	Confidence        string        `json:"confidence"`
	Source            string        `json:"source"`
	Weather           AdviceWeather `json:"weather"`
	CheckedAt         string        `json:"checked_at,omitempty"`
	Location          *Location     `json:"location,omitempty"`
	Errors            []ErrorItem   `json:"errors,omitempty"`
}

func LocationFromConfig(name string, lat, lon float64, timezone string) Location {
	return Location{
		Name:      name,
		Latitude:  lat,
		Longitude: lon,
		Timezone:  timezone,
	}
}

func NowOutputFromObservation(obs Observation) NowOutput {
	return NowOutput{
		Source:          obs.Source,
		Location:        obs.Location,
		CheckedAt:       obs.CheckedAt,
		Current:         obs.Current,
		Confidence:      obs.Confidence,
		RawObservations: obs.RawObservations,
		Errors:          obs.Errors,
	}
}

func Round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func KelvinToC(k float64) float64 {
	return Round1(k - 273.15)
}

func MpsToKmh(mps float64) float64 {
	return Round1(mps * 3.6)
}

func Ptr(v float64) *float64 {
	v = Round1(v)
	return &v
}

func RiskRank(risk string) int {
	switch risk {
	case RiskLow:
		return 1
	case RiskMedium:
		return 2
	case RiskHigh:
		return 3
	default:
		return 0
	}
}

func MaxRisk(values ...string) string {
	best := RiskUnknown
	for _, value := range values {
		if RiskRank(value) > RiskRank(best) {
			best = value
		}
	}
	if best == RiskUnknown {
		return RiskLow
	}
	return best
}

func RiskAtLeast(risk, threshold string) bool {
	return RiskRank(risk) >= RiskRank(threshold)
}

func IconToCloudCover(icon int) (pct int, description string) {
	switch icon {
	case 1:
		return 5, "clear"
	case 2:
		return 25, "mostly_clear"
	case 3:
		return 50, "partly_cloudy"
	case 4:
		return 90, "overcast"
	case 5:
		return 100, "fog"
	case 6:
		return 75, "light_rain"
	case 7:
		return 85, "rain"
	case 8:
		return 95, "heavy_rain"
	case 9:
		return 90, "thunderstorm"
	case 10:
		return 85, "snow"
	case 11:
		return 75, "light_snow"
	case 12:
		return 95, "heavy_snow"
	case 13:
		return 90, "sleet"
	case 14:
		return 70, "light_rain_showers"
	case 15:
		return 80, "rain_showers"
	case 16:
		return 85, "heavy_rain_showers"
	case 17:
		return 60, "haze"
	case 18:
		return 80, "rain_showers"
	case 19:
		return 85, "thunderstorm"
	case 20:
		return 90, "heavy_rain"
	case 21:
		return 90, "snow_showers"
	case 22:
		return 95, "heavy_snow_showers"
	default:
		if icon > 20 {
			return 80, "precipitation"
		}
		return 50, "unknown"
	}
}
