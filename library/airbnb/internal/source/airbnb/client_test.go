package airbnb

import (
	"strings"
	"testing"
)

func TestListingFromPDPSectionsUsesTitleDefault(t *testing.T) {
	root := map[string]any{
		"presentation": map[string]any{
			"stayProductDetailPage": map[string]any{
				"sections": map[string]any{
					"sections": []any{
						map[string]any{
							"sectionId": "BOOK_IT_SIDEBAR",
							"section": map[string]any{
								"section": map[string]any{
									"calendarTitle":    "Select dates",
									"maxGuestCapacity": float64(6),
									"structuredDisplayPrice": map[string]any{
										"primaryLine": map[string]any{
											"price":     "$381",
											"qualifier": "/night",
										},
										"secondaryLine": map[string]any{
											"price": "$1,143 for 3 nights",
										},
										"explanationData": map[string]any{
											"priceItems": []any{
												map[string]any{"title": "Cleaning fee", "price": "$295"},
												map[string]any{"title": "Service fee", "price": "$172"},
												map[string]any{"title": "Taxes", "price": "$118"},
											},
										},
									},
								},
							},
						},
						map[string]any{
							"sectionId": "TITLE_DEFAULT",
							"section": map[string]any{
								"title": "Tahoe Keys Breeze-Boat Dock, Hot Tub, BBQ",
							},
						},
						map[string]any{
							"sectionId": "MEET_YOUR_HOST",
							"section": map[string]any{
								"cardData": map[string]any{
									"name": "RnR Vacation Rentals",
								},
								"about": "RnR Vacation Rentals aims to provide handpicked homes.",
							},
						},
						map[string]any{
							"sectionId": "LOCATION_DEFAULT",
							"section": map[string]any{
								"subtitle": "South Lake Tahoe, California, United States",
							},
						},
					},
				},
			},
		},
	}

	listing := listingFromPDPSections(root, "37124493")
	if listing.Title != "Tahoe Keys Breeze-Boat Dock, Hot Tub, BBQ" {
		t.Fatalf("Title = %q, want real TITLE_DEFAULT title", listing.Title)
	}
	if listing.Title == "Select dates" {
		t.Fatalf("Title came from BOOK_IT_SIDEBAR calendarTitle")
	}
	if listing.HostName != "RnR Vacation Rentals" {
		t.Fatalf("HostName = %q", listing.HostName)
	}
	if listing.SleepsMax != 6 {
		t.Fatalf("SleepsMax = %d, want 6", listing.SleepsMax)
	}
	if listing.City != "South Lake Tahoe" {
		t.Fatalf("City = %q, want parsed city", listing.City)
	}
	if listing.Region != "California" {
		t.Fatalf("Region = %q, want parsed region", listing.Region)
	}
	if listing.PriceTotal != 1143 || listing.PerNightPrice != 381 {
		t.Fatalf("PriceTotal/PerNightPrice = %v/%v, want 1143/381", listing.PriceTotal, listing.PerNightPrice)
	}
	if listing.PriceBreakdown == nil || listing.PriceBreakdown.Fees["cleaning"] != 295 {
		t.Fatalf("PriceBreakdown cleaning fee = %#v, want 295", listing.PriceBreakdown)
	}
}

func TestBookingURLIncludesStayDatesAndGuests(t *testing.T) {
	u := BookingURL("37124493", "2026-05-16", "2026-05-19", 4, 1, 0, 0)

	for _, want := range []string{
		"https://www.airbnb.com/rooms/37124493",
		"check_in=2026-05-16",
		"check_out=2026-05-19",
		"adults=4",
		"children=1",
	} {
		if !strings.Contains(u, want) {
			t.Fatalf("BookingURL = %q, missing %q", u, want)
		}
	}
}

func TestMarkAvailabilityFromPricing(t *testing.T) {
	available := Listing{PriceBreakdown: &PriceBreakdown{Total: 250}}
	markAvailabilityFromPricing(&available, "2026-05-16", "2026-05-19", "test")
	if available.Available == nil || !*available.Available || available.AvailabilityStatus != "available" {
		t.Fatalf("available listing status = %#v/%q", available.Available, available.AvailabilityStatus)
	}

	unavailable := Listing{}
	markAvailabilityFromPricing(&unavailable, "2026-05-16", "2026-05-19", "test")
	if unavailable.Available == nil || *unavailable.Available || unavailable.AvailabilityStatus != "unavailable" {
		t.Fatalf("unavailable listing status = %#v/%q", unavailable.Available, unavailable.AvailabilityStatus)
	}
}

func TestAmountFromTextParsesCurrencyCodes(t *testing.T) {
	for input, want := range map[string]float64{
		"DKK 1,200 total": 1200,
		"1.200 DKK total": 1200,
		"kr 850":          850,
		"€1.234,56 total": 1234.56,
	} {
		if got := amountFromText(input); got != want {
			t.Fatalf("amountFromText(%q) = %v, want %v", input, got, want)
		}
	}
}
