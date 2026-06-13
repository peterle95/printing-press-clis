package kiwi

import (
	"encoding/json"
	"os"
	"testing"

	"flight-pp-cli/internal/flight"
)

func TestNormalizeKiwiSearch(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/fixtures/kiwi_search.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var body response
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	results := Normalize(body, flight.FlightSearchRequest{
		Origin:      "BER",
		Destination: "CPH",
		Currency:    "EUR",
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	result := results[0]
	if result.Provider != "kiwi" || result.TotalPrice != 54 || result.DeepLink == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(result.Risks) < 2 {
		t.Fatalf("expected self-transfer and separate-ticket risks: %#v", result.Risks)
	}
	if result.Baggage.CheckedBagIncluded == nil || *result.Baggage.CheckedBagIncluded {
		t.Fatalf("expected checked baggage to be marked not included: %#v", result.Baggage)
	}
}
