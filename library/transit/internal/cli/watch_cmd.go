package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	opts := boardOptions{}
	var every time.Duration
	var noClear bool
	var count int
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Refresh the departure board repeatedly",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp(flags)
			if err != nil {
				return err
			}
			if every <= 0 {
				every = time.Duration(a.config.Defaults.RefreshSeconds) * time.Second
			}
			if every < 10*time.Second {
				return fmt.Errorf("--every must be at least 10s to avoid unnecessary API load")
			}
			iteration := 0
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			for {
				out, err := buildBoard(cmd, flags, opts, true)
				if err != nil {
					return err
				}
				if wantsJSON(flags) {
					if err := enc.Encode(out); err != nil {
						return err
					}
				} else {
					if !noClear {
						fmt.Print("\033[H\033[2J")
					}
					if err := printBoard(flags, out); err != nil {
						return err
					}
					fmt.Printf("\nUpdated %s. Refreshing every %s.\n", time.Now().In(time.Local).Format("15:04:05"), every)
				}
				iteration++
				if count > 0 && iteration >= count {
					return nil
				}
				timer := time.NewTimer(every)
				select {
				case <-cmd.Context().Done():
					timer.Stop()
					return cmd.Context().Err()
				case <-timer.C:
				}
			}
		},
	}
	addBoardFlags(cmd, &opts)
	cmd.Flags().DurationVar(&every, "every", 0, "Refresh interval")
	cmd.Flags().BoolVar(&noClear, "no-clear", false, "Do not clear the terminal between refreshes")
	cmd.Flags().IntVar(&count, "count", 0, "Number of refreshes before exiting (0 means forever)")
	return cmd
}
