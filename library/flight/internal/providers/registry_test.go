package providers

import (
	"testing"
	"time"

	"flight-pp-cli/internal/config"
	"flight-pp-cli/internal/flight"
)

func TestBuildKnownProviders(t *testing.T) {
	cfg := config.Default()
	list, errs := Build(cfg, t.TempDir(), time.Second, []string{"google", "amadeus"})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(list))
	}
	for _, provider := range list {
		var _ flight.Provider = provider
	}
}
