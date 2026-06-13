package cli

import (
	"os"

	"github.com/spf13/cobra"

	"flight-pp-cli/internal/flight"
	"flight-pp-cli/internal/providers"
)

func newProvidersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "Inspect provider availability",
	}
	cmd.AddCommand(newProvidersStatusCmd(flags))
	return cmd
}

func newProvidersStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print enabled providers, missing API keys, and availability",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(flags)
			defer cancel()
			list := providers.All(app.config, app.cacheDir, flags.timeout)
			statuses := make([]flight.ProviderStatus, 0, len(list))
			for _, provider := range list {
				statuses = append(statuses, provider.Status(ctx))
			}
			return outputValue(flags, statuses, func() error {
				return printStatuses(os.Stdout, statuses)
			})
		},
	}
}
