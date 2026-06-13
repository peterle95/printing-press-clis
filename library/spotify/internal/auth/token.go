package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"spotify-pp-cli/internal/config"
)

type Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (t Token) Valid() bool {
	return t.AccessToken != "" && time.Now().Before(t.ExpiresAt.Add(-30*time.Second))
}

type TokenStore interface {
	Load() (Token, error)
	Save(Token) error
	Delete() error
	Name() string
}

type FileTokenStore struct {
	Path string
}

func (s FileTokenStore) Name() string {
	return s.Path
}

func (s FileTokenStore) Load() (Token, error) {
	b, err := os.ReadFile(s.Path)
	if err != nil {
		return Token{}, err
	}
	var token Token
	if err := json.Unmarshal(b, &token); err != nil {
		return Token{}, err
	}
	return token, nil
}

func (s FileTokenStore) Save(token Token) error {
	if err := os.MkdirAll(filepathDir(s.Path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, b, 0o600)
}

func (s FileTokenStore) Delete() error {
	if err := os.Remove(s.Path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

type KeychainTokenStore struct {
	file FileTokenStore
}

func NewDefaultTokenStore() (TokenStore, error) {
	tokenPath, err := config.TokenPath()
	if err != nil {
		return nil, err
	}
	return KeychainTokenStore{file: FileTokenStore{Path: tokenPath}}, nil
}

func (s KeychainTokenStore) Name() string {
	if s.available() {
		return "OS keychain with file fallback"
	}
	return s.file.Name()
}

func (s KeychainTokenStore) Load() (Token, error) {
	if raw, err := s.readKeychain(); err == nil && strings.TrimSpace(raw) != "" {
		var token Token
		if err := json.Unmarshal([]byte(raw), &token); err == nil {
			return token, nil
		}
	}
	return s.file.Load()
}

func (s KeychainTokenStore) Save(token Token) error {
	b, err := json.Marshal(token)
	if err != nil {
		return err
	}
	if err := s.writeKeychain(string(b)); err == nil {
		return nil
	}
	return s.file.Save(token)
}

func (s KeychainTokenStore) Delete() error {
	if s.available() {
		_ = s.deleteKeychain()
	}
	return s.file.Delete()
}

func (s KeychainTokenStore) available() bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := exec.LookPath("security")
		return err == nil
	case "linux":
		_, err := exec.LookPath("secret-tool")
		return err == nil
	default:
		return false
	}
}

func (s KeychainTokenStore) readKeychain() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("security", "find-generic-password", "-a", keychainAccount, "-s", keychainService, "-w").Output()
		return string(out), err
	case "linux":
		out, err := exec.Command("secret-tool", "lookup", "service", keychainService, "account", keychainAccount).Output()
		return string(out), err
	default:
		return "", fmt.Errorf("keychain not supported on %s", runtime.GOOS)
	}
}

func (s KeychainTokenStore) writeKeychain(value string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("security", "add-generic-password", "-a", keychainAccount, "-s", keychainService, "-w", value, "-U").Run()
	case "linux":
		cmd := exec.Command("secret-tool", "store", "--label", "Printing Press Spotify OAuth Token", "service", keychainService, "account", keychainAccount)
		cmd.Stdin = strings.NewReader(value)
		return cmd.Run()
	default:
		return fmt.Errorf("keychain not supported on %s", runtime.GOOS)
	}
}

func (s KeychainTokenStore) deleteKeychain() error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("security", "delete-generic-password", "-a", keychainAccount, "-s", keychainService).Run()
	case "linux":
		return exec.Command("secret-tool", "clear", "service", keychainService, "account", keychainAccount).Run()
	default:
		return nil
	}
}

func filepathDir(path string) string {
	idx := strings.LastIndex(path, string(os.PathSeparator))
	if idx <= 0 {
		return "."
	}
	return path[:idx]
}

const (
	keychainService = "printing-press-spotify"
	keychainAccount = "spotify-pp-cli"
)
