package accounts

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ProviderGmail        = "gmail"
	ProviderProton       = "proton"
	ProviderProtonExport = "proton-export"
)

type Config struct {
	Accounts map[string]Account `yaml:"accounts" json:"accounts"`
	Path     string             `yaml:"-" json:"path"`
}

type Account struct {
	Name            string `yaml:"-" json:"name"`
	Address         string `yaml:"address" json:"address"`
	Provider        string `yaml:"provider" json:"provider"`
	Token           string `yaml:"token,omitempty" json:"token,omitempty"`
	Username        string `yaml:"username,omitempty" json:"username,omitempty"`
	PasswordCommand string `yaml:"password_command,omitempty" json:"password_command,omitempty"`
	IMAPHost        string `yaml:"imap_host,omitempty" json:"imap_host,omitempty"`
	SMTPHost        string `yaml:"smtp_host,omitempty" json:"smtp_host,omitempty"`
	ArchiveDir      string `yaml:"archive_dir,omitempty" json:"archive_dir,omitempty"`
	DraftDir        string `yaml:"draft_dir,omitempty" json:"draft_dir,omitempty"`
	ExportTool      string `yaml:"export_tool,omitempty" json:"export_tool,omitempty"`
	AutoRefreshDays int    `yaml:"auto_refresh_days,omitempty" json:"auto_refresh_days,omitempty"`
	IMAPStartTLS    *bool  `yaml:"imap_starttls,omitempty" json:"imap_starttls,omitempty"`
	SMTPStartTLS    *bool  `yaml:"smtp_starttls,omitempty" json:"smtp_starttls,omitempty"`
}

func Load(path string) (*Config, error) {
	resolved, err := ResolvePath(path)
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	cfg.Path = resolved
	data, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading account config %s: %w", resolved, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing account config %s: %w", resolved, err)
	}
	if cfg.Accounts == nil {
		cfg.Accounts = map[string]Account{}
	}
	cfg.applyDefaults()
	return cfg, nil
}

func (c *Config) Save(path string) error {
	if path == "" {
		path = c.Path
	}
	if path == "" {
		var err error
		path, err = ResolvePath("")
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	clean := *c
	clean.Path = ""
	for name, account := range clean.Accounts {
		account.Name = ""
		account.Provider = CanonicalProvider(account.Provider)
		clean.Accounts[name] = account
	}
	data, err := yaml.Marshal(clean)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func ResolvePath(path string) (string, error) {
	if path == "" {
		path = os.Getenv("MAIL_PP_ACCOUNTS")
	}
	if path == "" {
		path = "~/.local/share/printing-press/mail-private/accounts.yaml"
	}
	return ExpandPath(path)
}

func ExpandPath(path string) (string, error) {
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
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:]), nil
	}
	return "", fmt.Errorf("cannot expand path %q", path)
}

func DefaultConfig() *Config {
	return &Config{
		Accounts: map[string]Account{
			"gmail-main": {
				Address:  "user@example.com",
				Provider: "gmail-api",
				Token:    "~/.config/printing-press/google-private/tokens/gmail-main-token.json",
			},
			"gmail-alt": {
				Address:  "alternate@example.com",
				Provider: "gmail-api",
				Token:    "~/.config/printing-press/google-private/tokens/gmail-alt-token.json",
			},
			"proton-free": {
				Address:    "user@proton.me",
				Provider:   "proton-export-eml",
				Username:   "user",
				ArchiveDir: "~/.local/share/printing-press/mail-private/proton-export/user",
				DraftDir:   "~/.local/share/printing-press/mail-private/proton-drafts",
			},
		},
	}
}

func (c *Config) applyDefaults() {
	defaults := DefaultConfig()
	for name, account := range c.Accounts {
		account.Name = name
		account.Provider = NormalizeProvider(account.Provider)
		if account.Token == "" && account.Provider == ProviderGmail {
			if def, ok := defaults.Accounts[name]; ok {
				account.Token = def.Token
			}
		}
		if account.IMAPHost == "" && account.Provider == ProviderProton {
			account.IMAPHost = "127.0.0.1:1143"
		}
		if account.SMTPHost == "" && account.Provider == ProviderProton {
			account.SMTPHost = "127.0.0.1:1025"
		}
		if account.Username == "" && account.Provider == ProviderProton {
			account.Username = strings.TrimSuffix(account.Address, "@proton.me")
		}
		if account.Provider == ProviderProtonExport {
			if account.ArchiveDir == "" {
				account.ArchiveDir = "~/.local/share/printing-press/mail-private/proton-export/" + strings.TrimSuffix(account.Address, "@proton.me")
			}
			if account.DraftDir == "" {
				account.DraftDir = "~/.local/share/printing-press/mail-private/proton-drafts"
			}
			if account.Username == "" {
				account.Username = strings.TrimSuffix(account.Address, "@proton.me")
			}
		}
		c.Accounts[name] = account
	}
}

func NormalizeProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "gmail", "gmail-api":
		return ProviderGmail
	case "proton", "proton-bridge", "proton-bridge-imap-smtp", "imap-smtp":
		return ProviderProton
	case "proton-export", "proton-export-eml", "local-eml", "eml":
		return ProviderProtonExport
	default:
		return strings.ToLower(strings.TrimSpace(provider))
	}
}

func CanonicalProvider(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderGmail:
		return "gmail-api"
	case ProviderProton:
		return "proton-bridge-imap-smtp"
	case ProviderProtonExport:
		return "proton-export-eml"
	default:
		return strings.TrimSpace(provider)
	}
}

func (c *Config) Names() []string {
	names := make([]string, 0, len(c.Accounts))
	for name := range c.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (c *Config) List() []Account {
	names := c.Names()
	out := make([]Account, 0, len(names))
	for _, name := range names {
		account := c.Accounts[name]
		account.Name = name
		out = append(out, account)
	}
	return out
}

func (c *Config) Resolve(identifier string) (Account, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return Account{}, fmt.Errorf("--account is required")
	}
	if account, ok := c.Accounts[identifier]; ok {
		account.Name = identifier
		account.Provider = NormalizeProvider(account.Provider)
		return account, nil
	}
	var matches []Account
	for name, account := range c.Accounts {
		if strings.EqualFold(account.Address, identifier) {
			account.Name = name
			account.Provider = NormalizeProvider(account.Provider)
			matches = append(matches, account)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return Account{}, fmt.Errorf("account address %q is ambiguous", identifier)
	}
	return Account{}, fmt.Errorf("unknown account %q", identifier)
}

func (c *Config) ResolveByMessageID(id string) (Account, error) {
	parts := strings.SplitN(id, ":", 3)
	if len(parts) != 3 {
		return Account{}, fmt.Errorf("message id %q does not include provider and account; pass --account", id)
	}
	return c.Resolve(parts[1])
}

func (a Account) TokenPath() (string, error) {
	if a.Token == "" {
		return "", fmt.Errorf("account %s has no token path configured", a.Name)
	}
	return ExpandPath(a.Token)
}

func (a Account) ExpandedArchiveDir() (string, error) {
	if a.ArchiveDir == "" {
		return "", fmt.Errorf("account %s has no archive_dir configured", a.Name)
	}
	return ExpandPath(a.ArchiveDir)
}

func (a Account) ExpandedDraftDir() (string, error) {
	if a.DraftDir == "" {
		return ExpandPath("~/.local/share/printing-press/mail-private/proton-drafts")
	}
	return ExpandPath(a.DraftDir)
}

func (a Account) IMAPStartTLSEnabled() bool {
	if a.IMAPStartTLS == nil {
		return true
	}
	return *a.IMAPStartTLS
}

func (a Account) SMTPStartTLSEnabled() bool {
	if a.SMTPStartTLS == nil {
		return true
	}
	return *a.SMTPStartTLS
}

func (a Account) BridgePassword(ctx context.Context) (string, error) {
	if strings.TrimSpace(a.PasswordCommand) == "" {
		return "", fmt.Errorf("account %s has no password_command configured; run `mail-pp-cli proton bridge configure --account %s --password-command '<command>'`", a.Name, a.Name)
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", a.PasswordCommand)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-lc", a.PasswordCommand)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("password_command failed for account %s: %w: %s", a.Name, err, strings.TrimSpace(stderr.String()))
	}
	password := strings.TrimRight(stdout.String(), "\r\n")
	if password == "" {
		return "", fmt.Errorf("password_command produced no password for account %s", a.Name)
	}
	return password, nil
}
