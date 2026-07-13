// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

// Package googlecalendar implements the API-specific OAuth and Calendar REST adapter.
// PATCH: Add isolated Google OAuth without storing client credentials in normal config.
package googlecalendar

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"trello-calendar-pp-cli/internal/config"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const CalendarScope = "https://www.googleapis.com/auth/calendar.events"

func OAuthConfig(cfg *config.Config) (*oauth2.Config, error) {
	var missing []string
	if cfg.GoogleClientID == "" {
		missing = append(missing, "GOOGLE_CLIENT_ID")
	}
	if cfg.GoogleClientSecret == "" {
		missing = append(missing, "GOOGLE_CLIENT_SECRET")
	}
	if cfg.GoogleRedirectURI == "" {
		missing = append(missing, "GOOGLE_REDIRECT_URI")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing Google OAuth environment variables: %s", strings.Join(missing, ", "))
	}
	return &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURI,
		Scopes:       []string{CalendarScope},
		Endpoint:     google.Endpoint,
	}, nil
}

func Login(ctx context.Context, cfg *config.Config, store TokenStore, noBrowser bool, out io.Writer) error {
	oauthCfg, err := OAuthConfig(cfg)
	if err != nil {
		return err
	}
	redirect, err := url.Parse(oauthCfg.RedirectURL)
	if err != nil || redirect.Scheme != "http" || !isLoopback(redirect.Hostname()) {
		return fmt.Errorf("GOOGLE_REDIRECT_URI must be an http://localhost or http://127.0.0.1 callback")
	}
	listener, err := net.Listen("tcp", redirect.Host)
	if err != nil {
		return fmt.Errorf("listen for Google OAuth callback: %w", err)
	}
	defer listener.Close()

	state, err := randomState()
	if err != nil {
		return err
	}
	verifier := oauth2.GenerateVerifier()
	authURL := oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("prompt", "consent"))
	result := make(chan string, 1)
	errs := make(chan error, 1)
	mux := http.NewServeMux()
	path := redirect.Path
	if path == "" {
		path = "/"
	}
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "invalid OAuth state", http.StatusBadRequest)
			errs <- fmt.Errorf("google OAuth state mismatch")
			return
		}
		if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
			http.Error(w, "authorization failed", http.StatusBadRequest)
			errs <- fmt.Errorf("google OAuth authorization failed: %s", oauthErr)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing authorization code", http.StatusBadRequest)
			errs <- fmt.Errorf("google OAuth callback contained no code")
			return
		}
		fmt.Fprintln(w, "Google Calendar authentication complete. You can close this window.")
		result <- code
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			errs <- serveErr
		}
	}()
	defer server.Shutdown(context.Background())

	fmt.Fprintf(out, "Open this URL to authorize Google Calendar:\n\n%s\n\n", authURL)
	if !noBrowser && os.Getenv("PRINTING_PRESS_VERIFY") == "" {
		_ = openBrowser(authURL)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	var code string
	select {
	case code = <-result:
	case err := <-errs:
		return err
	case <-waitCtx.Done():
		return fmt.Errorf("google OAuth callback timed out: %w", waitCtx.Err())
	}
	token, err := oauthCfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return fmt.Errorf("exchange Google OAuth code: %w", err)
	}
	if token.RefreshToken == "" {
		return fmt.Errorf("google did not return a refresh token; revoke prior consent and retry")
	}
	if err := store.Save(token); err != nil {
		return err
	}
	fmt.Fprintf(out, "Google refresh token stored securely at %s\n", store.Path)
	return nil
}

func NewHTTPClient(ctx context.Context, cfg *config.Config, store TokenStore, persistRefresh bool) (*http.Client, error) {
	oauthCfg, err := OAuthConfig(cfg)
	if err != nil {
		return nil, err
	}
	token, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("load Google OAuth token (run 'trello-calendar-pp-cli auth google'): %w", err)
	}
	source := oauthCfg.TokenSource(ctx, token)
	if persistRefresh {
		source = &persistingTokenSource{source: source, store: store, previous: token}
	}
	return oauth2.NewClient(ctx, source), nil
}

type persistingTokenSource struct {
	mu       sync.Mutex
	source   oauth2.TokenSource
	store    TokenStore
	previous *oauth2.Token
}

func (s *persistingTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, err := s.source.Token()
	if err != nil {
		return nil, fmt.Errorf("refresh Google OAuth token: %w", err)
	}
	if token.RefreshToken == "" && s.previous != nil {
		token.RefreshToken = s.previous.RefreshToken
	}
	if s.previous == nil || token.AccessToken != s.previous.AccessToken || !token.Expiry.Equal(s.previous.Expiry) {
		if err := s.store.Save(token); err != nil {
			return nil, err
		}
		s.previous = token
	}
	return token, nil
}

func randomState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate OAuth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "[::1]" || host == "::1"
}

func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}
