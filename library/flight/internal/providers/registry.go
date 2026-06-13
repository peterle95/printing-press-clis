package providers

import (
	"fmt"
	"strings"
	"time"

	"flight-pp-cli/internal/config"
	"flight-pp-cli/internal/flight"
	"flight-pp-cli/internal/providers/amadeus"
	"flight-pp-cli/internal/providers/google"
	"flight-pp-cli/internal/providers/kiwi"
	"flight-pp-cli/internal/providers/stub"
)

func Build(cfg config.Config, cacheDir string, timeout time.Duration, names []string) ([]flight.Provider, []error) {
	if len(names) == 0 {
		names = cfg.EnabledProviderNames()
	}
	var out []flight.Provider
	var errs []error
	for _, name := range names {
		provider, err := One(cfg, cacheDir, timeout, name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out = append(out, provider)
	}
	return out, errs
}

func All(cfg config.Config, cacheDir string, timeout time.Duration) []flight.Provider {
	out := []flight.Provider{}
	for _, name := range config.ProviderNames() {
		provider, err := One(cfg, cacheDir, timeout, name)
		if err == nil {
			out = append(out, provider)
		}
	}
	return out
}

func One(cfg config.Config, cacheDir string, timeout time.Duration, name string) (flight.Provider, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "amadeus":
		return amadeus.New(cfg.Providers.Amadeus, cacheDir, timeout), nil
	case "kiwi":
		return kiwi.New(cfg.Providers.Kiwi, timeout), nil
	case "google", "google-flights":
		return google.New(cfg.Providers.Google, timeout), nil
	case "skyscanner":
		return stub.Skyscanner(cfg.Providers.Skyscanner), nil
	case "expedia":
		return stub.Expedia(cfg.Providers.Expedia), nil
	case "kayak":
		return stub.Kayak(cfg.Providers.Kayak), nil
	case "travelpayouts":
		return stub.Travelpayouts(cfg.Providers.Travelpayouts), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", name)
	}
}
