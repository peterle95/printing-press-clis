package transit

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	StatusCatchable = "CATCHABLE"
	StatusLeaveSoon = "LEAVE SOON"
	StatusLeaveNow  = "LEAVE NOW"
	StatusTooLate   = "TOO LATE"
)

func BerlinLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return time.Local
	}
	return loc
}

func EffectiveDeparture(dep Departure) *time.Time {
	if dep.When != nil {
		return dep.When
	}
	return dep.PlannedWhen
}

func EffectiveLegDeparture(leg Leg) *time.Time {
	if leg.Departure != nil {
		return leg.Departure
	}
	return leg.PlannedDeparture
}

func EffectiveLegArrival(leg Leg) *time.Time {
	if leg.Arrival != nil {
		return leg.Arrival
	}
	return leg.PlannedArrival
}

func MinutesUntil(now time.Time, when *time.Time) int {
	if when == nil {
		return 0
	}
	minutes := when.Sub(now).Minutes()
	if minutes >= 0 {
		return int(math.Ceil(minutes))
	}
	return int(math.Floor(minutes))
}

func WalkingMinutes(distanceMeters int, speed string) int {
	if distanceMeters <= 0 {
		return 0
	}
	metersPerMinute := 80.0
	switch strings.ToLower(strings.TrimSpace(speed)) {
	case "slow":
		metersPerMinute = 60
	case "fast":
		metersPerMinute = 100
	}
	return int(math.Ceil(float64(distanceMeters) / metersPerMinute))
}

func Catchability(now time.Time, depTime *time.Time, walkingMinutes int, bufferMinutes int, cancelled bool) string {
	if cancelled || depTime == nil {
		return StatusTooLate
	}
	minutesLeft := MinutesUntil(now, depTime)
	if minutesLeft < 0 || minutesLeft < walkingMinutes {
		return StatusTooLate
	}
	if bufferMinutes < 0 {
		bufferMinutes = 0
	}
	if minutesLeft <= walkingMinutes+bufferMinutes {
		return StatusLeaveNow
	}
	if minutesLeft <= walkingMinutes+bufferMinutes+3 {
		return StatusLeaveSoon
	}
	return StatusCatchable
}

func FormatDelay(delay *int, realtime bool, cancelled bool) string {
	if cancelled {
		return "cancelled"
	}
	if delay == nil {
		if realtime {
			return "0m"
		}
		return "sched"
	}
	minutes := int(math.Round(float64(*delay) / 60.0))
	if minutes > 0 {
		return fmt.Sprintf("+%dm", minutes)
	}
	if minutes < 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return "0m"
}

func FormatClock(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.In(BerlinLocation()).Format("15:04")
}

func FormatDateTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.In(BerlinLocation()).Format("2006-01-02 15:04")
}

func ProductLabel(product string) string {
	switch strings.ToLower(strings.TrimSpace(product)) {
	case "suburban":
		return "S"
	case "subway":
		return "U"
	case "regional", "express":
		return "RE"
	case "tram":
		return "tram"
	case "bus":
		return "bus"
	case "ferry":
		return "ferry"
	default:
		if product == "" {
			return "-"
		}
		return product
	}
}

func ProductNames(products ProductFlags) []string {
	names := []string{}
	if products.Suburban {
		names = append(names, "suburban")
	}
	if products.Subway {
		names = append(names, "subway")
	}
	if products.Tram {
		names = append(names, "tram")
	}
	if products.Bus {
		names = append(names, "bus")
	}
	if products.Ferry {
		names = append(names, "ferry")
	}
	if products.Express {
		names = append(names, "express")
	}
	if products.Regional {
		names = append(names, "regional")
	}
	return names
}

func RemarkTexts(remarks []Remark) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, remark := range remarks {
		text := strings.TrimSpace(remark.Text)
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		out = append(out, text)
	}
	sort.Strings(out)
	return out
}

func DistanceMeters(lat1, lon1, lat2, lon2 float64) int {
	const earthRadius = 6371000.0
	lat1r := lat1 * math.Pi / 180
	lat2r := lat2 * math.Pi / 180
	dlat := (lat2 - lat1) * math.Pi / 180
	dlon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1r)*math.Cos(lat2r)*math.Sin(dlon/2)*math.Sin(dlon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return int(math.Round(earthRadius * c))
}

type BoundingBox struct {
	North float64 `json:"north"`
	West  float64 `json:"west"`
	South float64 `json:"south"`
	East  float64 `json:"east"`
}

func BoundingBoxAround(latitude, longitude float64, radiusMeters int) BoundingBox {
	if radiusMeters < 0 {
		radiusMeters = 0
	}
	const earthRadius = 6371000.0
	latDelta := float64(radiusMeters) / earthRadius * 180 / math.Pi
	latRad := latitude * math.Pi / 180
	lonDelta := 0.0
	if math.Abs(math.Cos(latRad)) > 0.000001 {
		lonDelta = float64(radiusMeters) / (earthRadius * math.Cos(latRad)) * 180 / math.Pi
	}
	return BoundingBox{
		North: latitude + latDelta,
		South: latitude - latDelta,
		West:  longitude - lonDelta,
		East:  longitude + lonDelta,
	}
}

func ParseArriveBy(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	loc := BerlinLocation()
	now = now.In(loc)
	for _, layout := range []string{"15:04", "15:04:05"} {
		parsed, err := time.ParseInLocation(layout, value, loc)
		if err == nil {
			target := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, loc)
			if target.Before(now) {
				target = target.Add(24 * time.Hour)
			}
			return target, nil
		}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04", "2006-01-02T15:04"} {
		parsed, err := time.ParseInLocation(layout, value, loc)
		if err == nil {
			return parsed.In(loc), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse arrive-by %q: use HH:MM or RFC3339", value)
}

type JourneySummary struct {
	Departure        *time.Time `json:"departure,omitempty"`
	FirstStop        string     `json:"first_stop,omitempty"`
	FirstStopBy      *time.Time `json:"first_stop_by,omitempty"`
	Lines            []string   `json:"lines"`
	Transfers        int        `json:"transfers"`
	Arrival          *time.Time `json:"arrival,omitempty"`
	FinalWalk        string     `json:"final_walk,omitempty"`
	Warnings         []string   `json:"warnings,omitempty"`
	RefreshToken     string     `json:"refreshToken,omitempty"`
	Risky            bool       `json:"risky"`
	FirstTransitLine string     `json:"first_transit_line,omitempty"`
	FirstTransitAt   *time.Time `json:"first_transit_at,omitempty"`
}

func SummarizeJourney(j Journey) JourneySummary {
	summary := JourneySummary{
		Lines:        []string{},
		Warnings:     []string{},
		RefreshToken: j.RefreshToken,
	}
	if len(j.Legs) == 0 {
		summary.Risky = true
		summary.Warnings = append(summary.Warnings, "journey has no legs")
		return summary
	}
	summary.Departure = EffectiveLegDeparture(j.Legs[0])
	summary.Arrival = EffectiveLegArrival(j.Legs[len(j.Legs)-1])
	transitLegs := 0
	for i, leg := range j.Legs {
		if leg.Cancelled {
			summary.Risky = true
			summary.Warnings = append(summary.Warnings, "cancelled leg")
		}
		if leg.Reachable != nil && !*leg.Reachable {
			summary.Risky = true
			summary.Warnings = append(summary.Warnings, "unreachable connection")
		}
		if leg.DepartureDelay != nil && *leg.DepartureDelay >= 300 {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("%s delayed %s", legLabel(leg), FormatDelay(leg.DepartureDelay, true, false)))
		}
		if leg.ArrivalDelay != nil && *leg.ArrivalDelay >= 300 {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("arrival delayed %s", FormatDelay(leg.ArrivalDelay, true, false)))
		}
		if leg.Walking {
			if i == 0 {
				summary.FirstStop = leg.Destination.DisplayName()
				summary.FirstStopBy = EffectiveLegArrival(leg)
			}
			if i == len(j.Legs)-1 {
				summary.FinalWalk = fmt.Sprintf("%dm walk", WalkingMinutes(leg.Distance, "normal"))
			}
			continue
		}
		transitLegs++
		line := strings.TrimSpace(leg.Line.Name)
		if line == "" {
			line = strings.TrimSpace(leg.Line.ProductName)
		}
		if line == "" {
			line = "transit"
		}
		summary.Lines = append(summary.Lines, line)
		if summary.FirstTransitLine == "" {
			summary.FirstTransitLine = line
			summary.FirstTransitAt = EffectiveLegDeparture(leg)
			if summary.FirstStop == "" {
				summary.FirstStop = leg.Origin.DisplayName()
				summary.FirstStopBy = EffectiveLegDeparture(leg)
			}
		}
	}
	if transitLegs > 0 {
		summary.Transfers = transitLegs - 1
	}
	if summary.FinalWalk == "" {
		last := j.Legs[len(j.Legs)-1]
		if last.Walking {
			summary.FinalWalk = fmt.Sprintf("%dm walk", WalkingMinutes(last.Distance, "normal"))
		} else {
			summary.FinalWalk = "none"
		}
	}
	return summary
}

func FormatRouteSummary(summary JourneySummary) string {
	lines := "walk only"
	if len(summary.Lines) > 0 {
		lines = strings.Join(summary.Lines, " -> ")
	}
	warnings := "none"
	if len(summary.Warnings) > 0 {
		warnings = strings.Join(summary.Warnings, "; ")
	}
	return fmt.Sprintf("%s -> %s | %s | transfers: %d | final walk: %s | warnings: %s",
		FormatClock(summary.Departure),
		FormatClock(summary.Arrival),
		lines,
		summary.Transfers,
		summary.FinalWalk,
		warnings,
	)
}

func legLabel(leg Leg) string {
	if leg.Line.Name != "" {
		return leg.Line.Name
	}
	return leg.Destination.DisplayName()
}
