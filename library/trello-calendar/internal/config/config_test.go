package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrecedenceAndCredentialsStayOutOfTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := []byte("timezone = \"Europe/Berlin\"\ntrello_board_id = \"file-board\"\ntrello_list_id = \"file-list\"\ngoogle_calendar_id = \"file-calendar\"\nduration_minutes = 90\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRELLO_BOARD_ID", "env-board")
	t.Setenv("TRELLO_API_KEY", "key")
	t.Setenv("TRELLO_TOKEN", "token")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TrelloBoardID != "env-board" || cfg.TrelloListID != "file-list" || cfg.DurationMinutes != 90 {
		t.Fatalf("unexpected precedence: %#v", cfg)
	}
	if cfg.AuthHeader() != `OAuth oauth_consumer_key="key", oauth_token="token"` {
		t.Fatalf("unexpected auth header")
	}
}

func TestBoardAwareDefaultsAndLegacyListCompatibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := []byte("timezone = \"Europe/Berlin\"\ntrello_board_id = \"board\"\ngoogle_calendar_id = \"primary\"\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidatePlanner(); err != nil {
		t.Fatalf("board-aware planner must not require trello_list_id: %v", err)
	}
	if cfg.TrelloListID != "" {
		t.Fatalf("unexpected legacy list id: %q", cfg.TrelloListID)
	}
	if cfg.DoingListName != "Doing" {
		t.Fatalf("unexpected names: %#v", cfg)
	}
	if len(cfg.SourceListNames) != 2 || cfg.SourceListNames[0] != "Peter" || cfg.SourceListNames[1] != "Peter & Liliia" {
		t.Fatalf("unexpected source lists: %#v", cfg.SourceListNames)
	}
	if len(cfg.ExcludeListNames) != 2 || cfg.ExcludeListNames[0] != "Doing" || cfg.ExcludeListNames[1] != "Done" {
		t.Fatalf("unexpected exclude lists: %#v", cfg.ExcludeListNames)
	}

	legacyPath := filepath.Join(t.TempDir(), "legacy.toml")
	legacyData := []byte("timezone = \"Europe/Berlin\"\ntrello_board_id = \"board\"\ntrello_list_id = \"list\"\ngoogle_calendar_id = \"primary\"\nsource_list_names = []\n")
	if err := os.WriteFile(legacyPath, legacyData, 0o600); err != nil {
		t.Fatal(err)
	}
	legacy, err := Load(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.TrelloListID != "list" || len(legacy.SourceListNames) != 0 {
		t.Fatalf("legacy compatibility broken: %#v", legacy)
	}
}

func TestLoadRejectsUnknownAndInvalidSettings(t *testing.T) {
	for name, contents := range map[string]string{
		"unknown": "unknown_key = true\n",
		"window":  "day_start = \"18:00\"\nday_end = \"09:00\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
