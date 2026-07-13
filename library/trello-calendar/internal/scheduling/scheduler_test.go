package scheduling

import (
	"testing"
	"time"
)

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func defaultOptions(t *testing.T) Options {
	return Options{Location: berlin(t), DurationMinutes: 60, PreferredTime: "10:00", DayStart: "09:00", DayEnd: "18:00", MaxEventsPerDay: 3}
}

func TestNextWeekMondayAndSunday(t *testing.T) {
	loc := berlin(t)
	tests := []struct {
		name string
		now  time.Time
		want string
	}{
		{"sunday", time.Date(2026, 7, 12, 12, 0, 0, 0, loc), "2026-07-13"},
		{"monday", time.Date(2026, 7, 13, 8, 0, 0, 0, loc), "2026-07-20"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := NextWeek(tt.now, loc)
			if got := start.Format("2006-01-02"); got != tt.want {
				t.Fatalf("start=%s want=%s", got, tt.want)
			}
			if got := end.Sub(start); got != 7*24*time.Hour {
				t.Fatalf("ordinary week duration=%s", got)
			}
		})
	}
}

func TestNextWeekAcrossBerlinDST(t *testing.T) {
	loc := berlin(t)
	start, end := NextWeek(time.Date(2026, 3, 22, 12, 0, 0, 0, loc), loc)
	if start.Format("2006-01-02") != "2026-03-23" || end.Format("2006-01-02") != "2026-03-30" {
		t.Fatalf("unexpected range %s..%s", start, end)
	}
	if end.Sub(start) != 167*time.Hour {
		t.Fatalf("DST week should be 167 elapsed hours, got %s", end.Sub(start))
	}
}

func TestCapacityAndOneCardPerDay(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, loc)
	monday := time.Date(2026, 7, 13, 0, 0, 0, 0, loc)
	cards := []Card{{ID: "a", Name: "A", Position: 1}, {ID: "b", Name: "B", Position: 2}}
	for count := 0; count <= 3; count++ {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			var events []Event
			for i := 0; i < count; i++ {
				start := monday.Add(time.Duration(12+i) * time.Hour)
				events = append(events, Event{ID: string(rune('a' + i)), Start: start, End: start.Add(15 * time.Minute)})
			}
			plan, err := BuildPlan(now, cards, events, nil, defaultOptions(t), nil)
			if err != nil {
				t.Fatal(err)
			}
			if count < 3 && (len(plan.Assignments) == 0 || plan.Assignments[0].Date != "2026-07-13") {
				t.Fatalf("count %d should allow Monday", count)
			}
			if count == 3 && (plan.Days[0].Assignment != nil || plan.Days[0].Skipped != "daily event limit reached") {
				t.Fatalf("count 3 should skip Monday: %#v", plan.Days[0])
			}
			seen := map[string]bool{}
			for _, assignment := range plan.Assignments {
				if seen[assignment.Date] {
					t.Fatalf("more than one card assigned to %s", assignment.Date)
				}
				seen[assignment.Date] = true
			}
		})
	}
}

func TestAllDayAndTimedEventsCount(t *testing.T) {
	loc := berlin(t)
	day := time.Date(2026, 7, 13, 0, 0, 0, 0, loc)
	events := []Event{
		{ID: "all", Start: day, End: day.AddDate(0, 0, 1), AllDay: true},
		{ID: "timed", Start: day.Add(12 * time.Hour), End: day.Add(13 * time.Hour)},
		{ID: "cancelled", Status: "cancelled", Start: day, End: day.Add(time.Hour)},
	}
	if got := len(EventsForDay(events, day)); got != 2 {
		t.Fatalf("count=%d want=2", got)
	}
	_, _, ok, err := FindAvailableSlot(day, EventsForDay(events, day), defaultOptions(t))
	if err != nil || ok {
		t.Fatalf("all-day event should block all slots, ok=%v err=%v", ok, err)
	}
}

func TestPreferredAndFallbackSlots(t *testing.T) {
	loc := berlin(t)
	day := time.Date(2026, 7, 13, 0, 0, 0, 0, loc)
	opts := defaultOptions(t)
	start, _, ok, err := FindAvailableSlot(day, nil, opts)
	if err != nil || !ok || start.Format("15:04") != "10:00" {
		t.Fatalf("preferred slot: %s ok=%v err=%v", start, ok, err)
	}
	events := []Event{{Start: day.Add(9 * time.Hour), End: day.Add(11 * time.Hour)}}
	start, _, ok, err = FindAvailableSlot(day, events, opts)
	if err != nil || !ok || start.Format("15:04") != "11:00" {
		t.Fatalf("fallback slot: %s ok=%v err=%v", start, ok, err)
	}
	events = []Event{{Start: day.Add(9 * time.Hour), End: day.Add(18 * time.Hour)}}
	_, _, ok, err = FindAvailableSlot(day, events, opts)
	if err != nil || ok {
		t.Fatalf("full day should have no slot, ok=%v err=%v", ok, err)
	}
}

func TestWeekendsAndDuplicates(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, loc)
	cards := make([]Card, 7)
	for i := range cards {
		cards[i] = Card{ID: string(rune('a' + i)), Name: "card", Position: float64(i)}
	}
	plan, err := BuildPlan(now, cards, nil, map[string]bool{"a": true}, defaultOptions(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Days) != 5 || len(plan.Assignments) != 5 || plan.Assignments[0].Card.ID != "b" {
		t.Fatalf("default weekday/duplicate behavior wrong: days=%d assignments=%#v", len(plan.Days), plan.Assignments)
	}
	opts := defaultOptions(t)
	opts.IncludeWeekends = true
	plan, err = BuildPlan(now, cards, nil, nil, opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Days) != 7 || len(plan.Assignments) != 7 {
		t.Fatalf("weekends not included: days=%d assignments=%d", len(plan.Days), len(plan.Assignments))
	}
}

func TestOrdering(t *testing.T) {
	d1 := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	cards := []Card{
		{ID: "undated-2", Position: 2}, {ID: "late", Due: &d2, Position: 0},
		{ID: "undated-1", Position: 1}, {ID: "early", Due: &d1, Position: 9},
	}
	ordered := (DueDateOrderer{}).Order(cards)
	want := []string{"early", "late", "undated-1", "undated-2"}
	for i := range want {
		if ordered[i].ID != want[i] {
			t.Fatalf("order[%d]=%s want=%s", i, ordered[i].ID, want[i])
		}
	}
}

func TestSourceEventPreventsSecondCard(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, loc)
	monday := time.Date(2026, 7, 13, 10, 0, 0, 0, loc)
	events := []Event{{Start: monday, End: monday.Add(time.Hour), Properties: map[string]string{"source": Source}}}
	plan, err := BuildPlan(now, []Card{{ID: "a", Name: "A"}}, events, nil, defaultOptions(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Assignments[0].Date != "2026-07-14" {
		t.Fatalf("expected Tuesday assignment, got %s", plan.Assignments[0].Date)
	}
}

func TestDeterministicEventID(t *testing.T) {
	a := DeterministicEventID("primary", "board", "card")
	b := DeterministicEventID("primary", "board", "card")
	if a != b || len(a) != 67 {
		t.Fatalf("invalid deterministic ID %q", a)
	}
}
