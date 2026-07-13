// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package googlecalendar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/oauth2"
)

type TokenStore struct{ Path string }

type storedToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
}

func (s TokenStore) Load() (*oauth2.Token, error) {
	info, err := os.Lstat(s.Path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("token path must be a regular non-symlink file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("token file permissions must be 0600 or stricter")
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, fmt.Errorf("read Google token: %w", err)
	}
	var stored storedToken
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("parse Google token: %w", err)
	}
	if stored.AccessToken == "" && stored.RefreshToken == "" {
		return nil, fmt.Errorf("google token contains no access or refresh token")
	}
	return &oauth2.Token{AccessToken: stored.AccessToken, TokenType: stored.TokenType, RefreshToken: stored.RefreshToken, Expiry: stored.Expiry}, nil
}

func (s TokenStore) Save(token *oauth2.Token) error {
	if token == nil {
		return fmt.Errorf("cannot store a nil Google token")
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create token directory: %w", err)
	}
	if info, err := os.Lstat(dir); err != nil {
		return err
	} else if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("token directory must be a non-symlink directory")
	}
	if err := os.Chmod(dir, 0o700); err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("secure token directory: %w", err)
	}
	stored := storedToken{AccessToken: token.AccessToken, TokenType: token.TokenType, RefreshToken: token.RefreshToken, Expiry: token.Expiry}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Google token: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".google-token-*")
	if err != nil {
		return fmt.Errorf("create temporary token file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.Path); err != nil {
		return fmt.Errorf("replace Google token: %w", err)
	}
	return os.Chmod(s.Path, 0o600)
}

func (s TokenStore) PermissionStatus() error {
	info, err := os.Lstat(s.Path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("token file is not a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("token file mode is %04o; expected 0600", info.Mode().Perm())
	}
	parent, err := os.Lstat(filepath.Dir(s.Path))
	if err != nil {
		return err
	}
	if parent.Mode()&os.ModeSymlink != 0 || !parent.IsDir() {
		return fmt.Errorf("token directory is not a regular directory")
	}
	if runtime.GOOS != "windows" && parent.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("token directory mode is %04o; expected 0700", parent.Mode().Perm())
	}
	return nil
}
