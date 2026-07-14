package scheduling

import "testing"

func TestSelectEligibleBoardAwarePolicy(t *testing.T) {
	policy := SelectionPolicy{
		SourceListNames:  []string{"Peter", "Peter & Liliia"},
		ExcludeListNames: []string{"Doing", "Done"},
		DoingListName:    "Doing",
		PeterMemberID:    "peter",
	}
	cards := []Card{
		{ID: "peter", Name: "Peter task", ListName: "Peter", Labels: readyLabels()},
		{ID: "shared-peter", Name: "Shared assigned", ListName: "Peter & Liliia", MemberIDs: []string{"peter"}, Labels: readyLabels()},
		{ID: "shared-liliia", Name: "Shared unassigned", ListName: "Peter & Liliia", Labels: readyLabels()},
		{ID: "doing", Name: "Doing", ListName: "Doing"},
		{ID: "manual", Name: "Manual", ListName: "Peter", Labels: []Label{{Name: "P1"}, {Name: "T60"}}},
		{ID: "blocked", Name: "Blocked", ListName: "Peter", Labels: append(readyLabels(), Label{Name: "BLOCKED"})},
		{ID: "duplicate", Name: "Duplicate", ListName: "Peter", Labels: readyLabels()},
	}
	eligible, decisions := SelectEligible(cards, policy, map[string]bool{"duplicate": true})
	if len(eligible) != 3 || eligible[0].ID != "peter" || eligible[1].ID != "shared-peter" || eligible[2].ID != "duplicate" {
		t.Fatalf("eligible=%#v", eligible)
	}
	wantReasons := map[string]string{
		"shared-liliia": "shared list card is not assigned to Peter",
		"doing":         "excluded list",
		"manual":        "missing AUTO label",
		"blocked":       "blocked by BLOCKED label",
		"duplicate":     "already scheduled; retry move",
	}
	for _, decision := range decisions {
		if want, ok := wantReasons[decision.CardID]; ok && decision.Reason != want {
			t.Fatalf("%s reason=%q want %q", decision.CardID, decision.Reason, want)
		}
	}
}

func TestSelectEligibleCanAllowSharedLiliiaCards(t *testing.T) {
	policy := SelectionPolicy{SourceListNames: []string{"Peter & Liliia"}, AllowLiliiaCards: true}
	eligible, _ := SelectEligible([]Card{{ID: "shared", Name: "Shared", ListName: "Peter & Liliia", Labels: readyLabels()}}, policy, nil)
	if len(eligible) != 1 || eligible[0].ID != "shared" {
		t.Fatalf("eligible=%#v", eligible)
	}
}

func TestSelectEligibleRequiresPlannerLabels(t *testing.T) {
	policy := SelectionPolicy{SourceListNames: []string{"Peter"}}
	cards := []Card{
		{ID: "missing-priority", Name: "Missing priority", ListName: "Peter", Labels: []Label{{Name: "T60"}, {Name: "AUTO"}}},
		{ID: "multiple-priority", Name: "Multiple priority", ListName: "Peter", Labels: []Label{{Name: "P1"}, {Name: "P2"}, {Name: "T60"}, {Name: "AUTO"}}},
		{ID: "missing-duration", Name: "Missing duration", ListName: "Peter", Labels: []Label{{Name: "P1"}, {Name: "AUTO"}}},
		{ID: "multiple-duration", Name: "Multiple duration", ListName: "Peter", Labels: []Label{{Name: "P1"}, {Name: "T30"}, {Name: "T60"}, {Name: "AUTO"}}},
		{ID: "missing-auto", Name: "Missing auto", ListName: "Peter", Labels: []Label{{Name: "P1"}, {Name: "T60"}}},
		{ID: "waiting", Name: "Waiting", ListName: "Peter", Labels: append(readyLabels(), Label{Name: "WAITING"})},
		{ID: "needs-decision", Name: "Needs decision", ListName: "Peter", Labels: append(readyLabels(), Label{Name: "NEEDS-DECISION"})},
		{ID: "multi-day", Name: "Multi day", ListName: "Peter", Labels: []Label{{Name: "P1"}, {Name: "TMULTI Multi-day"}, {Name: "AUTO"}}},
	}
	eligible, decisions := SelectEligible(cards, policy, nil)
	if len(eligible) != 0 {
		t.Fatalf("eligible=%#v", eligible)
	}
	want := map[string]string{
		"missing-priority":  "missing priority label",
		"multiple-priority": "multiple priority labels",
		"missing-duration":  "missing duration label",
		"multiple-duration": "multiple duration labels",
		"missing-auto":      "missing AUTO label",
		"waiting":           "blocked by WAITING label",
		"needs-decision":    "blocked by NEEDS-DECISION label",
		"multi-day":         "multi-day card",
	}
	for _, decision := range decisions {
		if decision.Reason != want[decision.CardID] {
			t.Fatalf("%s reason=%q want %q", decision.CardID, decision.Reason, want[decision.CardID])
		}
	}
}

func TestSelectEligibleIgnoresLabelColors(t *testing.T) {
	policy := SelectionPolicy{SourceListNames: []string{"Peter"}}
	labels := []Label{{Name: "P2 Medium", Color: "green"}, {Name: "T90 90m", Color: "red"}, {Name: "AUTO", Color: "blue"}}
	eligible, _ := SelectEligible([]Card{{ID: "card", Name: "Card", ListName: "Peter", Labels: labels}}, policy, nil)
	if len(eligible) != 1 || eligible[0].EstimatedMinutes != 90 {
		t.Fatalf("eligible=%#v", eligible)
	}
}

func readyLabels() []Label {
	return []Label{{Name: "P1 High", Color: "red"}, {Name: "T60 1h", Color: "blue"}, {Name: "AUTO", Color: "green"}}
}
