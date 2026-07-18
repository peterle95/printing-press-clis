// Package sqlite provides the local source of truth for normalized portfolio data.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrStale               = errors.New("stored value is stale")
	ErrIdempotencyConflict = errors.New("idempotency key conflicts with an existing request")
)

// Store owns one private SQLite database.
type Store struct {
	db   *sql.DB
	path string
	now  func() time.Time
}

// Open creates or opens path, applies embedded migrations, and enables the
// foreign-key, WAL, and busy-timeout safety settings on every connection.
func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("database path is required")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	if err := preparePrivateFile(absPath); err != nil {
		return nil, err
	}

	dsn := "file:" + filepath.ToSlash(absPath) + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	store := &Store{db: db, path: absPath, now: time.Now}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	if err := store.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func preparePrivateFile(path string) error {
	dir := filepath.Dir(path)
	_, statErr := os.Stat(dir)
	createdDirectory := os.IsNotExist(statErr)
	if statErr != nil && !createdDirectory {
		return fmt.Errorf("inspect database directory: %w", statErr)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	if createdDirectory {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("secure database directory: %w", err)
		}
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("database path must not be a symbolic link")
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect database path: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("create database file: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return fmt.Errorf("secure database file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close database file: %w", err)
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Path() string { return s.path }

// DB exposes the handle for read-only diagnostics. Callers must not close it.
func (s *Store) DB() *sql.DB { return s.db }
