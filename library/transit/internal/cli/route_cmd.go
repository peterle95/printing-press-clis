package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"transit-pp-cli/internal/provider/vbb"
	"transit-pp-cli/internal/transit"
	"transit-pp-cli/internal/ui"
)

type routeOutput struct {
	From       transit.Location         `json:"from"`
	To         transit.Location         `json:"to"`
	ArrivalBy  *time.Time               `json:"arrival_by,omitempty"`
	Journeys   []transit.Journey        `json:"journeys"`
	Summaries  []transit.JourneySummary `json:"summaries"`
	EarlierRef string                   `json:"earlierRef,omitempty"`
	LaterRef   string                   `json:"laterRef,omitempty"`
}

func newRouteCmd(flags *rootFlags) *cobra.Command {
	var from string
	var to string
	var arriveBy string
	var results int
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Plan journeys from home to a destination",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp(flags)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(flags)
			defer cancel()
			if from == "" {
				from = "home"
			}
			if from != "home" {
				return fmt.Errorf("only --from home is currently supported")
			}
			if results <= 0 {
				results = 3
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
			destQuery.Results = results
			var arrival *time.Time
			if arriveBy != "" {
				target, err := transit.ParseArriveBy(arriveBy, time.Now())
				if err != nil {
					return err
				}
				arrival = &target
				destQuery.Arrival = arrival
			}
			resp, err := a.client.Journeys(ctx, destQuery)
			if err != nil {
				return err
			}
			out := makeRouteOutput(home, destination, arrival, resp)
			return outputValue(flags, out, func() error {
				fmt.Printf("Routes from %s to %s\n\n", home.DisplayName(), destination.DisplayName())
				return printRouteSummaries(out.Summaries)
			})
		},
	}
	cmd.Flags().StringVar(&from, "from", "home", "Origin label")
	cmd.Flags().StringVar(&to, "to", "", "Destination address, stop, or location name")
	cmd.Flags().StringVar(&arriveBy, "arrive-by", "", "Arrive by local Berlin time, e.g. 09:30")
	cmd.Flags().IntVar(&results, "results", 3, "Number of journeys")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

func makeRouteOutput(home transit.Location, destination transit.Location, arrival *time.Time, resp transit.JourneyResponse) routeOutput {
	summaries := make([]transit.JourneySummary, 0, len(resp.Journeys))
	for _, journey := range resp.Journeys {
		summaries = append(summaries, transit.SummarizeJourney(journey))
	}
	return routeOutput{
		From:       home,
		To:         destination,
		ArrivalBy:  arrival,
		Journeys:   resp.Journeys,
		Summaries:  summaries,
		EarlierRef: resp.EarlierRef,
		LaterRef:   resp.LaterRef,
	}
}

func printRouteSummaries(summaries []transit.JourneySummary) error {
	if len(summaries) == 0 {
		fmt.Println("No journeys found.")
		return nil
	}
	table := ui.NewTable(os.Stdout)
	table.Row("DEP", "WALK TO", "LINES", "XFER", "ARR", "FINAL WALK", "WARNINGS")
	for _, summary := range summaries {
		walkTo := "-"
		if summary.FirstStop != "" {
			walkTo = fmt.Sprintf("%s by %s", summary.FirstStop, transit.FormatClock(summary.FirstStopBy))
		}
		table.Row(
			transit.FormatClock(summary.Departure),
			walkTo,
			firstNonEmpty(strings.Join(summary.Lines, " -> "), "walk only"),
			summary.Transfers,
			transit.FormatClock(summary.Arrival),
			summary.FinalWalk,
			joinOrDash(summary.Warnings),
		)
	}
	return table.Flush()
}

func journeyQueryForHome(home transit.Location, base vbb.JourneyQuery) vbb.JourneyQuery {
	lat, lon, _ := home.Coordinates()
	base.FromLatitude = lat
	base.FromLongitude = lon
	base.FromAddress = firstNonEmpty(home.Address, home.DisplayName(), "home")
	base.Remarks = true
	base.Stopovers = true
	base.StartWalking = true
	return base
}
