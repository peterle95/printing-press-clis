package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func authCmd(f *flags) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Use pytr's interactive web login"}
	cmd.AddCommand(authLoginCmd(f), authStatusCmd(f))
	return cmd
}

func authLoginCmd(f *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Delegate an interactive web login to pytr",
		Long:  "Run pytr's web login without accepting a phone PIN or second-factor code as a tr argument.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if f.NoInput {
				return fmt.Errorf("auth login requires an interactive terminal; remove --no-input/--agent")
			}
			cfg, _, err := loadConfig(f)
			if err != nil {
				return err
			}
			adapter, err := newTradeRepublicAdapter(cfg, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			if err := adapter.Login(ctx); err != nil {
				return err
			}
			return emit(cmd, f, envelope{"version": 1, "authenticated": true, "adapter": "pytr", "method": "web"}, "pytr web login completed")
		},
	}
}

func authStatusCmd(f *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check whether the configured pytr executable is available",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, _, err := loadConfig(f)
			if err != nil {
				return err
			}
			adapter, err := newTradeRepublicAdapter(cfg, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			status := adapter.Status(ctx)
			human := "pytr unavailable"
			if status.Available {
				human = fmt.Sprintf("pytr available (%s)", status.Version)
			}
			return emit(cmd, f, envelope{"version": 1, "adapter": "pytr", "status": status, "note": "availability does not reveal or validate stored credentials"}, human)
		},
	}
}
