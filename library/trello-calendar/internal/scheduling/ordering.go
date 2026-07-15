// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package scheduling

import (
	"sort"
	"strings"
	"time"
)

type CardOrderer interface {
	Order([]Card) []Card
}

type DueDateOrderer struct{}

func (DueDateOrderer) Order(cards []Card) []Card {
	ordered := append([]Card(nil), cards...)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		switch {
		case a.Due != nil && b.Due != nil:
			if !a.Due.Equal(*b.Due) {
				return a.Due.Before(*b.Due)
			}
		case a.Due != nil:
			return true
		case b.Due != nil:
			return false
		}
		// PATCH: Keep due dates and priority ahead of age, then favour ready cards that have waited longest.
		if priorityRank(a.Priority) != priorityRank(b.Priority) {
			return priorityRank(a.Priority) < priorityRank(b.Priority)
		}
		if ageScore(a.CreatedAt) != ageScore(b.CreatedAt) {
			return ageScore(a.CreatedAt) > ageScore(b.CreatedAt)
		}
		if a.Position != b.Position {
			return a.Position < b.Position
		}
		return a.ID < b.ID
	})
	return ordered
}

func priorityRank(value string) int {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "CRITICAL", "P0", "P1": return 0
	case "HIGH", "P2": return 1
	case "NORMAL", "P3": return 2
	case "LOW", "P4", "P5": return 3
	default: return 4
	}
}

func ageScore(createdAt *time.Time) int {
	if createdAt == nil { return 0 }
	age := time.Since(*createdAt)
	switch {
	case age >= 180*24*time.Hour: return 15
	case age >= 90*24*time.Hour: return 10
	case age >= 30*24*time.Hour: return 5
	default: return 0
	}
}
