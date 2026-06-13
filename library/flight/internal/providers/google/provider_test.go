package google

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"

	"flight-pp-cli/internal/flight"
)

func TestBuildGoogleFlightsURL(t *testing.T) {
	req := flight.FlightSearchRequest{
		Origin:      "BER",
		Destination: "CPH",
		DepartDate:  "2026-07-10",
		ReturnDate:  "2026-07-14",
		Adults:      1,
		Currency:    "EUR",
		Cabin:       flight.CabinEconomy,
	}
	link := BuildGoogleFlightsURL(req)
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse link: %v", err)
	}
	if parsed.Host != "www.google.com" || !strings.Contains(parsed.Path, "/travel/flights") {
		t.Fatalf("unexpected URL: %s", link)
	}
	query := parsed.Query().Get("q")
	for _, want := range []string{"BER", "CPH", "2026-07-10", "2026-07-14"} {
		if !strings.Contains(query, want) {
			t.Fatalf("query %q missing %q", query, want)
		}
	}
}

func TestNormalizeThirdPartyResponse(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/fixtures/google_flights_api.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var body thirdPartyResponse
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	results := normalizeThirdParty(body, flight.FlightSearchRequest{
		Origin:      "BER",
		Destination: "CPH",
		DepartDate:  "2026-07-10",
		Currency:    "EUR",
		Adults:      1,
	})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].PriceAvailable || results[0].TotalPrice != 88 {
		t.Fatalf("unexpected first price: %#v", results[0])
	}
	if len(results[1].Risks) == 0 || len(results[1].Warnings) == 0 {
		t.Fatalf("expected risks and warnings on second result: %#v", results[1])
	}
}
