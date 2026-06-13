package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"spotify-pp-cli/internal/library"
	"spotify-pp-cli/internal/playlists"
	"spotify-pp-cli/internal/ui"
)

func newPlaylistsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "playlists", Short: "Manage Spotify playlists"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List current user's playlists",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			scanner := library.Scanner{Client: app.Client, Store: app.Store, Market: app.Config.Market}
			playlists, err := scanner.ScanPlaylists(ctx)
			if err != nil {
				return err
			}
			return outputValue(flags, playlists, func() error {
				table := ui.NewTable(cmd.OutOrStdout())
				table.Row("Name", "Tracks", "Public", "Collaborative", "ID")
				for _, playlist := range playlists {
					public := "unknown"
					if playlist.Public != nil {
						public = fmt.Sprintf("%v", *playlist.Public)
					}
					table.Row(playlist.Name, playlist.Tracks.Total, public, playlist.Collaborative, playlist.ID)
				}
				return table.Flush()
			})
		},
	})
	return cmd
}

func newPlaylistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "playlist", Short: "Inspect and clean one playlist"}
	cmd.AddCommand(newPlaylistShowCmd(flags))
	cmd.AddCommand(newPlaylistScanCmd(flags))
	cmd.AddCommand(newPlaylistCreateCmd(flags))
	cmd.AddCommand(newPlaylistDedupeCmd(flags))
	cmd.AddCommand(newPlaylistSortCmd(flags))
	cmd.AddCommand(newPlaylistRenameCmd(flags))
	return cmd
}

func newPlaylistShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show cached playlist details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			playlist, ok, err := app.Store.PlaylistByName(ctx, args[0])
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("playlist %q not found in local cache; run 'spotify-pp-cli playlists list' first", args[0])
			}
			return outputValue(flags, playlist, func() error {
				fmt.Printf("Name: %s\nID: %s\nTracks: %d\nSnapshot: %s\n", playlist.Name, playlist.ID, playlist.TrackCount, playlist.SnapshotID)
				return nil
			})
		},
	}
}

func newPlaylistScanCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "scan <name>",
		Short: "Fetch and cache playlist items",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			scanner := library.Scanner{Client: app.Client, Store: app.Store, Market: app.Config.Market}
			apiPlaylists, err := scanner.ScanPlaylists(ctx)
			if err != nil {
				return err
			}
			playlist, ok := library.PlaylistByName(apiPlaylists, args[0])
			if !ok {
				return fmt.Errorf("playlist %q not found", args[0])
			}
			summary, err := scanner.ScanPlaylist(ctx, playlist)
			if err != nil {
				return err
			}
			return outputValue(flags, summary, func() error {
				fmt.Printf("Cached %d playlist items.\n", summary.PlaylistItem)
				return nil
			})
		},
	}
}

func newPlaylistCreateCmd(flags *rootFlags) *cobra.Command {
	var private bool
	var confirmFlag bool
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a playlist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirmFlag {
				fmt.Printf("Dry-run: would create playlist %q.\n", args[0])
				return nil
			}
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			playlist, err := app.Client.CreatePlaylist(ctx, args[0], !private, "Created by spotify-pp-cli")
			if err != nil {
				return err
			}
			_ = app.Store.UpsertPlaylist(ctx, library.PlaylistRecord(playlist))
			return outputValue(flags, playlist, func() error {
				fmt.Printf("Created playlist %q (%s).\n", playlist.Name, playlist.ID)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&private, "private", false, "Create a private playlist")
	cmd.Flags().BoolVar(&confirmFlag, "confirm", false, "Apply the create")
	return cmd
}

func newPlaylistDedupeCmd(flags *rootFlags) *cobra.Command {
	var dryRun bool
	var confirmFlag bool
	var keep string
	cmd := &cobra.Command{
		Use:   "dedupe <name>",
		Short: "Plan duplicate removal for a playlist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			playlist, ok, err := app.Store.PlaylistByName(ctx, args[0])
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("playlist %q not found in local cache; run 'spotify-pp-cli playlist scan %q' first", args[0], args[0])
			}
			items, err := app.Store.PlaylistTracks(ctx, playlist.ID)
			if err != nil {
				return err
			}
			instances := make([]playlists.TrackInstance, 0, len(items))
			for _, item := range items {
				instances = append(instances, playlists.TrackInstance{PlaylistID: playlist.ID, TrackID: item.TrackID, TrackURI: item.TrackURI, Position: item.Position})
			}
			plan := playlists.DedupePlan(instances, keep)
			if flags.asJSON {
				return printJSON(plan)
			}
			for _, item := range plan {
				fmt.Printf("Remove duplicate %s at position %d (%s)\n", item.TrackID, item.Position, item.Reason)
			}
			if len(plan) == 0 {
				fmt.Println("No duplicates found.")
			}
			if dryRun || !confirmFlag {
				fmt.Println("No changes made.")
				return nil
			}
			return fmt.Errorf("confirmed exact-position playlist dedupe is not enabled yet; this command currently plans safely but will not risk removing all copies of a duplicate URI")
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "Show duplicate removals without changing Spotify")
	cmd.Flags().BoolVar(&confirmFlag, "confirm", false, "Apply duplicate removal")
	cmd.Flags().StringVar(&keep, "keep", "first", "Duplicate to keep: first or last")
	return cmd
}

func newPlaylistSortCmd(flags *rootFlags) *cobra.Command {
	var by string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "sort <name>",
		Short: "Plan playlist sorting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			valid := map[string]bool{"artist": true, "album": true, "added_at": true, "genre": true, "rules-priority": true}
			if !valid[by] {
				return fmt.Errorf("--by must be artist, album, added_at, genre, or rules-priority")
			}
			if !dryRun {
				return fmt.Errorf("playlist sort execution is intentionally dry-run only in this MVP; use --dry-run")
			}
			fmt.Printf("Dry-run: would sort playlist %q by %s using cached playlist items.\n", args[0], by)
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", "artist", "Sort key")
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "Show sort order without changing Spotify")
	return cmd
}

func newPlaylistRenameCmd(flags *rootFlags) *cobra.Command {
	var confirmFlag bool
	cmd := &cobra.Command{
		Use:   "rename <old name> <new name>",
		Short: "Rename a playlist",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirmFlag {
				fmt.Printf("Dry-run: would rename playlist %q to %q.\n", args[0], args[1])
				return nil
			}
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			scanner := library.Scanner{Client: app.Client, Store: app.Store, Market: app.Config.Market}
			apiPlaylists, err := scanner.ScanPlaylists(ctx)
			if err != nil {
				return err
			}
			playlist, ok := library.PlaylistByName(apiPlaylists, args[0])
			if !ok {
				return fmt.Errorf("playlist %q not found", args[0])
			}
			if err := app.Client.ChangePlaylistDetails(ctx, playlist.ID, args[1], nil); err != nil {
				return err
			}
			fmt.Printf("Renamed playlist %q to %q.\n", args[0], args[1])
			return nil
		},
	}
	cmd.Flags().BoolVar(&confirmFlag, "confirm", false, "Apply rename")
	return cmd
}

func normalizePlaylistName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
