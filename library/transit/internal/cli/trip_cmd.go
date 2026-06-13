package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"transit-pp-cli/internal/transit"
	"transit-pp-cli/internal/ui"
)

func newTripCmd(flags *rootFlags) *cobra.Command {
	var id string
	var line string
	cmd := &cobra.Command{
		Use:   "trip",
		Short: "Inspect one VBB trip by trip ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp(flags)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(flags)
			defer cancel()
			resp, err := a.client.Trip(ctx, id, line)
			if err != nil {
				return err
			}
			return outputValue(flags, resp, func() error {
				trip := resp.Trip
				fmt.Printf("%s toward %s\n", firstNonEmpty(trip.Line.Name, "Trip"), firstNonEmpty(trip.Direction, trip.Destination.DisplayName()))
				fmt.Printf("%s -> %s (%s - %s)\n\n", trip.Origin.DisplayName(), trip.Destination.DisplayName(), transit.FormatClock(trip.Departure), transit.FormatClock(trip.Arrival))
				if len(trip.Stopovers) == 0 {
					fmt.Println("No stopovers returned.")
					return nil
				}
				table := ui.NewTable(os.Stdout)
				table.Row("STOP", "ARR", "DEP", "PLAT", "DELAY", "REMARKS")
				for _, stopover := range trip.Stopovers {
					delay := stopover.DepartureDelay
					if delay == nil {
						delay = stopover.ArrivalDelay
					}
					platform := firstNonEmpty(stopover.DeparturePlatform, stopover.ArrivalPlatform, stopover.PlannedDeparturePlatform, stopover.PlannedArrivalPlatform, "-")
					table.Row(
						stopover.Stop.DisplayName(),
						transit.FormatClock(stopover.Arrival),
						transit.FormatClock(stopover.Departure),
						platform,
						transit.FormatDelay(delay, delay != nil, false),
						joinOrDash(transit.RemarkTexts(stopover.Remarks)),
					)
				}
				return table.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Trip ID")
	cmd.Flags().StringVar(&line, "line", "", "Optional line name hint")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}
