// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package scheduling

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

func BuildPlan(now time.Time, cards []Card, events []Event, duplicates map[string]bool, options Options, orderer CardOrderer) (Plan, error) {
	if options.Location == nil {
		return Plan{}, fmt.Errorf("location is required")
	}
	if orderer == nil {
		orderer = DueDateOrderer{}
	}
	start, end := NextWeek(now, options.Location)
	eligible := make([]Card, 0, len(cards))
	for _, card := range cards {
		if card.Closed || duplicates[card.ID] {
			continue
		}
		eligible = append(eligible, card)
	}
	eligible = orderer.Order(eligible)

	plan := Plan{
		Timezone:    options.Location.String(),
		WindowStart: start.Format("2006-01-02"),
		WindowEnd:   end.AddDate(0, 0, -1).Format("2006-01-02"),
	}
	cardIndex := 0
	for _, day := range WeekDays(start, options.IncludeWeekends) {
		dayEvents := EventsForDay(events, day)
		decision := DayDecision{
			Date:           day.Format("2006-01-02"),
			Weekday:        day.Weekday().String(),
			ExistingEvents: len(dayEvents),
		}
		switch {
		case HasSourceEvent(dayEvents):
			decision.Skipped = "a Trello card is already scheduled on this day"
		case cardIndex >= len(eligible):
			decision.Skipped = "no unscheduled cards"
		default:
			card := eligible[cardIndex]
			cardOptions := options
			// PATCH: Board-aware cards can carry discovered Time estimates that override default duration.
			if card.EstimatedMinutes > 0 {
				cardOptions.DurationMinutes = card.EstimatedMinutes
			}
			slotStart, slotEnd, ok, err := FindAvailableSlot(day, dayEvents, cardOptions)
			if err != nil {
				return Plan{}, err
			}
			if !ok {
				decision.Skipped = "no suitable free slot"
			} else {
				assignment := Assignment{
					Card:           card,
					Date:           day.Format("2006-01-02"),
					Start:          slotStart,
					End:            slotEnd,
					ExistingEvents: len(dayEvents),
				}
				decision.Assignment = &assignment
				plan.Assignments = append(plan.Assignments, assignment)
				cardIndex++
			}
		}
		plan.Days = append(plan.Days, decision)
	}
	if cardIndex < len(eligible) {
		plan.Unscheduled = append(plan.Unscheduled, eligible[cardIndex:]...)
	}
	return plan, nil
}

func EventsForDay(events []Event, day time.Time) []Event {
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)
	result := make([]Event, 0)
	for _, event := range events {
		if strings.EqualFold(event.Status, "cancelled") {
			continue
		}
		if event.Start.Before(dayEnd) && event.End.After(dayStart) {
			result = append(result, event)
		}
	}
	return result
}

func HasSourceEvent(events []Event) bool {
	for _, event := range events {
		if event.Properties["source"] == Source {
			return true
		}
	}
	return false
}

func FindAvailableSlot(day time.Time, events []Event, options Options) (time.Time, time.Time, bool, error) {
	dayStartMinutes, err := clockMinutes(options.DayStart)
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	dayEndMinutes, err := clockMinutes(options.DayEnd)
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	preferredMinutes, err := clockMinutes(options.PreferredTime)
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	// PATCH: Planner tasks always run inside the requested 08:00–17:00 window.
	dayStartMinutes, dayEndMinutes = 8*60, 17*60
	if options.DurationMinutes <= 0 || dayStartMinutes >= dayEndMinutes {
		return time.Time{}, time.Time{}, false, fmt.Errorf("invalid scheduling window")
	}
	minutes := []int{preferredMinutes}
	for candidate := dayStartMinutes; candidate+options.DurationMinutes <= dayEndMinutes; candidate += 30 {
		if candidate != preferredMinutes {
			minutes = append(minutes, candidate)
		}
	}
	for _, candidate := range minutes {
		if candidate < dayStartMinutes || candidate+options.DurationMinutes > dayEndMinutes {
			continue
		}
		start := time.Date(day.Year(), day.Month(), day.Day(), candidate/60, candidate%60, 0, 0, options.Location)
		end := start.Add(time.Duration(options.DurationMinutes) * time.Minute)
		if slotFree(start, end, events) {
			return start, end, true, nil
		}
	}
	return time.Time{}, time.Time{}, false, nil
}

func slotFree(start, end time.Time, events []Event) bool {
	for _, event := range events {
		if event.AllDay {
			return false
		}
		// PATCH: Timed events require a two-hour buffer on either side, which also excludes overlaps.
		if !(end.Before(event.Start.Add(-2*time.Hour)) || end.Equal(event.Start.Add(-2*time.Hour)) || start.After(event.End.Add(2*time.Hour)) || start.Equal(event.End.Add(2*time.Hour))) {
			return false
		}
	}
	return true
}

func SlotAvailable(start, end time.Time, events []Event) bool {
	return slotFree(start, end, events)
}

func clockMinutes(value string) (int, error) {
	t, err := time.Parse("15:04", value)
	if err != nil {
		return 0, fmt.Errorf("invalid time %q; use HH:MM", value)
	}
	return t.Hour()*60 + t.Minute(), nil
}

func DeterministicEventID(calendarID, boardID, cardID string) string {
	sum := sha256.Sum256([]byte(calendarID + "\x00" + boardID + "\x00" + cardID))
	return "tcc" + hex.EncodeToString(sum[:])
}

func SortEvents(events []Event) {
	sort.SliceStable(events, func(i, j int) bool { return events[i].Start.Before(events[j].Start) })
}
