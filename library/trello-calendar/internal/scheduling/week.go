// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package scheduling

import "time"

func NextWeek(now time.Time, location *time.Location) (time.Time, time.Time) {
	local := now.In(location)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	daysUntilMonday := (int(time.Monday) - int(today.Weekday()) + 7) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}
	start := today.AddDate(0, 0, daysUntilMonday)
	return start, start.AddDate(0, 0, 7)
}

func WeekDays(start time.Time, includeWeekends bool) []time.Time {
	days := make([]time.Time, 0, 7)
	for offset := 0; offset < 7; offset++ {
		day := start.AddDate(0, 0, offset)
		if !includeWeekends && (day.Weekday() == time.Saturday || day.Weekday() == time.Sunday) {
			continue
		}
		days = append(days, day)
	}
	return days
}
