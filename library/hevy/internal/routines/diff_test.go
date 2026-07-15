package routines

import (
	"hevy-pp-cli/internal/plans"
	"testing"
)

func TestDiff(t *testing.T) {
	p := plans.Plan{Version: 1, Name: "P", Routines: []plans.Routine{{Name: "Push", Exercises: []plans.Exercise{{Name: "Bench"}}}}}
	changes := Diff(p, []plans.Routine{{Name: "Push", Exercises: []plans.Exercise{{Name: "Press"}}}, {Name: "Old"}})
	if len(changes) != 2 {
		t.Fatalf("got %#v", changes)
	}
}
