package flight

import (
	"math"
	"sort"
	"strings"
)

func SortResults(results []FlightSearchResult, mode string) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = SortBest
	}
	for i := range results {
		results[i].Score = BestScore(results[i], results)
	}
	sort.SliceStable(results, func(i, j int) bool {
		a := results[i]
		b := results[j]
		switch mode {
		case SortPrice:
			return priceLess(a, b)
		case SortDuration:
			if a.DurationMinutes == b.DurationMinutes {
				return priceLess(a, b)
			}
			if a.DurationMinutes == 0 {
				return false
			}
			if b.DurationMinutes == 0 {
				return true
			}
			return a.DurationMinutes < b.DurationMinutes
		default:
			if a.Score == b.Score {
				return priceLess(a, b)
			}
			return a.Score < b.Score
		}
	})
}

func BestScore(result FlightSearchResult, peers []FlightSearchResult) float64 {
	if !result.PriceAvailable {
		return 10000
	}
	minPrice, maxPrice := priceRange(peers)
	priceNorm := 0.0
	if maxPrice > minPrice {
		priceNorm = (result.TotalPrice - minPrice) / (maxPrice - minPrice)
	}
	durationHours := float64(result.DurationMinutes) / 60
	score := priceNorm*70 + durationHours*2 + float64(result.Stops)*12
	if baggageUnknown(result.Baggage) {
		score += 12
	}
	for _, marker := range append(result.Risks, result.Warnings...) {
		switch {
		case containsFold(marker, "self-transfer"), containsFold(marker, "virtual interlining"):
			score += 60
		case containsFold(marker, "separate"):
			score += 35
		case containsFold(marker, "overnight"):
			score += 25
		case containsFold(marker, "airport change"):
			score += 25
		case containsFold(marker, "baggage"):
			score += 8
		}
	}
	return math.Round(score*100) / 100
}

func priceLess(a, b FlightSearchResult) bool {
	if a.PriceAvailable != b.PriceAvailable {
		return a.PriceAvailable
	}
	if !a.PriceAvailable {
		return a.Provider < b.Provider
	}
	if a.TotalPrice == b.TotalPrice {
		return a.DurationMinutes < b.DurationMinutes
	}
	return a.TotalPrice < b.TotalPrice
}

func priceRange(results []FlightSearchResult) (float64, float64) {
	minPrice := math.MaxFloat64
	maxPrice := 0.0
	for _, result := range results {
		if !result.PriceAvailable {
			continue
		}
		if result.TotalPrice < minPrice {
			minPrice = result.TotalPrice
		}
		if result.TotalPrice > maxPrice {
			maxPrice = result.TotalPrice
		}
	}
	if minPrice == math.MaxFloat64 {
		return 0, 0
	}
	return minPrice, maxPrice
}

func baggageUnknown(bag Baggage) bool {
	return bag.PersonalItemIncluded == nil && bag.CabinBagIncluded == nil && bag.CheckedBagIncluded == nil && bag.Notes == ""
}

func containsFold(value, needle string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(needle))
}

func LimitResults(results []FlightSearchResult, limit int) []FlightSearchResult {
	if limit <= 0 || len(results) <= limit {
		return results
	}
	return results[:limit]
}
