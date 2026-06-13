package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"spotify-pp-cli/internal/classifier"
	"spotify-pp-cli/internal/client"
	"spotify-pp-cli/internal/library"
	"spotify-pp-cli/internal/organizer"
	"spotify-pp-cli/internal/rules"
	"spotify-pp-cli/internal/store"
	"spotify-pp-cli/internal/ui"
)

func newLikedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "liked", Short: "Scan, classify, copy, and move Liked Songs"}
	cmd.AddCommand(newLikedScanCmd(flags))
	cmd.AddCommand(newLikedListCmd(flags))
	cmd.AddCommand(newLikedExportCmd(flags))
	cmd.AddCommand(newLikedPlanCmd(flags))
	cmd.AddCommand(newLikedCopyCmd(flags))
	cmd.AddCommand(newLikedMoveCmd(flags))
	return cmd
}

func newLikedScanCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var refreshArtists bool
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Fetch Liked Songs from Spotify and cache tracks/artists locally",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			scanner := library.Scanner{Client: app.Client, Store: app.Store, Market: app.Config.Market}
			summary, err := scanner.ScanLiked(ctx, limit, refreshArtists, func(done, total int) {
				if !flags.asJSON {
					fmt.Fprintf(os.Stderr, "Scanned %d/%d liked tracks\r", done, total)
				}
			})
			if !flags.asJSON {
				fmt.Fprintln(os.Stderr)
			}
			if err != nil {
				return err
			}
			return outputValue(flags, summary, func() error {
				fmt.Printf("Cached %d liked tracks and %d artists.\n", summary.Tracks, summary.Artists)
				return nil
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum liked tracks to scan")
	cmd.Flags().BoolVar(&refreshArtists, "refresh-artists", false, "Refresh artist metadata even if cached")
	return cmd
}

func newLikedListCmd(flags *rootFlags) *cobra.Command {
	var genre string
	var unclassified bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List cached liked songs",
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
			rows := filterLikedRows(tracks, classes, genre, unclassified)
			return outputValue(flags, rows, func() error {
				table := ui.NewTable(cmd.OutOrStdout())
				table.Row("Track", "Album", "Target", "Confidence")
				for _, row := range rows {
					table.Row(row.Track.Name, row.Track.AlbumName, row.Class.TargetPlaylistName, row.Class.Confidence)
				}
				return table.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&genre, "genre", "", "Filter by matched rule, primary genre, or target playlist")
	cmd.Flags().BoolVar(&unclassified, "unclassified", false, "Only show tracks without cached classification")
	return cmd
}

func newLikedExportCmd(flags *rootFlags) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export cached liked songs",
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
			rows := filterLikedRows(tracks, classes, "", false)
			switch format {
			case "json":
				return printJSON(rows)
			case "csv":
				csvRows := [][]string{{"id", "uri", "name", "album", "target_playlist", "confidence"}}
				for _, row := range rows {
					csvRows = append(csvRows, []string{row.Track.ID, row.Track.URI, row.Track.Name, row.Track.AlbumName, row.Class.TargetPlaylistName, row.Class.Confidence})
				}
				return printCSV(csvRows)
			case "md":
				fmt.Println("| Track | Album | Target playlist | Confidence |")
				fmt.Println("| --- | --- | --- | --- |")
				for _, row := range rows {
					fmt.Printf("| %s | %s | %s | %s |\n", escapeMD(row.Track.Name), escapeMD(row.Track.AlbumName), escapeMD(row.Class.TargetPlaylistName), row.Class.Confidence)
				}
				return nil
			default:
				return fmt.Errorf("--format must be json, csv, or md")
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "Export format: json, csv, md")
	return cmd
}

func newLikedPlanCmd(flags *rootFlags) *cobra.Command {
	var rulesPath string
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Print proposed liked-song organization actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			plan, _, err := buildLikedPlan(ctx, app, rulesPath, "liked copy", true, false)
			if err != nil {
				return err
			}
			return printPlan(flags, cmd, plan)
		},
	}
	cmd.Flags().StringVar(&rulesPath, "rules", "", "Rules file path")
	return cmd
}

func newLikedCopyCmd(flags *rootFlags) *cobra.Command {
	return newLikedApplyCmd(flags, "copy")
}

func newLikedMoveCmd(flags *rootFlags) *cobra.Command {
	return newLikedApplyCmd(flags, "move")
}

func newLikedApplyCmd(flags *rootFlags, mode string) *cobra.Command {
	var dryRun bool
	var confirmFlag bool
	var rulesPath string
	var only string
	var onlyUnclassified bool
	cmd := &cobra.Command{
		Use:   mode,
		Short: fmt.Sprintf("%s liked songs into rule-target playlists", strings.Title(mode)),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			if confirmFlag && !cmd.Flags().Changed("dry-run") {
				dryRun = false
			}
			if !confirmFlag {
				dryRun = true
			}
			if mode == "move" && !confirmFlag {
				dryRun = true
			}
			plan, rf, err := buildLikedPlan(ctx, app, rulesPath, "liked "+mode, dryRun, onlyUnclassified)
			if err != nil {
				return err
			}
			plan.Items = filterPlanItems(plan.Items, only)
			recountPlan(&plan)
			if dryRun {
				return printPlan(flags, cmd, plan)
			}
			if mode == "move" {
				fmt.Printf("This will remove %d tracks from Liked Songs after adding them to playlists.\n", len(plan.Items))
				if err := confirm(flags, true, "move liked songs"); err != nil {
					return err
				}
			}
			return executeLikedOperation(ctx, app, rf, plan, mode)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "Show planned changes without writing to Spotify")
	cmd.Flags().BoolVar(&confirmFlag, "confirm", false, "Apply changes")
	cmd.Flags().StringVar(&rulesPath, "rules", "", "Rules file path")
	cmd.Flags().StringVar(&only, "only", "", "Filter actions, for example genre=techno")
	cmd.Flags().BoolVar(&onlyUnclassified, "only-unclassified", false, "Only operate on tracks without cached classification")
	return cmd
}

func buildLikedPlan(ctx context.Context, app *app, rulesPath, opType string, dryRun bool, onlyUnclassified bool) (organizer.Plan, *rules.File, error) {
	rf, _, err := loadRulesFile(rulesPath)
	if err != nil {
		return organizer.Plan{}, nil, err
	}
	tracks, results, err := classifyAll(ctx, app.Store, rf, onlyUnclassified)
	if err != nil {
		return organizer.Plan{}, nil, err
	}
	plan := organizer.BuildPlan(opType, dryRun, tracks, results)
	plan.RemoveAfterAdd = strings.Contains(opType, "move")
	return plan, rf, nil
}

func executeLikedOperation(ctx context.Context, app *app, rf *rules.File, plan organizer.Plan, mode string) error {
	opID, err := app.Store.CreateOperation(ctx, plan.OperationType, false, organizer.SummaryJSON(plan))
	if err != nil {
		return err
	}
	status := "success"
	errText := ""
	undo := map[string]any{"playlist_adds": map[string][]string{}, "library_removed": []string{}}
	defer func() {
		undoJSON, _ := json.Marshal(undo)
		_ = app.Store.CompleteOperation(ctx, opID, status, string(undoJSON), errText)
	}()

	scanner := library.Scanner{Client: app.Client, Store: app.Store, Market: app.Config.Market}
	playlists, err := scanner.ScanPlaylists(ctx)
	if err != nil {
		status = "failed"
		errText = err.Error()
		return err
	}
	playlistByName := map[string]clientPlaylist{}
	for _, playlist := range playlists {
		playlistByName[strings.ToLower(playlist.Name)] = clientPlaylist{id: playlist.ID, name: playlist.Name}
	}
	targets := playlistTargets(rf)
	itemsByTarget := map[string][]organizer.PlanItem{}
	for _, item := range plan.Items {
		itemsByTarget[item.TargetPlaylist] = append(itemsByTarget[item.TargetPlaylist], item)
	}
	for targetName, items := range itemsByTarget {
		target := targets[strings.ToLower(targetName)]
		playlist, ok := playlistByName[strings.ToLower(targetName)]
		if !ok {
			if !target.createIfMissing {
				status = "partial"
				for _, item := range items {
					_ = app.Store.AddOperationItem(ctx, opItem(opID, item, "", "playlist_add", "failed", "target playlist missing and create_if_missing=false"))
				}
				continue
			}
			created, err := app.Client.CreatePlaylist(ctx, targetName, target.public, "Created by spotify-pp-cli")
			if err != nil {
				status = "partial"
				for _, item := range items {
					_ = app.Store.AddOperationItem(ctx, opItem(opID, item, "", "playlist_add", "failed", err.Error()))
				}
				continue
			}
			_ = app.Store.UpsertPlaylist(ctx, library.PlaylistRecord(created))
			playlist = clientPlaylist{id: created.ID, name: created.Name}
			playlistByName[strings.ToLower(targetName)] = playlist
		}
		if _, err := scanner.ScanPlaylist(ctx, playlist.asClient()); err != nil {
			status = "partial"
			for _, item := range items {
				_ = app.Store.AddOperationItem(ctx, opItem(opID, item, playlist.id, "playlist_add", "failed", err.Error()))
			}
			continue
		}
		existing, err := app.Store.ExistingTrackIDsInPlaylist(ctx, playlist.id)
		if err != nil {
			return err
		}
		var toAdd []string
		var addedItems []organizer.PlanItem
		for _, item := range items {
			if existing[item.Track.ID] {
				_ = app.Store.AddOperationItem(ctx, opItem(opID, item, playlist.id, "playlist_add", "skipped", "track already exists in playlist"))
				continue
			}
			toAdd = append(toAdd, item.Track.URI)
			addedItems = append(addedItems, item)
		}
		if len(toAdd) == 0 {
			continue
		}
		if _, err := app.Client.AddPlaylistItems(ctx, playlist.id, toAdd); err != nil {
			status = "partial"
			for _, item := range addedItems {
				_ = app.Store.AddOperationItem(ctx, opItem(opID, item, playlist.id, "playlist_add", "failed", err.Error()))
			}
			continue
		}
		undo["playlist_adds"].(map[string][]string)[playlist.id] = append(undo["playlist_adds"].(map[string][]string)[playlist.id], toAdd...)
		for _, item := range addedItems {
			_ = app.Store.AddOperationItem(ctx, opItem(opID, item, playlist.id, "playlist_add", "success", "added to playlist"))
		}
		if mode != "move" {
			continue
		}
		if _, err := scanner.ScanPlaylist(ctx, playlist.asClient()); err != nil {
			status = "partial"
			errText = err.Error()
			continue
		}
		verified, err := app.Store.ExistingTrackIDsInPlaylist(ctx, playlist.id)
		if err != nil {
			return err
		}
		var removeURIs []string
		for _, item := range addedItems {
			if verified[item.Track.ID] {
				removeURIs = append(removeURIs, item.Track.URI)
			}
		}
		if len(removeURIs) == 0 {
			continue
		}
		if err := app.Client.RemoveLibraryItems(ctx, removeURIs); err != nil {
			status = "partial"
			errText = err.Error()
			for _, item := range addedItems {
				_ = app.Store.AddOperationItem(ctx, opItem(opID, item, playlist.id, "library_remove", "failed", err.Error()))
			}
			continue
		}
		undo["library_removed"] = append(undo["library_removed"].([]string), removeURIs...)
		for _, item := range addedItems {
			_ = app.Store.AddOperationItem(ctx, opItem(opID, item, playlist.id, "library_remove", "success", "verified playlist membership before removing from library"))
		}
	}
	fmt.Printf("Operation %d completed with status %s.\n", opID, status)
	return nil
}

func printPlan(flags *rootFlags, cmd *cobra.Command, plan organizer.Plan) error {
	return outputValue(flags, plan, func() error {
		fmt.Println("Liked Songs organization plan")
		fmt.Println()
		table := ui.NewTable(cmd.OutOrStdout())
		table.Row("Target playlist", "Tracks", "Confidence")
		names := make([]string, 0, len(plan.ByPlaylist))
		for name := range plan.ByPlaylist {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			table.Row(name, plan.ByPlaylist[name], confidenceSummary(plan.Items, name))
		}
		if err := table.Flush(); err != nil {
			return err
		}
		fmt.Println()
		fmt.Printf("Operation: %s", plan.OperationType)
		if plan.DryRun {
			fmt.Print(" dry-run")
		}
		fmt.Println()
		fmt.Printf("Tracks scanned: %d\n", plan.TracksScanned)
		fmt.Printf("Classified: %d\n", plan.Classified)
		fmt.Printf("Needs review: %d\n", plan.NeedsReview)
		fmt.Printf("Would remove from Liked Songs after successful playlist verification: %v\n", plan.RemoveAfterAdd)
		fmt.Println()
		fmt.Println("No changes made. Run with:")
		fmt.Println("spotify-pp-cli liked copy --confirm")
		fmt.Println("or")
		fmt.Println("spotify-pp-cli liked move --confirm")
		return nil
	})
}

type likedRow struct {
	Track store.TrackRecord          `json:"track"`
	Class store.ClassificationRecord `json:"classification"`
}

func filterLikedRows(tracks []store.TrackRecord, classes map[string]store.ClassificationRecord, genre string, unclassified bool) []likedRow {
	var rows []likedRow
	needle := strings.ToLower(genre)
	for _, track := range tracks {
		class, ok := classes[track.ID]
		if unclassified && ok {
			continue
		}
		if genre != "" && !classificationMatches(class, needle) {
			continue
		}
		rows = append(rows, likedRow{Track: track, Class: class})
	}
	return rows
}

func filterPlanItems(items []organizer.PlanItem, only string) []organizer.PlanItem {
	if only == "" {
		return items
	}
	parts := strings.SplitN(only, "=", 2)
	if len(parts) != 2 || parts[0] != "genre" {
		return items
	}
	needle := strings.ToLower(parts[1])
	out := items[:0]
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Classification.MatchedRule), needle) ||
			strings.Contains(strings.ToLower(item.Classification.PrimaryGenre), needle) ||
			strings.Contains(strings.ToLower(item.TargetPlaylist), needle) {
			out = append(out, item)
		}
	}
	return out
}

func recountPlan(plan *organizer.Plan) {
	plan.TracksScanned = len(plan.Items)
	plan.Classified = 0
	plan.NeedsReview = 0
	plan.ByPlaylist = map[string]int{}
	plan.ByConfidence = map[string]int{}
	for _, item := range plan.Items {
		plan.ByPlaylist[item.TargetPlaylist]++
		plan.ByConfidence[item.Classification.Confidence]++
		if item.Classification.Confidence == classifier.ConfidenceNone || item.Classification.Confidence == classifier.ConfidenceLow {
			plan.NeedsReview++
		} else {
			plan.Classified++
		}
	}
}

func classificationMatches(class store.ClassificationRecord, needle string) bool {
	return strings.Contains(strings.ToLower(class.MatchedRule), needle) ||
		strings.Contains(strings.ToLower(class.PrimaryGenre), needle) ||
		strings.Contains(strings.ToLower(class.TargetPlaylistName), needle)
}

func confidenceSummary(items []organizer.PlanItem, playlist string) string {
	seen := map[string]bool{}
	for _, item := range items {
		if item.TargetPlaylist == playlist {
			seen[item.Classification.Confidence] = true
		}
	}
	order := []string{classifier.ConfidenceHigh, classifier.ConfidenceMedium, classifier.ConfidenceLow, classifier.ConfidenceNone}
	var out []string
	for _, key := range order {
		if seen[key] {
			out = append(out, key)
		}
	}
	return strings.Join(out, "/")
}

type targetRule struct {
	createIfMissing bool
	public          bool
}

func playlistTargets(rf *rules.File) map[string]targetRule {
	out := map[string]targetRule{}
	for _, resolved := range rf.SortedRules() {
		out[strings.ToLower(resolved.Rule.Name)] = targetRule{createIfMissing: resolved.Rule.CreateIfMissing, public: resolved.Rule.Public}
	}
	out[strings.ToLower(rf.Fallback.Name)] = targetRule{createIfMissing: rf.Fallback.CreateIfMissing, public: rf.Fallback.Public}
	return out
}

type clientPlaylist struct {
	id   string
	name string
}

func (p clientPlaylist) asClient() client.Playlist {
	return client.Playlist{ID: p.id, Name: p.name}
}

func opItem(opID int64, item organizer.PlanItem, playlistID, action, status, reason string) store.OperationItemRecord {
	return store.OperationItemRecord{
		OperationID:      opID,
		TrackID:          item.Track.ID,
		TrackURI:         item.Track.URI,
		Source:           "liked",
		TargetPlaylistID: playlistID,
		Action:           action,
		Status:           status,
		Reason:           reason,
	}
}

func escapeMD(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}
