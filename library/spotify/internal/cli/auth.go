package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"spotify-pp-cli/internal/auth"
	"spotify-pp-cli/internal/config"
)

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage Spotify OAuth login"}
	cmd.AddCommand(newAuthLoginCmd(flags))
	cmd.AddCommand(newAuthStatusCmd(flags))
	cmd.AddCommand(newAuthLogoutCmd(flags))
	return cmd
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var noBrowser bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authorize with Spotify using Authorization Code with PKCE",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			store, err := auth.NewDefaultTokenStore()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			return auth.Login(ctx, cfg, store, auth.LoginOptions{NoBrowser: noBrowser, Out: os.Stdout})
		},
	}
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the auth URL without opening a browser")
	cmd.Flags().BoolVar(&noBrowser, "manual", false, "Alias for --no-browser")
	return cmd
}

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether a Spotify token is stored and still valid",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := auth.NewDefaultTokenStore()
			if err != nil {
				return err
			}
			token, err := store.Load()
			status := map[string]any{
				"logged_in": false,
				"store":     store.Name(),
			}
			if err == nil {
				status["logged_in"] = token.AccessToken != ""
				status["expires_at"] = token.ExpiresAt
				status["valid_now"] = token.Valid()
				status["scope"] = token.Scope
			}
			return outputValue(flags, status, func() error {
				if err != nil {
					fmt.Printf("Not logged in (%v)\n", err)
					return nil
				}
				fmt.Printf("Logged in: %v\n", token.AccessToken != "")
				fmt.Printf("Token store: %s\n", store.Name())
				fmt.Printf("Expires at: %s\n", token.ExpiresAt.Format(time.RFC3339))
				fmt.Printf("Valid now: %v\n", token.Valid())
				fmt.Printf("Scopes: %s\n", token.Scope)
				return nil
			})
		},
	}
}

func newAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Delete the stored Spotify token",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := auth.NewDefaultTokenStore()
			if err != nil {
				return err
			}
			if err := store.Delete(); err != nil {
				return err
			}
			return outputValue(flags, map[string]any{"logged_out": true}, func() error {
				fmt.Println("Logged out. Stored token removed.")
				return nil
			})
		},
	}
}
