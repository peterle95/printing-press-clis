// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

// Package workflow orchestrates live reads, pure planning, and confirmed writes.
// PATCH: Keep cross-API mutation outside the pure scheduler and behind injectable interfaces.
package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"
	"trello-calendar-pp-cli/internal/scheduling"
	trelloadapter "trello-calendar-pp-cli/internal/trello"
)

type TrelloService interface {
	ValidateList(boardID, listID string) (string, error)
	ListOpenCards(listID string) ([]scheduling.Card, error)
	DiscoverBoard(boardID string) (trelloadapter.BoardDiscovery, error)
	ListOpenCardsInList(listID, listName string, fields trelloadapter.FieldMapping) ([]scheduling.Card, error)
	AddComment(cardID, text string) error
	MoveCard(cardID, targetListID string) error
}

type CalendarService interface {
	ListEvents(ctx context.Context, start, end time.Time) ([]scheduling.Event, error)
	ListEventColors(ctx context.Context) (map[string]bool, error)
	FindCard(ctx context.Context, boardID, cardID string) (bool, error)
	CreateEvent(ctx context.Context, boardID string, assignment scheduling.Assignment, titlePrefix, description, colorID string) (eventID string, created bool, err error)
	CheckAccess(ctx context.Context) error
}

// PATCH: Optional Calendar capabilities support reviewing existing Doing cards
// without forcing existing test doubles or alternate Calendar adapters to grow
// the core scheduling interface.
type CalendarEventFinder interface {
	FindCardEvent(ctx context.Context, boardID, cardID string) (scheduling.Event, bool, error)
}

type CalendarEventRescheduler interface {
	RescheduleEvent(ctx context.Context, eventID string, start, end time.Time) error
}

type Clock func() time.Time

type Service struct {
	Trello   TrelloService
	Calendar CalendarService
	Now      Clock
	BoardID  string
	ListID   string
	Options  scheduling.Options
	Orderer  scheduling.CardOrderer
	Policy   scheduling.SelectionPolicy
	DoingID  string
	DoneID   string
}

const DoingCapacity = 4

type DoingReviewItem struct {
	Card       scheduling.Card        `json:"card"`
	Completed  bool                   `json:"completed"`
	Action     string                 `json:"action"`
	EventID    string                 `json:"event_id,omitempty"`
	Assignment *scheduling.Assignment `json:"assignment,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

type DoingReviewResult struct {
	DoingCount     int               `json:"doing_count"`
	RemainingCount int               `json:"remaining_doing_count"`
	Items          []DoingReviewItem `json:"items"`
	Plan           scheduling.Plan   `json:"plan"`
}

type PlanResult struct {
	ListName  string                    `json:"list_name"`
	Warnings  []string                  `json:"warnings,omitempty"`
	Discovery any                       `json:"discovery,omitempty"`
	Decisions []scheduling.CardDecision `json:"decisions,omitempty"`
	Cards     []scheduling.Card         `json:"cards"`
	Plan      scheduling.Plan           `json:"plan"`
}

type AssignmentResult struct {
	CardID     string `json:"card_id"`
	CardName   string `json:"card_name"`
	Date       string `json:"date"`
	Start      string `json:"start"`
	End        string `json:"end"`
	EventID    string `json:"event_id,omitempty"`
	Status     string `json:"status"`
	MoveStatus string `json:"trello_move_status,omitempty"`
	Error      string `json:"error,omitempty"`
}

type ExecutionResult struct {
	DryRun   bool               `json:"dry_run"`
	Planned  int                `json:"planned"`
	Created  int                `json:"created"`
	Existing int                `json:"existing"`
	Moved    int                `json:"moved"`
	Failed   int                `json:"failed"`
	Results  []AssignmentResult `json:"results"`
}

func (s *Service) Cards(ctx context.Context) ([]scheduling.Card, string, []string, error) {
	if strings.TrimSpace(s.ListID) == "" {
		result, err := s.BoardCards(ctx)
		return result.Cards, s.Policy.DoingListName, result.Warnings, err
	}
	listName, err := s.Trello.ValidateList(s.BoardID, s.ListID)
	if err != nil {
		return nil, "", nil, err
	}
	warnings := []string{}
	targetName := s.Policy.DoingListName
	if targetName == "" {
		targetName = "Doing"
	}
	if listName != targetName {
		warnings = append(warnings, fmt.Sprintf("configured Trello list is named %q, not %s", listName, targetName))
	}
	cards, err := s.Trello.ListOpenCards(s.ListID)
	if err != nil {
		return nil, listName, warnings, err
	}
	for index := range cards {
		exists, err := s.Calendar.FindCard(ctx, s.BoardID, cards[index].ID)
		if err != nil {
			return nil, listName, warnings, fmt.Errorf("check card %s scheduling status: %w", cards[index].ID, err)
		}
		cards[index].Scheduled = exists
	}
	return s.OrdererOrDefault().Order(cards), listName, warnings, nil
}

func (s *Service) BoardCards(ctx context.Context) (PlanResult, error) {
	discovery, err := s.Trello.DiscoverBoard(s.BoardID)
	if err != nil {
		return PlanResult{}, err
	}
	warnings := append([]string{}, discovery.Warnings...)
	listsByName := map[string]trelloadapter.DiscoveredList{}
	for _, list := range discovery.Lists {
		listsByName[strings.ToLower(strings.TrimSpace(list.Name))] = list
	}
	doing := listsByName[strings.ToLower(strings.TrimSpace(s.Policy.DoingListName))]
	if doing.ID == "" {
		warnings = append(warnings, fmt.Sprintf("target list %q not discovered", s.Policy.DoingListName))
	}
	s.DoingID = doing.ID
	done := listsByName[strings.ToLower(strings.TrimSpace(s.Policy.DoneListName))]
	s.DoneID = done.ID
	var cards []scheduling.Card
	for _, name := range s.Policy.SourceListNames {
		list := listsByName[strings.ToLower(strings.TrimSpace(name))]
		if list.ID == "" {
			warnings = append(warnings, fmt.Sprintf("source list %q not discovered", name))
			continue
		}
		listCards, err := s.Trello.ListOpenCardsInList(list.ID, list.Name, discovery.Fields)
		if err != nil {
			return PlanResult{}, err
		}
		cards = append(cards, listCards...)
	}
	for index := range cards {
		exists, err := s.Calendar.FindCard(ctx, s.BoardID, cards[index].ID)
		if err != nil {
			return PlanResult{}, fmt.Errorf("check card %s scheduling status: %w", cards[index].ID, err)
		}
		cards[index].Scheduled = exists
	}
	return PlanResult{ListName: strings.Join(s.Policy.SourceListNames, ", "), Warnings: warnings, Discovery: discovery, Cards: s.OrdererOrDefault().Order(cards)}, nil
}

// PATCH: Prepare and optionally execute the human-confirmed Doing-card review.
// A completed card moves to Done; an incomplete card retains its list position
// and gets an existing Calendar event moved, or a missing event created, in the
// next scheduling week.
func (s *Service) ReviewDoing(ctx context.Context, completed map[string]bool, dryRun bool) (DoingReviewResult, error) {
	if strings.TrimSpace(s.ListID) != "" {
		return DoingReviewResult{}, fmt.Errorf("Doing review requires board-aware list discovery; unset trello_list_id")
	}
	discovery, err := s.Trello.DiscoverBoard(s.BoardID)
	if err != nil {
		return DoingReviewResult{}, err
	}
	lists := map[string]trelloadapter.DiscoveredList{}
	for _, list := range discovery.Lists {
		lists[strings.ToLower(strings.TrimSpace(list.Name))] = list
	}
	doing := lists[strings.ToLower(strings.TrimSpace(s.Policy.DoingListName))]
	done := lists[strings.ToLower(strings.TrimSpace(s.Policy.DoneListName))]
	if doing.ID == "" {
		return DoingReviewResult{}, fmt.Errorf("target list %q not discovered", s.Policy.DoingListName)
	}
	s.DoingID, s.DoneID = doing.ID, done.ID
	cards, err := s.Trello.ListOpenCardsInList(doing.ID, doing.Name, discovery.Fields)
	if err != nil {
		return DoingReviewResult{}, err
	}
	result := DoingReviewResult{DoingCount: len(cards)}
	var remaining []scheduling.Card
	for _, card := range cards {
		if !completed[card.ID] {
			remaining = append(remaining, card)
		}
	}
	result.RemainingCount = len(remaining)

	start, end := scheduling.NextWeek(s.Now(), s.Options.Location)
	events, err := s.Calendar.ListEvents(ctx, start, end)
	if err != nil {
		return DoingReviewResult{}, fmt.Errorf("list Google Calendar events for Doing review: %w", err)
	}
	cardEvents := map[string]scheduling.Event{}
	if finder, ok := s.Calendar.(CalendarEventFinder); ok {
		for _, card := range remaining {
			event, found, findErr := finder.FindCardEvent(ctx, s.BoardID, card.ID)
			if findErr != nil {
				return DoingReviewResult{}, fmt.Errorf("find Calendar event for card %s: %w", card.ID, findErr)
			}
			if found {
				cardEvents[card.ID] = event
			}
		}
	}
	// Existing events for the cards being rescheduled must not block their new
	// slots. Other Calendar events continue to reserve their dates and times.
	filteredEvents := make([]scheduling.Event, 0, len(events))
	for _, event := range events {
		remove := false
		for _, own := range cardEvents {
			if own.ID != "" && own.ID == event.ID {
				remove = true
				break
			}
		}
		if !remove {
			filteredEvents = append(filteredEvents, event)
		}
	}
	duplicates := map[string]bool{}
	plan, err := scheduling.BuildPlan(s.Now(), remaining, filteredEvents, duplicates, s.Options, s.OrdererOrDefault())
	if err != nil {
		return DoingReviewResult{}, err
	}
	result.Plan = plan
	assignments := map[string]scheduling.Assignment{}
	for _, assignment := range plan.Assignments {
		assignments[assignment.Card.ID] = assignment
	}
	for _, card := range cards {
		item := DoingReviewItem{Card: card, Completed: completed[card.ID]}
		if item.Completed {
			item.Action = "move-to-done"
			if done.ID == "" {
				item.Action = "failed"
				item.Error = fmt.Sprintf("target list %q not discovered", s.Policy.DoneListName)
			} else if dryRun {
				item.Action = "would-move-to-done"
			} else if err := s.Trello.MoveCard(card.ID, done.ID); err != nil {
				item.Action = "failed"
				item.Error = err.Error()
			}
			result.Items = append(result.Items, item)
			continue
		}
		assignment, hasAssignment := assignments[card.ID]
		if hasAssignment {
			item.Assignment = &assignment
		}
		if event, found := cardEvents[card.ID]; found {
			item.EventID = event.ID
		}
		switch {
		case !hasAssignment:
			item.Action = "unscheduled"
			item.Error = "no suitable free slot in next week"
		case dryRun:
			if item.EventID != "" {
				item.Action = "would-reschedule"
			} else {
				item.Action = "would-create-event"
			}
		default:
			if item.EventID != "" {
				rescheduler, ok := s.Calendar.(CalendarEventRescheduler)
				if !ok {
					item.Action = "failed"
					item.Error = "Calendar adapter does not support rescheduling"
				} else if err := rescheduler.RescheduleEvent(ctx, item.EventID, assignment.Start, assignment.End); err != nil {
					item.Action = "failed"
					item.Error = err.Error()
				} else {
					item.Action = "rescheduled"
				}
			} else {
				eventID, created, createErr := s.Calendar.CreateEvent(ctx, s.BoardID, assignment, s.Options.TitlePrefix, EventDescription(card), priorityColor(card.Priority, s.Options.PriorityColors))
				if createErr != nil {
					item.Action = "failed"
					item.Error = createErr.Error()
				} else {
					// PATCH: Preserve CreateEvent's verified created-versus-reconciled outcome.
					if created {
						item.Action = "created-event"
					} else {
						item.Action = "reconciled-event"
					}
					item.EventID = eventID
				}
			}
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

// PATCH: Build a capped refill plan so Doing never exceeds four open cards.
func (s *Service) PlanTopUp(ctx context.Context, capacity int, reserved []scheduling.Assignment) (PlanResult, error) {
	if capacity <= 0 {
		return PlanResult{ListName: strings.Join(s.Policy.SourceListNames, ", "), Plan: scheduling.Plan{Timezone: s.Options.Location.String()}}, nil
	}
	base, err := s.BoardCards(ctx)
	if err != nil {
		return PlanResult{}, err
	}
	duplicates := make(map[string]bool, len(base.Cards))
	for _, card := range base.Cards {
		duplicates[card.ID] = card.Scheduled
	}
	planningCards, decisions := scheduling.SelectEligible(base.Cards, s.Policy, duplicates)
	// PATCH: A source-list card with an existing deterministic event still
	// needs the move retry so it can occupy one of the four Doing slots.
	for _, card := range planningCards {
		if card.Scheduled && card.ListID != "" && s.DoingID != "" && card.ListID != s.DoingID {
			duplicates[card.ID] = false
		}
	}
	if len(planningCards) > capacity {
		planningCards = planningCards[:capacity]
	}
	start, end := scheduling.NextWeek(s.Now(), s.Options.Location)
	events, err := s.Calendar.ListEvents(ctx, start, end)
	if err != nil {
		return PlanResult{}, fmt.Errorf("list Google Calendar events: %w", err)
	}
	for _, assignment := range reserved {
		events = append(events, scheduling.Event{ID: "planned-" + assignment.Card.ID, Summary: assignment.Card.Name, Start: assignment.Start, End: assignment.End, Properties: map[string]string{"source": scheduling.Source}})
	}
	plan, err := scheduling.BuildPlan(s.Now(), planningCards, events, duplicates, s.Options, s.OrdererOrDefault())
	if err != nil {
		return PlanResult{}, err
	}
	base.Decisions, base.Plan = decisions, plan
	return base, nil
}

func (s *Service) Plan(ctx context.Context) (PlanResult, error) {
	var base PlanResult
	if strings.TrimSpace(s.ListID) == "" {
		var err error
		base, err = s.BoardCards(ctx)
		if err != nil {
			return PlanResult{}, err
		}
	} else {
		cards, listName, warnings, err := s.Cards(ctx)
		if err != nil {
			return PlanResult{}, err
		}
		base = PlanResult{ListName: listName, Warnings: warnings, Cards: cards}
	}
	duplicates := make(map[string]bool, len(base.Cards))
	for _, card := range base.Cards {
		duplicates[card.ID] = card.Scheduled
	}
	planningCards := base.Cards
	if strings.TrimSpace(s.ListID) == "" {
		var decisions []scheduling.CardDecision
		planningCards, decisions = scheduling.SelectEligible(base.Cards, s.Policy, duplicates)
		base.Decisions = decisions
		// PATCH: Keep already-scheduled source-list cards executable so reruns can retry Trello moves without creating duplicate Calendar events.
		for _, card := range planningCards {
			if card.Scheduled && card.ListID != "" && s.DoingID != "" && card.ListID != s.DoingID {
				duplicates[card.ID] = false
			}
		}
	}
	start, end := scheduling.NextWeek(s.Now(), s.Options.Location)
	events, err := s.Calendar.ListEvents(ctx, start, end)
	if err != nil {
		return PlanResult{}, fmt.Errorf("list Google Calendar events: %w", err)
	}
	colors, err := s.Calendar.ListEventColors(ctx)
	if err != nil {
		base.Warnings = append(base.Warnings, "Calendar color discovery unavailable; creating events without priority colors: "+err.Error())
	} else {
		s.Options.PriorityColors = availablePriorityColors(s.Options.PriorityColors, colors)
	}
	plan, err := scheduling.BuildPlan(s.Now(), planningCards, events, duplicates, s.Options, s.OrdererOrDefault())
	if err != nil {
		return PlanResult{}, err
	}
	base.Plan = plan
	return base, nil
}

func (s *Service) Execute(ctx context.Context, plan scheduling.Plan, dryRun, commentOnCard bool) ExecutionResult {
	result := ExecutionResult{DryRun: dryRun, Planned: len(plan.Assignments)}
	for _, assignment := range plan.Assignments {
		item := AssignmentResult{
			CardID: assignment.Card.ID, CardName: assignment.Card.Name, Date: assignment.Date,
			Start: assignment.Start.Format(time.RFC3339), End: assignment.End.Format(time.RFC3339),
		}
		if dryRun {
			item.Status = "dry-run"
			if s.DoingID != "" {
				item.MoveStatus = "would-move-to-" + s.Policy.DoingListName
			}
			result.Results = append(result.Results, item)
			continue
		}
		exists, err := s.Calendar.FindCard(ctx, s.BoardID, assignment.Card.ID)
		if err != nil {
			s.fail(&result, &item, fmt.Errorf("duplicate recheck: %w", err))
			continue
		}
		if exists {
			item.Status = "already-scheduled"
			result.Existing++
			s.moveAfterEvent(&result, &item, assignment.Card)
			result.Results = append(result.Results, item)
			continue
		}
		dayStart := time.Date(assignment.Start.Year(), assignment.Start.Month(), assignment.Start.Day(), 0, 0, 0, 0, s.Options.Location)
		events, err := s.Calendar.ListEvents(ctx, dayStart, dayStart.AddDate(0, 0, 1))
		if err != nil {
			s.fail(&result, &item, fmt.Errorf("day recheck: %w", err))
			continue
		}
		active := scheduling.EventsForDay(events, dayStart)
		switch {
		case scheduling.HasSourceEvent(active):
			err = fmt.Errorf("another Trello card is already scheduled on this day")
		case !scheduling.SlotAvailable(assignment.Start, assignment.End, active):
			err = fmt.Errorf("confirmed slot is no longer available")
		}
		if err != nil {
			s.fail(&result, &item, err)
			continue
		}
		description := EventDescription(assignment.Card)
		colorID := priorityColor(assignment.Card.Priority, s.Options.PriorityColors)
		eventID, created, err := s.Calendar.CreateEvent(ctx, s.BoardID, assignment, s.Options.TitlePrefix, description, colorID)
		if err != nil {
			s.fail(&result, &item, fmt.Errorf("create event: %w", err))
			continue
		}
		item.EventID = eventID
		if created {
			item.Status = "created"
			result.Created++
		} else {
			item.Status = "reconciled"
			result.Existing++
		}
		if commentOnCard {
			comment := fmt.Sprintf("Scheduled in Google Calendar for %s from %s to %s.", assignment.Date, assignment.Start.Format("15:04"), assignment.End.Format("15:04"))
			if err := s.Trello.AddComment(assignment.Card.ID, comment); err != nil {
				item.Status = "event-created-comment-failed"
				item.Error = err.Error()
				result.Failed++
			}
		}
		s.moveAfterEvent(&result, &item, assignment.Card)
		result.Results = append(result.Results, item)
	}
	return result
}

func (s *Service) moveAfterEvent(result *ExecutionResult, item *AssignmentResult, card scheduling.Card) {
	if s.DoingID == "" || card.ListID == "" || card.ListID == s.DoingID {
		return
	}
	if err := s.Trello.MoveCard(card.ID, s.DoingID); err != nil {
		item.MoveStatus = "failed"
		item.Error = strings.TrimSpace(strings.TrimSpace(item.Error) + "; move Trello card: " + err.Error())
		item.Status = "event-created-move-failed"
		result.Failed++
		return
	}
	item.MoveStatus = "moved"
	result.Moved++
}

func (s *Service) fail(result *ExecutionResult, item *AssignmentResult, err error) {
	item.Status = "failed"
	item.Error = err.Error()
	result.Failed++
	result.Results = append(result.Results, *item)
}

func (s *Service) OrdererOrDefault() scheduling.CardOrderer {
	if s.Orderer != nil {
		return s.Orderer
	}
	return scheduling.DueDateOrderer{}
}

func EventDescription(card scheduling.Card) string {
	var lines []string
	// PATCH: Preserve non-empty Trello descriptions and include only meaningful scheduling metadata.
	if strings.TrimSpace(card.Description) != "" {
		lines = append(lines, card.Description, "")
	}
	if card.URL != "" {
		lines = append(lines, "Card: "+card.URL)
	}
	if card.ID != "" {
		lines = append(lines, "Trello card ID: "+card.ID)
	}
	if card.Priority != "" {
		lines = append(lines, "Priority: "+card.Priority)
	}
	if card.EstimatedMinutes > 0 {
		lines = append(lines, fmt.Sprintf("Duration: %d minutes", card.EstimatedMinutes))
	}
	return strings.Join(lines, "\n")
}

func availablePriorityColors(configured map[string]string, available map[string]bool) map[string]string {
	result := map[string]string{}
	for priority, colorID := range configured {
		if available[colorID] {
			result[priority] = colorID
		}
	}
	return result
}

func priorityColor(priority string, colors map[string]string) string {
	return colors[schedulingPriority(priority)]
}

func schedulingPriority(priority string) string {
	switch strings.ToUpper(strings.TrimSpace(priority)) {
	case "CRITICAL", "P0", "P1":
		return "critical"
	case "HIGH", "P2":
		return "high"
	case "NORMAL", "P3":
		return "normal"
	case "LOW", "P4", "P5":
		return "low"
	default:
		return ""
	}
}
