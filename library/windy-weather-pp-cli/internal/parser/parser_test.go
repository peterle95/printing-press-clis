// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.
package parser

import (
	"fmt"
	"testing"
	"time"

	"windy-weather-pp-cli/internal/config"
	"windy-weather-pp-cli/internal/weather"
	"windy-weather-pp-cli/internal/windy"
)

func TestParseNormalizesWindyPointData(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	checkedAt := time.Date(2026, 5, 20, 16, 0, 0, 0, loc)
	first := checkedAt.UnixMilli()
	second := checkedAt.Add(3 * time.Hour).UnixMilli()
	nowBody := `{"ts":1779285600000,"mm":1.2,"rain":0,"snowPrecip":0,"convPrecip":1.1,"temp":290.62,"wind":5.1}`
	forecastBody := fmt.Sprintf(`{
  "header": {"model": "ECMWF", "refTime": "2026-05-20T00:00:00Z", "tzName": "Europe/Berlin"},
  "data": {
    "ts": [%d, %d],
    "mm": [1.2, 0],
    "rain": [0, 0],
    "snowPrecip": [0, 0],
    "convPrecip": [1.1, 0],
    "temp": [290.62, 289.0],
    "wind": [5.1, 3.0]
  }
}`, first, second)

	result := &windy.Result{
		Success: true,
		EndpointsUsed: []string{
			"https://node.windy.com/forecast/point/now/ecmwf/v1.0/52.520/13.405?refTime=2026052000",
			"https://node.windy.com/forecast/point/ecmwf/v2.9/52.520/13.405?refTime=2026052000&step=3",
		},
		Responses: []windy.NetworkResponse{
			{URL: "https://node.windy.com/forecast/point/now/ecmwf/v1.0/52.520/13.405?refTime=2026052000", Status: 200, ContentType: "application/json", Body: nowBody},
			{URL: "https://node.windy.com/forecast/point/ecmwf/v2.9/52.520/13.405?refTime=2026052000&step=3", Status: 200, ContentType: "application/json", Body: forecastBody},
		},
	}

	obs := Parse(result, config.DefaultConfig(), checkedAt)
	if len(obs.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", obs.Errors)
	}
	if obs.Current.Summary != "Rain nearby" {
		t.Fatalf("summary = %q", obs.Current.Summary)
	}
	if obs.Current.RainRisk != weather.RiskMedium {
		t.Fatalf("rain risk = %q", obs.Current.RainRisk)
	}
	if obs.Current.ThunderstormRisk != weather.RiskLow {
		t.Fatalf("thunder risk = %q", obs.Current.ThunderstormRisk)
	}
	if obs.Current.TemperatureC == nil || *obs.Current.TemperatureC != 17.5 {
		t.Fatalf("temperature = %v", obs.Current.TemperatureC)
	}
	if obs.Current.WindSpeedKmh == nil || *obs.Current.WindSpeedKmh != 18.4 {
		t.Fatalf("wind = %v", obs.Current.WindSpeedKmh)
	}
	if obs.RawObservations.Model == nil || *obs.RawObservations.Model != "ECMWF" {
		t.Fatalf("model = %v", obs.RawObservations.Model)
	}
	if len(obs.Forecast) != 2 {
		t.Fatalf("forecast length = %d", len(obs.Forecast))
	}
}

func TestParseReportsNoUsableEndpoint(t *testing.T) {
	obs := Parse(&windy.Result{Success: true}, config.DefaultConfig(), time.Now())
	if len(obs.Errors) == 0 {
		t.Fatalf("expected no usable endpoint error")
	}
	if obs.Current.Confidence != weather.ConfidenceLow {
		t.Fatalf("confidence = %q", obs.Current.Confidence)
	}
}
