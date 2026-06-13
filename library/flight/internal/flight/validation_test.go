package flight

import "testing"

func TestValidateSearchRequest(t *testing.T) {
	req := FlightSearchRequest{
		Origin:      "ber",
		Destination: "cph",
		DepartDate:  "2026-07-10",
		ReturnDate:  "2026-07-14",
		Adults:      1,
		Currency:    "eur",
		Cabin:       CabinEconomy,
		Bags:        BagsNone,
		Sort:        SortBest,
	}
	if err := ValidateSearchRequest(req); err != nil {
		t.Fatalf("expected valid request: %v", err)
	}
	req.ReturnDate = "2026-07-01"
	if err := ValidateSearchRequest(req); err == nil {
		t.Fatalf("expected return-before-depart validation error")
	}
}

func TestValidateSearchRequestRejectsBadCodes(t *testing.T) {
	req := FlightSearchRequest{
		Origin:      "BERLIN",
		Destination: "CPH",
		DepartDate:  "2026-07-10",
		Adults:      1,
		Currency:    "EUR",
		Cabin:       CabinEconomy,
		Bags:        BagsNone,
		Sort:        SortBest,
	}
	if err := ValidateSearchRequest(req); err == nil {
		t.Fatalf("expected invalid origin error")
	}
}
