package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ProjectName       = "printing-press"
	AppName           = "transit"
	DefaultConfigFile = "config.yaml"
	DefaultBaseURL    = "https://v6.vbb.transport.rest"
)

type Config struct {
	Provider string         `yaml:"provider" json:"provider"`
	BaseURL  string         `yaml:"base_url" json:"base_url"`
	Home     HomeConfig     `yaml:"home" json:"home"`
	Defaults DefaultsConfig `yaml:"defaults" json:"defaults"`
	Path     string         `yaml:"-" json:"path,omitempty"`
}

type HomeConfig struct {
	Label     string   `yaml:"label" json:"label"`
	Address   string   `yaml:"address" json:"address,omitempty"`
	Latitude  *float64 `yaml:"latitude" json:"latitude,omitempty"`
	Longitude *float64 `yaml:"longitude" json:"longitude,omitempty"`
}

type DefaultsConfig struct {
	RadiusMeters           int        `yaml:"radius_meters" json:"radius_meters"`
	DepartureWindowMinutes int        `yaml:"departure_window_minutes" json:"departure_window_minutes"`
	RefreshSeconds         int        `yaml:"refresh_seconds" json:"refresh_seconds"`
	WalkingSpeed           string     `yaml:"walking_speed" json:"walking_speed"`
	SafetyBufferMinutes    int        `yaml:"safety_buffer_minutes" json:"safety_buffer_minutes"`
	Modes                  ModeConfig `yaml:"modes" json:"modes"`
}

type ModeConfig struct {
	Suburban bool `yaml:"suburban" json:"suburban"`
	Subway   bool `yaml:"subway" json:"subway"`
	Tram     bool `yaml:"tram" json:"tram"`
	Bus      bool `yaml:"bus" json:"bus"`
	Ferry    bool `yaml:"ferry" json:"ferry"`
	Express  bool `yaml:"express" json:"express"`
	Regional bool `yaml:"regional" json:"regional"`
}

func Default() Config {
	return Config{
		Provider: "vbb_transport_rest",
		BaseURL:  DefaultBaseURL,
		Home: HomeConfig{
			Label: "home",
		},
		Defaults: DefaultsConfig{
			RadiusMeters:           1000,
			DepartureWindowMinutes: 20,
			RefreshSeconds:         30,
			WalkingSpeed:           "normal",
			SafetyBufferMinutes:    5,
			Modes: ModeConfig{
				Suburban: true,
				Subway:   true,
				Tram:     true,
				Bus:      true,
				Ferry:    false,
				Express:  false,
				Regional: true,
			},
		},
	}
}

func Load(path string) (Config, error) {
	resolved, err := ResolveConfigPath(path)
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	cfg.Path = resolved
	if data, err := os.ReadFile(resolved); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse config %s: %w", resolved, err)
		}
	} else if !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("read config %s: %w", resolved, err)
	}
	cfg.Path = resolved
	cfg.applyDefaults()
	cfg.applyEnv()
	return cfg, nil
}

func Save(path string, cfg Config) error {
	resolved, err := ResolveConfigPath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
		return err
	}
	cfg.Path = ""
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(resolved, data, 0o600)
}

func ResolveConfigPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = os.Getenv("TRANSIT_PP_CONFIG")
	}
	if strings.TrimSpace(path) == "" {
		dir, err := ConfigDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(dir, DefaultConfigFile)
	}
	return expandHome(path)
}

func ConfigDir() (string, error) {
	base := ""
	if runtime.GOOS == "windows" {
		base = os.Getenv("APPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Roaming")
		}
	} else {
		base = os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, ProjectName, AppName), nil
}

func CacheDir() (string, error) {
	base := ""
	if runtime.GOOS == "windows" {
		base = os.Getenv("LOCALAPPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Local")
		}
	} else {
		base = os.Getenv("XDG_CACHE_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".cache")
		}
	}
	return filepath.Join(base, ProjectName, AppName), nil
}

func (c Config) HomeHasCoordinates() bool {
	return c.Home.Latitude != nil && c.Home.Longitude != nil
}

func (c *Config) applyDefaults() {
	if strings.TrimSpace(c.Provider) == "" {
		c.Provider = "vbb_transport_rest"
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		c.BaseURL = DefaultBaseURL
	}
	if strings.TrimSpace(c.Home.Label) == "" {
		c.Home.Label = "home"
	}
	if c.Defaults.RadiusMeters <= 0 {
		c.Defaults.RadiusMeters = 1000
	}
	if c.Defaults.DepartureWindowMinutes <= 0 {
		c.Defaults.DepartureWindowMinutes = 20
	}
	if c.Defaults.RefreshSeconds <= 0 {
		c.Defaults.RefreshSeconds = 30
	}
	if strings.TrimSpace(c.Defaults.WalkingSpeed) == "" {
		c.Defaults.WalkingSpeed = "normal"
	}
	if c.Defaults.SafetyBufferMinutes < 0 {
		c.Defaults.SafetyBufferMinutes = 0
	}
	if !c.Defaults.Modes.Any() {
		c.Defaults.Modes = Default().Defaults.Modes
	}
}

func (c *Config) applyEnv() {
	if value := strings.TrimSpace(os.Getenv("TRANSIT_PP_BASE_URL")); value != "" {
		c.BaseURL = value
	}
}

func (m ModeConfig) Any() bool {
	return m.Suburban || m.Subway || m.Tram || m.Bus || m.Ferry || m.Express || m.Regional
}

func expandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
