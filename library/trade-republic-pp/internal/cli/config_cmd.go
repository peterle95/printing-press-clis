package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"trade-republic-pp-cli/config"
)

const disableKillSwitchChallenge = "DISABLE KILL SWITCH FOR PAPER PREVIEWS"

func configCmd(f *flags) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Inspect or initialize non-secret configuration"}
	cmd.AddCommand(configInitCmd(f), configShowCmd(f), configPathCmd(f), configKillSwitchCmd(f))
	return cmd
}

func configInitCmd(f *flags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a private default configuration file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := f.Config
			if path == "" {
				var err error
				path, err = config.ConfigPath()
				if err != nil {
					return err
				}
			}
			if _, err := os.Stat(path); err == nil && !force {
				return fmt.Errorf("configuration already exists at %s (use --force to replace it)", path)
			} else if err != nil && !os.IsNotExist(err) {
				return err
			}
			cfg, err := config.Default()
			if err != nil {
				return err
			}
			written, err := config.Save(path, cfg)
			if err != nil {
				return err
			}
			return emit(cmd, f, envelope{"version": 1, "path": written, "config": cfg}, "wrote "+written)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "replace an existing configuration")
	return cmd
}

func configShowCmd(f *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show effective non-secret configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, path, err := loadConfig(f)
			if err != nil {
				return err
			}
			return emit(cmd, f, envelope{"version": 1, "path": path, "config": cfg}, fmt.Sprintf("config: %s\ndatabase: %s\nkill switch: %t\npaper trading: %t", path, cfg.DatabasePath, cfg.Risk.KillSwitch, cfg.Risk.PaperTrading))
		},
	}
}

func configPathCmd(f *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the configuration path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := f.Config
			if path == "" {
				var err error
				path, err = config.ConfigPath()
				if err != nil {
					return err
				}
			}
			return emit(cmd, f, envelope{"version": 1, "path": path}, path)
		},
	}
}

func configKillSwitchCmd(f *flags) *cobra.Command {
	var challenge string
	cmd := &cobra.Command{
		Use:   "kill-switch <status|on|off>",
		Short: "Inspect or change the local paper-preview kill switch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadConfig(f)
			if err != nil {
				return err
			}
			action := strings.ToLower(args[0])
			switch action {
			case "status":
			case "on":
				cfg.Risk.KillSwitch = true
				if _, err := config.Save(path, cfg); err != nil {
					return err
				}
			case "off":
				if !cfg.Risk.PaperTrading {
					return fmt.Errorf("refusing to disable the kill switch outside paper-trading mode")
				}
				provided := strings.TrimSpace(challenge)
				if provided == "" && !f.NoInput {
					fmt.Fprintf(cmd.ErrOrStderr(), "Type %q to continue: ", disableKillSwitchChallenge)
					line, readErr := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
					if readErr != nil && strings.TrimSpace(line) == "" {
						return readErr
					}
					provided = strings.TrimSpace(line)
				}
				if provided != disableKillSwitchChallenge {
					return fmt.Errorf("kill switch unchanged: exact paper-only challenge did not match")
				}
				cfg.Risk.KillSwitch = false
				if _, err := config.Save(path, cfg); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unknown kill-switch action %q (use status, on, or off)", args[0])
			}
			return emit(cmd, f, envelope{"version": 1, "kill_switch": cfg.Risk.KillSwitch, "paper_trading": cfg.Risk.PaperTrading}, fmt.Sprintf("kill switch: %t", cfg.Risk.KillSwitch))
		},
	}
	cmd.Flags().StringVar(&challenge, "challenge", "", "exact paper-only challenge (prefer interactive input)")
	return cmd
}
