// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"strings"
	"time"
	"trello-calendar-pp-cli/internal/config"
	"trello-calendar-pp-cli/internal/googlecalendar"
	"trello-calendar-pp-cli/internal/trello"

	"github.com/spf13/cobra"
)

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// PATCH: Replace cookie-oriented generated checks with dual-API planner diagnostics.
func newDoctorCmd(flags *rootFlags) *cobra.Command {
	var failOn string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check configuration, Trello, Google Calendar, timezone, and token security",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			checks := runDoctorChecks(cmd, flags)
			if flags.asJSON {
				if err := flags.printJSON(cmd, map[string]any{"checks": checks}); err != nil {
					return err
				}
			} else {
				for _, check := range checks {
					fmt.Fprintf(cmd.OutOrStdout(), "%-5s %-24s %s\n", strings.ToUpper(check.Status), check.Name, check.Detail)
				}
			}
			return doctorFailure(failOn, checks)
		},
	}
	cmd.Flags().StringVar(&failOn, "fail-on", "", "Exit nonzero at this level: warn or error")
	return cmd
}

func runDoctorChecks(cmd *cobra.Command, flags *rootFlags) []doctorCheck {
	checks := make([]doctorCheck, 0, 10)
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return append(checks, doctorCheck{"configuration", "error", err.Error()})
	}
	checks = append(checks, doctorCheck{"configuration", "ok", cfg.Path})
	if err := cfg.ValidatePlanner(); err != nil {
		checks = append(checks, doctorCheck{"planner IDs", "error", err.Error()})
	} else {
		checks = append(checks, doctorCheck{"planner IDs", "ok", "board, list, and calendar resolved"})
	}
	if _, err := time.LoadLocation(cfg.Timezone); err != nil {
		checks = append(checks, doctorCheck{"timezone", "error", err.Error()})
	} else {
		checks = append(checks, doctorCheck{"timezone", "ok", cfg.Timezone})
	}
	missingTrello := missingEnv("TRELLO_API_KEY", "TRELLO_TOKEN")
	if len(missingTrello) > 0 {
		checks = append(checks, doctorCheck{"Trello authentication", "error", "missing: " + strings.Join(missingTrello, ", ")})
	} else {
		checks = append(checks, doctorCheck{"Trello authentication", "ok", "environment credentials present"})
		if cfg.TrelloBoardID != "" && cfg.TrelloListID != "" {
			client, clientErr := flags.newClient()
			if clientErr != nil {
				checks = append(checks, doctorCheck{"Trello access", "error", clientErr.Error()})
			} else {
				client.DryRun = false
				client.NoCache = true
				name, accessErr := trello.New(client).ValidateList(cfg.TrelloBoardID, cfg.TrelloListID)
				if accessErr != nil {
					checks = append(checks, doctorCheck{"Trello access", "error", accessErr.Error()})
				} else if name != "Doing" {
					checks = append(checks, doctorCheck{"Trello access", "warn", fmt.Sprintf("list accessible but named %q", name)})
				} else {
					checks = append(checks, doctorCheck{"Trello access", "ok", "board and Doing list accessible"})
				}
			}
		}
	}
	missingGoogle := missingEnv("GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URI")
	if len(missingGoogle) > 0 {
		checks = append(checks, doctorCheck{"Google OAuth configuration", "error", "missing: " + strings.Join(missingGoogle, ", ")})
	} else {
		checks = append(checks, doctorCheck{"Google OAuth configuration", "ok", "client environment variables present"})
	}
	store := googlecalendar.TokenStore{Path: cfg.TokenPath()}
	if err := store.PermissionStatus(); err != nil {
		checks = append(checks, doctorCheck{"token permissions", "error", err.Error()})
	} else {
		checks = append(checks, doctorCheck{"token permissions", "ok", "token file 0600 and directory 0700"})
	}
	if len(missingGoogle) == 0 {
		httpClient, authErr := googlecalendar.NewHTTPClient(cmd.Context(), cfg, store, false)
		if authErr != nil {
			checks = append(checks, doctorCheck{"Google authentication", "error", authErr.Error()})
		} else {
			checks = append(checks, doctorCheck{"Google authentication", "ok", "token loaded and refresh source initialized"})
			location, _ := time.LoadLocation(cfg.Timezone)
			calendar := googlecalendar.NewClient(cfg.GoogleBaseURL, cfg.GoogleCalendarID, location, httpClient)
			if err := calendar.CheckAccess(cmd.Context()); err != nil {
				checks = append(checks, doctorCheck{"Google Calendar access", "error", err.Error()})
			} else {
				checks = append(checks, doctorCheck{"Google Calendar access", "ok", "calendar is writable"})
			}
		}
	}
	return checks
}

func missingEnv(names ...string) []string {
	var missing []string
	for _, name := range names {
		if strings.TrimSpace(os.Getenv(name)) == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

func doctorFailure(level string, checks []doctorCheck) error {
	if level == "" {
		return nil
	}
	if level != "warn" && level != "error" {
		return usageErr(fmt.Errorf("--fail-on must be warn or error"))
	}
	for _, check := range checks {
		if check.Status == "error" || (level == "warn" && check.Status == "warn") {
			return fmt.Errorf("doctor found %s checks", check.Status)
		}
	}
	return nil
}
