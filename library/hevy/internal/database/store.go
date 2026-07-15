// Package database persists imported workouts in SQLite with idempotent writes.
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"hevy-pp-cli/internal/csvimport"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }
type ImportSummary struct {
	SourcePath, FileHash                                                   string
	RowsRead, WorkoutsInserted, WorkoutsUpdated, SetsInserted, RowsSkipped int
	Warnings                                                               []string
	Skipped                                                                bool
}
type Workout struct {
	ID, Title, StartedAt, EndedAt string
	ExerciseCount, SetCount       int
	Volume                        float64
}
type Exercise struct {
	Name     string
	SetCount int
	Volume   float64
}
type SetRecord struct {
	Exercise, StartedAt, WorkoutTitle string
	Weight, Reps, RPE                 float64
	HasWeight, HasReps                bool
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db}
	if err = s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS imports (id TEXT PRIMARY KEY, source_filename TEXT NOT NULL, source_path TEXT, file_hash TEXT NOT NULL UNIQUE, file_size INTEGER, imported_at TEXT NOT NULL, csv_format TEXT, rows_read INTEGER, workouts_inserted INTEGER, workouts_updated INTEGER, rows_skipped INTEGER, warnings_json TEXT, status TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS workouts (id TEXT PRIMARY KEY, fingerprint TEXT NOT NULL UNIQUE, title TEXT NOT NULL, started_at TEXT NOT NULL, ended_at TEXT, start_original TEXT, end_original TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS exercises (id TEXT PRIMARY KEY, normalized_name TEXT NOT NULL UNIQUE, name TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS workout_exercises (id TEXT PRIMARY KEY, workout_id TEXT NOT NULL REFERENCES workouts(id) ON DELETE CASCADE, exercise_id TEXT NOT NULL REFERENCES exercises(id), sequence INTEGER NOT NULL, UNIQUE(workout_id, sequence))`,
		`CREATE TABLE IF NOT EXISTS sets (id TEXT PRIMARY KEY, fingerprint TEXT NOT NULL UNIQUE, workout_exercise_id TEXT NOT NULL REFERENCES workout_exercises(id) ON DELETE CASCADE, set_index INTEGER, set_type TEXT, superset_id TEXT, weight_kg REAL, reps REAL, duration_seconds REAL, distance_meters REAL, rpe REAL, notes TEXT, raw_json TEXT)`,
		`CREATE INDEX IF NOT EXISTS idx_workouts_started ON workouts(started_at)`,
		`CREATE INDEX IF NOT EXISTS idx_sets_exercise ON sets(workout_exercise_id)`,
		`CREATE TABLE IF NOT EXISTS browser_sessions (id TEXT PRIMARY KEY, status TEXT NOT NULL, checked_at TEXT NOT NULL, detail TEXT)`,
		`CREATE TABLE IF NOT EXISTS sync_runs (id TEXT PRIMARY KEY, started_at TEXT NOT NULL, completed_at TEXT, status TEXT NOT NULL, summary_json TEXT)`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(1,?)`, time.Now().UTC().Format(time.RFC3339))
	return err
}
func (s *Store) Imported(hash string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT count(*) FROM imports WHERE file_hash=? AND status='success'`, hash).Scan(&n)
	return n > 0, err
}
func (s *Store) ImportCSV(ctx context.Context, path string, parsed csvimport.Result, dryRun bool) (ImportSummary, error) {
	abs, _ := filepath.Abs(path)
	sum := ImportSummary{SourcePath: abs, FileHash: parsed.FileHash, RowsRead: parsed.RowsRead, Warnings: parsed.Warnings}
	seen, err := s.Imported(parsed.FileHash)
	if err != nil {
		return sum, err
	}
	if seen {
		sum.Skipped = true
		return sum, nil
	}
	if dryRun {
		return s.simulate(parsed, sum), nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return sum, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	workoutIDs := map[string]string{}
	exerciseSeq := map[string]int{}
	workoutExerciseIDs := map[string]string{}
	for sequence, row := range parsed.Sets {
		started, err := csvimport.ParseTime(row.StartText)
		if err != nil {
			return sum, fmt.Errorf("row %d start time: %w", sequence+2, err)
		}
		ended := time.Time{}
		if row.EndText != "" {
			ended, _ = csvimport.ParseTime(row.EndText)
		}
		wf := csvimport.Fingerprint(row.WorkoutTitle, started.UTC().Format(time.RFC3339), ended.UTC().Format(time.RFC3339))
		wid := wf
		if _, ok := workoutIDs[wf]; !ok {
			res, e := tx.ExecContext(ctx, `INSERT INTO workouts(id,fingerprint,title,started_at,ended_at,start_original,end_original,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(fingerprint) DO UPDATE SET title=excluded.title,ended_at=excluded.ended_at,updated_at=excluded.updated_at`, wid, wf, row.WorkoutTitle, started.UTC().Format(time.RFC3339), nullTime(ended), row.StartText, row.EndText, now, now)
			if e != nil {
				return sum, e
			}
			n, _ := res.RowsAffected()
			if n == 1 {
				sum.WorkoutsInserted++
			} else {
				sum.WorkoutsUpdated++
			}
			workoutIDs[wf] = wid
		}
		en := csvimport.Fingerprint(row.ExerciseTitle)
		_, e := tx.ExecContext(ctx, `INSERT INTO exercises(id,normalized_name,name) VALUES(?,?,?) ON CONFLICT(normalized_name) DO NOTHING`, en, normalized(row.ExerciseTitle), row.ExerciseTitle)
		if e != nil {
			return sum, e
		}
		key := wf + "/" + en
		if _, ok := workoutExerciseIDs[key]; !ok {
			exerciseSeq[wf]++
			weid := csvimport.Fingerprint(wid, en, fmt.Sprint(exerciseSeq[wf]))
			_, e = tx.ExecContext(ctx, `INSERT INTO workout_exercises(id,workout_id,exercise_id,sequence) VALUES(?,?,?,?) ON CONFLICT(workout_id,sequence) DO NOTHING`, weid, wid, en, exerciseSeq[wf])
			if e != nil {
				return sum, e
			}
			workoutExerciseIDs[key] = weid
		}
		sf := csvimport.Fingerprint(wf, en, fmt.Sprint(row.SetIndex), fmt.Sprint(sequence), row.SetType, fmt.Sprint(row.WeightKG), fmt.Sprint(row.Reps))
		raw, _ := json.Marshal(row.Raw)
		res, e := tx.ExecContext(ctx, `INSERT INTO sets(id,fingerprint,workout_exercise_id,set_index,set_type,superset_id,weight_kg,reps,duration_seconds,distance_meters,rpe,notes,raw_json) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(fingerprint) DO NOTHING`, sf, sf, workoutExerciseIDs[key], row.SetIndex, row.SetType, row.SupersetID, row.WeightKG, row.Reps, row.DurationSeconds, row.DistanceMeters, row.RPE, row.Notes, string(raw))
		if e != nil {
			return sum, e
		}
		if n, _ := res.RowsAffected(); n == 1 {
			sum.SetsInserted++
		}
	}
	st, _ := os.Stat(path)
	warn, _ := json.Marshal(sum.Warnings)
	_, err = tx.ExecContext(ctx, `INSERT INTO imports(id,source_filename,source_path,file_hash,file_size,imported_at,csv_format,rows_read,workouts_inserted,workouts_updated,rows_skipped,warnings_json,status) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, csvimport.Fingerprint(parsed.FileHash, now), filepath.Base(path), abs, parsed.FileHash, size(st), now, parsed.Format, sum.RowsRead, sum.WorkoutsInserted, sum.WorkoutsUpdated, sum.RowsSkipped, string(warn), "success")
	if err != nil {
		return sum, err
	}
	if err = tx.Commit(); err != nil {
		return sum, err
	}
	return sum, nil
}
func (s *Store) simulate(r csvimport.Result, sum ImportSummary) ImportSummary {
	seen := map[string]bool{}
	for _, x := range r.Sets {
		k := csvimport.Fingerprint(x.WorkoutTitle, x.StartText, x.EndText)
		if !seen[k] {
			sum.WorkoutsInserted++
			seen[k] = true
		}
		sum.SetsInserted++
	}
	return sum
}
func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}
func size(i os.FileInfo) any {
	if i == nil {
		return nil
	}
	return i.Size()
}
func normalized(s string) string {
	var out []rune
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			r += 32
		}
		out = append(out, r)
	}
	return string(out)
}
func (s *Store) Workouts(since time.Time, limit int) ([]Workout, error) {
	q := `SELECT w.id,w.title,w.started_at,COALESCE(w.ended_at,''),COUNT(DISTINCT we.id),COUNT(st.id),COALESCE(SUM(CASE WHEN st.weight_kg IS NOT NULL AND st.reps IS NOT NULL THEN st.weight_kg*st.reps ELSE 0 END),0) FROM workouts w LEFT JOIN workout_exercises we ON we.workout_id=w.id LEFT JOIN sets st ON st.workout_exercise_id=we.id`
	a := []any{}
	if !since.IsZero() {
		q += " WHERE w.started_at>=?"
		a = append(a, since.UTC().Format(time.RFC3339))
	}
	q += " GROUP BY w.id ORDER BY w.started_at DESC"
	if limit > 0 {
		q += " LIMIT ?"
		a = append(a, limit)
	}
	rows, e := s.db.Query(q, a...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []Workout
	for rows.Next() {
		var w Workout
		if e = rows.Scan(&w.ID, &w.Title, &w.StartedAt, &w.EndedAt, &w.ExerciseCount, &w.SetCount, &w.Volume); e != nil {
			return nil, e
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
func (s *Store) Workout(id string) (Workout, error) {
	ws, e := s.Workouts(time.Time{}, 0)
	if e != nil {
		return Workout{}, e
	}
	for _, w := range ws {
		if w.ID == id {
			return w, nil
		}
	}
	return Workout{}, fmt.Errorf("workout not found: %s", id)
}
func (s *Store) Exercises() ([]Exercise, error) {
	rows, e := s.db.Query(`SELECT e.name,COUNT(st.id),COALESCE(SUM(CASE WHEN st.weight_kg IS NOT NULL AND st.reps IS NOT NULL THEN st.weight_kg*st.reps ELSE 0 END),0) FROM exercises e LEFT JOIN workout_exercises we ON we.exercise_id=e.id LEFT JOIN sets st ON st.workout_exercise_id=we.id GROUP BY e.id ORDER BY e.name`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var o []Exercise
	for rows.Next() {
		var x Exercise
		if e = rows.Scan(&x.Name, &x.SetCount, &x.Volume); e != nil {
			return nil, e
		}
		o = append(o, x)
	}
	return o, rows.Err()
}
func (s *Store) ExerciseSets(name string, since time.Time) ([]SetRecord, error) {
	q := `SELECT e.name,w.started_at,w.title,st.weight_kg,st.reps,st.rpe FROM sets st JOIN workout_exercises we ON we.id=st.workout_exercise_id JOIN exercises e ON e.id=we.exercise_id JOIN workouts w ON w.id=we.workout_id WHERE lower(e.normalized_name)=lower(?)`
	a := []any{name}
	if !since.IsZero() {
		q += " AND w.started_at>=?"
		a = append(a, since.UTC().Format(time.RFC3339))
	}
	q += " ORDER BY w.started_at DESC"
	r, e := s.db.Query(q, a...)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	var out []SetRecord
	for r.Next() {
		var x SetRecord
		var weight, reps, rpe sql.NullFloat64
		if e = r.Scan(&x.Exercise, &x.StartedAt, &x.WorkoutTitle, &weight, &reps, &rpe); e != nil {
			return nil, e
		}
		x.Weight, x.Reps, x.RPE = weight.Float64, reps.Float64, rpe.Float64
		x.HasWeight, x.HasReps = weight.Valid, reps.Valid
		out = append(out, x)
	}
	return out, r.Err()
}
