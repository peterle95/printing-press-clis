// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"strings"
	"trello-calendar-pp-cli/internal/config"
	"trello-calendar-pp-cli/internal/googlecalendar"

	"github.com/spf13/cobra"
)

// PATCH: Trello credentials are environment-only; add the Google OAuth flow required by the planner.
func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage Trello and Google authentication"}
	cmd.AddCommand(newGoogleAuthCmd(flags), newAuthStatusCmd(flags), newAuthLogoutCmd(flags))
	return cmd
}

func newGoogleAuthCmd(flags *rootFlags) *cobra.Command {
	var noBrowser bool
	cmd := &cobra.Command{
		Use:         "google",
		Short:       "Complete the Google Calendar OAuth login",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			store := googlecalendar.TokenStore{Path: cfg.TokenPath()}
			if err := googlecalendar.Login(cmd.Context(), cfg, store, noBrowser, cmd.OutOrStdout()); err != nil {
				return authErr(err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the authorization URL without launching a browser")
	return cmd
}

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Trello and Google authentication status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			status := map[string]any{
				"trello": map[string]any{"configured": cfg.AuthHeader() != "", "source": cfg.AuthSource},
				"google": map[string]any{"configured": false, "token_path": cfg.TokenPath()},
			}
			store := googlecalendar.TokenStore{Path: cfg.TokenPath()}
			if token, err := store.Load(); err == nil {
				status["google"] = map[string]any{"configured": true, "has_refresh_token": token.RefreshToken != "", "token_path": cfg.TokenPath()}
			}
			if flags.asJSON {
				return flags.printJSON(cmd, status)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Trello: %s\n", configuredText(cfg.AuthHeader() != ""))
			googleStatus := status["google"].(map[string]any)
			fmt.Fprintf(cmd.OutOrStdout(), "Google Calendar: %s\n", configuredText(googleStatus["configured"].(bool)))
			fmt.Fprintf(cmd.OutOrStdout(), "Token file: %s\n", cfg.TokenPath())
			return nil
		},
	}
}

func newAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the locally stored Google OAuth token",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if err := os.Remove(cfg.TokenPath()); err != nil && !os.IsNotExist(err) {
				return configErr(err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Google OAuth token removed. Trello environment variables were not changed.")
			return nil
		},
	}
}

func configuredText(value bool) string {
	if value {
		return "configured"
	}
	return "not configured"
}

var _ = strings.TrimSpace
