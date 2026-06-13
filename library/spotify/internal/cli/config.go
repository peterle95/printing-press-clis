package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"spotify-pp-cli/internal/config"
)

func newConfigCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage Spotify CLI config"}
	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Print the active config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			payload := map[string]any{"path": path, "config": cfg}
			return outputValue(flags, payload, func() error {
				fmt.Printf("Config path: %s\n", path)
				fmt.Printf("client_id: %s\n", redacted(cfg.ClientID))
				fmt.Printf("redirect_uri: %s\n", cfg.RedirectURI)
				fmt.Printf("market: %s\n", cfg.Market)
				fmt.Printf("default_mode: %s\n", cfg.DefaultMode)
				fmt.Printf("liked_move_remove_after_add: %v\n", cfg.LikedMoveRemoveAfterAdd)
				fmt.Printf("use_deprecated_artist_genres: %v\n", cfg.UseDeprecatedArtistGenres)
				return nil
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set one config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			if err := config.Set(&cfg, args[0], args[1]); err != nil {
				return err
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			return outputValue(flags, map[string]any{"path": path, "key": args[0], "value": args[1]}, func() error {
				fmt.Printf("Updated %s in %s\n", args[0], path)
				return nil
			})
		},
	})
	return cmd
}

func redacted(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 6 {
		return "***"
	}
	return value[:3] + "***" + value[len(value)-3:]
}
