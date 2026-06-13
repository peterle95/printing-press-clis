package stub

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"flight-pp-cli/internal/config"
	"flight-pp-cli/internal/flight"
)

type Provider struct {
	name       string
	enabled    bool
	available  bool
	mode       string
	missing    []string
	warnings   []string
	searchFunc func(flight.FlightSearchRequest) (flight.FlightSearchResult, error)
}

func (p Provider) Name() string {
	return p.name
}

func (p Provider) Status(ctx context.Context) flight.ProviderStatus {
	return flight.ProviderStatus{
		Name:      p.name,
		Enabled:   p.enabled,
		Available: p.enabled && p.available,
		Mode:      p.mode,
		Missing:   append([]string{}, p.missing...),
		Warnings:  append([]string{}, p.warnings...),
	}
}

func (p Provider) Search(ctx context.Context, request flight.FlightSearchRequest) ([]flight.FlightSearchResult, error) {
	if !p.enabled {
		return nil, fmt.Errorf("%s is disabled in config", p.name)
	}
	if !p.available {
		if len(p.missing) > 0 {
			return nil, fmt.Errorf("%s API key/partner access not configured: %s", p.name, strings.Join(p.missing, ", "))
		}
		return nil, fmt.Errorf("%s provider is not available in this MVP", p.name)
	}
	if p.searchFunc == nil {
		return nil, fmt.Errorf("%s provider search is not implemented in this MVP", p.name)
	}
	result, err := p.searchFunc(request)
	if err != nil {
		return nil, err
	}
	return []flight.FlightSearchResult{result}, nil
}

func Skyscanner(cfg config.APIKeyConfig) Provider {
	missing := []string{}
	if cfg.Enabled && cfg.APIKey == "" {
		missing = append(missing, "providers.skyscanner.api_key / partner access")
	}
	return Provider{
		name:      "skyscanner",
		enabled:   cfg.Enabled,
		available: false,
		mode:      "official-api",
		missing:   missing,
		warnings:  []string{"Skyscanner direct scraping is intentionally not implemented; official Travel API adapter is a future provider."},
		searchFunc: func(request flight.FlightSearchRequest) (flight.FlightSearchResult, error) {
			return flight.FlightSearchResult{}, fmt.Errorf("Skyscanner official Travel API adapter is a configured future provider")
		},
	}
}

func Expedia(cfg config.ModeAPIKeyConfig) Provider {
	mode := cfg.Mode
	if mode == "" {
		mode = "deeplink"
	}
	available := cfg.Enabled && mode == "deeplink"
	missing := []string{}
	if cfg.Enabled && mode != "deeplink" && cfg.APIKey == "" {
		missing = append(missing, "providers.expedia.api_key")
	}
	return Provider{
		name:      "expedia",
		enabled:   cfg.Enabled,
		available: available,
		mode:      mode,
		missing:   missing,
		warnings:  []string{"Expedia deep-link mode generates a URL but no comparable price data."},
		searchFunc: func(request flight.FlightSearchRequest) (flight.FlightSearchResult, error) {
			return manualResult("expedia", request, expediaURL(request), "Expedia deep-link generated; open manually to compare."), nil
		},
	}
}

func Kayak(cfg config.ModeAPIKeyConfig) Provider {
	mode := cfg.Mode
	if mode == "" {
		mode = "deeplink"
	}
	available := cfg.Enabled && mode == "deeplink"
	missing := []string{}
	if cfg.Enabled && mode != "deeplink" && cfg.APIKey == "" {
		missing = append(missing, "providers.kayak.api_key / affiliate access")
	}
	return Provider{
		name:      "kayak",
		enabled:   cfg.Enabled,
		available: available,
		mode:      mode,
		missing:   missing,
		warnings:  []string{"KAYAK deep-link mode generates a URL but no comparable price data."},
		searchFunc: func(request flight.FlightSearchRequest) (flight.FlightSearchResult, error) {
			return manualResult("kayak", request, kayakURL(request), "KAYAK deep-link generated; open manually to compare."), nil
		},
	}
}

func Travelpayouts(cfg config.TravelpayoutsConfig) Provider {
	missing := []string{}
	if cfg.Enabled && cfg.Token == "" {
		missing = append(missing, "TRAVELPAYOUTS_TOKEN or providers.travelpayouts.token")
	}
	return Provider{
		name:      "travelpayouts",
		enabled:   cfg.Enabled,
		available: false,
		mode:      "cached-api",
		missing:   missing,
		warnings:  []string{"Travelpayouts adapter is reserved for cached/stale price trend data and is not implemented in this MVP."},
	}
}

func manualResult(provider string, request flight.FlightSearchRequest, link, warning string) flight.FlightSearchResult {
	return flight.FlightSearchResult{
		Provider:       provider,
		PriceAvailable: false,
		Currency:       request.Currency,
		DeepLink:       link,
		Origin:         request.Origin,
		Destination:    request.Destination,
		DepartAt:       request.DepartDate,
		ReturnDepartAt: request.ReturnDate,
		Warnings:       []string{warning},
	}
}

func expediaURL(request flight.FlightSearchRequest) string {
	values := url.Values{}
	trip := "oneway"
	values.Set("leg1", fmt.Sprintf("from:%s,to:%s,departure:%sTANYT", request.Origin, destination(request.Destination), request.DepartDate))
	if request.ReturnDate != "" {
		trip = "roundtrip"
		values.Set("leg2", fmt.Sprintf("from:%s,to:%s,departure:%sTANYT", destination(request.Destination), request.Origin, request.ReturnDate))
	}
	values.Set("trip", trip)
	values.Set("passengers", "adults:"+strconv.Itoa(request.Adults))
	values.Set("mode", "search")
	return "https://www.expedia.com/Flights-Search?" + values.Encode()
}

func kayakURL(request flight.FlightSearchRequest) string {
	dest := destination(request.Destination)
	path := fmt.Sprintf("https://www.kayak.com/flights/%s-%s/%s", request.Origin, dest, request.DepartDate)
	if request.ReturnDate != "" {
		path += "/" + request.ReturnDate
	}
	values := url.Values{}
	if request.Adults > 1 {
		values.Set("adults", strconv.Itoa(request.Adults))
	}
	values.Set("sort", "price_a")
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

func destination(value string) string {
	if value == "ANYWHERE" {
		return "anywhere"
	}
	return value
}
