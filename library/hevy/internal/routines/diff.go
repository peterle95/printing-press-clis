// Package routines calculates typed, conservative changes between plans.
package routines

import (
	"fmt"
	"hevy-pp-cli/internal/plans"
	"strings"
)

type ChangeType string

const (
	ChangeCreate ChangeType = "create"
	ChangeUpdate ChangeType = "update"
	ChangeDelete ChangeType = "delete"
)

type RoutineChange struct {
	Type        ChangeType `json:"type"`
	RoutineName string     `json:"routine_name"`
	Field       string     `json:"field"`
	Before      any        `json:"before,omitempty"`
	After       any        `json:"after,omitempty"`
}

func Diff(local plans.Plan, remote []plans.Routine) []RoutineChange {
	changes := []RoutineChange{}
	r := map[string]plans.Routine{}
	for _, x := range remote {
		r[strings.ToLower(x.Name)] = x
	}
	l := map[string]bool{}
	for _, want := range local.Routines {
		k := strings.ToLower(want.Name)
		l[k] = true
		got, ok := r[k]
		if !ok {
			changes = append(changes, RoutineChange{ChangeCreate, want.Name, "routine", nil, want})
			continue
		}
		if len(got.Exercises) != len(want.Exercises) {
			changes = append(changes, RoutineChange{ChangeUpdate, want.Name, "exercise_count", len(got.Exercises), len(want.Exercises)})
			continue
		}
		for i, e := range want.Exercises {
			if !strings.EqualFold(e.Name, got.Exercises[i].Name) {
				changes = append(changes, RoutineChange{ChangeUpdate, want.Name, fmt.Sprintf("exercise[%d]", i), got.Exercises[i].Name, e.Name})
			}
			if len(e.Sets) != len(got.Exercises[i].Sets) {
				changes = append(changes, RoutineChange{ChangeUpdate, want.Name, e.Name + ".sets", len(got.Exercises[i].Sets), len(e.Sets)})
			}
			if !sameInt(e.RestSeconds, got.Exercises[i].RestSeconds) {
				changes = append(changes, RoutineChange{ChangeUpdate, want.Name, e.Name + ".rest_seconds", got.Exercises[i].RestSeconds, e.RestSeconds})
			}
		}
	}
	for _, got := range remote {
		if !l[strings.ToLower(got.Name)] {
			changes = append(changes, RoutineChange{ChangeDelete, got.Name, "routine", got, nil})
		}
	}
	return changes
}
func sameInt(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
