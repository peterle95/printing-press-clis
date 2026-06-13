package gmail

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"mail-pp-cli/internal/accounts"
)

const (
	ScopeReadonly = "https://www.googleapis.com/auth/gmail.readonly"
	ScopeCompose  = "https://www.googleapis.com/auth/gmail.compose"
	ScopeSend     = "https://www.googleapis.com/auth/gmail.send"
	ScopeModify   = "https://www.googleapis.com/auth/gmail.modify"
)

var DefaultCredentialsPath = defaultGoogleCredentialsPath("printing-press-gmail-cli.json")

func defaultGoogleCredentialsPath(filename string) string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return filename
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "printing-press", "google-private", "credentials", filename)
}

var DefaultScopes = []string{ScopeReadonly, ScopeCompose, ScopeSend, ScopeModify}

type LoginOptions struct {
	CredentialsPath string
	Scopes          []string
	Port            int
	NoBrowser       bool
	Out             io.Writer
}

type StoredToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
	Scopes       []string  `json:"scopes,omitempty"`
}

func Login(ctx context.Context, account accounts.Account, opts LoginOptions) error {
	if len(opts.Scopes) == 0 {
		opts.Scopes = DefaultScopes
	}
	opts.Scopes = NormalizeScopes(opts.Scopes)
	oauthCfg, err := oauthConfig(opts.CredentialsPath, opts.Scopes)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", opts.Port))
	if err != nil {
		return fmt.Errorf("starting OAuth callback server: %w", err)
	}
	defer listener.Close()
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", listener.Addr().(*net.TCPAddr).Port)
	oauthCfg.RedirectURL = redirectURI

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return err
	}
	state := hex.EncodeToString(stateBytes)
	authURL := oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))

	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	fmt.Fprintf(out, "OAuth callback listening on port %d\n", listener.Addr().(*net.TCPAddr).Port)
	fmt.Fprintln(out, "Open this Gmail OAuth consent URL:")
	fmt.Fprintln(out, authURL)
	if opts.NoBrowser {
		fmt.Fprintln(out, "Browser auto-open disabled by --no-browser.")
	} else if err := openBrowser(authURL); err != nil {
		fmt.Fprintf(out, "Could not open browser automatically: %v\n", err)
	} else {
		fmt.Fprintln(out, "Opened OAuth consent URL in your browser.")
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery == "" {
			http.Error(w, "missing OAuth query parameters; return to the Google consent page", http.StatusBadRequest)
			return
		}
		if got := r.URL.Query().Get("state"); got != state {
			if got != "" || r.URL.Query().Get("code") != "" || r.URL.Query().Get("error") != "" {
				errCh <- fmt.Errorf("state mismatch")
			}
			http.Error(w, "state mismatch; restart login if this came from Google OAuth", http.StatusBadRequest)
			return
		}
		if apiErr := r.URL.Query().Get("error"); apiErr != "" {
			errCh <- fmt.Errorf("auth error: %s", apiErr)
			http.Error(w, apiErr, http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("missing code in OAuth callback")
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h2>Gmail authentication successful.</h2><p>You can close this tab.</p></body></html>")
		codeCh <- code
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer server.Shutdown(context.Background())

	fmt.Fprintln(out, "Waiting for Google OAuth callback (timeout: 3m0s)...")
	var code string
	select {
	case <-ctx.Done():
		return ctx.Err()
	case code = <-codeCh:
	case err := <-errCh:
		return err
	case <-time.After(3 * time.Minute):
		return fmt.Errorf("authentication timed out after 3 minutes")
	}
	token, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("exchanging OAuth code for token: %w", err)
	}
	path, err := account.TokenPath()
	if err != nil {
		return err
	}
	if err := SaveStoredToken(path, StoredTokenFromOAuth(token, opts.Scopes)); err != nil {
		return err
	}
	fmt.Fprintf(out, "Gmail login complete for %s. Token saved to %s\n", account.Address, path)
	return nil
}

func oauthConfig(credentialsPath string, scopes []string) (*oauth2.Config, error) {
	if credentialsPath == "" {
		credentialsPath = DefaultCredentialsPath
	}
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("reading Google OAuth client JSON %s: %w", credentialsPath, err)
	}
	cfg, err := google.ConfigFromJSON(data, scopes...)
	if err != nil {
		return nil, fmt.Errorf("parsing Google OAuth client JSON %s: %w", credentialsPath, err)
	}
	return cfg, nil
}

func LoadStoredToken(path string) (StoredToken, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StoredToken{}, err
	}
	var token StoredToken
	if err := json.Unmarshal(data, &token); err != nil {
		return StoredToken{}, err
	}
	return token, nil
}

func SaveStoredToken(path string, token StoredToken) error {
	if err := os.MkdirAll(filepathDir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func StoredTokenFromOAuth(token *oauth2.Token, scopes []string) StoredToken {
	if token == nil {
		return StoredToken{Scopes: cleanScopes(scopes)}
	}
	return StoredToken{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
		Scopes:       cleanScopes(scopes),
	}
}

func (t StoredToken) OAuthToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  t.AccessToken,
		TokenType:    t.TokenType,
		RefreshToken: t.RefreshToken,
		Expiry:       t.Expiry,
	}
}

func EnsureScopes(token StoredToken, required []string, account string) error {
	required = NormalizeScopes(required)
	available := map[string]bool{}
	for _, scope := range NormalizeScopes(token.Scopes) {
		available[scope] = true
	}
	var missing []string
	for _, scope := range required {
		if !available[scope] {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("Gmail token for %s is missing required scope(s): %s; re-run `mail-pp-cli auth login gmail --account %s --scopes %s`",
		account, strings.Join(scopeDisplayNames(missing), ", "), account, strings.Join(missing, ","))
}

func NormalizeScopes(scopes []string) []string {
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		switch strings.TrimSpace(scope) {
		case "gmail.readonly", "readonly":
			out = append(out, ScopeReadonly)
		case "gmail.compose", "compose":
			out = append(out, ScopeCompose)
		case "gmail.send", "send":
			out = append(out, ScopeSend)
		case "gmail.modify", "modify":
			out = append(out, ScopeModify)
		default:
			out = append(out, strings.TrimSpace(scope))
		}
	}
	return cleanScopes(out)
}

func scopeDisplayNames(scopes []string) []string {
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		switch scope {
		case ScopeReadonly:
			out = append(out, "gmail.readonly ("+scope+")")
		case ScopeCompose:
			out = append(out, "gmail.compose ("+scope+")")
		case ScopeSend:
			out = append(out, "gmail.send ("+scope+")")
		case ScopeModify:
			out = append(out, "gmail.modify ("+scope+")")
		default:
			out = append(out, scope)
		}
	}
	return out
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

func cleanScopes(scopes []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" && !seen[scope] {
			seen[scope] = true
			out = append(out, scope)
		}
	}
	sort.Strings(out)
	return out
}

func filepathDir(path string) string {
	if idx := strings.LastIndex(path, string(os.PathSeparator)); idx > 0 {
		return path[:idx]
	}
	return "."
}
