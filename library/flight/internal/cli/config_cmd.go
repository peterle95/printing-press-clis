package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"flight-pp-cli/internal/config"
)

func newConfigCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage flight CLI config",
	}
	cmd.AddCommand(newConfigInitCmd(flags))
	cmd.AddCommand(newConfigPathCmd(flags))
	cmd.AddCommand(newConfigShowCmd(flags))
	return cmd
}

func newConfigInitCmd(flags *rootFlags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a config file with provider sections and empty API keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ResolveConfigPath(flags.config)
			if err != nil {
				return err
			}
			if _, err := os.Stat(path); err == nil && !force {
				return fmt.Errorf("config already exists at %s; pass --force to overwrite", path)
			} else if err != nil && !os.IsNotExist(err) {
				return err
			}
			cfg := config.Default()
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			return outputValue(flags, map[string]string{"path": path}, func() error {
				fmt.Printf("Created config: %s\n", path)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config")
	return cmd
}

func newConfigPathCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the resolved config path",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ResolveConfigPath(flags.config)
			if err != nil {
				return err
			}
			return outputValue(flags, map[string]string{"path": path}, func() error {
				fmt.Println(path)
				return nil
			})
		},
	}
}

func newConfigShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the effective config with env overrides applied",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.config)
			if err != nil {
				return err
			}
			return outputValue(flags, cfg, func() error {
				data, err := yaml.Marshal(cfg)
				if err != nil {
					return err
				}
				fmt.Print(string(data))
				return nil
			})
		},
	}
}
