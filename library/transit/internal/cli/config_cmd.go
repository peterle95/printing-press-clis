package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"transit-pp-cli/internal/config"
)

func newConfigCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage transit CLI config",
	}
	cmd.AddCommand(newConfigInitCmd(flags))
	cmd.AddCommand(newConfigPathCmd(flags))
	cmd.AddCommand(newConfigShowCmd(flags))
	cmd.AddCommand(newConfigSetHomeCmd(flags))
	return cmd
}

func newConfigInitCmd(flags *rootFlags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a local transit config",
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
		Short: "Print the effective config",
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

func newConfigSetHomeCmd(flags *rootFlags) *cobra.Command {
	var address string
	var label string
	var lat float64
	var lon float64
	var resolve bool
	cmd := &cobra.Command{
		Use:   "set-home",
		Short: "Set the local home address or coordinates",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.config)
			if err != nil {
				return err
			}
			if label == "" {
				label = "home"
			}
			cfg.Home.Label = label
			latSet := cmd.Flags().Changed("lat")
			lonSet := cmd.Flags().Changed("lon")
			if address == "" && (!latSet || !lonSet) {
				return fmt.Errorf("set-home requires --address or both --lat and --lon")
			}
			if address != "" {
				cfg.Home.Address = address
				cfg.Home.Latitude = nil
				cfg.Home.Longitude = nil
			}
			if latSet || lonSet {
				if !latSet || !lonSet {
					return fmt.Errorf("--lat and --lon must be provided together")
				}
				cfg.Home.Latitude = &lat
				cfg.Home.Longitude = &lon
			}
			if err := config.Save(cfg.Path, cfg); err != nil {
				return err
			}
			if resolve && address != "" && (!latSet && !lonSet) {
				app, err := loadApp(flags)
				if err != nil {
					return err
				}
				app.config = cfg
				ctx, cancel := context.WithTimeout(context.Background(), flags.timeout)
				defer cancel()
				if _, err := resolveAndSaveHome(ctx, app); err != nil {
					return err
				}
				cfg = app.config
			}
			return outputValue(flags, cfg.Home, func() error {
				if cfg.Home.Latitude != nil && cfg.Home.Longitude != nil {
					fmt.Printf("Saved home %q at %.6f, %.6f\n", cfg.Home.Label, *cfg.Home.Latitude, *cfg.Home.Longitude)
				} else {
					fmt.Printf("Saved home %q. Coordinates will be resolved on first use.\n", cfg.Home.Label)
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&address, "address", "", "Home address")
	cmd.Flags().StringVar(&label, "label", "home", "Home label")
	cmd.Flags().Float64Var(&lat, "lat", 0, "Home latitude")
	cmd.Flags().Float64Var(&lon, "lon", 0, "Home longitude")
	cmd.Flags().BoolVar(&resolve, "resolve", true, "Resolve address coordinates immediately when possible")
	return cmd
}
