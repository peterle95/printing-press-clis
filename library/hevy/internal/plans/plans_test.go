package plans

import (
	"strings"
	"testing"
)

func TestDecodeYAMLAndValidation(t *testing.T) {
	p, err := Decode([]byte("version: 1\nname: Push\nroutines:\n  - name: Main\n    exercises:\n      - name: Bench\n        sets:\n          - type: normal\n            target_reps: {min: 6, max: 8}\n            target_weight_kg: 80\n"), "plan.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Push" || len(p.Routines) != 1 {
		t.Fatalf("unexpected plan: %#v", p)
	}
}
func TestValidationFieldError(t *testing.T) {
	err := Validate(Plan{Version: 1, Name: "x", Routines: []Routine{{Name: "x", Exercises: []Exercise{{Name: "e", Sets: []Set{{TargetReps: &Reps{Min: 9, Max: 8}}}}}}}})
	if err == nil || !strings.Contains(err.Error(), "target_reps.min") {
		t.Fatalf("unexpected error: %v", err)
	}
}
