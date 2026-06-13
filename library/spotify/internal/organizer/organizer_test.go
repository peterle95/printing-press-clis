package organizer

import (
	"testing"

	"spotify-pp-cli/internal/classifier"
	"spotify-pp-cli/internal/store"
)

func TestBuildPlanDryRunCounts(t *testing.T) {
	tracks := []store.TrackRecord{
		{ID: "1", URI: "spotify:track:1", Name: "One"},
		{ID: "2", URI: "spotify:track:2", Name: "Two"},
	}
	results := map[string]classifier.Result{
		"1": {TrackID: "1", TargetPlaylistName: "Techno", Confidence: classifier.ConfidenceHigh},
		"2": {TrackID: "2", TargetPlaylistName: "Needs Review", Confidence: classifier.ConfidenceNone},
	}
	plan := BuildPlan("liked copy", true, tracks, results)
	if !plan.DryRun {
		t.Fatal("plan should be dry-run")
	}
	if plan.Classified != 1 || plan.NeedsReview != 1 {
		t.Fatalf("classified=%d needsReview=%d, want 1/1", plan.Classified, plan.NeedsReview)
	}
	if plan.ByPlaylist["Techno"] != 1 || plan.ByPlaylist["Needs Review"] != 1 {
		t.Fatalf("unexpected playlist counts: %#v", plan.ByPlaylist)
	}
}

func TestBatchStrings(t *testing.T) {
	got := BatchStrings([]string{"a", "b", "c", "d", "e"}, 2)
	if len(got) != 3 || len(got[0]) != 2 || len(got[2]) != 1 {
		t.Fatalf("unexpected batches: %#v", got)
	}
}

func TestBuildUndoPlan(t *testing.T) {
	op := store.OperationRecord{ID: 42}
	items := []store.OperationItemRecord{
		{Action: "playlist_add", Status: "success", TrackURI: "spotify:track:1", TargetPlaylistID: "p"},
		{Action: "library_remove", Status: "success", TrackURI: "spotify:track:1"},
		{Action: "playlist_add", Status: "failed", TrackURI: "spotify:track:2", TargetPlaylistID: "p"},
	}
	plan := BuildUndoPlan(op, items)
	if len(plan.AddToLibrary) != 1 {
		t.Fatalf("AddToLibrary length = %d, want 1", len(plan.AddToLibrary))
	}
	if len(plan.RemovePlaylist["p"]) != 1 {
		t.Fatalf("RemovePlaylist[p] length = %d, want 1", len(plan.RemovePlaylist["p"]))
	}
}
