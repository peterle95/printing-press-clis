package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"transit-pp-cli/internal/config"
	"transit-pp-cli/internal/transit"
	"transit-pp-cli/internal/ui"
)

type modeFilterFlags struct {
	bus      bool
	subway   bool
	suburban bool
	tram     bool
	regional bool
}

type boardOptions struct {
	from        string
	stop        string
	minutes     int
	radius      int
	line        string
	direction   string
	showTooLate bool
	modes       modeFilterFlags
}

type boardOutput struct {
	Title       string       `json:"title"`
	GeneratedAt time.Time    `json:"generated_at"`
	From        string       `json:"from,omitempty"`
	Window      int          `json:"window_minutes"`
	Departures  []boardEntry `json:"departures"`
	Hidden      int          `json:"hidden_too_late"`
}

type boardEntry struct {
	Status         string     `json:"status"`
	Line           string     `json:"line"`
	Product        string     `json:"product"`
	Direction      string     `json:"direction"`
	StopName       string     `json:"stop_name"`
	StopID         string     `json:"stop_id"`
	PlannedTime    string     `json:"planned_time"`
	RealtimeTime   string     `json:"realtime_time"`
	Delay          string     `json:"delay"`
	MinutesLeft    int        `json:"minutes_left"`
	Platform       string     `json:"platform"`
	Remarks        []string   `json:"remarks"`
	TripID         string     `json:"tripId,omitempty"`
	DistanceMeters int        `json:"distance_meters,omitempty"`
	WalkingMinutes int        `json:"walking_minutes,omitempty"`
	DepartureTime  *time.Time `json:"departure_time,omitempty"`
	ScheduledOnly  bool       `json:"scheduled_only"`
	Cancelled      bool       `json:"cancelled"`
}

func newBoardCmd(flags *rootFlags) *cobra.Command {
	opts := boardOptions{}
	cmd := &cobra.Command{
		Use:   "board",
		Short: "Show merged real-time departures",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := buildBoard(cmd, flags, opts, false)
			if err != nil {
				return err
			}
			return printBoard(flags, out)
		},
	}
	addBoardFlags(cmd, &opts)
	return cmd
}

func addBoardFlags(cmd *cobra.Command, opts *boardOptions) {
	cmd.Flags().StringVar(&opts.from, "from", "", "Origin label, usually home")
	cmd.Flags().StringVar(&opts.stop, "stop", "", "Fetch departures for one stop name")
	cmd.Flags().IntVar(&opts.minutes, "minutes", 0, "Departure window in minutes")
	cmd.Flags().IntVar(&opts.radius, "radius", 0, "Nearby stop radius in meters")
	cmd.Flags().StringVar(&opts.line, "line", "", "Filter by line name")
	cmd.Flags().StringVar(&opts.direction, "direction", "", "Filter by direction text")
	cmd.Flags().BoolVar(&opts.modes.bus, "bus", false, "Only bus departures")
	cmd.Flags().BoolVar(&opts.modes.subway, "subway", false, "Only subway departures")
	cmd.Flags().BoolVar(&opts.modes.suburban, "suburban", false, "Only S-Bahn departures")
	cmd.Flags().BoolVar(&opts.modes.tram, "tram", false, "Only tram departures")
	cmd.Flags().BoolVar(&opts.modes.regional, "regional", false, "Only regional departures")
	cmd.Flags().BoolVar(&opts.showTooLate, "show-too-late", false, "Include departures that are physically too late to catch")
}

func buildBoard(cmd *cobra.Command, flags *rootFlags, opts boardOptions, includeTooLate bool) (boardOutput, error) {
	a, err := loadApp(flags)
	if err != nil {
		return boardOutput{}, err
	}
	ctx, cancel := commandContext(flags)
	defer cancel()
	if opts.minutes <= 0 {
		opts.minutes = a.config.Defaults.DepartureWindowMinutes
	}
	if opts.radius <= 0 {
		opts.radius = a.config.Defaults.RadiusMeters
	}
	modes := selectedModes(a.config.Defaults.Modes, opts.modes)
	now := time.Now().In(transit.BerlinLocation())
	entries := []boardEntry{}
	hidden := 0
	title := fmt.Sprintf("Departures, next %d min", opts.minutes)
	fromLabel := opts.from
	if opts.stop != "" {
		stop, err := resolveStop(ctx, a, opts.stop)
		if err != nil {
			return boardOutput{}, err
		}
		deps, err := cachedDepartures(ctx, a, stop.ID, opts.minutes, modes)
		if err != nil {
			return boardOutput{}, err
		}
		title = fmt.Sprintf("Departures at %s, next %d min", stop.DisplayName(), opts.minutes)
		for _, dep := range deps {
			entry := makeBoardEntry(dep, now, 0, a.config.Defaults.SafetyBufferMinutes, a.config.Defaults.WalkingSpeed)
			if keepBoardEntry(entry, opts) {
				entries = append(entries, entry)
			}
		}
	} else {
		if strings.TrimSpace(fromLabel) == "" {
			fromLabel = "home"
		}
		if fromLabel != "home" {
			return boardOutput{}, fmt.Errorf("only --from home is currently supported")
		}
		home, err := homeLocation(ctx, a)
		if err != nil {
			return boardOutput{}, err
		}
		lat, lon, _ := home.Coordinates()
		stops, err := cachedNearby(ctx, a, lat, lon, 10, opts.radius)
		if err != nil {
			return boardOutput{}, err
		}
		title = fmt.Sprintf("Departures near %s, next %d min", home.DisplayName(), opts.minutes)
		for _, stop := range stops {
			distance := stop.Distance
			if distance <= 0 {
				if stopLat, stopLon, ok := stop.Coordinates(); ok {
					distance = transit.DistanceMeters(lat, lon, stopLat, stopLon)
				}
			}
			deps, err := cachedDepartures(ctx, a, stop.ID, opts.minutes, modes)
			if err != nil {
				if flags.debug {
					fmt.Fprintf(os.Stderr, "departures failed for %s: %s\n", stop.DisplayName(), err)
				}
				continue
			}
			for _, dep := range deps {
				entry := makeBoardEntry(dep, now, distance, a.config.Defaults.SafetyBufferMinutes, a.config.Defaults.WalkingSpeed)
				if !includeTooLate && !opts.showTooLate && entry.Status == transit.StatusTooLate {
					hidden++
					continue
				}
				if keepBoardEntry(entry, opts) {
					entries = append(entries, entry)
				}
			}
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].DepartureTime == nil {
			return false
		}
		if entries[j].DepartureTime == nil {
			return true
		}
		return entries[i].DepartureTime.Before(*entries[j].DepartureTime)
	})
	return boardOutput{
		Title:       title,
		GeneratedAt: now,
		From:        fromLabel,
		Window:      opts.minutes,
		Departures:  entries,
		Hidden:      hidden,
	}, nil
}

func makeBoardEntry(dep transit.Departure, now time.Time, distanceMeters int, bufferMinutes int, walkingSpeed string) boardEntry {
	depTime := transit.EffectiveDeparture(dep)
	walk := transit.WalkingMinutes(distanceMeters, walkingSpeed)
	realtime := dep.When != nil
	platform := firstNonEmpty(dep.Platform, dep.PlannedPlatform, "-")
	return boardEntry{
		Status:         transit.Catchability(now, depTime, walk, bufferMinutes, dep.Cancelled),
		Line:           firstNonEmpty(dep.Line.Name, "-"),
		Product:        dep.Line.Product,
		Direction:      firstNonEmpty(dep.Direction, "-"),
		StopName:       dep.Stop.DisplayName(),
		StopID:         dep.Stop.ID,
		PlannedTime:    transit.FormatClock(dep.PlannedWhen),
		RealtimeTime:   transit.FormatClock(depTime),
		Delay:          transit.FormatDelay(dep.Delay, realtime, dep.Cancelled),
		MinutesLeft:    transit.MinutesUntil(now, depTime),
		Platform:       platform,
		Remarks:        transit.RemarkTexts(dep.Remarks),
		TripID:         dep.TripID,
		DistanceMeters: distanceMeters,
		WalkingMinutes: walk,
		DepartureTime:  depTime,
		ScheduledOnly:  !realtime,
		Cancelled:      dep.Cancelled,
	}
}

func keepBoardEntry(entry boardEntry, opts boardOptions) bool {
	if opts.line != "" && !strings.EqualFold(entry.Line, opts.line) {
		return false
	}
	if opts.direction != "" && !strings.Contains(strings.ToLower(entry.Direction), strings.ToLower(opts.direction)) {
		return false
	}
	return true
}

func selectedModes(defaults config.ModeConfig, filters modeFilterFlags) transit.ProductFlags {
	if filters.bus || filters.subway || filters.suburban || filters.tram || filters.regional {
		return transit.ProductFlags{
			Bus:      filters.bus,
			Subway:   filters.subway,
			Suburban: filters.suburban,
			Tram:     filters.tram,
			Regional: filters.regional,
		}
	}
	return modeFlagsFromConfig(defaults)
}

func printBoard(flags *rootFlags, out boardOutput) error {
	return outputValue(flags, out, func() error {
		fmt.Println(out.Title)
		fmt.Println()
		if len(out.Departures) == 0 {
			fmt.Println("No catchable departures found.")
			if out.Hidden > 0 {
				fmt.Printf("%d too-late departures hidden.\n", out.Hidden)
			}
			return nil
		}
		table := ui.NewTable(os.Stdout)
		table.Row("STATUS", "LINE", "MODE", "DEPARTS", "DELAY", "MIN", "STOP", "DIRECTION", "PLAT", "REMARKS")
		for _, entry := range out.Departures {
			table.Row(
				entry.Status,
				entry.Line,
				transit.ProductLabel(entry.Product),
				entry.RealtimeTime,
				entry.Delay,
				entry.MinutesLeft,
				entry.StopName,
				entry.Direction,
				entry.Platform,
				joinOrDash(entry.Remarks),
			)
		}
		if err := table.Flush(); err != nil {
			return err
		}
		if out.Hidden > 0 {
			fmt.Printf("\n%d too-late departures hidden. Pass --show-too-late to include them.\n", out.Hidden)
		}
		return nil
	})
}
