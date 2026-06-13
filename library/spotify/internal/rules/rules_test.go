package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultRulesParseAndValidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.yaml")
	if err := os.WriteFile(path, DefaultYAML(), 0o600); err != nil {
		t.Fatal(err)
	}
	rf, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if errs := Validate(rf); len(errs) != 0 {
		t.Fatalf("expected valid default rules, got %v", errs)
	}
	if got := rf.Fallback.Name; got != "Needs Review" {
		t.Fatalf("fallback = %q, want Needs Review", got)
	}
}

func TestNormalizeGenre(t *testing.T) {
	tests := map[string]string{
		" Ambient-Techno!! ": "ambient techno",
		"Hip Hop":            "hip hop",
		"rock/alternative":   "rock alternative",
	}
	for input, want := range tests {
		if got := NormalizeGenre(input); got != want {
			t.Fatalf("NormalizeGenre(%q) = %q, want %q", input, got, want)
		}
	}
}
