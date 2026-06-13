package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"mail-pp-cli/internal/accounts"
	"mail-pp-cli/internal/providers/gmail"
)

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage provider authentication"}
	login := &cobra.Command{Use: "login", Short: "Run provider login flows"}
	login.AddCommand(newAuthLoginGmailCmd(flags))
	cmd.AddCommand(login)
	cmd.AddCommand(newAuthStatusCmd(flags))
	return cmd
}

func newAuthLoginGmailCmd(flags *rootFlags) *cobra.Command {
	var accountRef string
	var scopesText string
	var port int
	var noBrowser bool
	cmd := &cobra.Command{
		Use:   "gmail",
		Short: "Authenticate one Gmail account with the official Gmail API",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			account, err := app.config.Resolve(accountRef)
			if err != nil {
				return err
			}
			if accounts.NormalizeProvider(account.Provider) != accounts.ProviderGmail {
				return fmt.Errorf("account %s is not a Gmail account", account.Name)
			}
			scopes := gmail.DefaultScopes
			if scopesText != "" {
				scopes = splitScopes(scopesText)
			}
			ctx, cancel := commandContext(flags)
			defer cancel()
			return gmail.Login(ctx, account, gmail.LoginOptions{
				CredentialsPath: flags.credentialsPath,
				Scopes:          scopes,
				Port:            port,
				NoBrowser:       noBrowser,
				Out:             cmd.OutOrStdout(),
			})
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Gmail account name or address")
	cmd.Flags().StringVar(&scopesText, "scopes", "", "Comma-separated Gmail OAuth scopes; defaults to readonly, compose, send, modify")
	cmd.Flags().IntVar(&port, "port", 0, "Local OAuth callback port; 0 chooses a free port")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print OAuth URL without opening a browser")
	_ = cmd.MarkFlagRequired("account")
	return cmd
}

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	var accountRef string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show Gmail token status for one or all accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			var accountsToCheck []accounts.Account
			if accountRef != "" {
				account, err := app.config.Resolve(accountRef)
				if err != nil {
					return err
				}
				accountsToCheck = append(accountsToCheck, account)
			} else {
				for _, account := range app.config.List() {
					if accounts.NormalizeProvider(account.Provider) == accounts.ProviderGmail {
						accountsToCheck = append(accountsToCheck, account)
					}
				}
			}
			var statuses []map[string]any
			for _, account := range accountsToCheck {
				statuses = append(statuses, gmailStatus(account))
			}
			return outputValue(flags, map[string]any{"accounts": statuses}, func() error {
				for _, status := range statuses {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\tlogged_in=%v\ttoken=%s\n",
						status["account"], status["address"], status["logged_in"], status["token_path"])
					if errText, ok := status["error"].(string); ok && errText != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "  error: %s\n", errText)
					}
					if scopes, ok := status["scopes"].([]string); ok && len(scopes) > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "  scopes: %v\n", scopes)
					}
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Account name or address")
	return cmd
}

func gmailStatus(account accounts.Account) map[string]any {
	path, err := account.TokenPath()
	status := map[string]any{
		"account":   account.Name,
		"address":   account.Address,
		"provider":  "gmail",
		"logged_in": false,
	}
	if err != nil {
		status["error"] = err.Error()
		return status
	}
	status["token_path"] = path
	if _, err := os.Stat(path); err != nil {
		status["error"] = err.Error()
		return status
	}
	token, err := gmail.LoadStoredToken(path)
	if err != nil {
		status["error"] = err.Error()
		return status
	}
	status["logged_in"] = token.AccessToken != ""
	status["has_refresh_token"] = token.RefreshToken != ""
	status["scopes"] = token.Scopes
	if !token.Expiry.IsZero() {
		status["expires_at"] = token.Expiry.Format(time.RFC3339)
		status["valid_now"] = time.Now().Before(token.Expiry.Add(-30 * time.Second))
	}
	return status
}
