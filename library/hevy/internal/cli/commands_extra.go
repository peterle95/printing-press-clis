package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"hevy-pp-cli/internal/browser"
	"hevy-pp-cli/internal/plans"
	"hevy-pp-cli/internal/routines"
)

func plansCmd(f *flags) *cobra.Command {
	c := &cobra.Command{Use: "plans", Short: "Manage local YAML and JSON workout plans"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(_ *cobra.Command, _ []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		items, err := plans.List(a.cfg.PlansDirectory)
		if err != nil {
			return err
		}
		return emit(f, map[string]any{"version": 1, "plans": items}, fmt.Sprintf("%d plans", len(items)))
	}})
	c.AddCommand(&cobra.Command{Use: "show <name>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		p, _, err := plans.Find(a.cfg.PlansDirectory, args[0])
		if err != nil {
			return err
		}
		return emit(f, map[string]any{"version": 1, "plan": p}, p.Name)
	}})
	var force bool
	create := &cobra.Command{Use: "create <name>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		p := plans.Plan{Version: plans.CurrentVersion, Name: args[0], Routines: []plans.Routine{}}
		path, err := plans.Save(a.cfg.PlansDirectory, p, "yaml", force)
		if err != nil {
			return err
		}
		return emit(f, map[string]any{"version": 1, "path": path}, path)
	}}
	create.Flags().BoolVar(&force, "force", false, "overwrite an existing plan")
	c.AddCommand(create)
	var importForce bool
	imp := &cobra.Command{Use: "import <file>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		p, err := plans.Read(args[0])
		if err != nil {
			return err
		}
		path, err := plans.Save(a.cfg.PlansDirectory, p, "yaml", importForce)
		if err != nil {
			return err
		}
		return emit(f, map[string]any{"version": 1, "path": path}, path)
	}}
	imp.Flags().BoolVar(&importForce, "force", false, "overwrite an existing plan")
	c.AddCommand(imp)
	var format string
	export := &cobra.Command{Use: "export <name>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		p, _, err := plans.Find(a.cfg.PlansDirectory, args[0])
		if err != nil {
			return err
		}
		if format == "json" {
			b, _ := json.MarshalIndent(p, "", "  ")
			fmt.Println(string(b))
			return nil
		}
		if format != "yaml" {
			return fmt.Errorf("--format must be yaml or json")
		}
		b, err := yaml.Marshal(p)
		if err != nil {
			return err
		}
		fmt.Print(string(b))
		return nil
	}}
	export.Flags().StringVar(&format, "format", "", "yaml or json")
	c.AddCommand(export)
	c.AddCommand(&cobra.Command{Use: "validate <file>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		p, err := plans.Read(args[0])
		if err != nil {
			return err
		}
		return emit(f, map[string]any{"version": 1, "valid": true, "name": p.Name}, "valid")
	}})
	return c
}

func browserOptions(a *app) browser.Options {
	return browser.Options{Profile: a.cfg.BrowserProfileDir, Browser: a.cfg.Browser, Headed: a.f.Headed || a.cfg.BrowserHeaded, Debug: a.f.Debug, Timeout: a.f.Timeout}
}
func loginCmd(f *flags) *cobra.Command {
	var engine string
	c := &cobra.Command{Use: "login", Short: "Open a visible Hevy login browser", RunE: func(_ *cobra.Command, _ []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		o := browserOptions(a)
		o.Browser = engine
		return browser.Login(context.Background(), o)
	}}
	c.Flags().StringVar(&engine, "browser", "chromium", "browser engine")
	return c
}
func logoutCmd(f *flags) *cobra.Command {
	return &cobra.Command{Use: "logout", Short: "Remove only the local browser profile", RunE: func(_ *cobra.Command, _ []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		if !f.Yes {
			fmt.Fprint(os.Stderr, "Delete local browser profile? [y/N] ")
			s, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			if strings.ToLower(strings.TrimSpace(s)) != "y" {
				return fmt.Errorf("logout cancelled")
			}
		}
		if err := browser.Logout(a.cfg.BrowserProfileDir); err != nil {
			return err
		}
		return emit(f, map[string]any{"version": 1, "status": "logged_out"}, "local browser profile removed")
	}}
}
func authCmd(f *flags) *cobra.Command {
	c := &cobra.Command{Use: "auth"}
	c.AddCommand(&cobra.Command{Use: "status", RunE: func(_ *cobra.Command, _ []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		s := browser.AuthStatus(context.Background(), browserOptions(a))
		return emit(f, s, s.Status)
	}})
	return c
}
func routinesCmd(f *flags) *cobra.Command {
	c := &cobra.Command{Use: "routines", Short: "Inspect and safely compare website routines"}
	inspect := func(_ *cobra.Command, _ []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		rs, err := browser.Inspect(context.Background(), browserOptions(a))
		if err != nil {
			return err
		}
		return emit(f, map[string]any{"version": 1, "routines": rs}, fmt.Sprintf("%d routines", len(rs)))
	}
	c.AddCommand(&cobra.Command{Use: "list", RunE: inspect}, &cobra.Command{Use: "inspect", RunE: inspect})
	var createExercises []string
	var createForce bool
	create := &cobra.Command{Use: "create <name>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		if !f.Yes {
			return fmt.Errorf("creating a live routine requires --yes")
		}
		remote, err := browser.Inspect(context.Background(), browserOptions(a))
		if err != nil {
			return err
		}
		for _, routine := range remote {
			if strings.EqualFold(routine.Name, args[0]) {
				if !createForce {
					return fmt.Errorf("routine already exists: %q (use --force to replace)", args[0])
				}
				if err := browser.DeleteRoutine(context.Background(), browserOptions(a), routine.Name); err != nil {
					return fmt.Errorf("delete existing routine: %w", err)
				}
				break
			}
		}
		exercises := make([]plans.Exercise, 0, len(createExercises))
		for _, name := range createExercises {
			sets := 3
			reps := 10
			if strings.EqualFold(name, "running") || strings.EqualFold(name, "treadmill") {
				sets = 1
				reps = 0
			}
			routineExercise := plans.Exercise{Name: name, Sets: make([]plans.Set, sets)}
			for i := range routineExercise.Sets {
				routineExercise.Sets[i].Type = "normal"
				if reps > 0 {
					routineExercise.Sets[i].TargetReps = &plans.Reps{Min: reps, Max: reps}
				}
			}
			exercises = append(exercises, routineExercise)
		}
		if err := browser.CreateRoutine(context.Background(), browserOptions(a), args[0], exercises); err != nil {
			return err
		}
		return emit(f, map[string]any{"version": 1, "created": args[0]}, "created "+args[0])
	}}
	create.Flags().StringArrayVar(&createExercises, "exercise", nil, "exercise to add; repeat flag; defaults to 3 sets of 10 reps")
	create.Flags().BoolVar(&createForce, "force", false, "replace existing routine")
	c.AddCommand(create)
	c.AddCommand(&cobra.Command{Use: "diff <plan>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		p, _, err := plans.Find(a.cfg.PlansDirectory, args[0])
		if err != nil {
			return err
		}
		remote, err := browser.Inspect(context.Background(), browserOptions(a))
		if err != nil {
			return err
		}
		ch := routines.Diff(p, remote)
		return emit(f, map[string]any{"version": 1, "changes": ch}, fmt.Sprintf("%d changes", len(ch)))
	}})
	var dry bool
	apply := &cobra.Command{Use: "apply <plan>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		a, close, err := open(f)
		if err != nil {
			return err
		}
		defer close()
		p, _, err := plans.Find(a.cfg.PlansDirectory, args[0])
		if err != nil {
			return err
		}
		if err = plans.Validate(p); err != nil {
			return err
		}
		status := browser.AuthStatus(context.Background(), browserOptions(a))
		if status.Status != "authenticated" {
			return fmt.Errorf("authentication is %s", status.Status)
		}
		remote, err := browser.Inspect(context.Background(), browserOptions(a))
		if err != nil {
			return err
		}
		ch := routines.Diff(p, remote)
		if dry {
			return emit(f, map[string]any{"version": 1, "dry_run": true, "changes": ch}, fmt.Sprintf("would apply %d changes", len(ch)))
		}
		if !f.Yes {
			return fmt.Errorf("live routine application requires --yes after reviewing --dry-run")
		}
		return fmt.Errorf("live routine application is unavailable until the current Hevy UI selectors and post-action verification are manually validated; no changes were made")
	}}
	apply.Flags().BoolVar(&dry, "dry-run", false, "make no changes")
	c.AddCommand(apply)
	return c
}
