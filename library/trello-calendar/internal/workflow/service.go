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
	if strings.TrimSpace(card.Description) != "" { lines = append(lines, card.Description, "") }
	if card.URL != "" { lines = append(lines, "Card: "+card.URL) }
	if card.ID != "" { lines = append(lines, "Trello card ID: "+card.ID) }
	if card.Priority != "" { lines = append(lines, "Priority: "+card.Priority) }
	if card.EstimatedMinutes > 0 { lines = append(lines, fmt.Sprintf("Duration: %d minutes", card.EstimatedMinutes)) }
	return strings.Join(lines, "\n")
}

func availablePriorityColors(configured map[string]string, available map[string]bool) map[string]string {
	result := map[string]string{}
	for priority, colorID := range configured { if available[colorID] { result[priority] = colorID } }
	return result
}

func priorityColor(priority string, colors map[string]string) string { return colors[schedulingPriority(priority)] }

func schedulingPriority(priority string) string {
	switch strings.ToUpper(strings.TrimSpace(priority)) {
	case "CRITICAL", "P0", "P1": return "critical"
	case "HIGH", "P2": return "high"
	case "NORMAL", "P3": return "normal"
	case "LOW", "P4", "P5": return "low"
	default: return ""
	}
}
