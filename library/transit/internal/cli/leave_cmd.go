package cli

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"transit-pp-cli/internal/transit"
	"transit-pp-cli/internal/ui"
)

type leaveOutput struct {
	From      transit.Location         `json:"from"`
	To        transit.Location         `json:"to"`
	ArrivalBy time.Time                `json:"arrival_by"`
	Buffer    int                      `json:"buffer_minutes"`
	Selected  transit.JourneySummary   `json:"selected"`
	Options   []transit.JourneySummary `json:"options"`
	Warnings  []string                 `json:"warnings,omitempty"`
}

func newLeaveCmd(flags *rootFlags) *cobra.Command {
	var to string
	var arriveBy string
	var buffer int
	cmd := &cobra.Command{
		Use:   "leave",
		Short: "Compute the latest safe time to leave home",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp(flags)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(flags)
			defer cancel()
			if !cmd.Flags().Changed("buffer") {
				buffer = a.config.Defaults.SafetyBufferMinutes
			}
			if arriveBy == "" {
				return fmt.Errorf("--arrive-by is required")
			}
			target, err := transit.ParseArriveBy(arriveBy, time.Now())
			if err != nil {
				return err
			}
			home, err := homeLocation(ctx, a)
			if err != nil {
				return err
			}
			destQuery, destination, err := resolveDestination(ctx, a, to)
			if err != nil {
				return err
			}
			destQuery = journeyQueryForHome(home, destQuery)
			destQuery.Results = 3
			destQuery.Arrival = &target
			resp, err := a.client.Journeys(ctx, destQuery)
			if err != nil {
				return err
			}
			out, err := makeLeaveOutput(home, destination, target, buffer, resp.Journeys)
			if err != nil {
				return err
			}
			return outputValue(flags, out, func() error {
				return printLeave(out)
			})
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "Destination address, stop, or location name")
	cmd.Flags().StringVar(&arriveBy, "arrive-by", "", "Arrive by local Berlin time, e.g. 09:30")
	cmd.Flags().IntVar(&buffer, "buffer", 0, "Safety buffer in minutes")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

func makeLeaveOutput(home transit.Location, destination transit.Location, target time.Time, buffer int, journeys []transit.Journey) (leaveOutput, error) {
	summaries := make([]transit.JourneySummary, 0, len(journeys))
	for _, journey := range journeys {
		s := transit.SummarizeJourney(journey)
		if s.Arrival != nil && s.Arrival.After(target.Add(59*time.Second)) {
			s.Risky = true
			s.Warnings = append(s.Warnings, "arrives after target")
		}
		summaries = append(summaries, s)
	}
	if len(summaries) == 0 {
		return leaveOutput{}, fmt.Errorf("no journeys found")
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].Departure == nil {
			return false
		}
		if summaries[j].Departure == nil {
			return true
		}
		return summaries[i].Departure.After(*summaries[j].Departure)
	})
	selected := summaries[0]
	for _, option := range summaries {
		if !option.Risky {
			selected = option
			break
		}
	}
	warnings := []string{}
	if selected.Risky {
		warnings = append(warnings, "only risky connections found")
	}
	if selected.Departure != nil {
		leaveAt := selected.Departure.Add(-time.Duration(buffer) * time.Minute)
		if leaveAt.Before(time.Now().In(transit.BerlinLocation())) {
			warnings = append(warnings, "safe leave time is already past")
		}
	}
	return leaveOutput{
		From:      home,
		To:        destination,
		ArrivalBy: target,
		Buffer:    buffer,
		Selected:  selected,
		Options:   summaries,
		Warnings:  warnings,
	}, nil
}

func printLeave(out leaveOutput) error {
	selected := out.Selected
	if selected.Departure == nil {
		return fmt.Errorf("selected journey has no departure time")
	}
	leaveAt := selected.Departure.Add(-time.Duration(out.Buffer) * time.Minute)
	takeLine := firstNonEmpty(selected.FirstTransitLine, "transit")
	takeAt := transit.FormatClock(selected.FirstTransitAt)
	fmt.Printf("Leave home at %s\n", leaveAt.In(transit.BerlinLocation()).Format("15:04"))
	if selected.FirstStop != "" {
		fmt.Printf("Walk to %s by %s\n", selected.FirstStop, transit.FormatClock(selected.FirstStopBy))
	}
	fmt.Printf("Take %s at %s\n", takeLine, takeAt)
	fmt.Printf("Arrive at %s\n", transit.FormatClock(selected.Arrival))
	if len(out.Warnings) > 0 {
		fmt.Printf("Warning: %s\n", joinOrDash(out.Warnings))
	}
	if len(selected.Warnings) > 0 {
		fmt.Printf("Journey warnings: %s\n", joinOrDash(selected.Warnings))
	}
	if len(out.Options) > 1 {
		fmt.Println()
		table := ui.NewTable(os.Stdout)
		table.Row("OPTION", "LEAVE", "LINES", "ARRIVE", "WARNINGS")
		for i, option := range out.Options {
			leave := "-"
			if option.Departure != nil {
				leave = option.Departure.Add(-time.Duration(out.Buffer) * time.Minute).In(transit.BerlinLocation()).Format("15:04")
			}
			table.Row(i+1, leave, joinOrDash(option.Lines), transit.FormatClock(option.Arrival), joinOrDash(option.Warnings))
		}
		return table.Flush()
	}
	return nil
}
