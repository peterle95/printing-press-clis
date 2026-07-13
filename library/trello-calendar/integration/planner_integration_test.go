//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"
	"trello-calendar-pp-cli/internal/client"
	"trello-calendar-pp-cli/internal/config"
	"trello-calendar-pp-cli/internal/googlecalendar"
	"trello-calendar-pp-cli/internal/scheduling"
	"trello-calendar-pp-cli/internal/trello"
)

// TestReadOnlyCredentials performs only live GET requests. Enable it manually
// with -tags=integration and TRELLO_CALENDAR_INTEGRATION_READ=1.
func TestReadOnlyCredentials(t *testing.T) {
	if os.Getenv("TRELLO_CALENDAR_INTEGRATION_READ") != "1" {
		t.Skip("set TRELLO_CALENDAR_INTEGRATION_READ=1 to enable live read checks")
	}
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidatePlanner(); err != nil {
		t.Fatal(err)
	}
	trelloClient := client.New(cfg, 30*time.Second, 0)
	trelloClient.NoCache = true
	if _, err := trello.New(trelloClient).ValidateList(cfg.TrelloBoardID, cfg.TrelloListID); err != nil {
		t.Fatal(err)
	}
	httpClient, err := googlecalendar.NewHTTPClient(context.Background(), cfg, googlecalendar.TokenStore{Path: cfg.TokenPath()}, false)
	if err != nil {
		t.Fatal(err)
	}
	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		t.Fatal(err)
	}
	calendar := googlecalendar.NewClient(cfg.GoogleBaseURL, cfg.GoogleCalendarID, location, httpClient)
	start, end := scheduling.NextWeek(time.Now(), location)
	if _, err := calendar.ListEvents(context.Background(), start, end); err != nil {
		t.Fatal(err)
	}
}
