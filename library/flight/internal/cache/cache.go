package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"flight-pp-cli/internal/flight"
)

type Store struct {
	Dir string
	TTL time.Duration
}

type Entry struct {
	Key       string                      `json:"key"`
	Provider  string                      `json:"provider"`
	StoredAt  time.Time                   `json:"storedAt"`
	ExpiresAt time.Time                   `json:"expiresAt"`
	Results   []flight.FlightSearchResult `json:"results"`
}

func New(dir string, ttl time.Duration) *Store {
	return &Store{Dir: dir, TTL: ttl}
}

func (s *Store) Read(provider string, request flight.FlightSearchRequest) ([]flight.FlightSearchResult, bool, error) {
	if s == nil || s.Dir == "" || s.TTL <= 0 {
		return nil, false, nil
	}
	key, err := Key(provider, request)
	if err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(s.path(key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false, err
	}
	if time.Now().After(entry.ExpiresAt) {
		return nil, false, nil
	}
	return entry.Results, true, nil
}

func (s *Store) Write(provider string, request flight.FlightSearchRequest, results []flight.FlightSearchResult) error {
	if s == nil || s.Dir == "" || s.TTL <= 0 {
		return nil
	}
	key, err := Key(provider, request)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	now := time.Now()
	entry := Entry{
		Key:       key,
		Provider:  provider,
		StoredAt:  now,
		ExpiresAt: now.Add(s.TTL),
		Results:   results,
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(key), data, 0o600)
}

func (s *Store) Clear() (int, error) {
	if s == nil || s.Dir == "" {
		return 0, nil
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if err := os.Remove(filepath.Join(s.Dir, entry.Name())); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *Store) path(key string) string {
	return filepath.Join(s.Dir, key+".json")
}

func Key(provider string, request flight.FlightSearchRequest) (string, error) {
	payload := struct {
		Version  int                        `json:"version"`
		Provider string                     `json:"provider"`
		Request  flight.FlightSearchRequest `json:"request"`
	}{
		Version:  1,
		Provider: provider,
		Request:  request,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("cache key: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
