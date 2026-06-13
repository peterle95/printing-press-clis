package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"spotify-pp-cli/internal/classifier"
	"spotify-pp-cli/internal/store"
	"spotify-pp-cli/internal/ui"
)

func newReviewCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "review", Short: "Review low-confidence or unclassified tracks"}
	cmd.AddCommand(newReviewListCmd(flags))
	cmd.AddCommand(newReviewNextCmd(flags))
	cmd.AddCommand(newReviewSetCmd(flags))
	cmd.AddCommand(newReviewApplyCmd(flags))
	return cmd
}

func newReviewListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tracks needing review",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			tracks, err := app.Store.LikedTracks(ctx)
			if err != nil {
				return err
			}
			classes, err := app.Store.Classifications(ctx)
			if err != nil {
				return err
			}
			var rows []likedRow
			for _, track := range tracks {
				class := classes[track.ID]
				if class.Confidence == "" || class.Confidence == classifier.ConfidenceLow || class.Confidence == classifier.ConfidenceNone {
					rows = append(rows, likedRow{Track: track, Class: class})
				}
			}
			return outputValue(flags, rows, func() error {
				table := ui.NewTable(cmd.OutOrStdout())
				table.Row("Track", "Target", "Confidence", "Reason")
				for _, row := range rows {
					table.Row(row.Track.Name, row.Class.TargetPlaylistName, row.Class.Confidence, row.Class.Explanation)
				}
				return table.Flush()
			})
		},
	}
}

func newReviewNextCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "Show the next track needing review",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			tracks, err := app.Store.LikedTracks(ctx)
			if err != nil {
				return err
			}
			classes, err := app.Store.Classifications(ctx)
			if err != nil {
				return err
			}
			for _, track := range tracks {
				class := classes[track.ID]
				if class.Confidence == "" || class.Confidence == classifier.ConfidenceLow || class.Confidence == classifier.ConfidenceNone {
					fmt.Printf("%s - %s\nTarget: %s\nReason: %s\n", track.Name, track.URI, class.TargetPlaylistName, class.Explanation)
					return nil
				}
			}
			fmt.Println("Review queue is empty.")
			return nil
		},
	}
}

func newReviewSetCmd(flags *rootFlags) *cobra.Command {
	var to string
	cmd := &cobra.Command{
		Use:   "set <track> --to <playlist>",
		Short: "Set a manual local review classification",
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
			err = app.Store.UpsertClassification(ctx, store.ClassificationRecord{
				TrackID:            track.ID,
				PrimaryGenre:       to,
				MatchedRule:        "manual_review",
				Confidence:         classifier.ConfidenceHigh,
				TargetPlaylistName: to,
				Explanation:        "manual review assignment",
			})
			if err != nil {
				return err
			}
			fmt.Printf("Set %q to %q in the local review cache.\n", track.Name, to)
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "Target playlist name")
	return cmd
}

func newReviewApplyCmd(flags *rootFlags) *cobra.Command {
	var confirmFlag bool
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply review assignments",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirmFlag {
				fmt.Println("Dry-run: review assignments are already stored locally. Use liked copy/move to apply them.")
				return nil
			}
			fmt.Println("Review assignments are stored locally. Run spotify-pp-cli liked copy --confirm to apply them.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&confirmFlag, "confirm", false, "Confirm apply")
	return cmd
}
