package transit

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestParseDepartureJSON(t *testing.T) {
	raw := []byte(`{
		"departures": [{
			"tripId": "trip-1",
			"stop": {"type":"stop","id":"900078102","name":"U Rathaus Neukoelln (Berlin)"},
			"when": "2026-05-27T08:42:00+02:00",
			"plannedWhen": "2026-05-27T08:40:00+02:00",
			"delay": 120,
			"platform": "2",
			"plannedPlatform": "1",
			"direction": "Rathaus Spandau",
			"line": {"name":"U7","product":"subway","productName":"U"},
			"remarks": [{"type":"hint","text":"Bicycle conveyance"}]
		}]
	}`)
	var parsed DepartureResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.Departures) != 1 {
		t.Fatalf("departures len = %d", len(parsed.Departures))
	}
	dep := parsed.Departures[0]
	if dep.TripID != "trip-1" || dep.Line.Name != "U7" || dep.Line.Product != "subway" {
		t.Fatalf("unexpected parsed departure: %+v", dep)
	}
	if dep.When == nil || dep.PlannedWhen == nil {
		t.Fatalf("expected realtime and planned times")
	}
	if got := FormatDelay(dep.Delay, true, false); got != "+2m" {
		t.Fatalf("delay = %s", got)
	}
}

func TestMinutesUntilDeparture(t *testing.T) {
	now := time.Date(2026, 5, 27, 8, 30, 0, 0, time.UTC)
	dep := now.Add(9*time.Minute + 1*time.Second)
	if got := MinutesUntil(now, &dep); got != 10 {
		t.Fatalf("minutes until = %d", got)
	}
	past := now.Add(-30 * time.Second)
	if got := MinutesUntil(now, &past); got != -1 {
		t.Fatalf("past minutes until = %d", got)
	}
}

func TestWalkingMinutes(t *testing.T) {
	if got := WalkingMinutes(161, "normal"); got != 3 {
		t.Fatalf("normal walking minutes = %d", got)
	}
	if got := WalkingMinutes(300, "fast"); got != 3 {
		t.Fatalf("fast walking minutes = %d", got)
	}
	if got := WalkingMinutes(121, "slow"); got != 3 {
		t.Fatalf("slow walking minutes = %d", got)
	}
}

func TestCatchabilityClassification(t *testing.T) {
	now := time.Date(2026, 5, 27, 8, 30, 0, 0, time.UTC)
	dep := now.Add(13 * time.Minute)
	if got := Catchability(now, &dep, 4, 5, false); got != StatusCatchable {
		t.Fatalf("expected catchable, got %s", got)
	}
	soon := now.Add(11 * time.Minute)
	if got := Catchability(now, &soon, 4, 5, false); got != StatusLeaveSoon {
		t.Fatalf("expected leave soon, got %s", got)
	}
	nowish := now.Add(9 * time.Minute)
	if got := Catchability(now, &nowish, 4, 5, false); got != StatusLeaveNow {
		t.Fatalf("expected leave now, got %s", got)
	}
	late := now.Add(3 * time.Minute)
	if got := Catchability(now, &late, 4, 5, false); got != StatusTooLate {
		t.Fatalf("expected too late, got %s", got)
	}
}

func TestFormatDelay(t *testing.T) {
	zero := 0
	plus := 60
	minus := -120
	if got := FormatDelay(&zero, true, false); got != "0m" {
		t.Fatalf("zero delay = %s", got)
	}
	if got := FormatDelay(&plus, true, false); got != "+1m" {
		t.Fatalf("plus delay = %s", got)
	}
	if got := FormatDelay(&minus, true, false); got != "-2m" {
		t.Fatalf("minus delay = %s", got)
	}
	if got := FormatDelay(nil, false, false); got != "sched" {
		t.Fatalf("scheduled delay = %s", got)
	}
	if got := FormatDelay(nil, false, true); got != "cancelled" {
		t.Fatalf("cancelled delay = %s", got)
	}
}

func TestRouteSummaryFormatting(t *testing.T) {
	loc := BerlinLocation()
	dep := time.Date(2026, 5, 27, 8, 0, 0, 0, loc)
	walkArr := dep.Add(5 * time.Minute)
	trainDep := dep.Add(6 * time.Minute)
	arr := dep.Add(25 * time.Minute)
	delay := 300
	journey := Journey{Legs: []Leg{
		{
			Origin:      Location{Address: "Example Home"},
			Destination: Location{Name: "U Example"},
			Departure:   &dep,
			Arrival:     &walkArr,
			Walking:     true,
			Distance:    300,
		},
		{
			Origin:         Location{Name: "U Example"},
			Destination:    Location{Name: "S Example"},
			Departure:      &trainDep,
			Arrival:        &arr,
			DepartureDelay: &delay,
			Line:           Line{Name: "U7", Product: "subway"},
		},
	}}
	summary := SummarizeJourney(journey)
	if summary.FirstStop != "U Example" || summary.FirstTransitLine != "U7" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	text := FormatRouteSummary(summary)
	if !strings.Contains(text, "U7") || !strings.Contains(text, "warnings: U7 delayed +5m") {
		t.Fatalf("route summary text missing expected parts: %s", text)
	}
}

func TestBoundingBoxCalculation(t *testing.T) {
	box := BoundingBoxAround(52.5, 13.4, 1000)
	if box.North <= 52.5 || box.South >= 52.5 || box.West >= 13.4 || box.East <= 13.4 {
		t.Fatalf("invalid bounding box: %+v", box)
	}
	latMeters := DistanceMeters(52.5, 13.4, box.North, 13.4)
	if math.Abs(float64(latMeters-1000)) > 5 {
		t.Fatalf("north edge distance = %dm", latMeters)
	}
}
