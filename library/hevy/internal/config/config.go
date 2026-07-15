// Package config resolves application paths and non-sensitive settings.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const appName = "hevy-cli"

type Config struct {
	DatabasePath      string `yaml:"database_path" json:"database_path"`
	PlansDirectory    string `yaml:"plans_directory" json:"plans_directory"`
	CSVDirectory      string `yaml:"csv_directory" json:"csv_directory"`
	BrowserProfileDir string `yaml:"browser_profile_directory" json:"browser_profile_directory"`
	Browser           string `yaml:"browser" json:"browser"`
	BrowserHeaded     bool   `yaml:"browser_headed" json:"browser_headed"`
	DefaultOutput     string `yaml:"default_output" json:"default_output"`
	LogLevel          string `yaml:"log_level" json:"log_level"`
}

func DataDir() (string, error) {
	if v := os.Getenv("HEVY_DATA_DIRECTORY"); v != "" {
		return filepath.Abs(v)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	// UserConfigDir is %LOCALAPPDATA% on Windows and ~/.config on Linux. Hevy
	// data belongs in ~/.local/share on Linux, so honour XDG_DATA_HOME first.
	if os.Getenv("XDG_DATA_HOME") != "" {
		return filepath.Join(os.Getenv("XDG_DATA_HOME"), appName), nil
	}
	if home, homeErr := os.UserHomeDir(); homeErr == nil && os.PathSeparator == '/' {
		return filepath.Join(home, ".local", "share", appName), nil
	}
	return filepath.Join(base, appName), nil
}

func ConfigPath() (string, error) { d, e := DataDir(); return filepath.Join(d, "config.yaml"), e }

func Default() (Config, error) {
	d, err := DataDir()
	if err != nil {
		return Config{}, err
	}
	return Config{DatabasePath: filepath.Join(d, "hevy.db"), PlansDirectory: filepath.Join(d, "plans"), BrowserProfileDir: filepath.Join(d, "browser-profile"), Browser: "chromium", BrowserHeaded: true, DefaultOutput: "table", LogLevel: "info"}, nil
}

func Load(path string) (Config, string, error) {
	cfg, err := Default()
	if err != nil {
		return Config{}, "", err
	}
	if path == "" {
		path, err = ConfigPath()
		if err != nil {
			return Config{}, "", err
		}
	}
	if raw, readErr := os.ReadFile(path); readErr == nil {
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return Config{}, path, fmt.Errorf("parse config %s: %w", path, err)
		}
	} else if !os.IsNotExist(readErr) {
		return Config{}, path, readErr
	}
	applyEnv(&cfg)
	if cfg.Browser == "" {
		cfg.Browser = "chromium"
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
	return atomicWrite(path, b, 0o600)
}

func atomicWrite(path string, b []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(mode); err == nil {
		_, err = f.Write(b)
		if err == nil {
			err = f.Sync()
		}
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func applyEnv(c *Config) {
	for _, item := range []struct {
		key string
		dst *string
	}{{"HEVY_DATABASE_PATH", &c.DatabasePath}, {"HEVY_PLANS_DIRECTORY", &c.PlansDirectory}, {"HEVY_CSV_DIRECTORY", &c.CSVDirectory}, {"HEVY_BROWSER_PROFILE_DIRECTORY", &c.BrowserProfileDir}, {"HEVY_BROWSER", &c.Browser}, {"HEVY_LOG_LEVEL", &c.LogLevel}} {
		if v := os.Getenv(item.key); v != "" {
			*item.dst = v
		}
	}
	if v := strings.ToLower(os.Getenv("HEVY_BROWSER_HEADED")); v == "true" {
		c.BrowserHeaded = true
	} else if v == "false" {
		c.BrowserHeaded = false
	}
}
