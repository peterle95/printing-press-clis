package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"flight-pp-cli/internal/config"
	"flight-pp-cli/internal/flight"
	"flight-pp-cli/internal/providers"
)

type searchFlags struct {
	from                string
	to                  string
	depart              string
	returnDate          string
	oneWay              bool
	adults              int
	children            int
	infants             int
	currency            string
	cabin               string
	maxStops            int
	maxStopsSet         bool
	directOnly          bool
	includeSelfTransfer bool
	bags                string
	providers           string
	sort                string
	limit               int
	openBest            bool
}

func newSearchCmd(root *rootFlags) *cobra.Command {
	flags := &searchFlags{}
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search comparable flight options across configured providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(root)
			if err != nil {
				return err
			}
			flags.maxStopsSet = cmd.Flags().Changed("max-stops")
			req := requestFromSearchFlags(flags, app.config)
			if err := flight.ValidateSearchRequest(req); err != nil {
				return err
			}
			req = flight.NormalizeRequest(req)
			selected := providerNames(flags.providers, app.config)
			summary := runSearch(root, app, req, selected)
			flight.SortResults(summary.Results, req.Sort)
			summary.Results = flight.LimitResults(summary.Results, req.Limit)
			if flags.openBest {
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
	addSearchFlags(cmd, flags)
	return cmd
}

func addSearchFlags(cmd *cobra.Command, flags *searchFlags) {
	cmd.Flags().StringVar(&flags.from, "from", "", "IATA origin airport/city code")
	cmd.Flags().StringVar(&flags.to, "to", "", "IATA destination airport/city code or anywhere")
	cmd.Flags().StringVar(&flags.depart, "depart", "", "Departure date YYYY-MM-DD")
	cmd.Flags().StringVar(&flags.returnDate, "return", "", "Return date YYYY-MM-DD")
	cmd.Flags().BoolVar(&flags.oneWay, "one-way", false, "Search a one-way flight")
	cmd.Flags().IntVar(&flags.adults, "adults", 0, "Number of adults")
	cmd.Flags().IntVar(&flags.children, "children", 0, "Number of children")
	cmd.Flags().IntVar(&flags.infants, "infants", 0, "Number of infants")
	cmd.Flags().StringVar(&flags.currency, "currency", "", "Currency code")
	cmd.Flags().StringVar(&flags.cabin, "cabin", "", "Cabin: economy, premium_economy, business, first")
	cmd.Flags().IntVar(&flags.maxStops, "max-stops", 0, "Maximum stops")
	cmd.Flags().BoolVar(&flags.directOnly, "direct-only", false, "Only direct flights")
	cmd.Flags().BoolVar(&flags.includeSelfTransfer, "include-self-transfer", false, "Allow self-transfer results when providers return them")
	cmd.Flags().StringVar(&flags.bags, "bags", "", "Baggage need: none, personal_item, cabin, checked")
	cmd.Flags().StringVar(&flags.providers, "providers", "", "Comma-separated provider list")
	cmd.Flags().StringVar(&flags.sort, "sort", "", "Sort by price, duration, or best")
	cmd.Flags().IntVar(&flags.limit, "limit", 0, "Maximum results")
	cmd.Flags().BoolVar(&flags.openBest, "open-best", false, "Open the best result booking/deep link")
	cmd.Flags().Lookup("max-stops").NoOptDefVal = "0"
}

func requestFromSearchFlags(flags *searchFlags, cfg config.Config) flight.FlightSearchRequest {
	req := flight.FlightSearchRequest{
		Origin:              firstNonEmpty(flags.from, cfg.Defaults.HomeAirport),
		Destination:         flags.to,
		DepartDate:          flags.depart,
		ReturnDate:          flags.returnDate,
		OneWay:              flags.oneWay,
		Adults:              flags.adults,
		Children:            flags.children,
		Infants:             flags.infants,
		Currency:            firstNonEmpty(flags.currency, cfg.Defaults.Currency),
		Cabin:               firstNonEmpty(flags.cabin, cfg.Defaults.Cabin),
		DirectOnly:          flags.directOnly,
		IncludeSelfTransfer: flags.includeSelfTransfer,
		Bags:                firstNonEmpty(flags.bags, flight.BagsNone),
		Sort:                firstNonEmpty(flags.sort, flight.SortBest),
		Limit:               flags.limit,
	}
	if req.Adults == 0 {
		req.Adults = cfg.Defaults.Adults
	}
	if flags.maxStopsSet {
		req.MaxStops = &flags.maxStops
	}
	return req
}

func providerNames(value string, cfg config.Config) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return config.ParseProviderList(value)
}

func runSearch(root *rootFlags, app *app, req flight.FlightSearchRequest, selected []string) flight.SearchSummary {
	ctx, cancel := commandContext(root)
	defer cancel()
	providerList, buildErrors := providers.Build(app.config, app.cacheDir, root.timeout, selected)
	summary := flight.SearchSummary{Request: req}
	for _, err := range buildErrors {
		summary.ProviderErrors = append(summary.ProviderErrors, flight.ProviderError{Provider: "provider", Error: err.Error()})
	}
	for _, provider := range providerList {
		if !root.noCache {
			if cached, ok, err := app.cache.Read(provider.Name(), req); err == nil && ok {
				summary.Results = append(summary.Results, cached...)
				continue
			} else if err != nil {
				summary.ProviderErrors = append(summary.ProviderErrors, flight.ProviderError{Provider: provider.Name(), Error: "cache read: " + err.Error()})
			}
		}
		results, err := provider.Search(ctx, req)
		if err != nil {
			summary.ProviderErrors = append(summary.ProviderErrors, flight.ProviderError{Provider: provider.Name(), Error: err.Error()})
			continue
		}
		if !root.noCache {
			if err := app.cache.Write(provider.Name(), req, results); err != nil {
				summary.ProviderErrors = append(summary.ProviderErrors, flight.ProviderError{Provider: provider.Name(), Error: "cache write: " + err.Error()})
			}
		}
		summary.Results = append(summary.Results, results...)
	}
	return summary
}

func openBest(results []flight.FlightSearchResult) error {
	for _, result := range results {
		link := firstNonEmpty(result.BookingLink, result.DeepLink)
		if link == "" {
			continue
		}
		return openURL(link)
	}
	return fmt.Errorf("no booking or deep link available")
}

func openURL(link string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
	case "darwin":
		cmd = exec.Command("open", link)
	default:
		cmd = exec.Command("xdg-open", link)
	}
	return cmd.Start()
}
