package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCacheCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage provider response cache",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "clear",
		Short: "Clear cached provider responses",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			count, err := app.cache.Clear()
			if err != nil {
				return err
			}
			return outputValue(flags, map[string]any{"cleared": count, "cacheDir": app.cacheDir}, func() error {
				fmt.Printf("Cleared %d cached file(s) from %s\n", count, app.cacheDir)
				return nil
			})
		},
	})
	return cmd
}
