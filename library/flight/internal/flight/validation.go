package flight

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var codePattern = regexp.MustCompile(`^[A-Z]{3}$`)

func NormalizeRequest(req FlightSearchRequest) FlightSearchRequest {
	req.Origin = strings.ToUpper(strings.TrimSpace(req.Origin))
	req.Destination = strings.ToUpper(strings.TrimSpace(req.Destination))
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	req.Cabin = strings.ToLower(strings.TrimSpace(req.Cabin))
	req.Bags = strings.ToLower(strings.TrimSpace(req.Bags))
	req.Sort = strings.ToLower(strings.TrimSpace(req.Sort))
	if req.Currency == "" {
		req.Currency = "EUR"
	}
	if req.Cabin == "" {
		req.Cabin = CabinEconomy
	}
	if req.Bags == "" {
		req.Bags = BagsNone
	}
	if req.Adults == 0 {
		req.Adults = 1
	}
	if req.Sort == "" {
		req.Sort = SortBest
	}
	if req.DirectOnly && req.MaxStops == nil {
		zero := 0
		req.MaxStops = &zero
	}
	if req.OneWay {
		req.ReturnDate = ""
	}
	return req
}

func ValidateSearchRequest(req FlightSearchRequest) error {
	req = NormalizeRequest(req)
	if !codePattern.MatchString(req.Origin) {
		return fmt.Errorf("--from must be a 3-letter IATA airport or city code")
	}
	if req.Destination != "ANYWHERE" && !codePattern.MatchString(req.Destination) {
		return fmt.Errorf("--to must be a 3-letter IATA airport/city code or \"anywhere\"")
	}
	depart, err := parseDate(req.DepartDate, "--depart")
	if err != nil {
		return err
	}
	if req.ReturnDate != "" {
		ret, err := parseDate(req.ReturnDate, "--return")
		if err != nil {
			return err
		}
		if ret.Before(depart) {
			return fmt.Errorf("--return must be on or after --depart")
		}
	}
	if req.Adults < 1 {
		return fmt.Errorf("--adults must be at least 1")
	}
	if req.Children < 0 {
		return fmt.Errorf("--children cannot be negative")
	}
	if req.Infants < 0 {
		return fmt.Errorf("--infants cannot be negative")
	}
	if req.Currency != "" && !codePattern.MatchString(req.Currency) {
		return fmt.Errorf("--currency must be a 3-letter currency code")
	}
	if !validValue(req.Cabin, []string{CabinEconomy, CabinPremiumEconomy, CabinBusiness, CabinFirst}) {
		return fmt.Errorf("--cabin must be one of economy, premium_economy, business, first")
	}
	if !validValue(req.Bags, []string{BagsNone, BagsPersonalItem, BagsCabin, BagsChecked}) {
		return fmt.Errorf("--bags must be one of none, personal_item, cabin, checked")
	}
	if req.MaxStops != nil && *req.MaxStops < 0 {
		return fmt.Errorf("--max-stops cannot be negative")
	}
	if !validValue(req.Sort, []string{SortPrice, SortDuration, SortBest}) {
		return fmt.Errorf("--sort must be one of price, duration, best")
	}
	return nil
}

func ValidateFlexibleRequest(req FlexibleSearchRequest) error {
	req.FlightSearchRequest = NormalizeRequest(req.FlightSearchRequest)
	if !codePattern.MatchString(req.Origin) {
		return fmt.Errorf("--from must be a 3-letter IATA airport or city code")
	}
	if req.Destination != "ANYWHERE" && !codePattern.MatchString(req.Destination) {
		return fmt.Errorf("--to must be a 3-letter IATA airport/city code or \"anywhere\"")
	}
	if _, err := time.Parse("2006-01", req.Month); err != nil {
		return fmt.Errorf("--month must use YYYY-MM")
	}
	if req.Adults < 1 {
		return fmt.Errorf("--adults must be at least 1")
	}
	if req.TripDays < 0 {
		return fmt.Errorf("--trip-days cannot be negative")
	}
	if req.MaxPrice < 0 {
		return fmt.Errorf("--max-price cannot be negative")
	}
	return nil
}

func parseDate(value, flag string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, fmt.Errorf("%s is required", flag)
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must use YYYY-MM-DD", flag)
	}
	return date, nil
}

func validValue(value string, allowed []string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
