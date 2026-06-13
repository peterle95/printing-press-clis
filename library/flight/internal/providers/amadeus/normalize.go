package amadeus

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"flight-pp-cli/internal/flight"
)

type flightOffersResponse struct {
	Data []offer `json:"data"`
}

type offer struct {
	ID               string            `json:"id"`
	Price            offerPrice        `json:"price"`
	Itineraries      []itinerary       `json:"itineraries"`
	TravelerPricings []travelerPricing `json:"travelerPricings"`
}

type offerPrice struct {
	Currency   string `json:"currency"`
	GrandTotal string `json:"grandTotal"`
	Total      string `json:"total"`
}

type itinerary struct {
	Duration string    `json:"duration"`
	Segments []segment `json:"segments"`
}

type segment struct {
	Departure   locationTime `json:"departure"`
	Arrival     locationTime `json:"arrival"`
	CarrierCode string       `json:"carrierCode"`
	Number      string       `json:"number"`
}

type locationTime struct {
	IATACode string `json:"iataCode"`
	At       string `json:"at"`
}

type travelerPricing struct {
	FareDetailsBySegment []fareDetails `json:"fareDetailsBySegment"`
}

type fareDetails struct {
	IncludedCheckedBags *includedCheckedBags `json:"includedCheckedBags,omitempty"`
}

type includedCheckedBags struct {
	Quantity *int `json:"quantity,omitempty"`
	Weight   *int `json:"weight,omitempty"`
}

func Normalize(body flightOffersResponse, request flight.FlightSearchRequest) []flight.FlightSearchResult {
	results := make([]flight.FlightSearchResult, 0, len(body.Data))
	for _, item := range body.Data {
		priceValue := item.Price.GrandTotal
		if priceValue == "" {
			priceValue = item.Price.Total
		}
		price, _ := strconv.ParseFloat(priceValue, 64)
		result := flight.FlightSearchResult{
			Provider:         "amadeus",
			ProviderResultID: item.ID,
			PriceAvailable:   price > 0,
			TotalPrice:       price,
			Currency:         firstNonEmpty(item.Price.Currency, request.Currency),
			Origin:           request.Origin,
			Destination:      request.Destination,
			Airlines:         []string{},
			FlightNumbers:    []string{},
		}
		duration := 0
		for i, itin := range item.Itineraries {
			duration += parseISODurationMinutes(itin.Duration)
			if len(itin.Segments) == 0 {
				continue
			}
			result.Stops += max(0, len(itin.Segments)-1)
			first := itin.Segments[0]
			last := itin.Segments[len(itin.Segments)-1]
			if i == 0 {
				result.Origin = first.Departure.IATACode
				result.Destination = last.Arrival.IATACode
				result.DepartAt = first.Departure.At
				result.ArriveAt = last.Arrival.At
			} else if i == 1 {
				result.ReturnDepartAt = first.Departure.At
				result.ReturnArriveAt = last.Arrival.At
			}
			for idx, seg := range itin.Segments {
				if seg.CarrierCode != "" {
					result.Airlines = appendUnique(result.Airlines, seg.CarrierCode)
				}
				if seg.CarrierCode != "" || seg.Number != "" {
					result.FlightNumbers = appendUnique(result.FlightNumbers, strings.TrimSpace(seg.CarrierCode+seg.Number))
				}
				if idx > 0 {
					prev := itin.Segments[idx-1]
					if prev.Arrival.IATACode != "" && seg.Departure.IATACode != "" && prev.Arrival.IATACode != seg.Departure.IATACode {
						result.Risks = appendUnique(result.Risks, "airport change during connection")
					}
					if isOvernightLayover(prev.Arrival.At, seg.Departure.At) {
						result.Warnings = appendUnique(result.Warnings, "overnight layover")
					}
				}
			}
		}
		if duration == 0 {
			duration = elapsedMinutes(result.DepartAt, result.ArriveAt) + elapsedMinutes(result.ReturnDepartAt, result.ReturnArriveAt)
		}
		result.DurationMinutes = duration
		result.Baggage = normalizeBaggage(item)
		if baggageUnknown(result.Baggage) {
			result.Warnings = appendUnique(result.Warnings, "baggage data missing")
		}
		results = append(results, result)
	}
	return results
}

func normalizeBaggage(item offer) flight.Baggage {
	checkedKnown := false
	checkedIncluded := false
	for _, pricing := range item.TravelerPricings {
		for _, fare := range pricing.FareDetailsBySegment {
			if fare.IncludedCheckedBags == nil {
				continue
			}
			checkedKnown = true
			if fare.IncludedCheckedBags.Quantity != nil && *fare.IncludedCheckedBags.Quantity > 0 {
				checkedIncluded = true
			}
			if fare.IncludedCheckedBags.Weight != nil && *fare.IncludedCheckedBags.Weight > 0 {
				checkedIncluded = true
			}
		}
	}
	if !checkedKnown {
		return flight.Baggage{}
	}
	return flight.Baggage{
		CheckedBagIncluded: boolPtr(checkedIncluded),
		Notes:              "Amadeus exposes checked-bag data when supplied by the offer; verify carry-on and fare rules before purchase.",
	}
}

func parseISODurationMinutes(value string) int {
	if value == "" {
		return 0
	}
	rest := strings.TrimPrefix(value, "P")
	days := 0
	if idx := strings.Index(rest, "D"); idx >= 0 {
		days, _ = strconv.Atoi(rest[:idx])
		rest = rest[idx+1:]
	}
	rest = strings.TrimPrefix(rest, "T")
	hours := 0
	minutes := 0
	if idx := strings.Index(rest, "H"); idx >= 0 {
		hours, _ = strconv.Atoi(rest[:idx])
		rest = rest[idx+1:]
	}
	if idx := strings.Index(rest, "M"); idx >= 0 {
		minutes, _ = strconv.Atoi(rest[:idx])
	}
	return days*24*60 + hours*60 + minutes
}

func elapsedMinutes(start, end string) int {
	startTime, err1 := parseProviderTime(start)
	endTime, err2 := parseProviderTime(end)
	if err1 != nil || err2 != nil || endTime.Before(startTime) {
		return 0
	}
	return int(endTime.Sub(startTime).Minutes())
}

func isOvernightLayover(arrive, depart string) bool {
	arriveTime, err1 := parseProviderTime(arrive)
	departTime, err2 := parseProviderTime(depart)
	if err1 != nil || err2 != nil || departTime.Before(arriveTime) {
		return false
	}
	if arriveTime.Format("2006-01-02") != departTime.Format("2006-01-02") {
		return true
	}
	return departTime.Sub(arriveTime) >= 8*time.Hour
}

func parseProviderTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
	}
	var last error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
		last = err
	}
	return time.Time{}, last
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func baggageUnknown(bag flight.Baggage) bool {
	return bag.PersonalItemIncluded == nil && bag.CabinBagIncluded == nil && bag.CheckedBagIncluded == nil && bag.Notes == ""
}

func boolPtr(value bool) *bool {
	return &value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
