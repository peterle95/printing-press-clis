// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package scheduling

import "sort"

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
		if a.Position != b.Position {
			return a.Position < b.Position
		}
		return a.ID < b.ID
	})
	return ordered
}
