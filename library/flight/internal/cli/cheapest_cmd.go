package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"flight-pp-cli/internal/flight"
	"flight-pp-cli/internal/providers"
)

type cheapestFlags struct {
	search   searchFlags
	month    string
	maxPrice float64
	tripDays int
}

func newCheapestCmd(root *rootFlags) *cobra.Command {
	flags := &cheapestFlags{}
	cmd := &cobra.Command{
		Use:   "cheapest",
		Short: "Run a flexible-date cheapest-flight search where providers support it",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(root)
			if err != nil {
				return err
			}
			flags.search.maxStopsSet = cmd.Flags().Changed("max-stops")
			base := requestFromSearchFlags(&flags.search, app.config)
			flex := flight.FlexibleSearchRequest{
				FlightSearchRequest: base,
				Month:               flags.month,
				MaxPrice:            flags.maxPrice,
				TripDays:            flags.tripDays,
			}
			if err := flight.ValidateFlexibleRequest(flex); err != nil {
				return err
			}
			flex.FlightSearchRequest = flight.NormalizeRequest(flex.FlightSearchRequest)
			selected := providerNames(flags.search.providers, app.config)
			summary := runCheapest(root, app, flex, selected)
			if flags.maxPrice > 0 {
				summary.Results = filterMaxPrice(summary.Results, flags.maxPrice)
			}
			flight.SortResults(summary.Results, flex.Sort)
			summary.Results = flight.LimitResults(summary.Results, flex.Limit)
			if flags.search.openBest {
				if err := openBest(summary.Results); err != nil {
					summary.ProviderErrors = append(summary.ProviderErrors, flight.ProviderError{Provider: "open-best", Error: err.Error()})
				}
			}
			if root.asJSON || root.agent {
				return printJSON(summary)
			}
			if err := printResults(os.Stdout, summary.Results); err != nil {
				return err
			}
			printProviderErrors(summary.ProviderErrors)
			return nil
		},
	}
	addSearchFlags(cmd, &flags.search)
	cmd.Flags().StringVar(&flags.month, "month", "", "Flexible search month YYYY-MM")
	cmd.Flags().Float64Var(&flags.maxPrice, "max-price", 0, "Maximum price")
	cmd.Flags().IntVar(&flags.tripDays, "trip-days", 0, "Desired trip length in days/nights where supported")
	_ = cmd.MarkFlagRequired("month")
	return cmd
}

func runCheapest(root *rootFlags, app *app, req flight.FlexibleSearchRequest, selected []string) flight.SearchSummary {
	ctx, cancel := commandContext(root)
	defer cancel()
	providerList, buildErrors := providers.Build(app.config, app.cacheDir, root.timeout, selected)
	summary := flight.SearchSummary{Request: req.FlightSearchRequest}
	summary.Request.DepartDate = req.Month
	for _, err := range buildErrors {
		summary.ProviderErrors = append(summary.ProviderErrors, flight.ProviderError{Provider: "provider", Error: err.Error()})
	}
	for _, provider := range providerList {
		flexible, ok := provider.(flight.FlexibleProvider)
		if !ok {
			summary.ProviderErrors = append(summary.ProviderErrors, flight.ProviderError{
				Provider: provider.Name(),
				Error:    fmt.Sprintf("%s flexible-date search is not supported in this MVP", provider.Name()),
			})
			continue
		}
		results, err := flexible.Cheapest(ctx, req)
		if err != nil {
			summary.ProviderErrors = append(summary.ProviderErrors, flight.ProviderError{Provider: provider.Name(), Error: err.Error()})
			continue
		}
		summary.Results = append(summary.Results, results...)
	}
	return summary
}

func filterMaxPrice(results []flight.FlightSearchResult, maxPrice float64) []flight.FlightSearchResult {
	filtered := make([]flight.FlightSearchResult, 0, len(results))
	for _, result := range results {
		if !result.PriceAvailable || result.TotalPrice <= maxPrice {
			filtered = append(filtered, result)
		}
	}
	return filtered
}
