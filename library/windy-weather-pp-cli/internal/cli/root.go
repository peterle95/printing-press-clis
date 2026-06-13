// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"windy-weather-pp-cli/internal/advice"
	"windy-weather-pp-cli/internal/cache"
	"windy-weather-pp-cli/internal/config"
	"windy-weather-pp-cli/internal/parser"
	"windy-weather-pp-cli/internal/weather"
	"windy-weather-pp-cli/internal/windy"
)

const (
	ExitOK           = 0
	ExitRuntimeError = 1
)

var version = "0.1.0"

type rootFlags struct {
	asJSON     bool
	agent      bool
	debug      bool
	noCache    bool
	configPath string
}

func Execute() error {
	cmd := RootCmd()
	return cmd.Execute()
}

func RootCmd() *cobra.Command {
	flags := &rootFlags{}
	root := &cobra.Command{
		Use:           "windy-weather-pp-cli",
		Short:         "Check rain and weather risk near home in Berlin using Windy.com",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.SetVersionTemplate("windy-weather-pp-cli {{ .Version }}\n")
	root.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "Output stable machine-readable JSON")
	root.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Agent mode: JSON output and non-interactive defaults")
	root.PersistentFlags().BoolVar(&flags.debug, "debug", false, "Print debug details to stderr")
	root.PersistentFlags().BoolVar(&flags.noCache, "no-cache", false, "Bypass the local weather cache")
	root.PersistentFlags().StringVar(&flags.configPath, "config", "", "Config file path")

	root.AddCommand(newNowCmd(flags))
	root.AddCommand(newRainCmd(flags))
	root.AddCommand(newDayCmd(flags, "today", 0))
	root.AddCommand(newDayCmd(flags, "tomorrow", 1))
	root.AddCommand(newWeekCmd(flags))
	root.AddCommand(newCalendarAdviceCmd(flags))
	root.AddCommand(newDebugNetworkCmd(flags))
	root.AddCommand(newDebugScreenshotCmd(flags))
	return root
}

func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	return ExitRuntimeError
}

func newNowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "now",
		Short: "Current weather summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			obs, _, err := loadObservation(cmd.Context(), flags)
			out := weather.NowOutputFromObservation(obs)
			if asJSON(flags) {
				_ = printJSON(out)
				return err
			}
			printCurrent(out, obs)
			return err
		},
	}
}

func newRainCmd(flags *rootFlags) *cobra.Command {
	var hours int
	cmd := &cobra.Command{
		Use:   "rain",
		Short: "Check rain and thunderstorm risk for the next N hours",
		RunE: func(cmd *cobra.Command, args []string) error {
			obs, _, err := loadObservation(cmd.Context(), flags)
			out := advice.AggregateRain(obs, hours)
			if asJSON(flags) {
				_ = printJSON(out)
				return err
			}
			fmt.Printf("%s: %s\n", obs.Location.Name, out.Summary)
			fmt.Printf("Rain risk: %s, thunderstorm risk: %s\n", out.RainRisk, out.ThunderstormRisk)
			fmt.Printf("Model confidence: %s (%.0f%%)", out.Confidence, pctOrDefault(obs.RawObservations.ModelAgreementPct))
			if obs.RawObservations.Model != nil {
				fmt.Printf(" (primary: %s)", *obs.RawObservations.Model)
			}
			models := extractModels(obs.RawObservations.Notes)
			if len(models) > 0 {
				fmt.Printf(", models: %s", strings.Join(models, ", "))
			}
			fmt.Println()
			if len(obs.RawObservations.ModelDivergence) > 0 {
				fmt.Println("Model divergence:")
				for _, d := range obs.RawObservations.ModelDivergence {
					fmt.Printf("  - %s\n", d)
				}
			}
			return err
		},
	}
	cmd.Flags().IntVar(&hours, "hours", 6, "Forecast period in hours")
	return cmd
}

func newWeekCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "week",
		Short: "7-day forecast summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			obs, _, err := loadObservation(cmd.Context(), flags)
			out := advice.WeekForecast(obs)
			if asJSON(flags) {
				_ = printJSON(out)
				return err
			}
			fmt.Printf("7-day forecast for %s\n\n", obs.Location.Name)
			fmt.Printf("Model confidence: %s (%.0f%%)", out.Confidence, pctOrDefault(obs.RawObservations.ModelAgreementPct))
			if obs.RawObservations.Model != nil {
				fmt.Printf(" (primary: %s)", *obs.RawObservations.Model)
			}
			models := extractModels(obs.RawObservations.Notes)
			if len(models) > 0 {
				fmt.Printf(", models: %s", strings.Join(models, ", "))
			}
			fmt.Println()
			if len(obs.RawObservations.ModelDivergence) > 0 {
				fmt.Println("Model divergence:")
				for _, d := range obs.RawObservations.ModelDivergence {
					fmt.Printf("  - %s\n", d)
				}
				fmt.Println()
			}
			fmt.Printf("%-12s %-20s %6s %6s %8s %10s %s\n", "Day", "Summary", "Min", "Max", "Rain", "Wind", "Cloud")
			fmt.Println(strings.Repeat("-", 85))
			for _, day := range out.Days {
				minT := "--"
				maxT := "--"
				if day.TemperatureMinC != nil {
					minT = fmt.Sprintf("%.0f", *day.TemperatureMinC)
				}
				if day.TemperatureMaxC != nil {
					maxT = fmt.Sprintf("%.0f", *day.TemperatureMaxC)
				}
				rain := "--"
				if day.MaxPrecipMm != nil {
					rain = fmt.Sprintf("%.1f", *day.MaxPrecipMm)
				}
				wind := "--"
				if day.WindSpeedMaxKmh != nil {
					wind = fmt.Sprintf("%.0f", *day.WindSpeedMaxKmh)
				}
				fmt.Printf("%-12s %-20s %3s°C  %3s°C %5smm %6skm/h %s\n",
					day.DayName, day.Summary, minT, maxT, rain, wind, day.CloudCover)
			}
			fmt.Println()
			if len(out.BestDays) > 0 {
				fmt.Println("Best weather days:")
				for _, d := range out.BestDays {
					fmt.Printf("  %s: %s\n", d.DayName, d.Reason)
				}
			}
			return err
		},
	}
}

func newDayCmd(flags *rootFlags, name string, offset int) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Short forecast for %s", name),
		RunE: func(cmd *cobra.Command, args []string) error {
			obs, _, err := loadObservation(cmd.Context(), flags)
			out := advice.DayForecast(obs, name, offset)
			if asJSON(flags) {
				_ = printJSON(out)
				return err
			}
			fmt.Printf("%s (%s): %s\n", strings.Title(name), out.Date, out.Summary)
			fmt.Printf("Rain risk: %s, thunderstorm risk: %s\n", out.RainRisk, out.ThunderstormRisk)
			if out.TemperatureMinC != nil && out.TemperatureMaxC != nil {
				fmt.Printf("Temperature: %.1f to %.1f C\n", *out.TemperatureMinC, *out.TemperatureMaxC)
			}
			if out.WindSpeedMaxKmh != nil {
				fmt.Printf("Wind: up to %.1f km/h\n", *out.WindSpeedMaxKmh)
			}
			fmt.Printf("Model confidence: %s (%.0f%%)", out.Confidence, pctOrDefault(obs.RawObservations.ModelAgreementPct))
			if obs.RawObservations.Model != nil {
				fmt.Printf(" (primary: %s)", *obs.RawObservations.Model)
			}
			models := extractModels(obs.RawObservations.Notes)
			if len(models) > 0 {
				fmt.Printf(", models: %s", strings.Join(models, ", "))
			}
			fmt.Println()
			if len(obs.RawObservations.ModelDivergence) > 0 {
				fmt.Println("Model divergence:")
				for _, d := range obs.RawObservations.ModelDivergence {
					fmt.Printf("  - %s\n", d)
				}
			}
			return err
		},
	}
}

func newCalendarAdviceCmd(flags *rootFlags) *cobra.Command {
	var hours int
	cmd := &cobra.Command{
		Use:   "calendar-advice",
		Short: "Return JSON advice for a later Google Calendar agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			obs, cfg, err := loadObservation(cmd.Context(), flags)
			out := advice.CalendarAdvice(obs, cfg, hours)
			if asJSON(flags) {
				_ = printJSON(out)
				return err
			}
			if out.ShouldCreateEvent {
				fmt.Printf("%s: %s\n", out.Title, out.Reason)
			} else {
				fmt.Println(out.Reason)
			}
			fmt.Printf("Model confidence: %s (%.0f%%)", out.Confidence, pctOrDefault(obs.RawObservations.ModelAgreementPct))
			if obs.RawObservations.Model != nil {
				fmt.Printf(" (primary: %s)", *obs.RawObservations.Model)
			}
			models := extractModels(obs.RawObservations.Notes)
			if len(models) > 0 {
				fmt.Printf(", models: %s", strings.Join(models, ", "))
			}
			fmt.Println()
			if len(obs.RawObservations.ModelDivergence) > 0 {
				fmt.Println("Model divergence:")
				for _, d := range obs.RawObservations.ModelDivergence {
					fmt.Printf("  - %s\n", d)
				}
			}
			return err
		},
	}
	cmd.Flags().IntVar(&hours, "hours", 6, "Forecast period in hours")
	return cmd
}

func newDebugNetworkCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "debug-network",
		Short: "Capture Windy network metadata for reverse engineering",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(cfg.Browser.TimeoutMs+10000)*time.Millisecond)
			defer cancel()
			result, runErr := windy.Run(ctx, cfg, windy.Options{DebugNetwork: true, Debug: flags.debug})
			out := map[string]any{
				"source":         weather.SourceWindy,
				"location":       weather.LocationFromConfig(cfg.DefaultLocation.Name, cfg.DefaultLocation.Latitude, cfg.DefaultLocation.Longitude, cfg.DefaultLocation.Timezone),
				"checked_at":     checkedAt(cfg).Format(time.RFC3339),
				"debug_file":     "",
				"response_count": 0,
				"endpoints_used": []string{},
				"errors":         []weather.ErrorItem{},
			}
			if result != nil {
				out["debug_file"] = result.DebugLogPath
				out["response_count"] = len(result.Responses)
				out["endpoints_used"] = result.EndpointsUsed
			}
			if runErr != nil {
				out["errors"] = []weather.ErrorItem{windy.ErrorItem(runErr)}
			}
			if asJSON(flags) {
				_ = printJSON(out)
				return runErr
			}
			fmt.Printf("Saved Windy network log: %s\n", out["debug_file"])
			fmt.Printf("Captured %d relevant responses\n", out["response_count"])
			return runErr
		},
	}
}

func newDebugScreenshotCmd(flags *rootFlags) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "debug-screenshot",
		Short: "Save a screenshot of the loaded Windy page",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			if output == "" {
				output = filepath.Join("debug", "windy-screenshot.png")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(cfg.Browser.TimeoutMs+10000)*time.Millisecond)
			defer cancel()
			result, runErr := windy.Run(ctx, cfg, windy.Options{ScreenshotPath: output, Debug: flags.debug})
			out := map[string]any{
				"source":           weather.SourceWindy,
				"location":         weather.LocationFromConfig(cfg.DefaultLocation.Name, cfg.DefaultLocation.Latitude, cfg.DefaultLocation.Longitude, cfg.DefaultLocation.Timezone),
				"checked_at":       checkedAt(cfg).Format(time.RFC3339),
				"screenshot_file":  output,
				"screenshot_saved": false,
				"errors":           []weather.ErrorItem{},
			}
			if result != nil {
				out["screenshot_file"] = result.ScreenshotPath
				out["screenshot_saved"] = result.ScreenshotSaved
			}
			if runErr != nil {
				out["errors"] = []weather.ErrorItem{windy.ErrorItem(runErr)}
			}
			if asJSON(flags) {
				_ = printJSON(out)
				return runErr
			}
			fmt.Printf("Saved Windy screenshot: %s\n", out["screenshot_file"])
			return runErr
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "Screenshot output path")
	return cmd
}

func loadObservation(ctx context.Context, flags *rootFlags) (weather.Observation, *config.Config, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		fallback := config.DefaultConfig()
		errItem := weather.ErrorItem{Code: "config_error", Message: err.Error()}
		return parser.EmptyObservation(fallback, time.Now(), &errItem), fallback, err
	}
	now := checkedAt(cfg)
	cachePath := cache.GetCachePath()
	if cfg.Cache.Enabled && !flags.noCache && cache.IsValid(cachePath, cfg.Cache.TTLMinutes) {
		if cached, err := cache.Load[weather.Observation](cachePath); err == nil {
			if flags.debug {
				fmt.Fprintln(os.Stderr, "cache hit:", cachePath)
			}
			return *cached, cfg, nil
		}
	}
	if flags.debug {
		fmt.Fprintln(os.Stderr, "cache miss; loading Windy")
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Browser.TimeoutMs+10000)*time.Millisecond)
	defer cancel()
	result, scrapeErr := windy.Run(runCtx, cfg, windy.Options{Debug: flags.debug})
	if scrapeErr != nil {
		errItem := windy.ErrorItem(scrapeErr)
		return parser.EmptyObservation(cfg, now, &errItem), cfg, scrapeErr
	}
	obs := parser.Parse(result, cfg, now)
	if len(obs.Errors) == 0 && cfg.Cache.Enabled {
		_ = cache.Save(cachePath, &obs)
	}
	return obs, cfg, nil
}

func checkedAt(cfg *config.Config) time.Time {
	now := time.Now()
	if cfg != nil && cfg.DefaultLocation.Timezone != "" {
		if loc, err := time.LoadLocation(cfg.DefaultLocation.Timezone); err == nil {
			return now.In(loc)
		}
	}
	return now
}

func asJSON(flags *rootFlags) bool {
	return flags.asJSON || flags.agent
}

func extractModels(notes []string) []string {
	for _, note := range notes {
		if strings.HasPrefix(note, "Models used: ") {
			return strings.Split(strings.TrimPrefix(note, "Models used: "), ", ")
		}
	}
	return nil
}

func pctOrDefault(pct *float64) float64 {
	if pct == nil {
		return 0
	}
	return *pct
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printCurrent(out weather.NowOutput, obs weather.Observation) {
	fmt.Printf("%s: %s\n", out.Location.Name, out.Current.Summary)
	fmt.Printf("Rain risk: %s, thunderstorm risk: %s\n", out.Current.RainRisk, out.Current.ThunderstormRisk)
	if out.Current.TemperatureC != nil {
		fmt.Printf("Temperature: %.1f C\n", *out.Current.TemperatureC)
	}
	if out.Current.WindSpeedKmh != nil {
		fmt.Printf("Wind: %.1f km/h\n", *out.Current.WindSpeedKmh)
	}
	if out.Current.CloudCover != "" {
		fmt.Printf("Cloud cover: %s", out.Current.CloudCover)
		if out.Current.CloudCoverPct != nil {
			fmt.Printf(" (%d%%)", *out.Current.CloudCoverPct)
		}
		fmt.Println()
	}
	if out.Current.HumidityPct != nil {
		fmt.Printf("Humidity: %d%%\n", *out.Current.HumidityPct)
	}
	fmt.Printf("Model confidence: %s (%.0f%%)", obs.Confidence, pctOrDefault(obs.RawObservations.ModelAgreementPct))
	if obs.RawObservations.Model != nil {
		fmt.Printf(" (primary: %s)", *obs.RawObservations.Model)
	}
	models := extractModels(obs.RawObservations.Notes)
	if len(models) > 0 {
		fmt.Printf(", models: %s", strings.Join(models, ", "))
	}
	fmt.Println()
	if len(obs.RawObservations.ModelDivergence) > 0 {
		fmt.Println("Model divergence:")
		for _, d := range obs.RawObservations.ModelDivergence {
			fmt.Printf("  - %s\n", d)
		}
	}
	if len(out.Errors) > 0 {
		fmt.Printf("Errors: %d (use --json for details)\n", len(out.Errors))
	}
}
