// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

// Package scheduling contains pure planning logic with no API dependencies.
// PATCH: Provide a testable scheduling abstraction for the cross-API workflow.
package scheduling

import "time"

const Source = "trello-calendar-cli"

type Label struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

type Card struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	URL         string     `json:"url"`
	Due         *time.Time `json:"due,omitempty"`
	DueComplete bool       `json:"due_complete"`
	Labels      []Label    `json:"labels,omitempty"`
	Position    float64    `json:"position"`
	Closed      bool       `json:"closed"`
	Scheduled   bool       `json:"scheduled,omitempty"`
}

type Event struct {
	ID         string            `json:"id"`
	Summary    string            `json:"summary,omitempty"`
	Status     string            `json:"status,omitempty"`
	Start      time.Time         `json:"start"`
	End        time.Time         `json:"end"`
	AllDay     bool              `json:"all_day"`
	Properties map[string]string `json:"private_properties,omitempty"`
}

type Options struct {
	Location        *time.Location
	DurationMinutes int
	PreferredTime   string
	DayStart        string
	DayEnd          string
	MaxEventsPerDay int
	IncludeWeekends bool
	TitlePrefix     string
}

type Assignment struct {
	Card           Card      `json:"card"`
	Date           string    `json:"date"`
	Start          time.Time `json:"start"`
	End            time.Time `json:"end"`
	ExistingEvents int       `json:"existing_events"`
}

type DayDecision struct {
	Date           string      `json:"date"`
	Weekday        string      `json:"weekday"`
	ExistingEvents int         `json:"existing_events"`
	Assignment     *Assignment `json:"assignment,omitempty"`
	Skipped        string      `json:"skipped,omitempty"`
}

type Plan struct {
	Timezone    string        `json:"timezone"`
	WindowStart string        `json:"window_start"`
	WindowEnd   string        `json:"window_end"`
	Days        []DayDecision `json:"days"`
	Assignments []Assignment  `json:"assignments"`
	Unscheduled []Card        `json:"unscheduled_cards,omitempty"`
}
