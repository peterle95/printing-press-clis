package classifier

import (
	"testing"

	"spotify-pp-cli/internal/rules"
	"spotify-pp-cli/internal/store"
)

func TestClassificationPriorityChoosesLowestPriorityRule(t *testing.T) {
	rf := &rules.File{
		Playlists: map[string]rules.PlaylistRule{
			"house": {Name: "House", Priority: 20, Match: rules.MatchRule{AnyArtistGenreContains: []string{"house"}}},
			"dance": {Name: "Dance", Priority: 10, Match: rules.MatchRule{AnyArtistGenreContains: []string{"deep house"}}},
		},
		Fallback: rules.FallbackRule{Name: "Needs Review"},
	}
	result := New(rf).Classify(track(), []store.ArtistRecord{{ID: "a", Name: "Artist", Genres: []string{"deep house"}}})
	if result.TargetPlaylistName != "Dance" {
		t.Fatalf("target = %q, want Dance", result.TargetPlaylistName)
	}
	if result.MatchedRule != "dance" {
		t.Fatalf("rule = %q, want dance", result.MatchedRule)
	}
	if result.Confidence != ConfidenceLow {
		t.Fatalf("confidence = %q, want low for ambiguous multi-match", result.Confidence)
	}
}

func TestFallbackClassification(t *testing.T) {
	rf := &rules.File{
		Playlists: map[string]rules.PlaylistRule{
			"techno": {Name: "Techno", Priority: 10, Match: rules.MatchRule{AnyArtistGenreContains: []string{"techno"}}},
		},
		Fallback: rules.FallbackRule{Name: "Needs Review"},
	}
	result := New(rf).Classify(track(), []store.ArtistRecord{{ID: "a", Name: "Artist"}})
	if result.TargetPlaylistName != "Needs Review" {
		t.Fatalf("target = %q, want Needs Review", result.TargetPlaylistName)
	}
	if result.Confidence != ConfidenceNone {
		t.Fatalf("confidence = %q, want none", result.Confidence)
	}
}

func track() store.TrackRecord {
	return store.TrackRecord{ID: "t", URI: "spotify:track:t", Name: "Track"}
}
