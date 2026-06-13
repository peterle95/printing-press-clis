package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadChannelIDFilter(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "channels.txt")
	if err := os.WriteFile(path, []byte("# comments are ignored\nUCone\n\nUCtwo channel title\n"), 0o600); err != nil {
		t.Fatalf("write channel file: %v", err)
	}

	filter, err := loadChannelIDFilter(path)
	if err != nil {
		t.Fatalf("loadChannelIDFilter returned error: %v", err)
	}
	for _, id := range []string{"UCone", "UCtwo"} {
		if !filter[id] {
			t.Fatalf("expected %s in filter, got %#v", id, filter)
		}
	}
	if filter["channel"] {
		t.Fatalf("expected only first whitespace-delimited field per line, got %#v", filter)
	}
}

func TestFilterSubscribedChannels(t *testing.T) {
	t.Parallel()

	channels := []subscribedChannel{
		{ChannelID: "UCkeep", ChannelTitle: "Keep"},
		{ChannelID: "UCdrop", ChannelTitle: "Drop"},
	}

	filtered := filterSubscribedChannels(channels, map[string]bool{"UCkeep": true})
	if len(filtered) != 1 {
		t.Fatalf("got %d channels, want 1: %#v", len(filtered), filtered)
	}
	if filtered[0].ChannelID != "UCkeep" {
		t.Fatalf("got channel %q, want UCkeep", filtered[0].ChannelID)
	}
}

func TestResolveAllBellChannelIDFileUsesEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "all-bell.txt")
	if err := os.WriteFile(path, []byte("UCone\n"), 0o600); err != nil {
		t.Fatalf("write channel file: %v", err)
	}
	t.Setenv("YOUTUBE_PP_ALL_BELL_CHANNEL_ID_FILE", path)

	got, err := resolveAllBellChannelIDFile("")
	if err != nil {
		t.Fatalf("resolveAllBellChannelIDFile returned error: %v", err)
	}
	if got != path {
		t.Fatalf("resolved path = %q, want %q", got, path)
	}
}

func TestResolveAllBellChannelIDFileExplicitWins(t *testing.T) {
	explicit := filepath.Join(t.TempDir(), "missing-is-ok-when-explicit.txt")
	t.Setenv("YOUTUBE_PP_ALL_BELL_CHANNEL_ID_FILE", filepath.Join(t.TempDir(), "other.txt"))

	got, err := resolveAllBellChannelIDFile(explicit)
	if err != nil {
		t.Fatalf("resolveAllBellChannelIDFile returned error: %v", err)
	}
	if got != explicit {
		t.Fatalf("resolved path = %q, want %q", got, explicit)
	}
}
