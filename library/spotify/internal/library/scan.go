package library

import (
	"context"
	"strings"
	"time"

	"spotify-pp-cli/internal/client"
	"spotify-pp-cli/internal/store"
)

type Scanner struct {
	Client *client.Client
	Store  *store.Store
	Market string
}

type ScanSummary struct {
	Tracks       int `json:"tracks"`
	Artists      int `json:"artists"`
	Playlists    int `json:"playlists"`
	PlaylistItem int `json:"playlist_items"`
}

func (s Scanner) ScanLiked(ctx context.Context, limit int, refreshArtists bool, progress func(done, total int)) (ScanSummary, error) {
	saved, err := s.Client.AllSavedTracks(ctx, limit, s.Market, progress)
	if err != nil {
		return ScanSummary{}, err
	}
	artistIDs := orderedSet{}
	for _, item := range saved {
		if item.Track.ID == "" {
			continue
		}
		if err := s.Store.UpsertTrack(ctx, TrackRecord(item.Track, item.AddedAt)); err != nil {
			return ScanSummary{}, err
		}
		for i, artist := range item.Track.Artists {
			if artist.ID == "" {
				continue
			}
			artistIDs.add(artist.ID)
			_ = s.Store.UpsertArtist(ctx, store.ArtistRecord{ID: artist.ID, URI: artist.URI, Name: artist.Name})
			if err := s.Store.UpsertTrackArtist(ctx, item.Track.ID, artist.ID, i); err != nil {
				return ScanSummary{}, err
			}
		}
	}
	artists, err := s.Client.GetArtists(ctx, artistIDs.values)
	if err != nil {
		return ScanSummary{}, err
	}
	for _, artist := range artists {
		if artist.ID == "" {
			continue
		}
		if err := s.Store.UpsertArtist(ctx, ArtistRecord(artist)); err != nil {
			return ScanSummary{}, err
		}
	}
	return ScanSummary{Tracks: len(saved), Artists: len(artists)}, nil
}

func (s Scanner) ScanPlaylists(ctx context.Context) ([]client.Playlist, error) {
	playlists, err := s.Client.CurrentUserPlaylists(ctx)
	if err != nil {
		return nil, err
	}
	for _, playlist := range playlists {
		if err := s.Store.UpsertPlaylist(ctx, PlaylistRecord(playlist)); err != nil {
			return nil, err
		}
	}
	return playlists, nil
}

func (s Scanner) ScanPlaylist(ctx context.Context, playlist client.Playlist) (ScanSummary, error) {
	items, err := s.Client.PlaylistItems(ctx, playlist.ID, s.Market)
	if err != nil {
		return ScanSummary{}, err
	}
	tracks := make([]store.PlaylistTrackRecord, 0, len(items))
	now := time.Now().UTC().Format(time.RFC3339)
	for position, item := range items {
		track := item.Track
		if track.ID == "" && item.Item.ID != "" {
			track = item.Item
		}
		trackID := track.ID
		if trackID == "" {
			trackID = track.URI
		}
		if track.ID != "" {
			if err := s.Store.UpsertTrack(ctx, TrackRecord(track, "")); err != nil {
				return ScanSummary{}, err
			}
			for i, artist := range track.Artists {
				if artist.ID == "" {
					continue
				}
				_ = s.Store.UpsertArtist(ctx, store.ArtistRecord{ID: artist.ID, URI: artist.URI, Name: artist.Name})
				if err := s.Store.UpsertTrackArtist(ctx, track.ID, artist.ID, i); err != nil {
					return ScanSummary{}, err
				}
			}
		}
		tracks = append(tracks, store.PlaylistTrackRecord{
			PlaylistID: playlist.ID,
			TrackID:    trackID,
			TrackURI:   track.URI,
			Position:   position,
			AddedAt:    item.AddedAt,
			AddedBy:    item.AddedBy.ID,
			IsLocal:    item.IsLocal || track.IsLocal || track.ID == "",
			LastSeenAt: now,
		})
	}
	if err := s.Store.ReplacePlaylistTracks(ctx, playlist.ID, tracks); err != nil {
		return ScanSummary{}, err
	}
	rec := PlaylistRecord(playlist)
	rec.TrackCount = len(items)
	rec.LastFetchedAt = now
	if err := s.Store.UpsertPlaylist(ctx, rec); err != nil {
		return ScanSummary{}, err
	}
	return ScanSummary{PlaylistItem: len(items)}, nil
}

func TrackRecord(track client.Track, addedAt string) store.TrackRecord {
	return store.TrackRecord{
		ID:           track.ID,
		URI:          track.URI,
		Name:         track.Name,
		AlbumName:    track.Album.Name,
		AlbumID:      track.Album.ID,
		DurationMS:   track.DurationMS,
		Explicit:     track.Explicit,
		Popularity:   track.Popularity,
		AddedAtLiked: addedAt,
		RawJSON:      client.RawJSON(track),
	}
}

func ArtistRecord(artist client.Artist) store.ArtistRecord {
	return store.ArtistRecord{
		ID:                        artist.ID,
		URI:                       artist.URI,
		Name:                      artist.Name,
		Genres:                    artist.Genres,
		DeprecatedGenresAvailable: len(artist.Genres) > 0,
		RawJSON:                   client.RawJSON(artist),
	}
}

func PlaylistRecord(playlist client.Playlist) store.PlaylistRecord {
	return store.PlaylistRecord{
		ID:            playlist.ID,
		URI:           playlist.URI,
		Name:          playlist.Name,
		OwnerID:       playlist.Owner.ID,
		OwnerName:     playlist.Owner.DisplayName,
		Public:        playlist.Public,
		Collaborative: playlist.Collaborative,
		SnapshotID:    playlist.SnapshotID,
		TrackCount:    playlist.Tracks.Total,
		RawJSON:       client.RawJSON(playlist),
	}
}

func PlaylistByName(playlists []client.Playlist, name string) (client.Playlist, bool) {
	for _, playlist := range playlists {
		if stringsEqualFold(playlist.Name, name) {
			return playlist, true
		}
	}
	return client.Playlist{}, false
}

type orderedSet struct {
	seen   map[string]bool
	values []string
}

func (s *orderedSet) add(v string) {
	if s.seen == nil {
		s.seen = map[string]bool{}
	}
	if s.seen[v] {
		return
	}
	s.seen[v] = true
	s.values = append(s.values, v)
}

func stringsEqualFold(a, b string) bool {
	return strings.EqualFold(a, b)
}
