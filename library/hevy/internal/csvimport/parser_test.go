package csvimport

import (
	"strings"
	"testing"
)

func TestHeaderKeyAndParse(t *testing.T) {
	if HeaderKey(" Workout Title ") != "workouttitle" {
		t.Fatal("header normalization failed")
	}
	input := "Workout Title,Workout Start,Exercise Title,Set Number,Weight kg,Reps\nPush,2026-01-02 10:00:00,Bench Press,1,80,8\n"
	result, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sets) != 1 || *result.Sets[0].WeightKG != 80 {
		t.Fatalf("unexpected sets: %#v", result.Sets)
	}
}
func TestParseTimeAndFingerprint(t *testing.T) {
	if _, err := ParseTime("2026-01-02 10:00:00"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseTime("Jul 18, 2026, 3:53 PM"); err != nil {
		t.Fatal(err)
	}
	if Fingerprint("Bench Press") != Fingerprint(" bench press ") {
		t.Fatal("fingerprints should normalize")
	}
}
