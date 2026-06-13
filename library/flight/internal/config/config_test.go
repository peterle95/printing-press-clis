package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigWithEnvOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("FLIGHT_PP_CONFIG", path)
	cfg := Default()
	cfg.Providers.Amadeus.ClientID = "from-file"
	if err := Save(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	t.Setenv("AMADEUS_CLIENT_ID", "from-env")
	t.Setenv("AMADEUS_CLIENT_SECRET", "secret-env")
	loaded, err := Load("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.Providers.Amadeus.ClientID != "from-env" {
		t.Fatalf("expected env client ID override, got %q", loaded.Providers.Amadeus.ClientID)
	}
	if loaded.Providers.Amadeus.ClientSecret != "secret-env" {
		t.Fatalf("expected env client secret override")
	}
	if loaded.Path != path {
		t.Fatalf("expected path %s, got %s", path, loaded.Path)
	}
}

func TestCacheDirUsesXDGCacheOnUnix(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("XDG cache assertion is Unix-specific")
	}
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", base)
	dir, err := CacheDir()
	if err != nil {
		t.Fatalf("cache dir: %v", err)
	}
	want := filepath.Join(base, ProjectName, AppName, "cache")
	if dir != want {
		t.Fatalf("expected %s, got %s", want, dir)
	}
}
