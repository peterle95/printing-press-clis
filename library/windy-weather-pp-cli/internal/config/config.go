// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Location struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Zoom      int     `json:"zoom"`
	Timezone  string  `json:"timezone"`
	WindyURL  string  `json:"windy_url"`
}

type Thresholds struct {
	RainRisk         string  `json:"rain_risk"`
	StrongWindKmh    float64 `json:"strong_wind_kmh"`
	ColdTemperatureC float64 `json:"cold_temperature_c"`
	HotTemperatureC  float64 `json:"hot_temperature_c"`
}

type Cache struct {
	Enabled    bool `json:"enabled"`
	TTLMinutes int  `json:"ttl_minutes"`
}

type Browser struct {
	Headless  bool `json:"headless"`
	TimeoutMs int  `json:"timeout_ms"`
}

type Config struct {
	DefaultLocation Location   `json:"default_location"`
	Thresholds      Thresholds `json:"thresholds"`
	Cache           Cache      `json:"cache"`
	Browser         Browser    `json:"browser"`
	Path            string     `json:"-"`
}

func DefaultConfig() *Config {
	return &Config{
		DefaultLocation: Location{
			Name:      "Berlin city center",
			Latitude:  52.520,
			Longitude: 13.405,
			Zoom:      10,
			Timezone:  "Europe/Berlin",
			WindyURL:  "https://www.windy.com/-Rain-thunder-rain?rain,52.520,13.405,10,p:cities,m:e6Fagxs",
		},
		Thresholds: Thresholds{
			RainRisk:         "medium",
			StrongWindKmh:    35,
			ColdTemperatureC: 5,
			HotTemperatureC:  30,
		},
		Cache: Cache{
			Enabled:    true,
			TTLMinutes: 20,
		},
		Browser: Browser{
			Headless:  true,
			TimeoutMs: 45000,
		},
	}
}

func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	path := configPath
	if path == "" {
		path = os.Getenv("WINDY_WEATHER_CONFIG")
	}
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".config", "windy-weather-pp-cli", "config.json")
	}
	cfg.Path = path

	// If file exists, read and parse it
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config JSON %s: %w", path, err)
		}
	} else if os.IsNotExist(err) {
		// Save default config so it exists for editing
		if err := cfg.Save(); err != nil {
			// Don't fail completely, just log or continue
		}
	}

	return cfg, nil
}

func (c *Config) Save() error {
	dir := filepath.Dir(c.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(c.Path, data, 0o600)
}
