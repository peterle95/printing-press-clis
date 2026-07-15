// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

// Package scheduling contains pure planning logic with no API dependencies.
// PATCH: Provide a testable scheduling abstraction for the cross-API workflow.
package scheduling

import "time"

const Source = "trello-calendar-cli"

type Label struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

type Card struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	URL              string     `json:"url"`
	Description      string     `json:"description,omitempty"`
	ListID           string     `json:"list_id,omitempty"`
	ListName         string     `json:"list_name,omitempty"`
	MemberIDs        []string   `json:"member_ids,omitempty"`
	Due              *time.Time `json:"due,omitempty"`
	DueDate          *time.Time `json:"due_date,omitempty"`
	DueComplete      bool       `json:"due_complete"`
	Completed        bool       `json:"completed"`
	Priority         string     `json:"priority,omitempty"`
	EstimatedMinutes int        `json:"estimated_minutes,omitempty"`
	Automation       string     `json:"automation,omitempty"`
	Status           string     `json:"status,omitempty"`
	CreatedAt        *time.Time `json:"created_at,omitempty"`
	LastActivityAt   *time.Time `json:"last_activity_at,omitempty"`
	FieldWarnings    []string   `json:"field_warnings,omitempty"`
	Labels           []Label    `json:"labels,omitempty"`
	Position         float64    `json:"position"`
	Closed           bool       `json:"closed"`
	Scheduled        bool       `json:"scheduled,omitempty"`
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
	BufferMinutes   int
	PreferredTime   string
	DayStart        string
	DayEnd          string
	MaxEventsPerDay int
	IncludeWeekends bool
	TitlePrefix     string
	PriorityColors  map[string]string
}

type Assignment struct {
	Card           Card      `json:"card"`
	Date           string    `json:"date"`
	Start          time.Time `json:"start"`
	End            time.Time `json:"end"`
	ExistingEvents int       `json:"existing_events"`
}

type CardDecision struct {
	CardID string `json:"card_id"`
	Name   string `json:"name"`
	List   string `json:"list,omitempty"`
	Action string `json:"action"`
	Reason string `json:"reason,omitempty"`
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
