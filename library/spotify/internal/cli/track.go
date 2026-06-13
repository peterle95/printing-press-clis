package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"spotify-pp-cli/internal/library"
	"spotify-pp-cli/internal/store"
)

func newTrackCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "track", Short: "Manually copy, move, or unlike one track"}
	cmd.AddCommand(newTrackCopyMoveCmd(flags, "copy"))
	cmd.AddCommand(newTrackCopyMoveCmd(flags, "move"))
	cmd.AddCommand(newTrackUnlikeCmd(flags))
	return cmd
}

func newTrackCopyMoveCmd(flags *rootFlags, mode string) *cobra.Command {
	var to string
	var confirmFlag bool
	var dryRun bool
	cmd := &cobra.Command{
		Use:   mode + " <track-url-or-id-or-query>",
		Short: strings.Title(mode) + " one track to a playlist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" {
				return fmt.Errorf("--to playlist name is required")
			}
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			track, err := resolveTrack(ctx, app, args[0])
			if err != nil {
				return err
			}
			if !confirmFlag {
				dryRun = true
			}
			summaryJSON := mustJSON(map[string]any{
				"track_id":   track.ID,
				"track_name": track.Name,
				"mode":       mode,
				"target":     to,
			})
			opID, err := app.Store.CreateOperation(ctx, "track_"+mode, dryRun, summaryJSON)
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					_ = app.Store.CompleteOperation(ctx, opID, "failed", "", err.Error())
				}
			}()
			if dryRun {
				fmt.Printf("Dry-run: would %s %q to playlist %q.\n", mode, track.Name, to)
				_ = app.Store.CompleteOperation(ctx, opID, "dry_run", "", "")
				return nil
			}
			scanner := library.Scanner{Client: app.Client, Store: app.Store, Market: app.Config.Market}
			apiPlaylists, err := scanner.ScanPlaylists(ctx)
			if err != nil {
				return err
			}
			playlist, ok := library.PlaylistByName(apiPlaylists, to)
			if !ok {
				playlist, err = app.Client.CreatePlaylist(ctx, to, false, "Created by spotify-pp-cli")
				if err != nil {
					return err
				}
				_ = app.Store.UpsertPlaylist(ctx, library.PlaylistRecord(playlist))
			}
			snapshot, err := app.Client.AddPlaylistItems(ctx, playlist.ID, []string{track.URI})
			if err != nil {
				return err
			}
			_ = app.Store.AddOperationItem(ctx, store.OperationItemRecord{
				OperationID:      opID,
				TrackID:          track.ID,
				TrackURI:         track.URI,
				Source:           "liked_songs",
				TargetPlaylistID: playlist.ID,
				Action:           "playlist_add",
				Status:           "pending_verification",
				Reason:           mode,
			})
			if _, err := scanner.ScanPlaylist(ctx, playlist); err != nil {
				return err
			}
			existing, err := app.Store.ExistingTrackIDsInPlaylist(ctx, playlist.ID)
			if err != nil {
				return err
			}
			if !existing[track.ID] {
				_ = app.Store.CompleteOperation(ctx, opID, "failed", "", "playlist add could not be verified")
				return fmt.Errorf("playlist add could not be verified; leaving Liked Songs unchanged")
			}
			_ = app.Store.AddOperationItem(ctx, store.OperationItemRecord{
				OperationID:      opID,
				TrackID:          track.ID,
				TrackURI:         track.URI,
				Source:           "liked_songs",
				TargetPlaylistID: playlist.ID,
				Action:           "playlist_add",
				Status:           "success",
				Reason:           "verified in playlist (snapshot: " + snapshot + ")",
			})
			if mode == "move" {
				if err := app.Client.RemoveLibraryItems(ctx, []string{track.URI}); err != nil {
					return err
				}
				_ = app.Store.AddOperationItem(ctx, store.OperationItemRecord{
					OperationID:      opID,
					TrackID:          track.ID,
					TrackURI:         track.URI,
					Source:           "liked_songs",
					TargetPlaylistID: playlist.ID,
					Action:           "library_remove",
					Status:           "success",
					Reason:           "removed after verified add",
				})
				fmt.Printf("Moved %q to %q and removed it from Liked Songs after verification.\n", track.Name, playlist.Name)
			} else {
				fmt.Printf("Copied %q to %q. Liked Songs unchanged.\n", track.Name, playlist.Name)
			}
			_ = app.Store.CompleteOperation(ctx, opID, "completed", "", "")
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "Target playlist name")
	cmd.Flags().BoolVar(&confirmFlag, "confirm", false, "Apply the change (default: dry-run)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show planned changes without writing to Spotify")
	return cmd
}

func newTrackUnlikeCmd(flags *rootFlags) *cobra.Command {
	var confirmFlag bool
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "unlike <track-url-or-id>",
		Short: "Remove one track from Liked Songs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			track, err := resolveTrack(ctx, app, args[0])
			if err != nil {
				return err
			}
			if !confirmFlag {
				dryRun = true
			}
			summaryJSON := mustJSON(map[string]any{
				"track_id":   track.ID,
				"track_name": track.Name,
			})
			opID, err := app.Store.CreateOperation(ctx, "track_unlike", dryRun, summaryJSON)
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					_ = app.Store.CompleteOperation(ctx, opID, "failed", "", err.Error())
				}
			}()
			if dryRun {
				fmt.Printf("Dry-run: would remove %q from Liked Songs.\n", track.Name)
				_ = app.Store.CompleteOperation(ctx, opID, "dry_run", "", "")
				return nil
			}
			if err := confirm(flags, true, "unlike track"); err != nil {
				return err
			}
			if err := app.Client.RemoveLibraryItems(ctx, []string{track.URI}); err != nil {
				return err
			}
			_ = app.Store.AddOperationItem(ctx, store.OperationItemRecord{
				OperationID: opID,
				TrackID:     track.ID,
				TrackURI:    track.URI,
				Source:      "liked_songs",
				Action:      "library_remove",
				Status:      "success",
				Reason:      "unliked",
			})
			fmt.Printf("Removed %q from Liked Songs.\n", track.Name)
			_ = app.Store.CompleteOperation(ctx, opID, "completed", "", "")
			return nil
		},
	}
	cmd.Flags().BoolVar(&confirmFlag, "confirm", false, "Apply the unlike (default: dry-run)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show planned changes without writing to Spotify")
	return cmd
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
