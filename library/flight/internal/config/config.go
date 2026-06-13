package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ProjectName       = "printing-press"
	AppName           = "flight"
	DefaultConfigFile = "config.yaml"
	DefaultCacheTTL   = 15
)

type Config struct {
	Providers ProvidersConfig `yaml:"providers" json:"providers"`
	Defaults  DefaultsConfig  `yaml:"defaults" json:"defaults"`
	Path      string          `yaml:"-" json:"path"`
}

type ProvidersConfig struct {
	Amadeus       AmadeusConfig       `yaml:"amadeus" json:"amadeus"`
	Kiwi          KiwiConfig          `yaml:"kiwi" json:"kiwi"`
	Google        GoogleConfig        `yaml:"google" json:"google"`
	Skyscanner    APIKeyConfig        `yaml:"skyscanner" json:"skyscanner"`
	Expedia       ModeAPIKeyConfig    `yaml:"expedia" json:"expedia"`
	Kayak         ModeAPIKeyConfig    `yaml:"kayak" json:"kayak"`
	Travelpayouts TravelpayoutsConfig `yaml:"travelpayouts" json:"travelpayouts"`
}

type AmadeusConfig struct {
	Enabled      bool   `yaml:"enabled" json:"enabled"`
	ClientID     string `yaml:"client_id" json:"client_id,omitempty"`
	ClientSecret string `yaml:"client_secret" json:"client_secret,omitempty"`
	Environment  string `yaml:"environment" json:"environment"`
}

type KiwiConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	APIKey  string `yaml:"api_key" json:"api_key,omitempty"`
}

type GoogleConfig struct {
	Enabled      bool   `yaml:"enabled" json:"enabled"`
	Mode         string `yaml:"mode" json:"mode"`
	SerpAPIKey   string `yaml:"serpapi_key" json:"serpapi_key,omitempty"`
	SearchAPIKey string `yaml:"searchapi_key" json:"searchapi_key,omitempty"`
}

type APIKeyConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	APIKey  string `yaml:"api_key" json:"api_key,omitempty"`
}

type ModeAPIKeyConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	APIKey  string `yaml:"api_key" json:"api_key,omitempty"`
	Mode    string `yaml:"mode" json:"mode,omitempty"`
}

type TravelpayoutsConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Token   string `yaml:"token" json:"token,omitempty"`
}

type DefaultsConfig struct {
	Currency        string `yaml:"currency" json:"currency"`
	Adults          int    `yaml:"adults" json:"adults"`
	Cabin           string `yaml:"cabin" json:"cabin"`
	HomeAirport     string `yaml:"home_airport" json:"home_airport"`
	CacheTTLMinutes int    `yaml:"cache_ttl_minutes" json:"cache_ttl_minutes"`
}

func Default() Config {
	return Config{
		Providers: ProvidersConfig{
			Amadeus: AmadeusConfig{
				Enabled:     true,
				Environment: "test",
			},
			Kiwi: KiwiConfig{
				Enabled: false,
			},
			Google: GoogleConfig{
				Enabled: true,
				Mode:    "deeplink",
			},
			Skyscanner: APIKeyConfig{
				Enabled: false,
			},
			Expedia: ModeAPIKeyConfig{
				Enabled: false,
				Mode:    "deeplink",
			},
			Kayak: ModeAPIKeyConfig{
				Enabled: false,
				Mode:    "deeplink",
			},
			Travelpayouts: TravelpayoutsConfig{
				Enabled: false,
			},
		},
		Defaults: DefaultsConfig{
			Currency:        "EUR",
			Adults:          1,
			Cabin:           "economy",
			HomeAirport:     "BER",
			CacheTTLMinutes: DefaultCacheTTL,
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
	if path == "" {
		path = os.Getenv("FLIGHT_PP_CONFIG")
	}
	if path == "" {
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
	return filepath.Join(base, ProjectName, AppName, "cache"), nil
}

func WatchesPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "watches.json"), nil
}

func ProviderNames() []string {
	return []string{"amadeus", "kiwi", "google", "skyscanner", "expedia", "kayak", "travelpayouts"}
}

func (c Config) EnabledProviderNames() []string {
	names := []string{}
	for _, name := range ProviderNames() {
		if c.ProviderEnabled(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (c Config) ProviderEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "amadeus":
		return c.Providers.Amadeus.Enabled
	case "kiwi":
		return c.Providers.Kiwi.Enabled
	case "google", "google-flights":
		return c.Providers.Google.Enabled
	case "skyscanner":
		return c.Providers.Skyscanner.Enabled
	case "expedia":
		return c.Providers.Expedia.Enabled
	case "kayak":
		return c.Providers.Kayak.Enabled
	case "travelpayouts":
		return c.Providers.Travelpayouts.Enabled
	default:
		return false
	}
}

func ParseProviderList(value string) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range strings.Split(value, ",") {
		name := strings.ToLower(strings.TrimSpace(item))
		if name == "google-flights" {
			name = "google"
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func (c *Config) applyDefaults() {
	if c.Providers.Amadeus.Environment == "" {
		c.Providers.Amadeus.Environment = "test"
	}
	if c.Providers.Google.Mode == "" {
		c.Providers.Google.Mode = "deeplink"
	}
	if c.Providers.Expedia.Mode == "" {
		c.Providers.Expedia.Mode = "deeplink"
	}
	if c.Providers.Kayak.Mode == "" {
		c.Providers.Kayak.Mode = "deeplink"
	}
	if c.Defaults.Currency == "" {
		c.Defaults.Currency = "EUR"
	}
	if c.Defaults.Adults == 0 {
		c.Defaults.Adults = 1
	}
	if c.Defaults.Cabin == "" {
		c.Defaults.Cabin = "economy"
	}
	if c.Defaults.HomeAirport == "" {
		c.Defaults.HomeAirport = "BER"
	}
	if c.Defaults.CacheTTLMinutes == 0 {
		c.Defaults.CacheTTLMinutes = DefaultCacheTTL
	}
}

func (c *Config) applyEnv() {
	if v := os.Getenv("AMADEUS_CLIENT_ID"); v != "" {
		c.Providers.Amadeus.ClientID = v
	}
	if v := os.Getenv("AMADEUS_CLIENT_SECRET"); v != "" {
		c.Providers.Amadeus.ClientSecret = v
	}
	if v := os.Getenv("AMADEUS_ENVIRONMENT"); v != "" {
		c.Providers.Amadeus.Environment = strings.ToLower(v)
	}
	if v := os.Getenv("KIWI_API_KEY"); v != "" {
		c.Providers.Kiwi.APIKey = v
	}
	if v := os.Getenv("SERPAPI_KEY"); v != "" {
		c.Providers.Google.SerpAPIKey = v
	}
	if v := os.Getenv("SEARCHAPI_KEY"); v != "" {
		c.Providers.Google.SearchAPIKey = v
	}
	if v := os.Getenv("TRAVELPAYOUTS_TOKEN"); v != "" {
		c.Providers.Travelpayouts.Token = v
	}
}

func expandHome(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		return filepath.Join(home, path[2:]), nil
	}
	return "", fmt.Errorf("cannot expand path %q", path)
}
