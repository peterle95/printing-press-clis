package flight

import "testing"

func TestSortResultsBestPenalizesRisk(t *testing.T) {
	results := []FlightSearchResult{
		{
			Provider:        "cheap-risky",
			PriceAvailable:  true,
			TotalPrice:      80,
			Currency:        "EUR",
			DurationMinutes: 540,
			Stops:           1,
			Risks:           []string{"self-transfer / virtual interlining", "separate-ticket itinerary"},
			Warnings:        []string{"overnight layover", "baggage data missing"},
		},
		{
			Provider:        "useful",
			PriceAvailable:  true,
			TotalPrice:      95,
			Currency:        "EUR",
			DurationMinutes: 80,
			Stops:           0,
			Baggage: Baggage{
				CheckedBagIncluded: boolPtr(true),
			},
		},
	}
	SortResults(results, SortBest)
	if results[0].Provider != "useful" {
		t.Fatalf("expected useful result first, got %s with score %.2f", results[0].Provider, results[0].Score)
	}
}

func TestSortResultsPrice(t *testing.T) {
	results := []FlightSearchResult{
		{Provider: "manual", PriceAvailable: false},
		{Provider: "expensive", PriceAvailable: true, TotalPrice: 200},
		{Provider: "cheap", PriceAvailable: true, TotalPrice: 100},
	}
	SortResults(results, SortPrice)
	if results[0].Provider != "cheap" || results[2].Provider != "manual" {
		t.Fatalf("unexpected price order: %#v", results)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
