package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"mail-pp-cli/internal/accounts"
)

func newAccountsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "accounts", Short: "Manage unified mail account config"}
	cmd.AddCommand(newAccountsListCmd(flags))
	cmd.AddCommand(newAccountsInitCmd(flags))
	return cmd
}

func newAccountsListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured mail accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			payload := map[string]any{
				"config_path": app.config.Path,
				"accounts":    app.config.List(),
			}
			return outputValue(flags, payload, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "Config: %s\n", app.config.Path)
				for _, account := range app.config.List() {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", account.Name, account.Address, accounts.NormalizeProvider(account.Provider))
				}
				return nil
			})
		},
	}
}

func newAccountsInitCmd(flags *rootFlags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write the default unified account config",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := accounts.ResolvePath(flags.configPath)
			if err != nil {
				return err
			}
			if !force {
				if _, err := accounts.Load(path); err == nil {
					// Load returns defaults when the file is absent, so stat to avoid
					// overwriting a real file.
					if exists(path) {
						return fmt.Errorf("account config already exists at %s; pass --force to overwrite", path)
					}
				}
			}
			cfg := accounts.DefaultConfig()
			cfg.Path = path
			if err := cfg.Save(path); err != nil {
				return err
			}
			return outputValue(flags, map[string]any{"written": true, "path": path}, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", path)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing account config")
	return cmd
}
