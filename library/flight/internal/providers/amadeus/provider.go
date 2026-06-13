package amadeus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"flight-pp-cli/internal/config"
	"flight-pp-cli/internal/flight"
)

const (
	testBaseURL       = "https://test.api.amadeus.com"
	productionBaseURL = "https://api.amadeus.com"
)

type Provider struct {
	cfg      config.AmadeusConfig
	http     *http.Client
	baseURL  string
	cacheDir string
}

func New(cfg config.AmadeusConfig, cacheDir string, timeout time.Duration) *Provider {
	baseURL := testBaseURL
	if strings.EqualFold(cfg.Environment, "production") {
		baseURL = productionBaseURL
	}
	return &Provider{
		cfg:      cfg,
		http:     &http.Client{Timeout: timeout},
		baseURL:  baseURL,
		cacheDir: cacheDir,
	}
}

func NewWithHTTP(cfg config.AmadeusConfig, cacheDir, baseURL string, client *http.Client) *Provider {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Provider{
		cfg:      cfg,
		http:     client,
		baseURL:  strings.TrimRight(baseURL, "/"),
		cacheDir: cacheDir,
	}
}

func (p *Provider) Name() string {
	return "amadeus"
}

func (p *Provider) Status(ctx context.Context) flight.ProviderStatus {
	status := flight.ProviderStatus{
		Name:    p.Name(),
		Enabled: p.cfg.Enabled,
		Mode:    p.cfg.Environment,
	}
	if !p.cfg.Enabled {
		status.Warnings = append(status.Warnings, "disabled in config")
		return status
	}
	if p.cfg.ClientID == "" {
		status.Missing = append(status.Missing, "AMADEUS_CLIENT_ID or providers.amadeus.client_id")
	}
	if p.cfg.ClientSecret == "" {
		status.Missing = append(status.Missing, "AMADEUS_CLIENT_SECRET or providers.amadeus.client_secret")
	}
	status.Available = len(status.Missing) == 0
	return status
}

func (p *Provider) Search(ctx context.Context, request flight.FlightSearchRequest) ([]flight.FlightSearchResult, error) {
	if request.Destination == "ANYWHERE" {
		return nil, fmt.Errorf("Amadeus Flight Offers Search requires a destination code")
	}
	if status := p.Status(ctx); !status.Available {
		return nil, fmt.Errorf("missing credentials: %s", strings.Join(status.Missing, ", "))
	}
	token, err := p.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	values.Set("originLocationCode", request.Origin)
	values.Set("destinationLocationCode", request.Destination)
	values.Set("departureDate", request.DepartDate)
	if request.ReturnDate != "" {
		values.Set("returnDate", request.ReturnDate)
	}
	values.Set("adults", strconv.Itoa(request.Adults))
	if request.Children > 0 {
		values.Set("children", strconv.Itoa(request.Children))
	}
	if request.Infants > 0 {
		values.Set("infants", strconv.Itoa(request.Infants))
	}
	values.Set("currencyCode", request.Currency)
	values.Set("travelClass", amadeusCabin(request.Cabin))
	if request.DirectOnly || (request.MaxStops != nil && *request.MaxStops == 0) {
		values.Set("nonStop", "true")
	}
	limit := request.Limit
	if limit <= 0 || limit > 250 {
		limit = 50
	}
	values.Set("max", strconv.Itoa(limit))

	endpoint := p.baseURL + "/v2/shopping/flight-offers?" + values.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Accept", "application/json")
	resp, err := p.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Amadeus flight search request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var apiErr apiError
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if msg := apiErr.Message(); msg != "" {
			return nil, fmt.Errorf("Amadeus failed: %s", msg)
		}
		return nil, fmt.Errorf("Amadeus failed: HTTP %d", resp.StatusCode)
	}
	var body flightOffersResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode Amadeus response: %w", err)
	}
	return Normalize(body, request), nil
}

func amadeusCabin(cabin string) string {
	switch cabin {
	case flight.CabinPremiumEconomy:
		return "PREMIUM_ECONOMY"
	case flight.CabinBusiness:
		return "BUSINESS"
	case flight.CabinFirst:
		return "FIRST"
	default:
		return "ECONOMY"
	}
}

type tokenCache struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (p *Provider) accessToken(ctx context.Context) (string, error) {
	if cached, ok := p.readTokenCache(); ok {
		return cached.AccessToken, nil
	}
	values := url.Values{}
	values.Set("grant_type", "client_credentials")
	values.Set("client_id", p.cfg.ClientID)
	values.Set("client_secret", p.cfg.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/security/oauth2/token", strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("Amadeus token request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var apiErr apiError
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if msg := apiErr.Message(); msg != "" {
			return "", fmt.Errorf("Amadeus auth failed: %s", msg)
		}
		return "", fmt.Errorf("Amadeus auth failed: HTTP %d", resp.StatusCode)
	}
	var token struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return "", fmt.Errorf("decode Amadeus token: %w", err)
	}
	if token.AccessToken == "" {
		return "", fmt.Errorf("Amadeus auth returned no access token")
	}
	expiresIn := time.Duration(token.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = 30 * time.Minute
	}
	cache := tokenCache{
		AccessToken: token.AccessToken,
		ExpiresAt:   time.Now().Add(expiresIn - time.Minute),
	}
	_ = p.writeTokenCache(cache)
	return token.AccessToken, nil
}

func (p *Provider) readTokenCache() (tokenCache, bool) {
	path := p.tokenCachePath()
	if path == "" {
		return tokenCache{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return tokenCache{}, false
	}
	var cached tokenCache
	if err := json.Unmarshal(data, &cached); err != nil {
		return tokenCache{}, false
	}
	if cached.AccessToken == "" || time.Now().After(cached.ExpiresAt) {
		return tokenCache{}, false
	}
	return cached, true
}

func (p *Provider) writeTokenCache(cache tokenCache) error {
	path := p.tokenCachePath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (p *Provider) tokenCachePath() string {
	if p.cacheDir == "" {
		return ""
	}
	env := strings.ToLower(strings.TrimSpace(p.cfg.Environment))
	if env == "" {
		env = "test"
	}
	return filepath.Join(p.cacheDir, "amadeus_token_"+env+".json")
}

type apiError struct {
	ErrorDescription string `json:"error_description"`
	Error            string `json:"error"`
	Errors           []struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
		Code   int    `json:"code"`
	} `json:"errors"`
}

func (e apiError) Message() string {
	if e.ErrorDescription != "" {
		return e.ErrorDescription
	}
	if e.Error != "" {
		return e.Error
	}
	if len(e.Errors) > 0 {
		if e.Errors[0].Detail != "" {
			return e.Errors[0].Detail
		}
		return e.Errors[0].Title
	}
	return ""
}
