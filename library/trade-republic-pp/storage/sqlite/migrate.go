package sqlite

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

var migrationNamePattern = regexp.MustCompile(`^(\d{3})_([a-z0-9_]+)\.sql$`)

type migration struct {
	version  int
	name     string
	checksum string
	sql      string
}

type MigrationRecord struct {
	Version   int
	Name      string
	Checksum  string
	AppliedAt time.Time
}

func embeddedMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationNamePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", entry.Name(), err)
		}
		body, err := migrationFiles.ReadFile(filepath.ToSlash(filepath.Join("migrations", entry.Name())))
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		digest := sha256.Sum256(body)
		migrations = append(migrations, migration{
			version:  version,
			name:     strings.TrimSuffix(entry.Name(), ".sql"),
			checksum: hex.EncodeToString(digest[:]),
			sql:      string(body),
		})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })
	for index, item := range migrations {
		if item.version != index+1 {
			return nil, fmt.Errorf("migration versions must be contiguous from 001: got %03d at index %d", item.version, index)
		}
	}
	return migrations, nil
}

func (s *Store) migrate(ctx context.Context) error {
	const createMigrations = `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		checksum TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`
	if _, err := s.db.ExecContext(ctx, createMigrations); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	migrations, err := embeddedMigrations()
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migrations: %w", err)
	}
	defer tx.Rollback()

	applied := map[int]MigrationRecord{}
	rows, err := tx.QueryContext(ctx, `SELECT version, name, checksum, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("read applied migrations: %w", err)
	}
	for rows.Next() {
		var record MigrationRecord
		var appliedAt string
		if err := rows.Scan(&record.Version, &record.Name, &record.Checksum, &appliedAt); err != nil {
			rows.Close()
			return fmt.Errorf("scan applied migration: %w", err)
		}
		record.AppliedAt, err = parseTime(appliedAt)
		if err != nil {
			rows.Close()
			return fmt.Errorf("parse migration %d timestamp: %w", record.Version, err)
		}
		applied[record.Version] = record
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close migration rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate applied migrations: %w", err)
	}

	for _, item := range migrations {
		if existing, ok := applied[item.version]; ok {
			if existing.Name != item.name || existing.Checksum != item.checksum {
				return fmt.Errorf("migration %03d integrity mismatch", item.version)
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, item.sql); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations(version, name, checksum, applied_at) VALUES(?, ?, ?, ?)`,
			item.version, item.name, item.checksum, formatTime(s.now().UTC()),
		); err != nil {
			return fmt.Errorf("record migration %s: %w", item.name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

func (s *Store) Migrations(ctx context.Context) ([]MigrationRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT version, name, checksum, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("query migrations: %w", err)
	}
	defer rows.Close()
	var records []MigrationRecord
	for rows.Next() {
		var record MigrationRecord
		var appliedAt string
		if err := rows.Scan(&record.Version, &record.Name, &record.Checksum, &appliedAt); err != nil {
			return nil, fmt.Errorf("scan migration: %w", err)
		}
		record.AppliedAt, err = parseTime(appliedAt)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }
