package kiwi

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

const defaultBaseURL = "https://api.tequila.kiwi.com"

type Provider struct {
	cfg     config.KiwiConfig
	http    *http.Client
	baseURL string
}

func New(cfg config.KiwiConfig, timeout time.Duration) *Provider {
	return &Provider{
		cfg:     cfg,
		http:    &http.Client{Timeout: timeout},
		baseURL: defaultBaseURL,
	}
}

func NewWithHTTP(cfg config.KiwiConfig, baseURL string, client *http.Client) *Provider {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Provider{cfg: cfg, http: client, baseURL: strings.TrimRight(baseURL, "/")}
}

func (p *Provider) Name() string {
	return "kiwi"
}

func (p *Provider) Status(ctx context.Context) flight.ProviderStatus {
	status := flight.ProviderStatus{
		Name:    p.Name(),
		Enabled: p.cfg.Enabled,
		Mode:    "tequila",
	}
	if !p.cfg.Enabled {
		status.Warnings = append(status.Warnings, "disabled in config")
		return status
	}
	if p.cfg.APIKey == "" {
		status.Missing = append(status.Missing, "KIWI_API_KEY or providers.kiwi.api_key")
	}
	status.Available = len(status.Missing) == 0
	return status
}

func (p *Provider) Search(ctx context.Context, request flight.FlightSearchRequest) ([]flight.FlightSearchResult, error) {
	values := p.baseSearchValues(request)
	values.Set("date_from", kiwiDate(request.DepartDate))
	values.Set("date_to", kiwiDate(request.DepartDate))
	if request.ReturnDate != "" {
		values.Set("return_from", kiwiDate(request.ReturnDate))
		values.Set("return_to", kiwiDate(request.ReturnDate))
	}
	return p.search(ctx, values, request)
}

func (p *Provider) Cheapest(ctx context.Context, request flight.FlexibleSearchRequest) ([]flight.FlightSearchResult, error) {
	values := p.baseSearchValues(request.FlightSearchRequest)
	first, last, err := monthRange(request.Month)
	if err != nil {
		return nil, err
	}
	values.Set("date_from", first)
	values.Set("date_to", last)
	if request.TripDays > 0 {
		values.Set("nights_in_dst_from", strconv.Itoa(request.TripDays))
		values.Set("nights_in_dst_to", strconv.Itoa(request.TripDays))
	}
	if request.MaxPrice > 0 {
		values.Set("price_to", strconv.FormatFloat(request.MaxPrice, 'f', 0, 64))
	}
	return p.search(ctx, values, request.FlightSearchRequest)
}

func (p *Provider) baseSearchValues(request flight.FlightSearchRequest) url.Values {
	values := url.Values{}
	values.Set("fly_from", request.Origin)
	if request.Destination == "ANYWHERE" {
		values.Set("fly_to", "anywhere")
	} else {
		values.Set("fly_to", request.Destination)
	}
	values.Set("adults", strconv.Itoa(request.Adults))
	if request.Children > 0 {
		values.Set("children", strconv.Itoa(request.Children))
	}
	if request.Infants > 0 {
		values.Set("infants", strconv.Itoa(request.Infants))
	}
	values.Set("curr", request.Currency)
	values.Set("selected_cabins", kiwiCabin(request.Cabin))
	if request.DirectOnly {
		values.Set("max_stopovers", "0")
	} else if request.MaxStops != nil {
		values.Set("max_stopovers", strconv.Itoa(*request.MaxStops))
	}
	limit := request.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	values.Set("limit", strconv.Itoa(limit))
	values.Set("sort", "price")
	values.Set("vehicle_type", "aircraft")
	values.Set("one_for_city", "1")
	return values
}

func (p *Provider) search(ctx context.Context, values url.Values, request flight.FlightSearchRequest) ([]flight.FlightSearchResult, error) {
	if status := p.Status(ctx); !status.Available {
		return nil, fmt.Errorf("missing credentials: %s", strings.Join(status.Missing, ", "))
	}
	endpoint := p.baseURL + "/v2/search?" + values.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("apikey", p.cfg.APIKey)
	httpReq.Header.Set("Accept", "application/json")
	resp, err := p.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Kiwi search request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var apiErr kiwiError
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Message != "" {
			return nil, fmt.Errorf("Kiwi failed: %s", apiErr.Message)
		}
		return nil, fmt.Errorf("Kiwi failed: HTTP %d", resp.StatusCode)
	}
	var body response
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode Kiwi response: %w", err)
	}
	return Normalize(body, request), nil
}

func kiwiCabin(cabin string) string {
	switch cabin {
	case flight.CabinPremiumEconomy:
		return "W"
	case flight.CabinBusiness:
		return "C"
	case flight.CabinFirst:
		return "F"
	default:
		return "M"
	}
}

func kiwiDate(value string) string {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return value
	}
	return parsed.Format("02/01/2006")
}

func monthRange(month string) (string, string, error) {
	first, err := time.Parse("2006-01", month)
	if err != nil {
		return "", "", fmt.Errorf("--month must use YYYY-MM")
	}
	last := first.AddDate(0, 1, -1)
	return first.Format("02/01/2006"), last.Format("02/01/2006"), nil
}

type kiwiError struct {
	Message string `json:"message"`
}
