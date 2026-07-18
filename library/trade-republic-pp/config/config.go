// Package config resolves private runtime paths and non-secret settings.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"trade-republic-pp-cli/internal/money"
)

const appName = "trade-republic"

type Duration time.Duration

func (d Duration) Duration() time.Duration { return time.Duration(d) }
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}
func (d *Duration) UnmarshalText(text []byte) error {
	value, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = Duration(value)
	return nil
}

type RiskConfig struct {
	KillSwitch      bool          `yaml:"kill_switch" json:"kill_switch"`
	PaperTrading    bool          `yaml:"paper_trading" json:"paper_trading"`
	MaxOrderValue   money.Decimal `yaml:"max_order_value" json:"max_order_value"`
	MaxDailyValue   money.Decimal `yaml:"max_daily_value" json:"max_daily_value"`
	PermittedISINs  []string      `yaml:"permitted_isins" json:"permitted_isins"`
	PriceMaxAge     Duration      `yaml:"price_max_age" json:"price_max_age"`
	BalanceMaxAge   Duration      `yaml:"balance_max_age" json:"balance_max_age"`
	PreviewValidity Duration      `yaml:"preview_validity" json:"preview_validity"`
}

type Config struct {
	DatabasePath       string     `yaml:"database_path" json:"database_path"`
	DocumentsDirectory string     `yaml:"documents_directory" json:"documents_directory"`
	StagingDirectory   string     `yaml:"staging_directory" json:"staging_directory"`
	AccountTimezone    string     `yaml:"account_timezone" json:"account_timezone"`
	BaseCurrency       string     `yaml:"base_currency" json:"base_currency"`
	PytrCommand        []string   `yaml:"pytr_command" json:"pytr_command"`
	PDFToTextCommand   []string   `yaml:"pdftotext_command" json:"pdftotext_command"`
	PytrTimeout        Duration   `yaml:"pytr_timeout" json:"pytr_timeout"`
	Risk               RiskConfig `yaml:"risk" json:"risk"`
}

func dataDirectory() (string, error) {
	if value := os.Getenv("TRPP_DATA_HOME"); value != "" {
		return filepath.Abs(value)
	}
	if value := os.Getenv("XDG_DATA_HOME"); value != "" {
		return filepath.Join(value, "printing-press", appName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if os.PathSeparator == '/' {
		return filepath.Join(home, ".local", "share", "printing-press", appName), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "printing-press", appName, "data"), nil
}

func ConfigPath() (string, error) {
	if value := os.Getenv("TRPP_CONFIG_HOME"); value != "" {
		path, err := filepath.Abs(value)
		return filepath.Join(path, "config.yaml"), err
	}
	if value := os.Getenv("XDG_CONFIG_HOME"); value != "" {
		return filepath.Join(value, "printing-press", appName, "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if os.PathSeparator == '/' {
		return filepath.Join(home, ".config", "printing-press", appName, "config.yaml"), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "printing-press", appName, "config.yaml"), nil
}

func Default() (Config, error) {
	data, err := dataDirectory()
	if err != nil {
		return Config{}, err
	}
	return Config{
		DatabasePath:       filepath.Join(data, "trade-republic.db"),
		DocumentsDirectory: filepath.Join(data, "documents"),
		StagingDirectory:   filepath.Join(data, "staging"),
		AccountTimezone:    "Europe/Berlin",
		BaseCurrency:       "EUR",
		PytrCommand:        []string{"pytr"},
		PDFToTextCommand:   []string{"pdftotext", "-layout"},
		PytrTimeout:        Duration(5 * time.Minute),
		Risk: RiskConfig{
			KillSwitch:      true,
			PaperTrading:    true,
			MaxOrderValue:   money.MustParse("1000"),
			MaxDailyValue:   money.MustParse("2500"),
			PriceMaxAge:     Duration(2 * time.Minute),
			BalanceMaxAge:   Duration(2 * time.Minute),
			PreviewValidity: Duration(5 * time.Minute),
		},
	}, nil
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
	applyEnvironment(&cfg)
	if err := cfg.Validate(); err != nil {
		return Config{}, path, err
	}
	return cfg, path, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.DatabasePath) == "" {
		return fmt.Errorf("database_path is required")
	}
	if len(c.PytrCommand) == 0 || strings.TrimSpace(c.PytrCommand[0]) == "" {
		return fmt.Errorf("pytr_command must contain an executable")
	}
	if len(c.PDFToTextCommand) == 0 || strings.TrimSpace(c.PDFToTextCommand[0]) == "" {
		return fmt.Errorf("pdftotext_command must contain an executable")
	}
	if _, err := time.LoadLocation(c.AccountTimezone); err != nil {
		return fmt.Errorf("account_timezone %q: %w", c.AccountTimezone, err)
	}
	if len(c.BaseCurrency) != 3 {
		return fmt.Errorf("base_currency must be an ISO 4217 code")
	}
	if c.Risk.MaxOrderValue <= 0 || c.Risk.MaxDailyValue <= 0 {
		return fmt.Errorf("risk value limits must be positive")
	}
	if c.Risk.PriceMaxAge.Duration() <= 0 || c.Risk.BalanceMaxAge.Duration() <= 0 || c.Risk.PreviewValidity.Duration() <= 0 {
		return fmt.Errorf("risk freshness and preview durations must be positive")
	}
	return nil
}

func Save(path string, cfg Config) (string, error) {
	if path == "" {
		var err error
		path, err = ConfigPath()
		if err != nil {
			return "", err
		}
	}
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	if err := atomicWrite(path, raw, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err = file.Chmod(mode); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func applyEnvironment(cfg *Config) {
	if value := os.Getenv("TRPP_DATABASE_PATH"); value != "" {
		cfg.DatabasePath = value
	}
	if value := os.Getenv("TRPP_DOCUMENTS_DIRECTORY"); value != "" {
		cfg.DocumentsDirectory = value
	}
	if value := os.Getenv("TRPP_ACCOUNT_TIMEZONE"); value != "" {
		cfg.AccountTimezone = value
	}
	// Environment may force execution off, never turn it on.
	if strings.EqualFold(os.Getenv("TRPP_KILL_SWITCH"), "true") {
		cfg.Risk.KillSwitch = true
	}
	if strings.EqualFold(os.Getenv("TRPP_PAPER_TRADING"), "true") {
		cfg.Risk.PaperTrading = true
	}
}
