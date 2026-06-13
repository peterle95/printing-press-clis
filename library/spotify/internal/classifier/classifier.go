package classifier

import (
	"fmt"
	"strings"
	"time"

	"spotify-pp-cli/internal/rules"
	"spotify-pp-cli/internal/store"
)

const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
	ConfidenceNone   = "none"
)

type Result struct {
	TrackID            string    `json:"track_id"`
	TrackURI           string    `json:"track_uri"`
	TrackName          string    `json:"track_name"`
	ArtistNames        []string  `json:"artist_names"`
	ArtistGenres       []string  `json:"artist_genres"`
	PrimaryGenre       string    `json:"primary_genre"`
	MatchedRule        string    `json:"matched_rule"`
	Confidence         string    `json:"confidence"`
	TargetPlaylistID   string    `json:"target_playlist_id,omitempty"`
	TargetPlaylistName string    `json:"target_playlist_name"`
	Explanation        string    `json:"explanation"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type Classifier struct {
	Rules *rules.File
}

type match struct {
	key         string
	rule        rules.PlaylistRule
	genre       string
	matcher     string
	exact       bool
	artistLevel bool
}

func New(ruleFile *rules.File) Classifier {
	return Classifier{Rules: ruleFile}
}

func (c Classifier) Classify(track store.TrackRecord, artists []store.ArtistRecord) Result {
	now := time.Now().UTC()
	artistNames := make([]string, 0, len(artists))
	genres := make([]string, 0)
	for _, artist := range artists {
		if artist.Name != "" {
			artistNames = append(artistNames, artist.Name)
		}
		genres = append(genres, artist.Genres...)
	}
	result := Result{
		TrackID:            track.ID,
		TrackURI:           track.URI,
		TrackName:          track.Name,
		ArtistNames:        artistNames,
		ArtistGenres:       genres,
		TargetPlaylistName: c.fallbackName(),
		MatchedRule:        "fallback",
		Confidence:         ConfidenceNone,
		Explanation:        "no rule matched artist genres",
		UpdatedAt:          now,
	}
	if c.Rules == nil {
		result.Explanation = "no rules file loaded"
		return result
	}
	if playlist := c.manualTrackOverride(track); playlist != "" {
		result.TargetPlaylistName = playlist
		result.PrimaryGenre = playlist
		result.MatchedRule = "manual_track_override"
		result.Confidence = ConfidenceHigh
		result.Explanation = fmt.Sprintf("manual track override routes %q to %q", track.URI, playlist)
		return result
	}
	if playlist := c.manualArtistOverride(artists); playlist != "" {
		result.TargetPlaylistName = playlist
		result.PrimaryGenre = playlist
		result.MatchedRule = "manual_artist_override"
		result.Confidence = ConfidenceHigh
		result.Explanation = fmt.Sprintf("manual artist override routes track to %q", playlist)
		return result
	}
	matches := c.findMatches(genres)
	if len(matches) == 0 {
		return result
	}
	chosen := matches[0]
	result.PrimaryGenre = chosen.genre
	result.MatchedRule = chosen.key
	result.TargetPlaylistName = chosen.rule.Name
	result.Confidence = ConfidenceMedium
	if chosen.exact {
		result.Confidence = ConfidenceHigh
	}
	if len(matches) > 1 {
		result.Confidence = ConfidenceLow
	}
	result.Explanation = fmt.Sprintf("artist genre %q matched rule %q via %q", chosen.genre, chosen.key, chosen.matcher)
	return result
}

func (c Classifier) findMatches(genres []string) []match {
	var matches []match
	for _, resolved := range c.Rules.SortedRules() {
		for _, artistGenre := range genres {
			normalizedGenre := rules.NormalizeGenre(artistGenre)
			if normalizedGenre == "" {
				continue
			}
			for _, rawMatcher := range resolved.Rule.Match.AnyArtistGenreContains {
				matcher := rules.NormalizeGenre(rawMatcher)
				if matcher == "" {
					continue
				}
				if normalizedGenre == matcher || strings.Contains(normalizedGenre, matcher) || strings.Contains(matcher, normalizedGenre) {
					matches = append(matches, match{
						key:         resolved.Key,
						rule:        resolved.Rule,
						genre:       normalizedGenre,
						matcher:     matcher,
						exact:       normalizedGenre == matcher,
						artistLevel: true,
					})
					break
				}
			}
			if len(matches) > 0 && matches[len(matches)-1].key == resolved.Key {
				break
			}
		}
	}
	return matches
}

func (c Classifier) fallbackName() string {
	if c.Rules == nil || c.Rules.Fallback.Name == "" {
		return "Needs Review"
	}
	return c.Rules.Fallback.Name
}

func (c Classifier) manualTrackOverride(track store.TrackRecord) string {
	for key, playlist := range c.Rules.ManualOverrides.Tracks {
		if key == track.URI || key == track.ID {
			return playlist
		}
	}
	return ""
}

func (c Classifier) manualArtistOverride(artists []store.ArtistRecord) string {
	for _, artist := range artists {
		for key, playlist := range c.Rules.ManualOverrides.Artists {
			if key == artist.URI || key == artist.ID {
				return playlist
			}
		}
	}
	return ""
}
