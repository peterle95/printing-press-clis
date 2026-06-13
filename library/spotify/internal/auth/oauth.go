package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"spotify-pp-cli/internal/config"
)

var RequiredScopes = []string{
	"user-library-read",
	"user-library-modify",
	"playlist-read-private",
	"playlist-read-collaborative",
	"playlist-modify-private",
	"playlist-modify-public",
	"user-read-private",
}

type LoginOptions struct {
	NoBrowser bool
	Out       io.Writer
}

func Login(ctx context.Context, cfg config.Config, store TokenStore, opts LoginOptions) error {
	if cfg.ClientID == "" {
		return fmt.Errorf("missing Spotify client ID; set SPOTIFY_CLIENT_ID or config client_id")
	}
	listener, redirectURI, err := listenCallback(cfg.RedirectURI)
	if err != nil {
		return err
	}
	defer listener.Close()
	codeCh := make(chan callbackResult, 1)
	server := &http.Server{ReadHeaderTimeout: 10 * time.Second}
	mux := http.NewServeMux()
	server.Handler = mux
	state, err := randomString(32)
	if err != nil {
		return err
	}
	verifier, err := randomString(64)
	if err != nil {
		return err
	}
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("state"); got != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			codeCh <- callbackResult{err: fmt.Errorf("state mismatch")}
			return
		}
		if apiErr := r.URL.Query().Get("error"); apiErr != "" {
			http.Error(w, apiErr, http.StatusBadRequest)
			codeCh <- callbackResult{err: errors.New(apiErr)}
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			codeCh <- callbackResult{err: fmt.Errorf("missing code")}
			return
		}
		fmt.Fprintln(w, "Spotify login complete. You can return to the terminal.")
		codeCh <- callbackResult{code: code}
	})
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Shutdown(context.Background())

	authURL := authorizeURL(cfg.ClientID, redirectURI, state, verifier)
	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	fmt.Fprintf(out, "Open this URL to authorize Spotify access:\n%s\n\n", authURL)
	if !opts.NoBrowser {
		if err := openBrowser(authURL); err != nil {
			fmt.Fprintf(out, "Could not open browser automatically: %v\n", err)
		}
	}
	var result callbackResult
	select {
	case <-ctx.Done():
		return ctx.Err()
	case result = <-codeCh:
	}
	if result.err != nil {
		return result.err
	}
	token, err := exchangeCode(ctx, cfg, redirectURI, result.code, verifier)
	if err != nil {
		return err
	}
	if err := store.Save(token); err != nil {
		return err
	}
	fmt.Fprintf(out, "Logged in. Token stored via %s.\n", store.Name())
	return nil
}

func authorizeURL(clientID, redirectURI, state, verifier string) string {
	values := url.Values{}
	values.Set("response_type", "code")
	values.Set("client_id", clientID)
	values.Set("scope", strings.Join(RequiredScopes, " "))
	values.Set("redirect_uri", redirectURI)
	values.Set("state", state)
	values.Set("code_challenge_method", "S256")
	values.Set("code_challenge", codeChallenge(verifier))
	return "https://accounts.spotify.com/authorize?" + values.Encode()
}

func exchangeCode(ctx context.Context, cfg config.Config, redirectURI, code, verifier string) (Token, error) {
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", redirectURI)
	values.Set("client_id", cfg.ClientID)
	values.Set("code_verifier", verifier)
	if cfg.ClientSecret != "" {
		values.Set("client_secret", cfg.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Token{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		return Token{}, fmt.Errorf("token exchange failed: spotify returned %s %v", resp.Status, body)
	}
	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return Token{}, err
	}
	return tr.toToken(), nil
}

func listenCallback(configured string) (net.Listener, string, error) {
	if configured != "" {
		u, err := url.Parse(configured)
		if err != nil {
			return nil, "", err
		}
		host := u.Host
		if !strings.Contains(host, ":") {
			host += ":80"
		}
		ln, err := net.Listen("tcp", host)
		return ln, configured, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}
	return ln, "http://" + ln.Addr().String() + "/callback", nil
}

func openBrowser(rawURL string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
	default:
		if _, err := exec.LookPath("wslview"); err == nil {
			return exec.Command("wslview", rawURL).Start()
		}
		if _, err := exec.LookPath("xdg-open"); err == nil {
			return exec.Command("xdg-open", rawURL).Start()
		}
		return fmt.Errorf("no browser opener found")
	}
}

func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomString(length int) (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._~-"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b), nil
}

type callbackResult struct {
	code string
	err  error
}
