package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"flight-pp-cli/internal/config"
	"flight-pp-cli/internal/flight"
)

type watchFlags struct {
	search   searchFlags
	maxPrice float64
	notify   string
}

func newWatchCmd(root *rootFlags) *cobra.Command {
	flags := &watchFlags{}
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Save a local price-watch search and print a schedulable command",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(root)
			if err != nil {
				return err
			}
			flags.search.maxStopsSet = cmd.Flags().Changed("max-stops")
			req := requestFromSearchFlags(&flags.search, app.config)
			if err := flight.ValidateSearchRequest(req); err != nil {
				return err
			}
			req = flight.NormalizeRequest(req)
			watch := flight.Watch{
				ID:        "watch-" + time.Now().UTC().Format("20060102T150405Z"),
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
				Request:   req,
				MaxPrice:  flags.maxPrice,
				Notify:    flags.notify,
				Providers: config.ParseProviderList(flags.search.providers),
			}
			path, err := config.WatchesPath()
			if err != nil {
				return err
			}
			if err := appendWatch(path, watch); err != nil {
				return err
			}
			payload := map[string]any{
				"watch":         watch,
				"path":          path,
				"manualCommand": manualWatchCommand(watch),
			}
			return outputValue(root, payload, func() error {
				fmt.Printf("Saved watch: %s\n", watch.ID)
				fmt.Printf("Path: %s\n", path)
				fmt.Println()
				fmt.Println("Manual check command:")
				fmt.Println(manualWatchCommand(watch))
				fmt.Println()
				fmt.Println("Schedule it with cron, systemd timers, launchd, or Windows Task Scheduler. The MVP does not run a daemon.")
				return nil
			})
		},
	}
	addSearchFlags(cmd, &flags.search)
	cmd.Flags().Float64Var(&flags.maxPrice, "max-price", 0, "Alert threshold price")
	cmd.Flags().StringVar(&flags.notify, "notify", "", "Notification channel label, for example telegram")
	return cmd
}

func appendWatch(path string, watch flight.Watch) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	watches := []flight.Watch{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &watches); err != nil {
			return fmt.Errorf("parse watches %s: %w", path, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	watches = append(watches, watch)
	data, err := json.MarshalIndent(watches, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func manualWatchCommand(watch flight.Watch) string {
	req := watch.Request
	parts := []string{
		"flight-pp-cli", "search",
		"--from", req.Origin,
		"--to", strings.ToLower(req.Destination),
		"--depart", req.DepartDate,
	}
	if req.ReturnDate != "" {
		parts = append(parts, "--return", req.ReturnDate)
	} else {
		parts = append(parts, "--one-way")
	}
	parts = append(parts,
		"--adults", fmt.Sprintf("%d", req.Adults),
		"--currency", req.Currency,
		"--cabin", req.Cabin,
		"--sort", "price",
		"--limit", "10",
	)
	if len(watch.Providers) > 0 {
		parts = append(parts, "--providers", strings.Join(watch.Providers, ","))
	}
	if watch.MaxPrice > 0 {
		parts = append(parts, "# alert when <= "+fmt.Sprintf("%.2f", watch.MaxPrice)+" "+req.Currency)
	}
	return strings.Join(parts, " ")
}
