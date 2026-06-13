package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"spotify-pp-cli/internal/config"
)

type Manager struct {
	Config ConfigView
	Store  TokenStore
	HTTP   *http.Client
	token  *Token
}

type ConfigView interface {
	ClientIDValue() string
	ClientSecretValue() string
}

type SpotifyConfig config.Config

func (c SpotifyConfig) ClientIDValue() string {
	return c.ClientID
}

func (c SpotifyConfig) ClientSecretValue() string {
	return c.ClientSecret
}

func NewManager(cfg config.Config, store TokenStore) *Manager {
	return &Manager{
		Config: SpotifyConfig(cfg),
		Store:  store,
		HTTP:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (m *Manager) AccessToken(ctx context.Context) (string, error) {
	token, err := m.currentToken()
	if err != nil {
		return "", err
	}
	if token.Valid() {
		return token.AccessToken, nil
	}
	if token.RefreshToken == "" {
		return "", fmt.Errorf("token expired and no refresh token is available; run spotify-pp-cli auth login")
	}
	if err := m.Refresh(ctx); err != nil {
		return "", err
	}
	token, err = m.currentToken()
	if err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

func (m *Manager) Refresh(ctx context.Context) error {
	token, err := m.currentToken()
	if err != nil {
		return err
	}
	if token.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}
	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", token.RefreshToken)
	values.Set("client_id", m.Config.ClientIDValue())
	if secret := m.Config.ClientSecretValue(); secret != "" {
		values.Set("client_secret", secret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := m.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		return fmt.Errorf("refresh token failed: spotify returned %s %v", resp.Status, body)
	}
	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return err
	}
	if tr.RefreshToken == "" {
		tr.RefreshToken = token.RefreshToken
	}
	refreshed := tr.toToken()
	if err := m.Store.Save(refreshed); err != nil {
		return err
	}
	m.token = &refreshed
	return nil
}

func (m *Manager) currentToken() (Token, error) {
	if m.token != nil {
		return *m.token, nil
	}
	token, err := m.Store.Load()
	if err != nil {
		return Token{}, fmt.Errorf("not logged in; run spotify-pp-cli auth login")
	}
	m.token = &token
	return token, nil
}

func (m *Manager) httpClient() *http.Client {
	if m.HTTP != nil {
		return m.HTTP
	}
	return http.DefaultClient
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func (r tokenResponse) toToken() Token {
	return Token{
		AccessToken:  r.AccessToken,
		TokenType:    r.TokenType,
		RefreshToken: r.RefreshToken,
		Scope:        r.Scope,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(r.ExpiresIn) * time.Second),
	}
}

const tokenURL = "https://accounts.spotify.com/api/token"
