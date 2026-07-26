package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"hevy-pp-cli/internal/config"
	"hevy-pp-cli/internal/csvimport"
	"hevy-pp-cli/internal/database"
	"hevy-pp-cli/internal/workouts"
)

const version = "0.1.0"

type flags struct {
	JSON, Plain, Quiet, Yes, Debug, Headed bool
	Config, DB                             string
	Timeout                                time.Duration
}
type app struct {
	cfg   config.Config
	store *database.Store
	f     *flags
}

func Execute() error { return RootCmd().Execute() }
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	return 1
}

func RootCmd() *cobra.Command {
	f := &flags{Timeout: 5 * time.Minute}
	root := &cobra.Command{Use: "hevy", Short: "Import Hevy CSV exports and safely manage local workout plans", SilenceErrors: true, SilenceUsage: true, Version: version}
	p := root.PersistentFlags()
	p.BoolVar(&f.JSON, "json", false, "write JSON to stdout")
	p.BoolVar(&f.Plain, "plain", false, "write plain text")
	p.BoolVar(&f.Quiet, "quiet", false, "suppress human output")
	p.BoolVar(&f.Yes, "yes", false, "skip confirmation prompts")
	p.BoolVar(&f.Debug, "debug", false, "enable safe diagnostics")
	p.BoolVar(&f.Headed, "headed", false, "show browser UI")
	p.StringVar(&f.Config, "config", "", "configuration file")
	p.StringVar(&f.DB, "db", "", "SQLite database path")
	p.DurationVar(&f.Timeout, "timeout", 5*time.Minute, "browser timeout")
	root.AddCommand(syncCmd(f), workoutsCmd(f), exercisesCmd(f), exerciseCmd(f), prsCmd(f), volumeCmd(f), plansCmd(f), loginCmd(f), logoutCmd(f), authCmd(f), routinesCmd(f))
	return root
}
func open(f *flags) (*app, func(), error) {
	cfg, _, err := config.Load(f.Config)
	if err != nil {
		return nil, nil, err
	}
	if f.DB != "" {
		cfg.DatabasePath = f.DB
	}
	store, err := database.Open(cfg.DatabasePath)
	if err != nil {
		return nil, nil, err
	}
	return &app{cfg: cfg, store: store, f: f}, func() { _ = store.Close() }, nil
}
func emit(f *flags, value any, human string) error {
	if f.JSON {
		b, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(os.Stdout, string(b))
		return err
	}
	if f.Quiet {
		return nil
	}
	_, err := fmt.Fprintln(os.Stdout, human)
	return err
}
func parseSince(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	var n int
	if strings.HasSuffix(s, "d") {
		if _, err := fmt.Sscanf(strings.TrimSuffix(s, "d"), "%d", &n); err == nil {
			return time.Now().AddDate(0, 0, -n), nil
		}
	}
	if strings.HasSuffix(s, "m") {
		if _, err := fmt.Sscanf(strings.TrimSuffix(s, "m"), "%d", &n); err == nil {
			return time.Now().AddDate(0, -n, 0), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid --since %q (use 30d, 6m, or a Go duration)", s)
}

func syncCmd(f *flags) *cobra.Command {
	var directory string
	var latest, dryRun, force bool
	cmd := &cobra.Command{Use: "sync [csv-file]", Short: "Import one or more Hevy CSV exports",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("accepts at most one CSV file")
			}
			if len(args) == 0 && directory == "" {
				return fmt.Errorf("specify a CSV file or --directory")
			}
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			a, close, err := open(f)
			if err != nil {
				return err
			}
			defer close()
			files := append([]string(nil), args...)
			if directory != "" {
				entries, err := os.ReadDir(directory)
				if err != nil {
					return err
				}
				files = nil
				for _, entry := range entries {
					if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".csv") {
						files = append(files, filepath.Join(directory, entry.Name()))
					}
				}
				sort.Strings(files)
				if latest && len(files) > 0 {
					sort.Slice(files, func(i, j int) bool {
						left, _ := os.Stat(files[i])
						right, _ := os.Stat(files[j])
						return left.ModTime().After(right.ModTime())
					})
					files = files[:1]
				}
			}
			if len(files) == 0 {
				return fmt.Errorf("no CSV files found")
			}
			summaries := make([]database.ImportSummary, 0, len(files))
			for _, file := range files {
				parsed, err := csvimport.ParseFile(file, 50<<20)
				if err != nil {
					return err
				}
				summary, err := a.store.ImportCSV(context.Background(), file, parsed, dryRun)
				if err != nil {
					return err
				}
				if force && summary.Skipped {
					summary.Warnings = append(summary.Warnings, "--force cannot duplicate stable set fingerprints")
				}
				summaries = append(summaries, summary)
			}
			return emit(f, map[string]any{"version": 1, "dry_run": dryRun, "imports": summaries}, fmt.Sprintf("processed %d CSV file(s)", len(summaries)))
		}}
	cmd.Flags().StringVar(&directory, "directory", "", "directory of CSV exports")
	cmd.Flags().BoolVar(&latest, "latest", false, "import only newest CSV")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "parse without database writes")
	cmd.Flags().BoolVar(&force, "force", false, "attempt a previously imported source")
	cmd.AddCommand(&cobra.Command{Use: "status", Short: "Show sync status", RunE: func(_ *cobra.Command, _ []string) error {
		return emit(f, map[string]any{"version": 1, "status": "available"}, "sync metadata is stored in SQLite")
	}})
	return cmd
}

func workoutsCmd(f *flags) *cobra.Command {
	var since string
	var limit int
	cmd := &cobra.Command{Use: "workouts", Short: "Query imported workouts"}
	list := &cobra.Command{Use: "list", RunE: func(_ *cobra.Command, _ []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		t, err := parseSince(since)
		if err != nil {
			return err
		}
		rows, err := a.store.Workouts(t, limit)
		if err != nil {
			return err
		}
		return emit(f, map[string]any{"version": 1, "workouts": rows}, fmt.Sprintf("%d workouts", len(rows)))
	}}
	list.Flags().StringVar(&since, "since", "", "period such as 30d")
	list.Flags().IntVar(&limit, "limit", 0, "maximum workouts")
	show := &cobra.Command{Use: "show <id>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		row, err := a.store.Workout(args[0])
		if err != nil {
			return err
		}
		return emit(f, map[string]any{"version": 1, "workout": row}, row.Title)
	}}
	cmd.AddCommand(list, show)
	return cmd
}
func exercisesCmd(f *flags) *cobra.Command {
	cmd := &cobra.Command{Use: "exercises", Short: "Query imported exercises"}
	cmd.AddCommand(&cobra.Command{Use: "list", RunE: func(_ *cobra.Command, _ []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		rows, err := a.store.Exercises()
		if err != nil {
			return err
		}
		return emit(f, map[string]any{"version": 1, "exercises": rows}, fmt.Sprintf("%d exercises", len(rows)))
	}})
	return cmd
}
func exerciseCmd(f *flags) *cobra.Command {
	var since string
	cmd := &cobra.Command{Use: "exercise", Short: "Inspect an exercise's history"}
	history := &cobra.Command{Use: "history <exercise>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		t, err := parseSince(since)
		if err != nil {
			return err
		}
		rows, err := a.store.ExerciseSets(args[0], t)
		if err != nil {
			return err
		}
		return emit(f, map[string]any{"version": 1, "exercise": args[0], "sets": rows}, fmt.Sprintf("%d sets", len(rows)))
	}}
	history.Flags().StringVar(&since, "since", "", "period such as 6m")
	cmd.AddCommand(history)
	return cmd
}
func prsCmd(f *flags) *cobra.Command {
	var name string
	cmd := &cobra.Command{Use: "prs", Short: "Calculate personal records (Epley estimated 1RM)", RunE: func(_ *cobra.Command, _ []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		names := []string{name}
		if name == "" {
			xs, err := a.store.Exercises()
			if err != nil {
				return err
			}
			names = nil
			for _, x := range xs {
				names = append(names, x.Name)
			}
		}
		out := []workouts.PR{}
		for _, n := range names {
			rows, err := a.store.ExerciseSets(n, time.Time{})
			if err != nil {
				return err
			}
			out = append(out, workouts.PRs(rows)...)
		}
		return emit(f, map[string]any{"version": 1, "formula": "Epley: weight * (1 + reps / 30)", "records": out}, fmt.Sprintf("%d records", len(out)))
	}}
	cmd.Flags().StringVar(&name, "exercise", "", "exercise name")
	return cmd
}
func volumeCmd(f *flags) *cobra.Command {
	var since, name, group string
	cmd := &cobra.Command{Use: "volume", Short: "Report volume as weight_kg * repetitions", RunE: func(_ *cobra.Command, _ []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		t, err := parseSince(since)
		if err != nil {
			return err
		}
		names := []string{name}
		if name == "" {
			xs, err := a.store.Exercises()
			if err != nil {
				return err
			}
			names = nil
			for _, x := range xs {
				names = append(names, x.Name)
			}
		}
		var rows []database.SetRecord
		for _, n := range names {
			sets, err := a.store.ExerciseSets(n, t)
			if err != nil {
				return err
			}
			rows = append(rows, sets...)
		}
		return emit(f, map[string]any{"version": 1, "definition": "weight_kg * repetitions", "volume": workouts.Volumes(rows, group)}, "volume report")
	}}
	cmd.Flags().StringVar(&since, "since", "", "period")
	cmd.Flags().StringVar(&name, "exercise", "", "exercise")
	cmd.Flags().StringVar(&group, "group", "day", "day or week")
	return cmd
}
