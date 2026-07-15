// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package scheduling

import "strings"

type SelectionPolicy struct {
	SourceListNames  []string
	ExcludeListNames []string
	DoingListName    string
	PeterMemberID    string
	AllowLiliiaCards bool
}

// PATCH: Keep board-aware weekly planning eligibility in pure code for safe tests.
func SelectEligible(cards []Card, policy SelectionPolicy, duplicates map[string]bool) ([]Card, []CardDecision) {
	sources := nameSet(policy.SourceListNames)
	excluded := nameSet(policy.ExcludeListNames)
	eligible := make([]Card, 0, len(cards))
	decisions := make([]CardDecision, 0, len(cards))
	for _, card := range cards {
		decision := CardDecision{CardID: card.ID, Name: card.Name, List: card.ListName, Action: "skip"}
		labelValidation := validateLabels(&card)
		switch {
		case card.Closed:
			decision.Reason = "card closed"
		case excluded[strings.ToLower(strings.TrimSpace(card.ListName))]:
			decision.Reason = "excluded list"
		case len(sources) > 0 && !sources[strings.ToLower(strings.TrimSpace(card.ListName))]:
			decision.Reason = "not in source list"
		case isSharedLiliiaList(card.ListName) && !policy.AllowLiliiaCards && !assignedToPeter(card, policy):
			decision.Reason = "shared list card is not assigned to Peter"
		case labelValidation != "":
			decision.Reason = labelValidation
		case duplicates[card.ID] || card.Scheduled:
			decision.Action = "select"
			decision.Reason = "already scheduled; retry move"
			eligible = append(eligible, card)
		default:
			decision.Action = "select"
			decision.Reason = "eligible"
			eligible = append(eligible, card)
		}
		decisions = append(decisions, decision)
	}
	return eligible, decisions
}

func nameSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		if trimmed := strings.ToLower(strings.TrimSpace(value)); trimmed != "" {
			result[trimmed] = true
		}
	}
	return result
}

func isSharedLiliiaList(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "liliia")
}

func assignedToPeter(card Card, policy SelectionPolicy) bool {
	if strings.TrimSpace(policy.PeterMemberID) == "" {
		return false
	}
	for _, id := range card.MemberIDs {
		if id == policy.PeterMemberID {
			return true
		}
	}
	return false
}

// PATCH: Planner labels are matched by name only. Trello label colors are visual decoration.
func validateLabels(card *Card) string {
	priority := 0
	duration := 0
	durationMinutes := 0
	for _, label := range card.Labels {
		name := labelName(label.Name)
		switch {
		case isPriorityLabel(name):
			priority++
			if card.Priority == "" {
				card.Priority = strings.TrimSpace(label.Name)
			}
		case name == "AUTO":
			card.Automation = "AUTO"
		case name == "BLOCKED" || name == "WAITING" || name == "NEEDS-DECISION":
			card.Status = name
		case strings.HasPrefix(name, "T"):
			duration++
			if name == "TMULTI" {
				card.EstimatedMinutes = 0
			} else if minutes := parseDurationLabel(name); minutes > 0 {
				durationMinutes = minutes
			}
		}
	}
	if priority == 0 {
		return "missing priority label"
	}
	if priority > 1 {
		return "multiple priority labels"
	}
	if duration == 0 {
		return "missing duration label"
	}
	if duration > 1 {
		return "multiple duration labels"
	}
	if hasLabelName(card.Labels, "TMULTI") {
		return "multi-day card"
	}
	if durationMinutes == 0 {
		return "duration label is invalid"
	}
	if !hasLabelName(card.Labels, "AUTO") {
		return "missing AUTO label"
	}
	for _, blocked := range []string{"BLOCKED", "WAITING", "NEEDS-DECISION"} {
		if hasLabelName(card.Labels, blocked) {
			return "blocked by " + blocked + " label"
		}
	}
	card.EstimatedMinutes = durationMinutes
	card.Status = "READY"
	return ""
}

func hasLabelName(labels []Label, want string) bool {
	want = labelName(want)
	for _, label := range labels {
		if labelName(label.Name) == want {
			return true
		}
	}
	return false
}

func labelName(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if fields := strings.Fields(value); len(fields) > 0 {
		return fields[0]
	}
	return value
}

func isPriorityLabel(name string) bool {
	if len(name) < 2 || name[0] != 'P' {
		return false
	}
	for _, r := range name[1:] {
		return r >= '0' && r <= '9'
	}
	return false
}

func parseDurationLabel(name string) int {
	if len(name) < 2 || name[0] != 'T' {
		return 0
	}
	var n int
	for _, r := range name[1:] {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
