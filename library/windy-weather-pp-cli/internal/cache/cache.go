// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.
package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CacheHeader struct {
	CheckedAt time.Time `json:"checked_at"`
}

func GetCachePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "windy-weather-pp-cli", "weather-cache.json")
}

func IsValid(path string, ttlMinutes int) bool {
	if ttlMinutes <= 0 {
		return false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var header CacheHeader
	if err := json.Unmarshal(data, &header); err != nil {
		var flexible struct {
			CheckedAt string `json:"checked_at"`
		}
		if err := json.Unmarshal(data, &flexible); err != nil {
			return false
		}
		checkedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(flexible.CheckedAt))
		if err != nil {
			return false
		}
		header.CheckedAt = checkedAt
	}
	if header.CheckedAt.IsZero() {
		return false
	}

	age := time.Since(header.CheckedAt)
	return age < time.Duration(ttlMinutes)*time.Minute
}

func Load[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading cache file: %w", err)
	}

	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing cache JSON: %w", err)
	}

	return &result, nil
}

func Save[T any](path string, data *T) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	serialized, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cache data: %w", err)
	}

	return os.WriteFile(path, serialized, 0o600)
}
