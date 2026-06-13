package gmail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"mail-pp-cli/internal/accounts"
	"mail-pp-cli/internal/mail"
)

const apiBase = "https://gmail.googleapis.com/gmail/v1"

type Provider struct {
	account        accounts.Account
	credentials    string
	timeout        time.Duration
	requiredScopes []string
	httpClient     *http.Client
}

func NewProvider(account accounts.Account, credentialsPath string, timeout time.Duration) *Provider {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Provider{account: account, credentials: credentialsPath, timeout: timeout}
}

func (p *Provider) AccountName() string {
	return p.account.Name
}

func (p *Provider) ProviderName() string {
	return "gmail"
}

func (p *Provider) withScopes(scopes ...string) *Provider {
	clone := *p
	clone.requiredScopes = scopes
	clone.httpClient = nil
	return &clone
}

func (p *Provider) client(ctx context.Context) (*http.Client, error) {
	if p.httpClient != nil {
		return p.httpClient, nil
	}
	tokenPath, err := p.account.TokenPath()
	if err != nil {
		return nil, err
	}
	stored, err := LoadStoredToken(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("loading Gmail token for %s from %s: %w", p.account.Address, tokenPath, err)
	}
	if err := EnsureScopes(stored, p.requiredScopes, p.account.Name); err != nil {
		return nil, err
	}
	oauthCfg, err := oauthConfig(p.credentials, append(stored.Scopes, p.requiredScopes...))
	if err != nil {
		return nil, err
	}
	source := &savingTokenSource{
		base:   oauthCfg.TokenSource(ctx, stored.OAuthToken()),
		path:   tokenPath,
		scopes: stored.Scopes,
	}
	p.httpClient = oauth2.NewClient(ctx, source)
	p.httpClient.Timeout = p.timeout
	return p.httpClient, nil
}

func (p *Provider) do(ctx context.Context, method, path string, params url.Values, body any, out any) error {
	client, err := p.client(ctx)
	if err != nil {
		return err
	}
	if params == nil {
		params = url.Values{}
	}
	target := apiBase + path
	if encoded := params.Encode(); encoded != "" {
		target += "?" + encoded
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("Gmail API %s %s returned HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

func (p *Provider) rawID(id string) (string, error) {
	return mail.RawMessageID("gmail", p.account.Name, id)
}

type savingTokenSource struct {
	base   oauth2.TokenSource
	path   string
	scopes []string
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	token, err := s.base.Token()
	if err != nil {
		return nil, err
	}
	_ = SaveStoredToken(s.path, StoredTokenFromOAuth(token, s.scopes))
	return token, nil
}

func headerValue(headers []gmailHeader, name string) string {
	for _, h := range headers {
		if strings.EqualFold(h.Name, name) {
			decoded, err := (&mime.WordDecoder{}).DecodeHeader(h.Value)
			if err == nil {
				return decoded
			}
			return h.Value
		}
	}
	return ""
}

func splitCSV(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
