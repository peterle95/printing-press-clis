// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaultConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DefaultLocation.Name != "Berlin city center" {
		t.Fatalf("default location name = %q", cfg.DefaultLocation.Name)
	}
	if cfg.DefaultLocation.Latitude != 52.520 || cfg.DefaultLocation.Longitude != 13.405 {
		t.Fatalf("default coordinates = %v,%v", cfg.DefaultLocation.Latitude, cfg.DefaultLocation.Longitude)
	}
	if cfg.Cache.TTLMinutes != 20 {
		t.Fatalf("default cache ttl = %d", cfg.Cache.TTLMinutes)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("default config was not written: %v", err)
	}
}

func TestLoadMergesCustomConfigWithDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{
  "thresholds": {
    "rain_risk": "high",
    "strong_wind_kmh": 42
  },
  "cache": {
    "enabled": false
  }
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Thresholds.RainRisk != "high" {
		t.Fatalf("rain threshold = %q", cfg.Thresholds.RainRisk)
	}
	if cfg.Thresholds.StrongWindKmh != 42 {
		t.Fatalf("wind threshold = %v", cfg.Thresholds.StrongWindKmh)
	}
	if cfg.Thresholds.HotTemperatureC != 30 {
		t.Fatalf("missing default was not preserved, hot temp = %v", cfg.Thresholds.HotTemperatureC)
	}
	if cfg.Cache.Enabled {
		t.Fatalf("cache enabled should be false")
	}
	if cfg.Cache.TTLMinutes != 20 {
		t.Fatalf("missing cache ttl default was not preserved, ttl = %d", cfg.Cache.TTLMinutes)
	}
}
