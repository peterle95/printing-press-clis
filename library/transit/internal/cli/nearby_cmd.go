package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"transit-pp-cli/internal/transit"
	"transit-pp-cli/internal/ui"
)

type nearbyOutput struct {
	From  transit.Location   `json:"from"`
	Stops []transit.Location `json:"stops"`
}

func newNearbyCmd(flags *rootFlags) *cobra.Command {
	var from string
	var radius int
	cmd := &cobra.Command{
		Use:   "nearby",
		Short: "Show nearby VBB stops",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp(flags)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(flags)
			defer cancel()
			if radius <= 0 {
				radius = a.config.Defaults.RadiusMeters
			}
			if strings.TrimSpace(from) == "" {
				from = "home"
			}
			if from != "home" {
				return fmt.Errorf("only --from home is currently supported")
			}
			home, err := homeLocation(ctx, a)
			if err != nil {
				return err
			}
			lat, lon, _ := home.Coordinates()
			stops, err := cachedNearby(ctx, a, lat, lon, 10, radius)
			if err != nil {
				return err
			}
			out := nearbyOutput{From: home, Stops: stops}
			return outputValue(flags, out, func() error {
				table := ui.NewTable(os.Stdout)
				table.Row("STOP", "DIST", "PRODUCTS", "ID")
				for _, stop := range stops {
					table.Row(stop.DisplayName(), fmt.Sprintf("%dm", stop.Distance), joinOrDash(transit.ProductNames(stop.Products)), stop.ID)
				}
				return table.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&from, "from", "home", "Origin label")
	cmd.Flags().IntVar(&radius, "radius", 0, "Search radius in meters")
	return cmd
}
