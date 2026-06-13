package amadeus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"flight-pp-cli/internal/config"
	"flight-pp-cli/internal/flight"
)

func TestAmadeusTokenCache(t *testing.T) {
	fixture, err := os.ReadFile("../../../testdata/fixtures/amadeus_flight_offers.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	tokenRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/security/oauth2/token":
			tokenRequests++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"token-1","expires_in":3600}`))
		case "/v2/shopping/flight-offers":
			if r.Header.Get("Authorization") != "Bearer token-1" {
				t.Fatalf("missing auth header: %s", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(fixture)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	provider := NewWithHTTP(config.AmadeusConfig{
		Enabled:      true,
		ClientID:     "client",
		ClientSecret: "secret",
		Environment:  "test",
	}, t.TempDir(), server.URL, server.Client())
	req := flight.FlightSearchRequest{
		Origin:      "BER",
		Destination: "CPH",
		DepartDate:  "2026-07-10",
		ReturnDate:  "2026-07-14",
		Adults:      1,
		Currency:    "EUR",
		Cabin:       flight.CabinEconomy,
		Limit:       2,
	}
	if _, err := provider.Search(context.Background(), req); err != nil {
		t.Fatalf("first search: %v", err)
	}
	if _, err := provider.Search(context.Background(), req); err != nil {
		t.Fatalf("second search: %v", err)
	}
	if tokenRequests != 1 {
		t.Fatalf("expected one token request, got %d", tokenRequests)
	}
}

func TestNormalizeAmadeusOffers(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/fixtures/amadeus_flight_offers.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var body flightOffersResponse
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	results := Normalize(body, flight.FlightSearchRequest{
		Origin:      "BER",
		Destination: "CPH",
		Currency:    "EUR",
	})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].TotalPrice != 79.99 || results[0].DurationMinutes != 125 {
		t.Fatalf("unexpected normalized first result: %#v", results[0])
	}
	if len(results[1].Warnings) == 0 {
		t.Fatalf("expected missing baggage or overnight warning on second result")
	}
}
