package browser

import "testing"

func TestParseRoutineText(t *testing.T) {
	routine, err := parseRoutineText("Chest", `Feedback
Routines
Chest
Sypine Press
3 sets · 10 reps
Treadmill
1 set
Created by
peterle1995`)
	if err != nil {
		t.Fatal(err)
	}
	if routine.Name != "Chest" || len(routine.Exercises) != 2 {
		t.Fatalf("got %#v", routine)
	}
	if len(routine.Exercises[0].Sets) != 3 || routine.Exercises[0].Sets[0].TargetReps.Min != 10 {
		t.Fatalf("got %#v", routine.Exercises[0])
	}
	if len(routine.Exercises[1].Sets) != 1 || routine.Exercises[1].Sets[0].TargetReps != nil {
		t.Fatalf("got %#v", routine.Exercises[1])
	}
}

func TestParseRoutineTextRequiresExercises(t *testing.T) {
	if _, err := parseRoutineText("Empty", "Empty\nCreated by\nowner"); err == nil {
		t.Fatal("expected missing exercise error")
	}
}
