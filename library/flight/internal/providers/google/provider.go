package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"flight-pp-cli/internal/config"
	"flight-pp-cli/internal/flight"
)

type Provider struct {
	cfg       config.GoogleConfig
	http      *http.Client
	serpURL   string
	searchURL string
}

func New(cfg config.GoogleConfig, timeout time.Duration) *Provider {
	return &Provider{
		cfg:       cfg,
		http:      &http.Client{Timeout: timeout},
		serpURL:   "https://serpapi.com/search.json",
		searchURL: "https://www.searchapi.io/api/v1/search",
	}
}

func NewWithHTTP(cfg config.GoogleConfig, serpURL, searchURL string, client *http.Client) *Provider {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Provider{
		cfg:       cfg,
		http:      client,
		serpURL:   serpURL,
		searchURL: searchURL,
	}
}

func (p *Provider) Name() string {
	return "google"
}

func (p *Provider) Status(ctx context.Context) flight.ProviderStatus {
	mode := strings.ToLower(strings.TrimSpace(p.cfg.Mode))
	if mode == "" {
		mode = "deeplink"
	}
	status := flight.ProviderStatus{
		Name:    p.Name(),
		Enabled: p.cfg.Enabled,
		Mode:    mode,
	}
	if !p.cfg.Enabled {
		status.Warnings = append(status.Warnings, "disabled in config")
		return status
	}
	switch mode {
	case "deeplink":
		status.Available = true
		status.Warnings = append(status.Warnings, "deep-link mode generates a URL but no comparable price data")
	case "serpapi":
		if p.cfg.SerpAPIKey == "" {
			status.Missing = append(status.Missing, "SERPAPI_KEY or providers.google.serpapi_key")
		}
		status.Available = len(status.Missing) == 0
	case "searchapi":
		if p.cfg.SearchAPIKey == "" {
			status.Missing = append(status.Missing, "SEARCHAPI_KEY or providers.google.searchapi_key")
		}
		status.Available = len(status.Missing) == 0
	default:
		status.Error = "mode must be deeplink, serpapi, or searchapi"
	}
	return status
}

func (p *Provider) Search(ctx context.Context, request flight.FlightSearchRequest) ([]flight.FlightSearchResult, error) {
	mode := strings.ToLower(strings.TrimSpace(p.cfg.Mode))
	if mode == "" {
		mode = "deeplink"
	}
	switch mode {
	case "deeplink":
		return []flight.FlightSearchResult{DeepLinkResult(request)}, nil
	case "serpapi":
		return p.searchSerpAPI(ctx, request)
	case "searchapi":
		return p.searchSearchAPI(ctx, request)
	default:
		return nil, fmt.Errorf("Google provider mode must be deeplink, serpapi, or searchapi")
	}
}

func (p *Provider) Cheapest(ctx context.Context, request flight.FlexibleSearchRequest) ([]flight.FlightSearchResult, error) {
	base := request.FlightSearchRequest
	base.DepartDate = request.Month
	result := DeepLinkResult(base)
	result.DeepLink = BuildGoogleFlightsMonthURL(request)
	result.Warnings = []string{"Google Flights flexible-date deep-link generated, but no API price data available; open manually to compare."}
	return []flight.FlightSearchResult{result}, nil
}

func DeepLinkResult(request flight.FlightSearchRequest) flight.FlightSearchResult {
	return flight.FlightSearchResult{
		Provider:       "google",
		PriceAvailable: false,
		Currency:       request.Currency,
		DeepLink:       BuildGoogleFlightsURL(request),
		Origin:         request.Origin,
		Destination:    request.Destination,
		DepartAt:       request.DepartDate,
		ReturnDepartAt: request.ReturnDate,
		Warnings:       []string{"Google Flights deep-link generated, but no API price data available; open manually to compare."},
	}
}

func BuildGoogleFlightsURL(request flight.FlightSearchRequest) string {
	parts := []string{
		"Flights",
		"from", request.Origin,
		"to", destinationText(request.Destination),
		"departing", request.DepartDate,
	}
	if request.ReturnDate != "" {
		parts = append(parts, "returning", request.ReturnDate)
	}
	if request.Adults > 0 {
		parts = append(parts, "for", strconv.Itoa(request.Adults), plural(request.Adults, "adult", "adults"))
	}
	if request.Children > 0 {
		parts = append(parts, strconv.Itoa(request.Children), plural(request.Children, "child", "children"))
	}
	if request.Cabin != "" && request.Cabin != flight.CabinEconomy {
		parts = append(parts, strings.ReplaceAll(request.Cabin, "_", " "))
	}
	values := url.Values{}
	values.Set("q", strings.Join(parts, " "))
	if request.Currency != "" {
		values.Set("curr", request.Currency)
	}
	return "https://www.google.com/travel/flights/search?" + values.Encode()
}

func BuildGoogleFlightsMonthURL(request flight.FlexibleSearchRequest) string {
	parts := []string{
		"Cheapest flights",
		"from", request.Origin,
		"to", destinationText(request.Destination),
		"in", request.Month,
	}
	if request.TripDays > 0 {
		parts = append(parts, "for", strconv.Itoa(request.TripDays), "days")
	}
	if request.Adults > 0 {
		parts = append(parts, "for", strconv.Itoa(request.Adults), plural(request.Adults, "adult", "adults"))
	}
	values := url.Values{}
	values.Set("q", strings.Join(parts, " "))
	if request.Currency != "" {
		values.Set("curr", request.Currency)
	}
	return "https://www.google.com/travel/flights/search?" + values.Encode()
}

func (p *Provider) searchSerpAPI(ctx context.Context, request flight.FlightSearchRequest) ([]flight.FlightSearchResult, error) {
	if p.cfg.SerpAPIKey == "" {
		return nil, fmt.Errorf("missing SERPAPI_KEY or providers.google.serpapi_key")
	}
	values := googleAPIValues(request)
	values.Set("api_key", p.cfg.SerpAPIKey)
	values.Set("engine", "google_flights")
	if request.ReturnDate == "" {
		values.Set("type", "2")
	} else {
		values.Set("type", "1")
	}
	return p.searchAPI(ctx, p.serpURL, values, request)
}

func (p *Provider) searchSearchAPI(ctx context.Context, request flight.FlightSearchRequest) ([]flight.FlightSearchResult, error) {
	if p.cfg.SearchAPIKey == "" {
		return nil, fmt.Errorf("missing SEARCHAPI_KEY or providers.google.searchapi_key")
	}
	values := googleAPIValues(request)
	values.Set("api_key", p.cfg.SearchAPIKey)
	values.Set("engine", "google_flights")
	if request.ReturnDate == "" {
		values.Set("flight_type", "one_way")
	} else {
		values.Set("flight_type", "round_trip")
	}
	return p.searchAPI(ctx, p.searchURL, values, request)
}

func googleAPIValues(request flight.FlightSearchRequest) url.Values {
	values := url.Values{}
	values.Set("departure_id", request.Origin)
	if request.Destination != "ANYWHERE" {
		values.Set("arrival_id", request.Destination)
	}
	values.Set("outbound_date", request.DepartDate)
	if request.ReturnDate != "" {
		values.Set("return_date", request.ReturnDate)
	}
	values.Set("currency", request.Currency)
	values.Set("adults", strconv.Itoa(request.Adults))
	if request.Children > 0 {
		values.Set("children", strconv.Itoa(request.Children))
	}
	return values
}

func (p *Provider) searchAPI(ctx context.Context, endpoint string, values url.Values, request flight.FlightSearchRequest) ([]flight.FlightSearchResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	resp, err := p.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Google Flights API request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var apiErr thirdPartyError
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Message != "" {
			return nil, fmt.Errorf("Google Flights API failed: %s", apiErr.Message)
		}
		return nil, fmt.Errorf("Google Flights API failed: HTTP %d", resp.StatusCode)
	}
	var body thirdPartyResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode Google Flights API response: %w", err)
	}
	return normalizeThirdParty(body, request), nil
}

func destinationText(destination string) string {
	if destination == "ANYWHERE" {
		return "anywhere"
	}
	return destination
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
}

type thirdPartyError struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}
