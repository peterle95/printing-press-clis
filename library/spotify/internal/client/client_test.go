package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeTokenProvider struct{}

func (fakeTokenProvider) AccessToken(context.Context) (string, error) { return "token", nil }
func (fakeTokenProvider) Refresh(context.Context) error               { return nil }

func TestSavedTracksUsesMockSpotifyAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("missing bearer token")
		}
		if r.URL.Path != "/me/tracks" {
			t.Fatalf("path = %s, want /me/tracks", r.URL.Path)
		}
		fmt.Fprint(w, `{"limit":1,"offset":0,"total":1,"items":[{"added_at":"2026-05-20T00:00:00Z","track":{"id":"t","uri":"spotify:track:t","name":"Track","artists":[],"album":{"name":"Album"}}}]}`)
	}))
	defer server.Close()
	c := New(fakeTokenProvider{})
	c.BaseURL = server.URL
	page, err := c.SavedTracks(context.Background(), 1, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Track.Name != "Track" {
		t.Fatalf("unexpected page: %#v", page)
	}
}

func TestClientBatchStrings(t *testing.T) {
	batches := BatchStrings([]string{"1", "2", "3"}, 2)
	if len(batches) != 2 || len(batches[1]) != 1 {
		t.Fatalf("unexpected batches: %#v", batches)
	}
}

func TestCreatePlaylistUsesCurrentUserEndpoint(t *testing.T) {
	var sawCreate bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/me":
			fmt.Fprint(w, `{"id":"user-1","display_name":"User"}`)
		case "/users/user-1/playlists":
			sawCreate = true
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			fmt.Fprint(w, `{"id":"p","uri":"spotify:playlist:p","name":"Techno","tracks":{"total":0}}`)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c := New(fakeTokenProvider{})
	c.BaseURL = server.URL
	playlist, err := c.CreatePlaylist(context.Background(), "Techno", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if !sawCreate || playlist.ID != "p" {
		t.Fatalf("playlist = %#v sawCreate=%v", playlist, sawCreate)
	}
}
