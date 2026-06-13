// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.
package advice

import (
	"fmt"
	"time"

	"windy-weather-pp-cli/internal/config"
	"windy-weather-pp-cli/internal/weather"
)

func CalendarAdvice(obs weather.Observation, cfg *config.Config, hours int) weather.CalendarAdvice {
	if hours <= 0 {
		hours = 6
	}
	rain := AggregateRain(obs, hours)
	current := obs.Current
	suggested := suggestedTime(obs.CheckedAt)

	out := weather.CalendarAdvice{
		ShouldCreateEvent: false,
		EventType:         "none",
		Title:             "",
		SuggestedTime:     nil,
		Reason:            "No medium-or-higher rain, thunderstorm, or wind risk was detected near home in the configured period.",
		Confidence:        rain.Confidence,
		Source:            obs.Source,
		Weather: weather.AdviceWeather{
			RainRisk:         rain.RainRisk,
			ThunderstormRisk: rain.ThunderstormRisk,
			TemperatureC:     current.TemperatureC,
			WindSpeedKmh:     current.WindSpeedKmh,
		},
		CheckedAt: obs.CheckedAt,
		Location:  &obs.Location,
		Errors:    obs.Errors,
	}

	strongWind := current.WindSpeedKmh != nil && *current.WindSpeedKmh >= cfg.Thresholds.StrongWindKmh
	for _, point := range rain.Forecast {
		if point.WindSpeedKmh != nil && *point.WindSpeedKmh >= cfg.Thresholds.StrongWindKmh {
			strongWind = true
			break
		}
	}
	switch {
	case weather.RiskAtLeast(rain.ThunderstormRisk, weather.RiskMedium):
		out.ShouldCreateEvent = true
		out.EventType = "weather_warning"
		out.Title = "Thunderstorm risk near home"
		out.SuggestedTime = &suggested
		out.Reason = "Windy rain/thunder data indicates medium or higher thunderstorm risk near home in the next configured period."
	case strongWind:
		out.ShouldCreateEvent = true
		out.EventType = "weather_warning"
		out.Title = "Strong wind near home"
		out.SuggestedTime = &suggested
		out.Reason = "Windy point forecast indicates strong wind near home."
	case weather.RiskAtLeast(rain.RainRisk, cfg.Thresholds.RainRisk):
		out.ShouldCreateEvent = true
		out.EventType = "weather_reminder"
		out.Title = "Check rain before going out"
		out.SuggestedTime = &suggested
		out.Reason = "Windy rain layer indicates medium or higher rain risk near home in the next configured period."
	}
	return out
}

func AggregateRain(obs weather.Observation, hours int) weather.RainOutput {
	checkedAt, _ := time.Parse(time.RFC3339, obs.CheckedAt)
	if hours <= 0 {
		hours = 6
	}
	until := checkedAt.Add(time.Duration(hours) * time.Hour)
	points := make([]weather.ForecastPoint, 0)
	rainRisk := obs.Current.RainRisk
	thunderRisk := obs.Current.ThunderstormRisk
	confidence := obs.Confidence
	for _, point := range obs.Forecast {
		pointTime, err := time.Parse(time.RFC3339, point.Time)
		if err != nil {
			continue
		}
		if pointTime.Before(checkedAt.Add(-90*time.Minute)) || pointTime.After(until) {
			continue
		}
		points = append(points, point)
		rainRisk = weather.MaxRisk(rainRisk, point.RainRisk)
		thunderRisk = weather.MaxRisk(thunderRisk, point.ThunderstormRisk)
	}
	return weather.RainOutput{
		Source:           obs.Source,
		Location:         obs.Location,
		CheckedAt:        obs.CheckedAt,
		PeriodHours:      hours,
		Summary:          rainSummary(rainRisk, thunderRisk),
		RainRisk:         rainRisk,
		ThunderstormRisk: thunderRisk,
		Confidence:       confidence,
		Forecast:         points,
		RawObservations:  obs.RawObservations,
		Errors:           obs.Errors,
	}
}

func DayForecast(obs weather.Observation, dayName string, offsetDays int) weather.DayForecastOutput {
	checkedAt, _ := time.Parse(time.RFC3339, obs.CheckedAt)
	target := checkedAt.AddDate(0, 0, offsetDays)
	date := target.Format("2006-01-02")
	points := make([]weather.ForecastPoint, 0)
	rainRisk := weather.RiskLow
	thunderRisk := weather.RiskLow
	confidence := obs.Confidence
	var minTemp, maxTemp, maxWind *float64
	var cloudCover string
	var avgCloudPct int
	cloudCount := 0
	for _, point := range obs.Forecast {
		pointTime, err := time.Parse(time.RFC3339, point.Time)
		if err != nil || pointTime.Format("2006-01-02") != date {
			continue
		}
		points = append(points, point)
		rainRisk = weather.MaxRisk(rainRisk, point.RainRisk)
		thunderRisk = weather.MaxRisk(thunderRisk, point.ThunderstormRisk)
		if point.TemperatureC != nil {
			if minTemp == nil || *point.TemperatureC < *minTemp {
				v := *point.TemperatureC
				minTemp = &v
			}
			if maxTemp == nil || *point.TemperatureC > *maxTemp {
				v := *point.TemperatureC
				maxTemp = &v
			}
		}
		if point.WindSpeedKmh != nil && (maxWind == nil || *point.WindSpeedKmh > *maxWind) {
			v := *point.WindSpeedKmh
			maxWind = &v
		}
		if point.CloudCoverPct != nil {
			avgCloudPct += *point.CloudCoverPct
			cloudCount++
		}
	}
	if len(points) == 0 {
		rainRisk = obs.Current.RainRisk
		thunderRisk = obs.Current.ThunderstormRisk
	}
	if cloudCount > 0 {
		avgCloudPct = avgCloudPct / cloudCount
		_, cloudCover = weather.IconToCloudCover(cloudCoverFromPct(avgCloudPct))
	}
	return weather.DayForecastOutput{
		Source:           obs.Source,
		Location:         obs.Location,
		CheckedAt:        obs.CheckedAt,
		Day:              dayName,
		Date:             date,
		Summary:          daySummary(rainRisk, thunderRisk, cloudCover),
		RainRisk:         rainRisk,
		ThunderstormRisk: thunderRisk,
		TemperatureMinC:  minTemp,
		TemperatureMaxC:  maxTemp,
		WindSpeedMaxKmh:  maxWind,
		Confidence:       confidence,
		Forecast:         points,
		RawObservations:  obs.RawObservations,
		Errors:           obs.Errors,
	}
}

func cloudCoverFromPct(pct int) int {
	switch {
	case pct <= 10:
		return 1
	case pct <= 35:
		return 2
	case pct <= 65:
		return 3
	default:
		return 4
	}
}

func daySummary(rainRisk, thunderRisk, cloudCover string) string {
	if weather.RiskAtLeast(thunderRisk, weather.RiskMedium) {
		return "Thunderstorms are possible"
	}
	if weather.RiskAtLeast(rainRisk, weather.RiskHigh) {
		return "Rain is likely"
	}
	if weather.RiskAtLeast(rainRisk, weather.RiskMedium) {
		return "Rain is possible"
	}
	switch cloudCover {
	case "clear":
		return "Sunny and clear"
	case "mostly_clear":
		return "Mostly sunny"
	case "partly_cloudy":
		return "Partly cloudy"
	case "overcast":
		return "Overcast"
	case "light_rain_showers", "rain_showers", "heavy_rain_showers":
		return "Showers possible"
	case "thunderstorm":
		return "Thunderstorms are possible"
	case "fog", "haze":
		return "Foggy or hazy"
	default:
		return "No significant rain signal"
	}
}

func suggestedTime(checkedAt string) string {
	parsed, err := time.Parse(time.RFC3339, checkedAt)
	if err != nil {
		return time.Now().Add(time.Hour).Format(time.RFC3339)
	}
	return parsed.Add(time.Hour).Truncate(time.Minute).Format(time.RFC3339)
}

func rainSummary(rainRisk, thunderRisk string) string {
	if weather.RiskAtLeast(thunderRisk, weather.RiskMedium) {
		return "Thunderstorms are possible"
	}
	if weather.RiskAtLeast(rainRisk, weather.RiskHigh) {
		return "Rain is likely"
	}
	if weather.RiskAtLeast(rainRisk, weather.RiskMedium) {
		return "Rain is possible"
	}
	return "No significant rain signal"
}

func WeekForecast(obs weather.Observation) weather.WeekForecastOutput {
	checkedAt, _ := time.Parse(time.RFC3339, obs.CheckedAt)
	days := make([]weather.WeekDaySummary, 0, 7)
	dayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

	for offset := 0; offset < 7; offset++ {
		target := checkedAt.AddDate(0, 0, offset)
		date := target.Format("2006-01-02")
		points := make([]weather.ForecastPoint, 0)
		rainRisk := weather.RiskLow
		thunderRisk := weather.RiskLow
		var minTemp, maxTemp, maxWind, maxPrecip *float64
		avgCloudPct := 0
		cloudCount := 0

		for _, point := range obs.Forecast {
			pointTime, err := time.Parse(time.RFC3339, point.Time)
			if err != nil || pointTime.Format("2006-01-02") != date {
				continue
			}
			points = append(points, point)
			rainRisk = weather.MaxRisk(rainRisk, point.RainRisk)
			thunderRisk = weather.MaxRisk(thunderRisk, point.ThunderstormRisk)
			if point.TemperatureC != nil {
				if minTemp == nil || *point.TemperatureC < *minTemp {
					v := *point.TemperatureC
					minTemp = &v
				}
				if maxTemp == nil || *point.TemperatureC > *maxTemp {
					v := *point.TemperatureC
					maxTemp = &v
				}
			}
			if point.WindSpeedKmh != nil && (maxWind == nil || *point.WindSpeedKmh > *maxWind) {
				v := *point.WindSpeedKmh
				maxWind = &v
			}
			if point.PrecipitationMm != nil && (maxPrecip == nil || *point.PrecipitationMm > *maxPrecip) {
				v := *point.PrecipitationMm
				maxPrecip = &v
			}
			if point.CloudCoverPct != nil {
				avgCloudPct += *point.CloudCoverPct
				cloudCount++
			}
		}

		cloudCover := "unknown"
		if cloudCount > 0 {
			avgCloudPct = avgCloudPct / cloudCount
			_, cloudCover = weather.IconToCloudCover(cloudCoverFromPct(avgCloudPct))
		}

		summary := daySummary(rainRisk, thunderRisk, cloudCover)
		dayName := dayNames[target.Weekday()]

		score := weatherScore(rainRisk, thunderRisk, avgCloudPct, maxWind, maxPrecip)

		days = append(days, weather.WeekDaySummary{
			DayName:          dayName + " " + target.Format("02.01."),
			Date:             date,
			Summary:          summary,
			RainRisk:         rainRisk,
			ThunderstormRisk: thunderRisk,
			TemperatureMinC:  minTemp,
			TemperatureMaxC:  maxTemp,
			WindSpeedMaxKmh:  maxWind,
			MaxPrecipMm:      maxPrecip,
			CloudCover:       cloudCover,
			CloudCoverPct:    avgCloudPct,
			Score:            score,
		})
	}

	bestDays := findBestDays(days)

	return weather.WeekForecastOutput{
		Source:          obs.Source,
		Location:        obs.Location,
		CheckedAt:       obs.CheckedAt,
		Days:            days,
		BestDays:        bestDays,
		Confidence:      obs.Confidence,
		RawObservations: obs.RawObservations,
		Errors:          obs.Errors,
	}
}

func weatherScore(rainRisk, thunderRisk string, cloudPct int, maxWind *float64, maxPrecip *float64) int {
	if maxPrecip == nil && maxWind == nil && cloudPct == 0 {
		return -1
	}
	score := 100
	if weather.RiskAtLeast(thunderRisk, weather.RiskHigh) {
		score -= 50
	} else if weather.RiskAtLeast(thunderRisk, weather.RiskMedium) {
		score -= 30
	}
	if weather.RiskAtLeast(rainRisk, weather.RiskHigh) {
		score -= 40
	} else if weather.RiskAtLeast(rainRisk, weather.RiskMedium) {
		score -= 20
	} else if rainRisk == weather.RiskLow && maxPrecip != nil && *maxPrecip > 0 {
		score -= 5
	}
	score -= cloudPct / 5
	if maxWind != nil {
		if *maxWind >= 40 {
			score -= 20
		} else if *maxWind >= 25 {
			score -= 10
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

func findBestDays(days []weather.WeekDaySummary) []weather.BestDay {
	type dayScore struct {
		idx   int
		score int
	}
	ranked := make([]dayScore, len(days))
	for i, d := range days {
		ranked[i] = dayScore{i, d.Score}
	}
	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].score > ranked[i].score {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}

	best := make([]weather.BestDay, 0, 3)
	for _, s := range ranked {
		if len(best) >= 3 {
			break
		}
		d := days[s.idx]
		reason := d.Summary
		if d.TemperatureMinC != nil && d.TemperatureMaxC != nil {
			reason += fmt.Sprintf(", %.0f-%.0f°C", *d.TemperatureMinC, *d.TemperatureMaxC)
		}
		best = append(best, weather.BestDay{
			DayName: d.DayName,
			Date:    d.Date,
			Reason:  reason,
		})
	}
	return best
}
