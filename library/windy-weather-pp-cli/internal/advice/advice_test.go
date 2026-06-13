// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.
package advice

import (
	"testing"

	"windy-weather-pp-cli/internal/config"
	"windy-weather-pp-cli/internal/weather"
)

func TestCalendarAdviceCreatesRainReminder(t *testing.T) {
	cfg := config.DefaultConfig()
	obs := baseObservation()
	obs.Current.RainRisk = weather.RiskLow
	obs.Forecast = []weather.ForecastPoint{
		{
			Time:             "2026-05-20T17:00:00+02:00",
			RainRisk:         weather.RiskMedium,
			ThunderstormRisk: weather.RiskLow,
			Confidence:       weather.ConfidenceMedium,
		},
	}
	out := CalendarAdvice(obs, cfg, 6)
	if !out.ShouldCreateEvent {
		t.Fatalf("expected calendar event")
	}
	if out.EventType != "weather_reminder" {
		t.Fatalf("event type = %q", out.EventType)
	}
	if out.Title != "Check rain before going out" {
		t.Fatalf("title = %q", out.Title)
	}
}

func TestCalendarAdviceSuppressesLowRisk(t *testing.T) {
	cfg := config.DefaultConfig()
	obs := baseObservation()
	out := CalendarAdvice(obs, cfg, 6)
	if out.ShouldCreateEvent {
		t.Fatalf("did not expect calendar event")
	}
	if out.EventType != "none" {
		t.Fatalf("event type = %q", out.EventType)
	}
}

func TestCalendarAdvicePrefersThunderstormWarning(t *testing.T) {
	cfg := config.DefaultConfig()
	obs := baseObservation()
	obs.Forecast = []weather.ForecastPoint{
		{
			Time:             "2026-05-20T18:00:00+02:00",
			RainRisk:         weather.RiskMedium,
			ThunderstormRisk: weather.RiskMedium,
			Confidence:       weather.ConfidenceMedium,
		},
	}
	out := CalendarAdvice(obs, cfg, 6)
	if !out.ShouldCreateEvent {
		t.Fatalf("expected calendar event")
	}
	if out.EventType != "weather_warning" {
		t.Fatalf("event type = %q", out.EventType)
	}
	if out.Title != "Thunderstorm risk near home" {
		t.Fatalf("title = %q", out.Title)
	}
}

func TestCalendarAdviceCreatesWindWarningFromForecast(t *testing.T) {
	cfg := config.DefaultConfig()
	obs := baseObservation()
	wind := 38.0
	obs.Forecast = []weather.ForecastPoint{
		{
			Time:             "2026-05-20T17:00:00+02:00",
			RainRisk:         weather.RiskLow,
			ThunderstormRisk: weather.RiskLow,
			WindSpeedKmh:     &wind,
			Confidence:       weather.ConfidenceMedium,
		},
	}
	out := CalendarAdvice(obs, cfg, 6)
	if !out.ShouldCreateEvent {
		t.Fatalf("expected calendar event")
	}
	if out.EventType != "weather_warning" {
		t.Fatalf("event type = %q", out.EventType)
	}
	if out.Title != "Strong wind near home" {
		t.Fatalf("title = %q", out.Title)
	}
}

func baseObservation() weather.Observation {
	temp := 18.2
	wind := 12.0
	return weather.Observation{
		Source:    weather.SourceWindy,
		CheckedAt: "2026-05-20T16:00:00+02:00",
		Location: weather.Location{
			Name:      "Berlin city center",
			Latitude:  52.520,
			Longitude: 13.405,
			Timezone:  "Europe/Berlin",
		},
		Current: weather.Current{
			Summary:          "No significant rain signal",
			TemperatureC:     &temp,
			RainRisk:         weather.RiskLow,
			ThunderstormRisk: weather.RiskLow,
			WindSpeedKmh:     &wind,
			Confidence:       weather.ConfidenceMedium,
		},
		Forecast: []weather.ForecastPoint{},
		RawObservations: weather.RawObservations{
			NetworkEndpointsUsed: []string{},
			Layer:                "rain",
			Notes:                []string{},
		},
		Errors: []weather.ErrorItem{},
	}
}
