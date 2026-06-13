package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type TrackRecord struct {
	ID           string `json:"id"`
	URI          string `json:"uri"`
	Name         string `json:"name"`
	AlbumName    string `json:"album_name"`
	AlbumID      string `json:"album_id"`
	DurationMS   int    `json:"duration_ms"`
	Explicit     bool   `json:"explicit"`
	Popularity   *int   `json:"popularity,omitempty"`
	AddedAtLiked string `json:"added_at_liked,omitempty"`
	FirstSeenAt  string `json:"first_seen_at,omitempty"`
	LastSeenAt   string `json:"last_seen_at,omitempty"`
	RawJSON      string `json:"raw_json,omitempty"`
}

type ArtistRecord struct {
	ID                        string   `json:"id"`
	URI                       string   `json:"uri"`
	Name                      string   `json:"name"`
	Genres                    []string `json:"genres"`
	DeprecatedGenresAvailable bool     `json:"deprecated_genres_available"`
	LastFetchedAt             string   `json:"last_fetched_at,omitempty"`
	RawJSON                   string   `json:"raw_json,omitempty"`
}

type PlaylistRecord struct {
	ID            string `json:"id"`
	URI           string `json:"uri"`
	Name          string `json:"name"`
	OwnerID       string `json:"owner_id"`
	OwnerName     string `json:"owner_name"`
	Public        *bool  `json:"public,omitempty"`
	Collaborative bool   `json:"collaborative"`
	SnapshotID    string `json:"snapshot_id"`
	TrackCount    int    `json:"track_count"`
	LastFetchedAt string `json:"last_fetched_at,omitempty"`
	RawJSON       string `json:"raw_json,omitempty"`
}

type PlaylistTrackRecord struct {
	PlaylistID string `json:"playlist_id"`
	TrackID    string `json:"track_id"`
	TrackURI   string `json:"track_uri"`
	Position   int    `json:"position"`
	AddedAt    string `json:"added_at"`
	AddedBy    string `json:"added_by"`
	IsLocal    bool   `json:"is_local"`
	LastSeenAt string `json:"last_seen_at"`
}

type ClassificationRecord struct {
	TrackID            string `json:"track_id"`
	PrimaryGenre       string `json:"primary_genre"`
	MatchedRule        string `json:"matched_rule"`
	Confidence         string `json:"confidence"`
	TargetPlaylistID   string `json:"target_playlist_id,omitempty"`
	TargetPlaylistName string `json:"target_playlist_name"`
	Explanation        string `json:"explanation"`
	UpdatedAt          string `json:"updated_at"`
}

type OperationRecord struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	CompletedAt string `json:"completed_at,omitempty"`
	DryRun      bool   `json:"dry_run"`
	SummaryJSON string `json:"summary_json"`
	UndoJSON    string `json:"undo_json,omitempty"`
	Error       string `json:"error,omitempty"`
}

type OperationItemRecord struct {
	OperationID      int64  `json:"operation_id"`
	TrackID          string `json:"track_id"`
	TrackURI         string `json:"track_uri"`
	Source           string `json:"source"`
	TargetPlaylistID string `json:"target_playlist_id"`
	Action           string `json:"action"`
	Status           string `json:"status"`
	Reason           string `json:"reason"`
	Error            string `json:"error,omitempty"`
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS tracks (
			id TEXT PRIMARY KEY,
			uri TEXT NOT NULL,
			name TEXT NOT NULL,
			album_name TEXT,
			album_id TEXT,
			duration_ms INTEGER,
			explicit INTEGER NOT NULL DEFAULT 0,
			popularity INTEGER NULL,
			added_at_liked TEXT,
			first_seen_at TEXT NOT NULL,
			last_seen_at TEXT NOT NULL,
			raw_json TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS artists (
			id TEXT PRIMARY KEY,
			uri TEXT NOT NULL,
			name TEXT NOT NULL,
			genres_json TEXT,
			deprecated_genres_available INTEGER NOT NULL DEFAULT 0,
			last_fetched_at TEXT,
			raw_json TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS track_artists (
			track_id TEXT NOT NULL,
			artist_id TEXT NOT NULL,
			artist_order INTEGER NOT NULL,
			PRIMARY KEY (track_id, artist_id)
		)`,
		`CREATE TABLE IF NOT EXISTS playlists (
			id TEXT PRIMARY KEY,
			uri TEXT NOT NULL,
			name TEXT NOT NULL,
			owner_id TEXT,
			owner_name TEXT,
			public INTEGER NULL,
			collaborative INTEGER NOT NULL DEFAULT 0,
			snapshot_id TEXT,
			track_count INTEGER NOT NULL DEFAULT 0,
			last_fetched_at TEXT,
			raw_json TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS playlist_tracks (
			playlist_id TEXT NOT NULL,
			track_id TEXT NOT NULL,
			position INTEGER NOT NULL,
			added_at TEXT,
			added_by TEXT,
			is_local INTEGER NOT NULL DEFAULT 0,
			last_seen_at TEXT,
			PRIMARY KEY (playlist_id, position)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_playlist_tracks_track ON playlist_tracks(track_id)`,
		`CREATE TABLE IF NOT EXISTS classification_cache (
			track_id TEXT PRIMARY KEY,
			primary_genre TEXT,
			matched_rule TEXT,
			confidence TEXT,
			target_playlist_id TEXT,
			target_playlist_name TEXT,
			explanation TEXT,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS operations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			completed_at TEXT,
			dry_run INTEGER NOT NULL DEFAULT 1,
			summary_json TEXT,
			undo_json TEXT,
			error TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS operation_items (
			operation_id INTEGER NOT NULL,
			track_id TEXT,
			track_uri TEXT,
			source TEXT,
			target_playlist_id TEXT,
			action TEXT NOT NULL,
			status TEXT NOT NULL,
			reason TEXT,
			error TEXT
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) UpsertTrack(ctx context.Context, track TrackRecord) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if track.FirstSeenAt == "" {
		track.FirstSeenAt = now
	}
	if track.LastSeenAt == "" {
		track.LastSeenAt = now
	}
	var popularity any
	if track.Popularity != nil {
		popularity = *track.Popularity
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO tracks
		(id, uri, name, album_name, album_id, duration_ms, explicit, popularity, added_at_liked, first_seen_at, last_seen_at, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			uri=excluded.uri,
			name=excluded.name,
			album_name=excluded.album_name,
			album_id=excluded.album_id,
			duration_ms=excluded.duration_ms,
			explicit=excluded.explicit,
			popularity=excluded.popularity,
			added_at_liked=COALESCE(excluded.added_at_liked, tracks.added_at_liked),
			last_seen_at=excluded.last_seen_at,
			raw_json=excluded.raw_json`,
		track.ID, track.URI, track.Name, track.AlbumName, track.AlbumID, track.DurationMS, boolInt(track.Explicit), popularity, track.AddedAtLiked, track.FirstSeenAt, track.LastSeenAt, track.RawJSON)
	return err
}

func (s *Store) UpsertArtist(ctx context.Context, artist ArtistRecord) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if artist.LastFetchedAt == "" {
		artist.LastFetchedAt = now
	}
	genresJSON := mustJSON(artist.Genres)
	_, err := s.db.ExecContext(ctx, `INSERT INTO artists
		(id, uri, name, genres_json, deprecated_genres_available, last_fetched_at, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			uri=excluded.uri,
			name=excluded.name,
			genres_json=excluded.genres_json,
			deprecated_genres_available=excluded.deprecated_genres_available,
			last_fetched_at=excluded.last_fetched_at,
			raw_json=excluded.raw_json`,
		artist.ID, artist.URI, artist.Name, genresJSON, boolInt(artist.DeprecatedGenresAvailable), artist.LastFetchedAt, artist.RawJSON)
	return err
}

func (s *Store) UpsertTrackArtist(ctx context.Context, trackID, artistID string, order int) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO track_artists (track_id, artist_id, artist_order)
		VALUES (?, ?, ?)
		ON CONFLICT(track_id, artist_id) DO UPDATE SET artist_order=excluded.artist_order`, trackID, artistID, order)
	return err
}

func (s *Store) UpsertPlaylist(ctx context.Context, playlist PlaylistRecord) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if playlist.LastFetchedAt == "" {
		playlist.LastFetchedAt = now
	}
	var public any
	if playlist.Public != nil {
		public = boolInt(*playlist.Public)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO playlists
		(id, uri, name, owner_id, owner_name, public, collaborative, snapshot_id, track_count, last_fetched_at, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			uri=excluded.uri,
			name=excluded.name,
			owner_id=excluded.owner_id,
			owner_name=excluded.owner_name,
			public=excluded.public,
			collaborative=excluded.collaborative,
			snapshot_id=excluded.snapshot_id,
			track_count=excluded.track_count,
			last_fetched_at=excluded.last_fetched_at,
			raw_json=excluded.raw_json`,
		playlist.ID, playlist.URI, playlist.Name, playlist.OwnerID, playlist.OwnerName, public, boolInt(playlist.Collaborative), playlist.SnapshotID, playlist.TrackCount, playlist.LastFetchedAt, playlist.RawJSON)
	return err
}

func (s *Store) ReplacePlaylistTracks(ctx context.Context, playlistID string, items []PlaylistTrackRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM playlist_tracks WHERE playlist_id=?`, playlistID); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO playlist_tracks
		(playlist_id, track_id, position, added_at, added_by, is_local, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, item := range items {
		if item.LastSeenAt == "" {
			item.LastSeenAt = now
		}
		if _, err := stmt.ExecContext(ctx, item.PlaylistID, item.TrackID, item.Position, item.AddedAt, item.AddedBy, boolInt(item.IsLocal), item.LastSeenAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) LikedTracks(ctx context.Context) ([]TrackRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, uri, name, album_name, album_id, duration_ms, explicit, popularity, added_at_liked, first_seen_at, last_seen_at, raw_json
		FROM tracks ORDER BY COALESCE(added_at_liked, first_seen_at) DESC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tracks []TrackRecord
	for rows.Next() {
		track, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}
	return tracks, rows.Err()
}

func (s *Store) TrackByIDOrURI(ctx context.Context, value string) (TrackRecord, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, uri, name, album_name, album_id, duration_ms, explicit, popularity, added_at_liked, first_seen_at, last_seen_at, raw_json
		FROM tracks WHERE id=? OR uri=? LIMIT 1`, value, value)
	track, err := scanTrack(row)
	if err == sql.ErrNoRows {
		return TrackRecord{}, false, nil
	}
	if err != nil {
		return TrackRecord{}, false, err
	}
	return track, true, nil
}

func (s *Store) SearchTracksLocal(ctx context.Context, query string, limit int) ([]TrackRecord, error) {
	if limit <= 0 {
		limit = 10
	}
	pattern := "%" + strings.ToLower(query) + "%"
	rows, err := s.db.QueryContext(ctx, `SELECT id, uri, name, album_name, album_id, duration_ms, explicit, popularity, added_at_liked, first_seen_at, last_seen_at, raw_json
		FROM tracks WHERE lower(name) LIKE ? OR lower(album_name) LIKE ? ORDER BY name LIMIT ?`, pattern, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tracks []TrackRecord
	for rows.Next() {
		track, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}
	return tracks, rows.Err()
}

func (s *Store) ArtistsForTrack(ctx context.Context, trackID string) ([]ArtistRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.id, a.uri, a.name, a.genres_json, a.deprecated_genres_available, a.last_fetched_at, a.raw_json
		FROM artists a JOIN track_artists ta ON ta.artist_id=a.id
		WHERE ta.track_id=?
		ORDER BY ta.artist_order`, trackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var artists []ArtistRecord
	for rows.Next() {
		artist, err := scanArtist(rows)
		if err != nil {
			return nil, err
		}
		artists = append(artists, artist)
	}
	return artists, rows.Err()
}

func (s *Store) UpsertClassification(ctx context.Context, rec ClassificationRecord) error {
	if rec.UpdatedAt == "" {
		rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO classification_cache
		(track_id, primary_genre, matched_rule, confidence, target_playlist_id, target_playlist_name, explanation, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(track_id) DO UPDATE SET
			primary_genre=excluded.primary_genre,
			matched_rule=excluded.matched_rule,
			confidence=excluded.confidence,
			target_playlist_id=excluded.target_playlist_id,
			target_playlist_name=excluded.target_playlist_name,
			explanation=excluded.explanation,
			updated_at=excluded.updated_at`,
		rec.TrackID, rec.PrimaryGenre, rec.MatchedRule, rec.Confidence, rec.TargetPlaylistID, rec.TargetPlaylistName, rec.Explanation, rec.UpdatedAt)
	return err
}

func (s *Store) Classifications(ctx context.Context) (map[string]ClassificationRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT track_id, primary_genre, matched_rule, confidence, target_playlist_id, target_playlist_name, explanation, updated_at FROM classification_cache`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]ClassificationRecord{}
	for rows.Next() {
		var rec ClassificationRecord
		if err := rows.Scan(&rec.TrackID, &rec.PrimaryGenre, &rec.MatchedRule, &rec.Confidence, &rec.TargetPlaylistID, &rec.TargetPlaylistName, &rec.Explanation, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		out[rec.TrackID] = rec
	}
	return out, rows.Err()
}

func (s *Store) Playlists(ctx context.Context) ([]PlaylistRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, uri, name, owner_id, owner_name, public, collaborative, snapshot_id, track_count, last_fetched_at, raw_json FROM playlists ORDER BY lower(name)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var playlists []PlaylistRecord
	for rows.Next() {
		playlist, err := scanPlaylist(rows)
		if err != nil {
			return nil, err
		}
		playlists = append(playlists, playlist)
	}
	return playlists, rows.Err()
}

func (s *Store) PlaylistByName(ctx context.Context, name string) (PlaylistRecord, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, uri, name, owner_id, owner_name, public, collaborative, snapshot_id, track_count, last_fetched_at, raw_json
		FROM playlists WHERE lower(name)=lower(?) ORDER BY last_fetched_at DESC LIMIT 1`, name)
	playlist, err := scanPlaylist(row)
	if err == sql.ErrNoRows {
		return PlaylistRecord{}, false, nil
	}
	if err != nil {
		return PlaylistRecord{}, false, err
	}
	return playlist, true, nil
}

func (s *Store) PlaylistTracks(ctx context.Context, playlistID string) ([]PlaylistTrackRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT playlist_id, track_id, position, added_at, added_by, is_local, last_seen_at
		FROM playlist_tracks WHERE playlist_id=? ORDER BY position`, playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PlaylistTrackRecord
	for rows.Next() {
		var item PlaylistTrackRecord
		var isLocal int
		if err := rows.Scan(&item.PlaylistID, &item.TrackID, &item.Position, &item.AddedAt, &item.AddedBy, &isLocal, &item.LastSeenAt); err != nil {
			return nil, err
		}
		item.IsLocal = isLocal != 0
		item.TrackURI = trackURI(item.TrackID)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ExistingTrackIDsInPlaylist(ctx context.Context, playlistID string) (map[string]bool, error) {
	items, err := s.PlaylistTracks(ctx, playlistID)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, item := range items {
		out[item.TrackID] = true
	}
	return out, nil
}

func (s *Store) CreateOperation(ctx context.Context, typ string, dryRun bool, summaryJSON string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO operations (type, status, created_at, dry_run, summary_json)
		VALUES (?, 'running', ?, ?, ?)`, typ, time.Now().UTC().Format(time.RFC3339), boolInt(dryRun), summaryJSON)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) CompleteOperation(ctx context.Context, id int64, status, undoJSON, errText string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE operations SET status=?, completed_at=?, undo_json=?, error=? WHERE id=?`,
		status, time.Now().UTC().Format(time.RFC3339), undoJSON, errText, id)
	return err
}

func (s *Store) AddOperationItem(ctx context.Context, item OperationItemRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO operation_items
		(operation_id, track_id, track_uri, source, target_playlist_id, action, status, reason, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.OperationID, item.TrackID, item.TrackURI, item.Source, item.TargetPlaylistID, item.Action, item.Status, item.Reason, item.Error)
	return err
}

func (s *Store) Operations(ctx context.Context) ([]OperationRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, type, status, created_at, COALESCE(completed_at,''), dry_run, COALESCE(summary_json,''), COALESCE(undo_json,''), COALESCE(error,'')
		FROM operations ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ops []OperationRecord
	for rows.Next() {
		op, err := scanOperation(rows)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, rows.Err()
}

func (s *Store) Operation(ctx context.Context, id int64) (OperationRecord, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, type, status, created_at, COALESCE(completed_at,''), dry_run, COALESCE(summary_json,''), COALESCE(undo_json,''), COALESCE(error,'')
		FROM operations WHERE id=?`, id)
	op, err := scanOperation(row)
	if err == sql.ErrNoRows {
		return OperationRecord{}, false, nil
	}
	if err != nil {
		return OperationRecord{}, false, err
	}
	return op, true, nil
}

func (s *Store) OperationItems(ctx context.Context, id int64) ([]OperationItemRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT operation_id, COALESCE(track_id,''), COALESCE(track_uri,''), COALESCE(source,''), COALESCE(target_playlist_id,''), action, status, COALESCE(reason,''), COALESCE(error,'')
		FROM operation_items WHERE operation_id=?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []OperationItemRecord
	for rows.Next() {
		var item OperationItemRecord
		if err := rows.Scan(&item.OperationID, &item.TrackID, &item.TrackURI, &item.Source, &item.TargetPlaylistID, &item.Action, &item.Status, &item.Reason, &item.Error); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTrack(row rowScanner) (TrackRecord, error) {
	var track TrackRecord
	var popularity sql.NullInt64
	var explicit int
	if err := row.Scan(&track.ID, &track.URI, &track.Name, &track.AlbumName, &track.AlbumID, &track.DurationMS, &explicit, &popularity, &track.AddedAtLiked, &track.FirstSeenAt, &track.LastSeenAt, &track.RawJSON); err != nil {
		return TrackRecord{}, err
	}
	track.Explicit = explicit != 0
	if popularity.Valid {
		v := int(popularity.Int64)
		track.Popularity = &v
	}
	return track, nil
}

func scanArtist(row rowScanner) (ArtistRecord, error) {
	var artist ArtistRecord
	var genresJSON string
	var deprecatedGenresAvailable int
	if err := row.Scan(&artist.ID, &artist.URI, &artist.Name, &genresJSON, &deprecatedGenresAvailable, &artist.LastFetchedAt, &artist.RawJSON); err != nil {
		return ArtistRecord{}, err
	}
	artist.DeprecatedGenresAvailable = deprecatedGenresAvailable != 0
	_ = json.Unmarshal([]byte(genresJSON), &artist.Genres)
	return artist, nil
}

func scanPlaylist(row rowScanner) (PlaylistRecord, error) {
	var playlist PlaylistRecord
	var public sql.NullInt64
	var collaborative int
	if err := row.Scan(&playlist.ID, &playlist.URI, &playlist.Name, &playlist.OwnerID, &playlist.OwnerName, &public, &collaborative, &playlist.SnapshotID, &playlist.TrackCount, &playlist.LastFetchedAt, &playlist.RawJSON); err != nil {
		return PlaylistRecord{}, err
	}
	if public.Valid {
		v := public.Int64 != 0
		playlist.Public = &v
	}
	playlist.Collaborative = collaborative != 0
	return playlist, nil
}

func scanOperation(row rowScanner) (OperationRecord, error) {
	var op OperationRecord
	var dryRun int
	if err := row.Scan(&op.ID, &op.Type, &op.Status, &op.CreatedAt, &op.CompletedAt, &dryRun, &op.SummaryJSON, &op.UndoJSON, &op.Error); err != nil {
		return OperationRecord{}, err
	}
	op.DryRun = dryRun != 0
	return op, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

func trackURI(trackID string) string {
	if trackID == "" || strings.HasPrefix(trackID, "spotify:") {
		return trackID
	}
	return fmt.Sprintf("spotify:track:%s", trackID)
}
