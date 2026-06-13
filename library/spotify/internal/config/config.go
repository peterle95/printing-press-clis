package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	AppName           = "spotify"
	ProjectName       = "printing-press"
	DefaultConfigFile = "config.yaml"
	DefaultRulesFile  = "rules.yaml"
	DefaultDBFile     = "spotify.db"
)

type Config struct {
	ClientID                     string `yaml:"client_id" json:"client_id"`
	ClientSecret                 string `yaml:"client_secret,omitempty" json:"client_secret,omitempty"`
	RedirectURI                  string `yaml:"redirect_uri,omitempty" json:"redirect_uri,omitempty"`
	Market                       string `yaml:"market,omitempty" json:"market,omitempty"`
	DefaultMode                  string `yaml:"default_mode,omitempty" json:"default_mode,omitempty"`
	LikedMoveRemoveAfterAdd      bool   `yaml:"liked_move_remove_after_add" json:"liked_move_remove_after_add"`
	UseDeprecatedArtistGenres    bool   `yaml:"use_deprecated_artist_genres,omitempty" json:"use_deprecated_artist_genres,omitempty"`
	AllowDeprecatedSavedTrackAPI bool   `yaml:"allow_deprecated_saved_track_api,omitempty" json:"allow_deprecated_saved_track_api,omitempty"`
}

func Default() Config {
	return Config{
		Market:                    "DE",
		DefaultMode:               "dry-run",
		LikedMoveRemoveAfterAdd:   false,
		UseDeprecatedArtistGenres: true,
	}
}

func Load(path string) (Config, string, error) {
	cfg := Default()
	if path == "" {
		var err error
		path, err = ConfigPath()
		if err != nil {
			return cfg, "", err
		}
	}
	if b, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return cfg, path, fmt.Errorf("parse config %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return cfg, path, err
	}
	applyEnv(&cfg)
	if cfg.DefaultMode == "" {
		cfg.DefaultMode = "dry-run"
	}
	return cfg, path, nil
}

func Save(path string, cfg Config) error {
	if path == "" {
		var err error
		path, err = ConfigPath()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func Set(cfg *Config, key, value string) error {
	switch key {
	case "client_id":
		cfg.ClientID = value
	case "client_secret":
		cfg.ClientSecret = value
	case "redirect_uri":
		cfg.RedirectURI = value
	case "market":
		cfg.Market = value
	case "default_mode":
		if value != "dry-run" && value != "live" {
			return fmt.Errorf("default_mode must be dry-run or live")
		}
		cfg.DefaultMode = value
	case "liked_move_remove_after_add":
		switch value {
		case "true":
			cfg.LikedMoveRemoveAfterAdd = true
		case "false":
			cfg.LikedMoveRemoveAfterAdd = false
		default:
			return fmt.Errorf("liked_move_remove_after_add must be true or false")
		}
	case "use_deprecated_artist_genres":
		switch value {
		case "true":
			cfg.UseDeprecatedArtistGenres = true
		case "false":
			cfg.UseDeprecatedArtistGenres = false
		default:
			return fmt.Errorf("use_deprecated_artist_genres must be true or false")
		}
	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	return nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DefaultConfigFile), nil
}

func RulesPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DefaultRulesFile), nil
}

func TokenPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "token.json"), nil
}

func DBPath() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, ProjectName, AppName, DefaultDBFile), nil
}

func ConfigDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, ProjectName, AppName), nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("SPOTIFY_CLIENT_ID"); v != "" {
		cfg.ClientID = v
	}
	if v := os.Getenv("SPOTIFY_CLIENT_SECRET"); v != "" {
		cfg.ClientSecret = v
	}
	if v := os.Getenv("SPOTIFY_REDIRECT_URI"); v != "" {
		cfg.RedirectURI = v
	}
}
