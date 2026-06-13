package playlists

import "testing"

func TestDedupePlanKeepsFirst(t *testing.T) {
	removals := DedupePlan([]TrackInstance{
		{TrackID: "a", TrackURI: "spotify:track:a", Position: 0},
		{TrackID: "b", TrackURI: "spotify:track:b", Position: 1},
		{TrackID: "a", TrackURI: "spotify:track:a", Position: 2},
	}, "first")
	if len(removals) != 1 {
		t.Fatalf("removals = %d, want 1", len(removals))
	}
	if removals[0].Position != 2 {
		t.Fatalf("removed position = %d, want 2", removals[0].Position)
	}
}

func TestDedupePlanKeepsLast(t *testing.T) {
	removals := DedupePlan([]TrackInstance{
		{TrackID: "a", TrackURI: "spotify:track:a", Position: 0},
		{TrackID: "a", TrackURI: "spotify:track:a", Position: 2},
	}, "last")
	if len(removals) != 1 || removals[0].Position != 0 {
		t.Fatalf("unexpected removals: %#v", removals)
	}
}
