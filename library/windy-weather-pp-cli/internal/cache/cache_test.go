// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.
package cache

import (
	"path/filepath"
	"testing"
	"time"
)

type cacheSample struct {
	CheckedAt time.Time `json:"checked_at"`
	Value     string    `json:"value"`
}

func TestCacheSaveLoadAndValidity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "weather-cache.json")
	sample := &cacheSample{CheckedAt: time.Now().Add(-5 * time.Minute), Value: "fresh"}
	if err := Save(path, sample); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !IsValid(path, 20) {
		t.Fatalf("cache should be valid")
	}
	loaded, err := Load[cacheSample](path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Value != "fresh" {
		t.Fatalf("loaded value = %q", loaded.Value)
	}
}

func TestCacheExpired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "weather-cache.json")
	sample := &cacheSample{CheckedAt: time.Now().Add(-30 * time.Minute), Value: "old"}
	if err := Save(path, sample); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if IsValid(path, 20) {
		t.Fatalf("cache should be expired")
	}
}
