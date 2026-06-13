package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"spotify-pp-cli/internal/classifier"
	"spotify-pp-cli/internal/ui"
)

func newClassifyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "classify", Short: "Classify liked songs or a single track"}
	cmd.AddCommand(newClassifyLikedCmd(flags))
	cmd.AddCommand(newClassifyTrackCmd(flags))
	return cmd
}

func newClassifyLikedCmd(flags *rootFlags) *cobra.Command {
	var onlyUnclassified bool
	var refresh bool
	var path string
	cmd := &cobra.Command{
		Use:   "liked",
		Short: "Classify cached liked songs with the rules file",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			rf, _, err := loadRulesFile(path)
			if err != nil {
				return err
			}
			if refresh {
				onlyUnclassified = false
			}
			tracks, results, err := classifyAll(ctx, app.Store, rf, onlyUnclassified)
			if err != nil {
				return err
			}
			return outputValue(flags, results, func() error {
				table := ui.NewTable(cmd.OutOrStdout())
				table.Row("Track", "Artists", "Target playlist", "Confidence", "Reason")
				for _, track := range tracks {
					result := results[track.ID]
					table.Row(track.Name, join(result.ArtistNames), result.TargetPlaylistName, result.Confidence, result.Explanation)
				}
				if err := table.Flush(); err != nil {
					return err
				}
				fmt.Printf("\nClassified %d tracks.\n", len(results))
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&onlyUnclassified, "only-unclassified", false, "Only classify tracks without cached classification")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh all cached classifications")
	cmd.Flags().StringVar(&path, "rules", "", "Rules file path")
	return cmd
}

func newClassifyTrackCmd(flags *rootFlags) *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "track <track-url-or-id>",
		Short: "Classify one track",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			rf, _, err := loadRulesFile(path)
			if err != nil {
				return err
			}
			track, err := resolveTrack(ctx, app, args[0])
			if err != nil {
				return err
			}
			result, err := classifyTrack(ctx, app.Store, rf, track)
			if err != nil {
				return err
			}
			return printClassification(flags, result)
		},
	}
	cmd.Flags().StringVar(&path, "rules", "", "Rules file path")
	return cmd
}

func printClassification(flags *rootFlags, result classifier.Result) error {
	return outputValue(flags, result, func() error {
		fmt.Printf("Track: %q", result.TrackName)
		if len(result.ArtistNames) > 0 {
			fmt.Printf(" - %s", join(result.ArtistNames))
		}
		fmt.Println()
		fmt.Printf("Artist genres: %s\n", join(result.ArtistGenres))
		fmt.Printf("Matched rule: %s\n", result.MatchedRule)
		fmt.Printf("Target playlist: %s\n", result.TargetPlaylistName)
		fmt.Printf("Confidence: %s\n", result.Confidence)
		fmt.Printf("Reason: %s\n", result.Explanation)
		return nil
	})
}

func join(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}
