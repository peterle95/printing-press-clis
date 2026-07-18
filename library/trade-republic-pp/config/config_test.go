package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadPrivateConfig(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DatabasePath = filepath.Join(t.TempDir(), "data.db")
	cfg.Risk.PermittedISINs = []string{"IE00B4L5Y983"}
	if _, err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions = %o", info.Mode().Perm())
	}
	loaded, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Risk.MaxOrderValue != cfg.Risk.MaxOrderValue || len(loaded.Risk.PermittedISINs) != 1 {
		t.Fatalf("loaded config mismatch: %#v", loaded)
	}
}

func TestEnvironmentCannotDisableSafety(t *testing.T) {
	t.Setenv("TRPP_KILL_SWITCH", "false")
	t.Setenv("TRPP_PAPER_TRADING", "false")
	cfg, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	applyEnvironment(&cfg)
	if !cfg.Risk.KillSwitch || !cfg.Risk.PaperTrading {
		t.Fatal("environment disabled an execution safety control")
	}
}
