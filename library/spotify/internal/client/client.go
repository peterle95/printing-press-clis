package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.spotify.com/v1"

type TokenProvider interface {
	AccessToken(context.Context) (string, error)
	Refresh(context.Context) error
}

type Client struct {
	BaseURL string
	HTTP    *http.Client
	Tokens  TokenProvider
}

type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	URI         string `json:"uri"`
}

type SavedTracksPage struct {
	Href   string       `json:"href"`
	Limit  int          `json:"limit"`
	Next   *string      `json:"next"`
	Offset int          `json:"offset"`
	Total  int          `json:"total"`
	Items  []SavedTrack `json:"items"`
}

type SavedTrack struct {
	AddedAt string `json:"added_at"`
	Track   Track  `json:"track"`
}

type Track struct {
	ID          string            `json:"id"`
	URI         string            `json:"uri"`
	Name        string            `json:"name"`
	DurationMS  int               `json:"duration_ms"`
	Explicit    bool              `json:"explicit"`
	Popularity  *int              `json:"popularity,omitempty"`
	Type        string            `json:"type"`
	IsLocal     bool              `json:"is_local"`
	Album       Album             `json:"album"`
	Artists     []SimpleArtist    `json:"artists"`
	ExternalIDs map[string]string `json:"external_ids"`
}

type Album struct {
	ID   string `json:"id"`
	URI  string `json:"uri"`
	Name string `json:"name"`
}

type SimpleArtist struct {
	ID   string `json:"id"`
	URI  string `json:"uri"`
	Name string `json:"name"`
}

type Artist struct {
	ID         string   `json:"id"`
	URI        string   `json:"uri"`
	Name       string   `json:"name"`
	Genres     []string `json:"genres"`
	Popularity *int     `json:"popularity,omitempty"`
}

type ArtistsResponse struct {
	Artists []Artist `json:"artists"`
}

type PlaylistsPage struct {
	Href   string     `json:"href"`
	Limit  int        `json:"limit"`
	Next   *string    `json:"next"`
	Offset int        `json:"offset"`
	Total  int        `json:"total"`
	Items  []Playlist `json:"items"`
}

type Playlist struct {
	ID            string `json:"id"`
	URI           string `json:"uri"`
	Name          string `json:"name"`
	Collaborative bool   `json:"collaborative"`
	Public        *bool  `json:"public"`
	SnapshotID    string `json:"snapshot_id"`
	Owner         Owner  `json:"owner"`
	Tracks        struct {
		Total int `json:"total"`
	} `json:"tracks"`
}

type Owner struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type PlaylistItemsPage struct {
	Href   string         `json:"href"`
	Limit  int            `json:"limit"`
	Next   *string        `json:"next"`
	Offset int            `json:"offset"`
	Total  int            `json:"total"`
	Items  []PlaylistItem `json:"items"`
}

type PlaylistItem struct {
	AddedAt string `json:"added_at"`
	AddedBy Owner  `json:"added_by"`
	IsLocal bool   `json:"is_local"`
	Track   Track  `json:"track"`
	Item    Track  `json:"item"`
}

type SnapshotResponse struct {
	SnapshotID string `json:"snapshot_id"`
}

type SearchTracksResponse struct {
	Tracks struct {
		Items []Track `json:"items"`
		Total int     `json:"total"`
	} `json:"tracks"`
}

func New(tokens TokenProvider) *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		Tokens:  tokens,
	}
}

func (c *Client) CurrentUser(ctx context.Context) (User, error) {
	var user User
	err := c.request(ctx, http.MethodGet, "/me", nil, nil, &user)
	return user, err
}

func (c *Client) SavedTracks(ctx context.Context, limit, offset int, market string) (SavedTracksPage, error) {
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	if market != "" {
		q.Set("market", market)
	}
	var page SavedTracksPage
	err := c.request(ctx, http.MethodGet, "/me/tracks", q, nil, &page)
	return page, err
}

func (c *Client) AllSavedTracks(ctx context.Context, max int, market string, progress func(done, total int)) ([]SavedTrack, error) {
	var out []SavedTrack
	offset := 0
	for {
		remaining := 50
		if max > 0 && max-len(out) < remaining {
			remaining = max - len(out)
		}
		if remaining <= 0 {
			return out, nil
		}
		page, err := c.SavedTracks(ctx, remaining, offset, market)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		if progress != nil {
			progress(len(out), page.Total)
		}
		if page.Next == nil || len(page.Items) == 0 || (max > 0 && len(out) >= max) {
			break
		}
		offset += len(page.Items)
	}
	return out, nil
}

func (c *Client) GetArtists(ctx context.Context, ids []string) ([]Artist, error) {
	var artists []Artist
	for _, chunk := range BatchStrings(ids, 50) {
		q := url.Values{}
		q.Set("ids", strings.Join(chunk, ","))
		var resp ArtistsResponse
		if err := c.request(ctx, http.MethodGet, "/artists", q, nil, &resp); err != nil {
			return nil, err
		}
		artists = append(artists, resp.Artists...)
	}
	return artists, nil
}

func (c *Client) CurrentUserPlaylists(ctx context.Context) ([]Playlist, error) {
	var out []Playlist
	offset := 0
	for {
		q := url.Values{}
		q.Set("limit", "50")
		q.Set("offset", strconv.Itoa(offset))
		var page PlaylistsPage
		if err := c.request(ctx, http.MethodGet, "/me/playlists", q, nil, &page); err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		if page.Next == nil || len(page.Items) == 0 {
			break
		}
		offset += len(page.Items)
	}
	return out, nil
}

func (c *Client) CreatePlaylist(ctx context.Context, name string, public bool, description string) (Playlist, error) {
	user, err := c.CurrentUser(ctx)
	if err != nil {
		return Playlist{}, err
	}
	body := map[string]any{
		"name":        name,
		"public":      public,
		"description": description,
	}
	var playlist Playlist
	err = c.request(ctx, http.MethodPost, "/users/"+url.PathEscape(user.ID)+"/playlists", nil, body, &playlist)
	return playlist, err
}

func (c *Client) ChangePlaylistDetails(ctx context.Context, playlistID, name string, public *bool) error {
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if public != nil {
		body["public"] = *public
	}
	return c.request(ctx, http.MethodPut, "/playlists/"+url.PathEscape(playlistID), nil, body, nil)
}

func (c *Client) PlaylistItems(ctx context.Context, playlistID, market string) ([]PlaylistItem, error) {
	var out []PlaylistItem
	offset := 0
	for {
		q := url.Values{}
		q.Set("limit", "50")
		q.Set("offset", strconv.Itoa(offset))
		q.Set("additional_types", "track")
		if market != "" {
			q.Set("market", market)
		}
		var page PlaylistItemsPage
		if err := c.request(ctx, http.MethodGet, "/playlists/"+url.PathEscape(playlistID)+"/items", q, nil, &page); err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		if page.Next == nil || len(page.Items) == 0 {
			break
		}
		offset += len(page.Items)
	}
	return out, nil
}

func (c *Client) AddPlaylistItems(ctx context.Context, playlistID string, uris []string) (string, error) {
	var snapshot string
	for _, chunk := range BatchStrings(uris, 100) {
		var resp SnapshotResponse
		body := map[string]any{"uris": chunk}
		if err := c.request(ctx, http.MethodPost, "/playlists/"+url.PathEscape(playlistID)+"/items", nil, body, &resp); err != nil {
			return snapshot, err
		}
		snapshot = resp.SnapshotID
	}
	return snapshot, nil
}

// ReplacePlaylistItems REPLACES all tracks in a playlist with the provided URIs.
// This is a DESTRUCTIVE operation that removes all existing tracks.
// Use AddPlaylistItems to append tracks without removing existing ones.
func (c *Client) ReplacePlaylistItems(ctx context.Context, playlistID string, uris []string) (string, error) {
	var snapshot string
	for _, chunk := range BatchStrings(uris, 100) {
		var resp SnapshotResponse
		body := map[string]any{"uris": chunk}
		if err := c.request(ctx, http.MethodPut, "/playlists/"+url.PathEscape(playlistID)+"/items", nil, body, &resp); err != nil {
			return snapshot, err
		}
		snapshot = resp.SnapshotID
	}
	return snapshot, nil
}

func (c *Client) RemovePlaylistItems(ctx context.Context, playlistID string, uris []string) (string, error) {
	var snapshot string
	for _, chunk := range BatchStrings(uris, 100) {
		items := make([]map[string]string, 0, len(chunk))
		for _, uri := range chunk {
			items = append(items, map[string]string{"uri": uri})
		}
		var resp SnapshotResponse
		if err := c.request(ctx, http.MethodDelete, "/playlists/"+url.PathEscape(playlistID)+"/items", nil, map[string]any{"items": items}, &resp); err != nil {
			return snapshot, err
		}
		snapshot = resp.SnapshotID
	}
	return snapshot, nil
}

func (c *Client) SaveLibraryItems(ctx context.Context, uris []string) error {
	for _, chunk := range BatchStrings(uris, 40) {
		q := url.Values{}
		q.Set("uris", strings.Join(chunk, ","))
		if err := c.request(ctx, http.MethodPut, "/me/library", q, nil, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) RemoveLibraryItems(ctx context.Context, uris []string) error {
	for _, chunk := range BatchStrings(uris, 40) {
		q := url.Values{}
		q.Set("uris", strings.Join(chunk, ","))
		if err := c.request(ctx, http.MethodDelete, "/me/library", q, nil, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) SearchTracks(ctx context.Context, query string, limit int, market string) ([]Track, error) {
	if limit <= 0 || limit > 10 {
		limit = 10
	}
	q := url.Values{}
	q.Set("type", "track")
	q.Set("q", query)
	q.Set("limit", strconv.Itoa(limit))
	if market != "" {
		q.Set("market", market)
	}
	var resp SearchTracksResponse
	if err := c.request(ctx, http.MethodGet, "/search", q, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Tracks.Items, nil
}

func (c *Client) GetTrack(ctx context.Context, id, market string) (Track, error) {
	q := url.Values{}
	if market != "" {
		q.Set("market", market)
	}
	var track Track
	err := c.request(ctx, http.MethodGet, "/tracks/"+url.PathEscape(id), q, nil, &track)
	return track, err
}

func (c *Client) request(ctx context.Context, method, path string, q url.Values, body any, out any) error {
	token, err := c.Tokens.AccessToken(ctx)
	if err != nil {
		return err
	}
	bodyBytes, err := encodeBody(body)
	if err != nil {
		return err
	}
	var lastErr error
	refreshed := false
	for attempt := 0; attempt < 5; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.url(path, q), bytes.NewReader(bodyBytes))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.httpClient().Do(req)
		if err != nil {
			lastErr = err
			if !sleep(ctx, backoff(attempt)) {
				return ctx.Err()
			}
			continue
		}
		err = c.handleResponse(ctx, resp, out)
		if err == nil {
			return nil
		}
		lastErr = err
		if resp.StatusCode == http.StatusUnauthorized && !refreshed {
			if refreshErr := c.Tokens.Refresh(ctx); refreshErr == nil {
				token, err = c.Tokens.AccessToken(ctx)
				if err != nil {
					return err
				}
				refreshed = true
				continue
			}
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			wait := retryAfter(resp.Header.Get("Retry-After"))
			if !sleep(ctx, wait) {
				return ctx.Err()
			}
			continue
		}
		if resp.StatusCode >= 500 {
			if !sleep(ctx, backoff(attempt)) {
				return ctx.Err()
			}
			continue
		}
		return err
	}
	return lastErr
}

func (c *Client) handleResponse(ctx context.Context, resp *http.Response, out any) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		if out == nil || resp.StatusCode == http.StatusNoContent {
			io.Copy(io.Discard, resp.Body)
			return nil
		}
		return json.NewDecoder(resp.Body).Decode(out)
	}
	var body bytes.Buffer
	_, _ = io.Copy(&body, resp.Body)
	return fmt.Errorf("spotify API %s returned %s: %s", resp.Request.URL.Path, resp.Status, strings.TrimSpace(body.String()))
}

func (c *Client) url(path string, q url.Values) string {
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	u := strings.TrimRight(base, "/") + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func encodeBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	return json.Marshal(body)
}

func retryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds < 1 {
		return time.Second
	}
	return time.Duration(seconds) * time.Second
}

func backoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt > 4 {
		attempt = 4
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func BatchStrings(values []string, size int) [][]string {
	if size <= 0 {
		return nil
	}
	var out [][]string
	for len(values) > 0 {
		n := size
		if len(values) < n {
			n = len(values)
		}
		out = append(out, append([]string(nil), values[:n]...))
		values = values[n:]
	}
	return out
}

func RawJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
