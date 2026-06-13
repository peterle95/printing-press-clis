package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Store struct {
	dir string
}

func New(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) LoadJSON(key string, maxAge time.Duration, dest any) (bool, error) {
	if s == nil || s.dir == "" || maxAge <= 0 {
		return false, nil
	}
	path := s.path(key)
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if time.Since(info.ModTime()) > maxAge {
		return false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return false, nil
	}
	return true, nil
}

func (s *Store) SaveJSON(key string, value any) error {
	if s == nil || s.dir == "" {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(key), data, 0o600)
}

func (s *Store) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}
