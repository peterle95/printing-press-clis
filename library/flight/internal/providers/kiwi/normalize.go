package kiwi

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"flight-pp-cli/internal/flight"
)

type response struct {
	Data []trip `json:"data"`
}

type trip struct {
	ID                 string             `json:"id"`
	Price              float64            `json:"price"`
	Currency           string             `json:"currency"`
	DeepLink           string             `json:"deep_link"`
	FlyFrom            string             `json:"flyFrom"`
	FlyTo              string             `json:"flyTo"`
	LocalDeparture     string             `json:"local_departure"`
	LocalArrival       string             `json:"local_arrival"`
	Airlines           []string           `json:"airlines"`
	Route              []route            `json:"route"`
	Duration           duration           `json:"duration"`
	VirtualInterlining bool               `json:"virtual_interlining"`
	HasAirportChange   bool               `json:"has_airport_change"`
	PnrCount           int                `json:"pnr_count"`
	BagsPrice          map[string]float64 `json:"bags_price"`
	Availability       availability       `json:"availability"`
	Quality            float64            `json:"quality"`
}

type route struct {
	ID                  string `json:"id"`
	FlyFrom             string `json:"flyFrom"`
	FlyTo               string `json:"flyTo"`
	CityFrom            string `json:"cityFrom"`
	CityTo              string `json:"cityTo"`
	LocalDeparture      string `json:"local_departure"`
	LocalArrival        string `json:"local_arrival"`
	Airline             string `json:"airline"`
	FlightNo            int    `json:"flight_no"`
	Return              int    `json:"return"`
	BagsRecheckRequired bool   `json:"bags_recheck_required"`
}

type duration struct {
	Departure int `json:"departure"`
	Return    int `json:"return"`
	Total     int `json:"total"`
}

type availability struct {
	Seats *int `json:"seats"`
}

func Normalize(body response, request flight.FlightSearchRequest) []flight.FlightSearchResult {
	results := make([]flight.FlightSearchResult, 0, len(body.Data))
	for _, item := range body.Data {
		outbound, inbound := splitRoute(item.Route)
		result := flight.FlightSearchResult{
			Provider:         "kiwi",
			ProviderResultID: item.ID,
			PriceAvailable:   item.Price > 0,
			TotalPrice:       item.Price,
			Currency:         firstNonEmpty(item.Currency, request.Currency),
			DeepLink:         item.DeepLink,
			Origin:           firstNonEmpty(item.FlyFrom, request.Origin),
			Destination:      firstNonEmpty(item.FlyTo, request.Destination),
			DepartAt:         item.LocalDeparture,
			ArriveAt:         item.LocalArrival,
			Airlines:         append([]string{}, item.Airlines...),
		}
		if len(outbound) > 0 {
			first := outbound[0]
			last := outbound[len(outbound)-1]
			result.Origin = first.FlyFrom
			result.Destination = last.FlyTo
			result.DepartAt = first.LocalDeparture
			result.ArriveAt = last.LocalArrival
			result.Stops += max(0, len(outbound)-1)
		}
		if len(inbound) > 0 {
			first := inbound[0]
			last := inbound[len(inbound)-1]
			result.ReturnDepartAt = first.LocalDeparture
			result.ReturnArriveAt = last.LocalArrival
			result.Stops += max(0, len(inbound)-1)
		}
		result.FlightNumbers = flightNumbers(item.Route)
		if len(result.Airlines) == 0 {
			result.Airlines = airlinesFromRoute(item.Route)
		}
		if item.Duration.Total > 0 {
			result.DurationMinutes = item.Duration.Total / 60
		} else {
			result.DurationMinutes = elapsedMinutes(result.DepartAt, result.ArriveAt) + elapsedMinutes(result.ReturnDepartAt, result.ReturnArriveAt)
		}
		result.Baggage = normalizeBaggage(item)
		result.Risks, result.Warnings = normalizeRisks(item)
		if result.Baggage.Notes == "" {
			result.Warnings = appendUnique(result.Warnings, "baggage data missing")
		}
		results = append(results, result)
	}
	return results
}

func splitRoute(routes []route) ([]route, []route) {
	var outbound []route
	var inbound []route
	for _, leg := range routes {
		if leg.Return > 0 {
			inbound = append(inbound, leg)
		} else {
			outbound = append(outbound, leg)
		}
	}
	if len(outbound) == 0 && len(inbound) == 0 {
		return routes, nil
	}
	sort.SliceStable(outbound, func(i, j int) bool {
		return outbound[i].LocalDeparture < outbound[j].LocalDeparture
	})
	sort.SliceStable(inbound, func(i, j int) bool {
		return inbound[i].LocalDeparture < inbound[j].LocalDeparture
	})
	return outbound, inbound
}

func normalizeBaggage(item trip) flight.Baggage {
	if len(item.BagsPrice) == 0 {
		return flight.Baggage{}
	}
	keys := make([]string, 0, len(item.BagsPrice))
	for key := range item.BagsPrice {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s bag from %.0f", key, item.BagsPrice[key]))
	}
	return flight.Baggage{
		CheckedBagIncluded: boolPtr(false),
		Notes:              "Checked bags appear priced separately by Kiwi: " + strings.Join(parts, ", "),
	}
}

func normalizeRisks(item trip) ([]string, []string) {
	var risks []string
	var warnings []string
	if item.VirtualInterlining {
		risks = appendUnique(risks, "self-transfer / virtual interlining")
	}
	if item.PnrCount > 1 {
		risks = appendUnique(risks, "separate-ticket itinerary")
	}
	if item.HasAirportChange {
		risks = appendUnique(risks, "airport change during connection")
	}
	for _, leg := range item.Route {
		if leg.BagsRecheckRequired {
			risks = appendUnique(risks, "baggage recheck required")
		}
	}
	for i := 1; i < len(item.Route); i++ {
		prev := item.Route[i-1]
		next := item.Route[i]
		if prev.Return != next.Return {
			continue
		}
		if prev.FlyTo != "" && next.FlyFrom != "" && prev.FlyTo != next.FlyFrom {
			risks = appendUnique(risks, "airport change during connection")
		}
		if isOvernightLayover(prev.LocalArrival, next.LocalDeparture) {
			warnings = appendUnique(warnings, "overnight layover")
		}
	}
	return risks, warnings
}

func flightNumbers(routes []route) []string {
	out := []string{}
	for _, leg := range routes {
		if leg.Airline == "" && leg.FlightNo == 0 {
			continue
		}
		out = appendUnique(out, strings.TrimSpace(leg.Airline+strconv.Itoa(leg.FlightNo)))
	}
	return out
}

func airlinesFromRoute(routes []route) []string {
	out := []string{}
	for _, leg := range routes {
		out = appendUnique(out, leg.Airline)
	}
	return out
}

func elapsedMinutes(start, end string) int {
	startTime, err1 := parseTime(start)
	endTime, err2 := parseTime(end)
	if err1 != nil || err2 != nil || endTime.Before(startTime) {
		return 0
	}
	return int(endTime.Sub(startTime).Minutes())
}

func isOvernightLayover(arrive, depart string) bool {
	arriveTime, err1 := parseTime(arrive)
	departTime, err2 := parseTime(depart)
	if err1 != nil || err2 != nil || departTime.Before(arriveTime) {
		return false
	}
	if arriveTime.Format("2006-01-02") != departTime.Format("2006-01-02") {
		return true
	}
	return departTime.Sub(arriveTime) >= 8*time.Hour
}

func parseTime(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05.000",
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

func boolPtr(value bool) *bool {
	return &value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
