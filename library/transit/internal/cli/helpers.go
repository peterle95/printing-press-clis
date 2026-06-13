package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"transit-pp-cli/internal/config"
	"transit-pp-cli/internal/provider/vbb"
	"transit-pp-cli/internal/transit"
)

const (
	geocodeTTL    = 30 * 24 * time.Hour
	nearbyTTL     = 24 * time.Hour
	departureTTL  = 20 * time.Second
	homeResultMax = 5
)

func modeFlagsFromConfig(cfg config.ModeConfig) transit.ProductFlags {
	return transit.ProductFlags{
		Suburban: cfg.Suburban,
		Subway:   cfg.Subway,
		Tram:     cfg.Tram,
		Bus:      cfg.Bus,
		Ferry:    cfg.Ferry,
		Express:  cfg.Express,
		Regional: cfg.Regional,
	}
}

func cacheKey(parts ...any) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, fmt.Sprint(part))
	}
	return strings.Join(out, "|")
}

func cachedLocations(ctx context.Context, a *app, query string, results int, stops bool, addresses bool, poi bool) ([]transit.Location, error) {
	key := cacheKey("locations", a.config.BaseURL, query, results, stops, addresses, poi)
	var out []transit.Location
	if !a.flags.noCache {
		if ok, err := a.cache.LoadJSON(key, geocodeTTL, &out); err != nil {
			return nil, err
		} else if ok {
			return out, nil
		}
	}
	out, err := a.client.Locations(ctx, query, results, stops, addresses, poi)
	if err != nil {
		return nil, err
	}
	_ = a.cache.SaveJSON(key, out)
	return out, nil
}

func cachedNearby(ctx context.Context, a *app, latitude, longitude float64, results int, distance int) ([]transit.Location, error) {
	key := cacheKey("nearby", a.config.BaseURL, latitude, longitude, results, distance)
	var out []transit.Location
	if !a.flags.noCache {
		if ok, err := a.cache.LoadJSON(key, nearbyTTL, &out); err != nil {
			return nil, err
		} else if ok {
			return out, nil
		}
	}
	out, err := a.client.Nearby(ctx, latitude, longitude, results, distance, true)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Distance < out[j].Distance })
	_ = a.cache.SaveJSON(key, out)
	return out, nil
}

func cachedDepartures(ctx context.Context, a *app, stopID string, minutes int, modes transit.ProductFlags) ([]transit.Departure, error) {
	key := cacheKey("departures", a.config.BaseURL, stopID, minutes, modes.Suburban, modes.Subway, modes.Tram, modes.Bus, modes.Ferry, modes.Express, modes.Regional)
	var out []transit.Departure
	if !a.flags.noCache {
		if ok, err := a.cache.LoadJSON(key, departureTTL, &out); err != nil {
			return nil, err
		} else if ok {
			return out, nil
		}
	}
	out, err := a.client.Departures(ctx, stopID, minutes, 20, modes)
	if err != nil {
		return nil, err
	}
	_ = a.cache.SaveJSON(key, out)
	return out, nil
}

func homeLocation(ctx context.Context, a *app) (transit.Location, error) {
	home := a.config.Home
	if a.config.HomeHasCoordinates() {
		return transit.Location{
			Type:      "location",
			Name:      home.Label,
			Address:   home.Address,
			Latitude:  *home.Latitude,
			Longitude: *home.Longitude,
		}, nil
	}
	if strings.TrimSpace(home.Address) == "" {
		return transit.Location{}, fmt.Errorf("home is not configured; run transit-pp-cli config set-home --address \"Examplestr. 1, 10115 Berlin\" or set --lat/--lon")
	}
	return resolveAndSaveHome(ctx, a)
}

func resolveAndSaveHome(ctx context.Context, a *app) (transit.Location, error) {
	results, err := cachedLocations(ctx, a, a.config.Home.Address, homeResultMax, false, true, false)
	if err != nil {
		return transit.Location{}, err
	}
	if len(results) == 0 {
		return transit.Location{}, fmt.Errorf("could not resolve home address %q", a.config.Home.Address)
	}
	selected, err := chooseLocation(results, a.flags, "home address")
	if err != nil {
		return transit.Location{}, err
	}
	lat, lon, ok := selected.Coordinates()
	if !ok {
		return transit.Location{}, fmt.Errorf("provider returned no coordinates for %q", selected.DisplayName())
	}
	a.config.Home.Latitude = &lat
	a.config.Home.Longitude = &lon
	if err := config.Save(a.config.Path, a.config); err != nil {
		return transit.Location{}, fmt.Errorf("save resolved home coordinates: %w", err)
	}
	return transit.Location{
		Type:      "location",
		Name:      a.config.Home.Label,
		Address:   a.config.Home.Address,
		Latitude:  lat,
		Longitude: lon,
	}, nil
}

func chooseLocation(results []transit.Location, flags *rootFlags, label string) (transit.Location, error) {
	if len(results) == 1 || flags.yes {
		return results[0], nil
	}
	if wantsJSON(flags) || !stdinIsTerminal() {
		candidates := make([]string, 0, len(results))
		for i, result := range results {
			candidates = append(candidates, fmt.Sprintf("%d: %s", i+1, result.DisplayName()))
		}
		return transit.Location{}, fmt.Errorf("multiple matches for %s; rerun interactively or pass --yes to accept the first match: %s", label, strings.Join(candidates, "; "))
	}
	fmt.Printf("Multiple matches for %s:\n", label)
	for i, result := range results {
		lat, lon, _ := result.Coordinates()
		fmt.Printf("  %d. %s (%.6f, %.6f)\n", i+1, result.DisplayName(), lat, lon)
	}
	fmt.Printf("Choose [1-%d]: ", len(results))
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	for i := range results {
		if answer == fmt.Sprint(i+1) {
			return results[i], nil
		}
	}
	return transit.Location{}, fmt.Errorf("no match selected")
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func resolveDestination(ctx context.Context, a *app, query string) (vbb.JourneyQuery, transit.Location, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return vbb.JourneyQuery{}, transit.Location{}, fmt.Errorf("destination is required")
	}
	if lat, lon, ok := parseCoordinates(query); ok {
		return vbb.JourneyQuery{
				ToLatitude:  lat,
				ToLongitude: lon,
				ToAddress:   "coordinates",
			}, transit.Location{
				Type:      "location",
				Name:      "coordinates",
				Latitude:  lat,
				Longitude: lon,
			}, nil
	}
	results, err := cachedLocations(ctx, a, query, 5, true, true, false)
	if err != nil {
		return vbb.JourneyQuery{}, transit.Location{}, err
	}
	if len(results) == 0 {
		return vbb.JourneyQuery{}, transit.Location{}, fmt.Errorf("no destination matched %q", query)
	}
	chosen := results[0]
	journey := vbb.JourneyQuery{}
	if chosen.Type == "stop" && chosen.ID != "" {
		journey.ToID = chosen.ID
		return journey, chosen, nil
	}
	lat, lon, ok := chosen.Coordinates()
	if !ok {
		return vbb.JourneyQuery{}, transit.Location{}, fmt.Errorf("destination %q has no coordinates", chosen.DisplayName())
	}
	journey.ToLatitude = lat
	journey.ToLongitude = lon
	journey.ToAddress = chosen.DisplayName()
	return journey, chosen, nil
}

func parseCoordinates(value string) (float64, float64, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "@")
	value = strings.ReplaceAll(value, ";", ",")
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		parts = strings.Fields(value)
	}
	if len(parts) != 2 {
		return 0, 0, false
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lon, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return 0, 0, false
	}
	return lat, lon, true
}

func resolveStop(ctx context.Context, a *app, query string) (transit.Location, error) {
	results, err := cachedLocations(ctx, a, query, 5, true, false, false)
	if err != nil {
		return transit.Location{}, err
	}
	if len(results) == 0 {
		return transit.Location{}, fmt.Errorf("no stop matched %q", query)
	}
	for _, result := range results {
		if result.Type == "stop" && result.ID != "" {
			return result, nil
		}
	}
	return transit.Location{}, fmt.Errorf("no stop matched %q", query)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func joinOrDash(values []string) string {
	clean := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			clean = append(clean, value)
		}
	}
	if len(clean) == 0 {
		return "-"
	}
	return strings.Join(clean, ", ")
}
