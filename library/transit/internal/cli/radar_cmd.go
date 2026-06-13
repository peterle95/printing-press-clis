package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"transit-pp-cli/internal/transit"
	"transit-pp-cli/internal/ui"
)

type radarOutput struct {
	From      transit.Location    `json:"from"`
	Radius    int                 `json:"radius_meters"`
	Bounds    transit.BoundingBox `json:"bounds"`
	Movements []transit.Movement  `json:"movements"`
}

func newRadarCmd(flags *rootFlags) *cobra.Command {
	var from string
	var radius int
	cmd := &cobra.Command{
		Use:   "radar",
		Short: "Show moving vehicles near home when VBB radar data is available",
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
			if radius <= 0 {
				radius = 1500
			}
			home, err := homeLocation(ctx, a)
			if err != nil {
				return err
			}
			lat, lon, _ := home.Coordinates()
			box := transit.BoundingBoxAround(lat, lon, radius)
			resp, err := a.client.Radar(ctx, box, 64, 30, 3)
			if err != nil {
				return err
			}
			out := radarOutput{From: home, Radius: radius, Bounds: box, Movements: resp.Movements}
			return outputValue(flags, out, func() error {
				if len(out.Movements) == 0 {
					fmt.Println("No moving vehicles reported nearby.")
					return nil
				}
				table := ui.NewTable(os.Stdout)
				table.Row("LINE", "MODE", "DIRECTION", "POSITION", "NEXT STOP")
				for _, movement := range out.Movements {
					position := "-"
					if movement.Location != nil {
						position = fmt.Sprintf("%.5f,%.5f", movement.Location.Latitude, movement.Location.Longitude)
					}
					next := "-"
					if len(movement.NextStopovers) > 0 {
						next = movement.NextStopovers[0].Stop.DisplayName()
					}
					table.Row(
						firstNonEmpty(movement.Line.Name, "-"),
						transit.ProductLabel(movement.Line.Product),
						firstNonEmpty(movement.Direction, "-"),
						position,
						next,
					)
				}
				return table.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&from, "from", "home", "Origin label")
	cmd.Flags().IntVar(&radius, "radius", 1500, "Radar radius in meters")
	return cmd
}
