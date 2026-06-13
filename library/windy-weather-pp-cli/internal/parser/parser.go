// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.
package parser

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	"windy-weather-pp-cli/internal/config"
	"windy-weather-pp-cli/internal/weather"
	"windy-weather-pp-cli/internal/windy"
)

type pointNow struct {
	Timestamp  float64 `json:"ts"`
	MM         float64 `json:"mm"`
	Rain       float64 `json:"rain"`
	SnowPrecip float64 `json:"snowPrecip"`
	ConvPrecip float64 `json:"convPrecip"`
	Temp       float64 `json:"temp"`
	Wind       float64 `json:"wind"`
	Gust       float64 `json:"gust"`
	Icon       int     `json:"icon,omitempty"`
	IsDay      float64 `json:"isDay,omitempty"`
	RhSurface  float64 `json:"rh-surface,omitempty"`
	Cbase      float64 `json:"cbase,omitempty"`
}

type pointForecast struct {
	Header forecastHeader `json:"header"`
	Data   forecastData   `json:"data"`
}

type multiModelForecast struct {
	Primary       *pointForecast
	Secondary     []*pointForecast
	Models        []string
	AgreementPct  float64
	Divergences   []string
}

type forecastHeader struct {
	Model   string `json:"model"`
	RefTime string `json:"refTime"`
	Update  string `json:"update"`
	TzName  string `json:"tzName"`
}

type forecastData struct {
	TS         []float64 `json:"ts"`
	MM         []float64 `json:"mm"`
	Rain       []float64 `json:"rain"`
	SnowPrecip []float64 `json:"snowPrecip"`
	ConvPrecip []float64 `json:"convPrecip"`
	Temp       []float64 `json:"temp"`
	Wind       []float64 `json:"wind"`
	Icon       []float64 `json:"icon"`
	IsDay      []float64 `json:"isDay"`
	Rh         []float64 `json:"rh"`
	Cbase      []float64 `json:"cbase"`
}

func Parse(result *windy.Result, cfg *config.Config, checkedAt time.Time) weather.Observation {
	obs := EmptyObservation(cfg, checkedAt, nil)
	if result == nil {
		obs.Errors = append(obs.Errors, weather.ErrorItem{Code: "scrape_failed", Message: "scraper returned no result"})
		return obs
	}

	obs.RawObservations.NetworkEndpointsUsed = uniqueEndpoints(result.EndpointsUsed)
	if len(obs.RawObservations.NetworkEndpointsUsed) == 0 {
		obs.RawObservations.NetworkEndpointsUsed = uniqueEndpointsFromResponses(result.Responses)
	}

	var now *pointNow
	var forecasts multiModelForecast
	for _, response := range result.Responses {
		if response.Status < 200 || response.Status >= 300 || response.Body == "" {
			continue
		}
		switch {
		case strings.Contains(response.URL, "/forecast/point/now/"):
			var parsed pointNow
			if err := json.Unmarshal([]byte(response.Body), &parsed); err != nil {
				obs.Errors = append(obs.Errors, weather.ErrorItem{Code: "malformed_response", Message: fmt.Sprintf("malformedpoint-now response from %s: %v", compactURL(response.URL), err)})
				continue
			}
			now = &parsed
		case strings.Contains(response.URL, "/forecast/point/ecmwf/v2.9"):
			var parsed pointForecast
			if err := json.Unmarshal([]byte(response.Body), &parsed); err != nil {
				obs.Errors = append(obs.Errors, weather.ErrorItem{Code: "malformed_response", Message: fmt.Sprintf("malformed point forecast response from %s: %v", compactURL(response.URL), err)})
				continue
			}
			forecasts.Primary = &parsed
			forecasts.Models = append(forecasts.Models, parsed.Header.Model)
		case strings.Contains(response.URL, "/forecast/point/gfs/v2.9"),
			strings.Contains(response.URL, "/forecast/point/icon/v2.9"):
			var parsed pointForecast
			if err := json.Unmarshal([]byte(response.Body), &parsed); err != nil {
				continue
			}
			forecasts.Secondary = append(forecasts.Secondary, &parsed)
			forecasts.Models = append(forecasts.Models, parsed.Header.Model)
		}
	}

	if forecasts.Primary != nil {
		if forecasts.Primary.Header.Model != "" {
			model := forecasts.Primary.Header.Model
			obs.RawObservations.Model = &model
		}
		obs.Forecast = forecastPoints(*forecasts.Primary, checkedAt.Location())
		confidence, agreementPct, divergences := computeConfidence(forecasts, obs.Forecast)
		obs.Confidence = confidence
		obs.RawObservations.ModelAgreementPct = &agreementPct
		if len(divergences) > 0 {
			obs.RawObservations.ModelDivergence = divergences
		}
	}
	if now != nil {
		obs.Current = currentFromPointNow(*now)
		obs.Current.Confidence = obs.Confidence
	}
	if now == nil && len(obs.Forecast) > 0 {
		obs.Current = currentFromForecast(obs.Forecast[0])
		obs.Current.Confidence = obs.Confidence
		obs.RawObservations.Notes = append(obs.RawObservations.Notes, "current conditions inferred from nearest point forecast because point-now data was unavailable")
	}

	if now == nil && forecasts.Primary == nil {
		obs.Current.Confidence = weather.ConfidenceLow
		obs.Confidence = weather.ConfidenceLow
		obs.Current.Summary = "No usable Windy point forecast found"
		obs.Current.RainRisk = weather.RiskUnknown
		obs.Current.ThunderstormRisk = weather.RiskUnknown
		obs.Errors = append(obs.Errors, weather.ErrorItem{Code: "no_usable_weather_endpoint", Message: "Windy loaded, but no usable public point forecast JSON endpoint was captured"})
		obs.RawObservations.Notes = append(obs.RawObservations.Notes, "Only page metadata, map tiles, or non-weather responses were available")
	} else {
		obs.RawObservations.Notes = append(obs.RawObservations.Notes, "Windy point forecast values are model data, not official precipitation probabilities")
		obs.RawObservations.Notes = append(obs.RawObservations.Notes, "Thunderstorm risk is inferred from convective precipitation where available")
		if len(forecasts.Models) > 1 {
			obs.RawObservations.Notes = append(obs.RawObservations.Notes, fmt.Sprintf("Models used: %s", strings.Join(forecasts.Models, ", ")))
		}
	}

	sort.Strings(obs.RawObservations.NetworkEndpointsUsed)
	return obs
}

func EmptyObservation(cfg *config.Config, checkedAt time.Time, errItem *weather.ErrorItem) weather.Observation {
	loc := checkedAt.Location()
	if cfg.DefaultLocation.Timezone != "" {
		if loaded, err := time.LoadLocation(cfg.DefaultLocation.Timezone); err == nil {
			loc = loaded
			checkedAt = checkedAt.In(loc)
		}
	}
	obs := weather.Observation{
		Source:    weather.SourceWindy,
		Location:  weather.LocationFromConfig(cfg.DefaultLocation.Name, cfg.DefaultLocation.Latitude, cfg.DefaultLocation.Longitude, cfg.DefaultLocation.Timezone),
		CheckedAt: checkedAt.In(loc).Format(time.RFC3339),
		Current: weather.Current{
			Summary:          "No weather data available",
			RainRisk:         weather.RiskUnknown,
			ThunderstormRisk: weather.RiskUnknown,
			Confidence:       weather.ConfidenceLow,
		},
		Forecast:   []weather.ForecastPoint{},
		Confidence: weather.ConfidenceLow,
		RawObservations: weather.RawObservations{
			NetworkEndpointsUsed: []string{},
			Layer:                "rain",
			Notes:                []string{},
		},
		Errors: []weather.ErrorItem{},
	}
	if errItem != nil {
		obs.Errors = append(obs.Errors, *errItem)
		obs.RawObservations.Notes = append(obs.RawObservations.Notes, errItem.Message)
	}
	return obs
}

func currentFromPointNow(now pointNow) weather.Current {
	precip := maxFloat(now.MM, now.Rain, now.SnowPrecip, now.ConvPrecip)
	rainRisk := rainRisk(precip)
	thunderRisk := thunderRisk(now.ConvPrecip)
	temp := weather.KelvinToC(now.Temp)
	wind := weather.MpsToKmh(now.Wind)
	out := weather.Current{
		Summary:          summary(rainRisk, thunderRisk),
		TemperatureC:     &temp,
		RainRisk:         rainRisk,
		ThunderstormRisk: thunderRisk,
		WindSpeedKmh:     &wind,
		Confidence:       weather.ConfidenceMedium,
	}
	if now.Icon > 0 {
		pct, desc := weather.IconToCloudCover(now.Icon)
		out.CloudCoverPct = &pct
		out.CloudCover = desc
	}
	if now.IsDay > 0 {
		isDay := now.IsDay > 0.5
		out.IsDay = &isDay
	}
	if now.RhSurface > 0 {
		humidity := int(weather.Round1(now.RhSurface))
		out.HumidityPct = &humidity
	}
	return out
}

func currentFromForecast(point weather.ForecastPoint) weather.Current {
	return weather.Current{
		Summary:          point.Summary,
		TemperatureC:     point.TemperatureC,
		RainRisk:         point.RainRisk,
		ThunderstormRisk: point.ThunderstormRisk,
		WindSpeedKmh:     point.WindSpeedKmh,
		CloudCoverPct:    point.CloudCoverPct,
		CloudCover:       point.CloudCover,
		IsDay:            point.IsDay,
		HumidityPct:      point.HumidityPct,
		Confidence:       point.Confidence,
	}
}

func forecastPoints(forecast pointForecast, loc *time.Location) []weather.ForecastPoint {
	data := forecast.Data
	ts := data.TS
	if len(ts) == 0 {
		return []weather.ForecastPoint{}
	}
	points := make([]weather.ForecastPoint, 0, len(ts))
	for i := range ts {
		precip := maxOptional(i, data.MM, data.Rain, data.SnowPrecip, data.ConvPrecip)
		convective := optional(i, data.ConvPrecip)
		rain := rainRisk(valueOrZero(precip))
		thunder := weather.RiskLow
		if convective != nil {
			thunder = thunderRisk(*convective)
		}
		point := weather.ForecastPoint{
			Time:                      time.UnixMilli(int64(ts[i])).In(loc).Format(time.RFC3339),
			Summary:                   summary(rain, thunder),
			PrecipitationMm:           precip,
			ConvectivePrecipitationMm: convective,
			RainRisk:                  rain,
			ThunderstormRisk:          thunder,
			Confidence:                weather.ConfidenceMedium,
		}
		if temp := optional(i, data.Temp); temp != nil {
			point.TemperatureC = weather.Ptr(weather.KelvinToC(*temp))
		}
		if wind := optional(i, data.Wind); wind != nil {
			point.WindSpeedKmh = weather.Ptr(weather.MpsToKmh(*wind))
		}
		if icon := optional(i, data.Icon); icon != nil {
			iconInt := int(*icon)
			pct, desc := weather.IconToCloudCover(iconInt)
			point.CloudCoverPct = &pct
			point.CloudCover = desc
		}
		if isDay := optional(i, data.IsDay); isDay != nil {
			isDayBool := *isDay > 0.5
			point.IsDay = &isDayBool
		}
		if rh := optional(i, data.Rh); rh != nil {
			humidity := int(*rh)
			point.HumidityPct = &humidity
		}
		points = append(points, point)
	}
	return points
}

func rainRisk(precipMm float64) string {
	switch {
	case precipMm >= 3:
		return weather.RiskHigh
	case precipMm >= 0.2:
		return weather.RiskMedium
	case precipMm > 0:
		return weather.RiskLow
	default:
		return weather.RiskLow
	}
}

func thunderRisk(convectiveMm float64) string {
	switch {
	case convectiveMm >= 5:
		return weather.RiskHigh
	case convectiveMm >= 2:
		return weather.RiskMedium
	case convectiveMm > 0:
		return weather.RiskLow
	default:
		return weather.RiskLow
	}
}

func summary(rain, thunder string) string {
	if weather.RiskAtLeast(thunder, weather.RiskMedium) {
		return "Thunderstorm risk nearby"
	}
	if weather.RiskAtLeast(rain, weather.RiskHigh) {
		return "Rain likely nearby"
	}
	if weather.RiskAtLeast(rain, weather.RiskMedium) {
		return "Rain nearby"
	}
	return "No significant rain signal"
}

func optional(index int, values []float64) *float64 {
	if index < 0 || index >= len(values) {
		return nil
	}
	v := weather.Round1(values[index])
	return &v
}

func maxOptional(index int, series ...[]float64) *float64 {
	var found bool
	var max float64
	for _, values := range series {
		if index < 0 || index >= len(values) {
			continue
		}
		if !found || values[index] > max {
			found = true
			max = values[index]
		}
	}
	if !found {
		return nil
	}
	max = weather.Round1(max)
	return &max
}

func valueOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func maxFloat(values ...float64) float64 {
	var max float64
	for i, v := range values {
		if i == 0 || v > max {
			max = v
		}
	}
	return max
}

func uniqueEndpointsFromResponses(responses []windy.NetworkResponse) []string {
	values := make([]string, 0, len(responses))
	for _, response := range responses {
		if response.Status >= 200 && response.Status < 300 {
			values = append(values, compactURL(response.URL))
		}
	}
	return uniqueEndpoints(values)
}

func uniqueEndpoints(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = compactURL(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func compactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	parsed.RawQuery = ""
	return parsed.String()
}

func computeConfidence(forecasts multiModelForecast, primaryPoints []weather.ForecastPoint) (string, float64, []string) {
	if len(forecasts.Secondary) == 0 {
		return weather.ConfidenceMedium, 0, nil
	}

	agreementCount := 0
	totalChecks := 0
	divergences := []string{}

	for i, point := range primaryPoints {
		primaryRain := rainRiskValue(point.PrecipitationMm)
		primaryRisk := point.RainRisk

		for si, sec := range forecasts.Secondary {
			if i >= len(sec.Data.Rain) {
				continue
			}
			secRain := sec.Data.Rain[i]
			secRisk := rainRisk(secRain)

			totalChecks++
			if primaryRisk == secRisk {
				agreementCount++
			} else if math.Abs(primaryRain-secRain) <= 0.5 {
				agreementCount++
			} else {
				secModel := forecasts.Models[si+1]
				if primaryRain > 0 || secRain > 0 {
					timeStr := ""
					if i < len(primaryPoints) {
						timeStr = primaryPoints[i].Time
					}
					div := fmt.Sprintf("%s predicts %.1fmm rain at %s, %s predicts %.1fmm",
						forecasts.Models[0], primaryRain, timeStr, secModel, secRain)
					divergences = append(divergences, div)
				}
			}
		}
	}

	if totalChecks == 0 {
		return weather.ConfidenceMedium, 0, nil
	}

	agreementPct := float64(agreementCount) / float64(totalChecks) * 100

	var confidence string
	switch {
	case agreementPct >= 80:
		confidence = weather.ConfidenceHigh
	case agreementPct >= 50:
		confidence = weather.ConfidenceMedium
	default:
		confidence = weather.ConfidenceLow
	}

	return confidence, agreementPct, divergences
}

func rainRiskValue(precip *float64) float64 {
	if precip == nil {
		return 0
	}
	return *precip
}
