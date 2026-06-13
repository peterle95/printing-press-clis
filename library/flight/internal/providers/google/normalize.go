package google

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"flight-pp-cli/internal/flight"
)

type thirdPartyResponse struct {
	SearchMetadata struct {
		GoogleFlightsURL string `json:"google_flights_url"`
		Status           string `json:"status"`
	} `json:"search_metadata"`
	BestFlights  []apiItinerary `json:"best_flights"`
	OtherFlights []apiItinerary `json:"other_flights"`
}

type apiItinerary struct {
	Price         any          `json:"price"`
	TotalDuration int          `json:"total_duration"`
	Flights       []apiFlight  `json:"flights"`
	Layovers      []apiLayover `json:"layovers"`
	BookingToken  string       `json:"booking_token"`
	Type          string       `json:"type"`
}

type apiFlight struct {
	DepartureAirport apiAirport `json:"departure_airport"`
	ArrivalAirport   apiAirport `json:"arrival_airport"`
	Airline          string     `json:"airline"`
	FlightNumber     string     `json:"flight_number"`
	Duration         int        `json:"duration"`
	Extensions       []string   `json:"extensions"`
	Overnight        bool       `json:"overnight"`
}

type apiAirport struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Time string `json:"time"`
}

type apiLayover struct {
	Duration  int    `json:"duration"`
	Name      string `json:"name"`
	ID        string `json:"id"`
	Overnight bool   `json:"overnight"`
}

func normalizeThirdParty(body thirdPartyResponse, request flight.FlightSearchRequest) []flight.FlightSearchResult {
	items := append([]apiItinerary{}, body.BestFlights...)
	items = append(items, body.OtherFlights...)
	results := make([]flight.FlightSearchResult, 0, len(items))
	for idx, item := range items {
		price, priceOK := parsePrice(item.Price)
		deepLink := body.SearchMetadata.GoogleFlightsURL
		if deepLink == "" {
			deepLink = BuildGoogleFlightsURL(request)
		}
		result := flight.FlightSearchResult{
			Provider:         "google",
			ProviderResultID: fmt.Sprintf("google-%d", idx+1),
			PriceAvailable:   priceOK,
			TotalPrice:       price,
			Currency:         request.Currency,
			DeepLink:         deepLink,
			Origin:           request.Origin,
			Destination:      request.Destination,
			DurationMinutes:  item.TotalDuration,
			Stops:            max(0, len(item.Flights)-1),
			Baggage: flight.Baggage{
				Notes: "Baggage details from third-party Google Flights APIs are incomplete; verify fare rules before purchase.",
			},
		}
		if len(item.Flights) > 0 {
			first := item.Flights[0]
			last := item.Flights[len(item.Flights)-1]
			result.Origin = first.DepartureAirport.ID
			result.Destination = last.ArrivalAirport.ID
			result.DepartAt = first.DepartureAirport.Time
			result.ArriveAt = last.ArrivalAirport.Time
		}
		for _, leg := range item.Flights {
			result.Airlines = appendUnique(result.Airlines, leg.Airline)
			result.FlightNumbers = appendUnique(result.FlightNumbers, leg.FlightNumber)
			for _, extension := range leg.Extensions {
				if containsFold(extension, "self transfer") {
					result.Risks = appendUnique(result.Risks, "self-transfer")
				}
				if containsFold(extension, "separate") {
					result.Risks = appendUnique(result.Risks, "separate-ticket itinerary")
				}
				if containsFold(extension, "baggage") {
					result.Warnings = appendUnique(result.Warnings, extension)
				}
			}
			if leg.Overnight {
				result.Warnings = appendUnique(result.Warnings, "overnight layover")
			}
		}
		for _, layover := range item.Layovers {
			if layover.Overnight {
				result.Warnings = appendUnique(result.Warnings, "overnight layover")
			}
		}
		if result.DurationMinutes == 0 {
			for _, leg := range item.Flights {
				result.DurationMinutes += leg.Duration
			}
		}
		if len(result.Warnings) == 0 {
			result.Warnings = append(result.Warnings, "third-party Google Flights data; verify on booking page")
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		results = append(results, DeepLinkResult(request))
	}
	return results
}

func parsePrice(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, typed > 0
	case int:
		return float64(typed), typed > 0
	case string:
		clean := strings.ReplaceAll(typed, ",", "")
		clean = strings.TrimLeft(clean, "$€£ ")
		parsed, err := strconv.ParseFloat(clean, 64)
		return parsed, err == nil && parsed > 0
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil && parsed > 0
	default:
		return 0, false
	}
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func containsFold(value, needle string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(needle))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
