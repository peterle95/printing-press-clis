package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"airbnb-pp-cli/internal/cliutil"
	"airbnb-pp-cli/internal/source/airbnb"
	"airbnb-pp-cli/internal/source/vrbo"
	"airbnb-pp-cli/internal/store"
	"github.com/spf13/cobra"
)

// pp:novel-static-reference
func newPlanCmd(flags *rootFlags) *cobra.Command {
	var checkin, checkout, backend string
	var guests, topN int
	var budget, maxTotalPrice float64
	var localFirst bool
	var dbPath string
	cmd := &cobra.Command{
		Use: "plan <city>",
		// PATCH: Plan now promises availability-checked, linked rooms instead of unverified direct-search leads.
		Short:       "Search for linked, date-priced rooms and rank verified options",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"city": args[0], "results": []any{}, "method": "dry_run"}, flags)
			}
			if checkin == "" || checkout == "" {
				return usageErr(fmt.Errorf("--checkin and --checkout are required"))
			}
			if err := validateDates(checkin, checkout); err != nil {
				return usageErr(err)
			}
			city := args[0]

			var urls []string
			failures := []map[string]any{}

			if localFirst {
				if dbPath == "" {
					dbPath = defaultDBPath("airbnb-pp-cli")
				}
				db, err := store.OpenWithContext(cmd.Context(), dbPath)
				if err != nil {
					return fmt.Errorf("opening local database: %w", err)
				}
				defer db.Close()

				filter := store.ListingFilter{
					Location:     city,
					Checkin:      checkin,
					Checkout:     checkout,
					MaxPrice:     maxTotalPrice,
					MinPrice:     0,
					PropertyType: "",
					Limit:        top(topN) * 2,
				}
				localResults, err := db.QueryListingsByPrice(filter)
				if err != nil {
					return fmt.Errorf("querying local listings: %w", err)
				}
				for _, raw := range localResults {
					var obj map[string]any
					if err := json.Unmarshal(raw, &obj); err == nil {
						if id, ok := obj["id"].(string); ok && id != "" {
							// PATCH: Preserve the user's dates in local-first recommendations so agents always have a usable booking link.
							if u := airbnb.BookingURL(id, checkin, checkout, guests, 0, 0, 0); u != "" {
								urls = append(urls, u)
							}
						}
					}
				}
			}

			if len(urls) == 0 {
				type source struct{ name string }
				results, errs := cliutil.FanoutRun(cmd.Context(), []source{{"airbnb"}, {"vrbo"}}, func(s source) string { return s.name }, func(ctx context.Context, s source) ([]string, error) {
					legCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
					defer cancel()
					switch s.name {
					case "airbnb":
						listings, _, err := airbnb.Search(legCtx, airbnb.SearchParams{Location: city, Checkin: checkin, Checkout: checkout, Adults: guests})
						var filtered []string
						for _, l := range listings {
							if !airbnb.ListingAvailableForRequestedDates(l) {
								continue
							}
							if maxTotalPrice > 0 && l.PriceTotal > 0 && l.PriceTotal > maxTotalPrice {
								continue
							}
							if budget > 0 && l.PriceBreakdown != nil && l.PriceBreakdown.Total > 0 && l.PriceBreakdown.Total > budget {
								continue
							}
							// PATCH: Prefer date-specific booking URLs and drop unlinked search hits.
							link := l.BookingURL
							if link == "" {
								link = l.URL
							}
							if link == "" {
								continue
							}
							filtered = append(filtered, link)
							if len(filtered) >= top(topN) {
								break
							}
						}
						return filtered, err
					default:
						return nil, vrbo.ErrDisabled
					}
				}, cliutil.WithConcurrency(2))
				cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)
				for _, e := range errs {
					reason := e.Err.Error()
					if vrbo.IsDisabled(e.Err) {
						reason = "vrbo_disabled"
					}
					failures = append(failures, map[string]any{"source": e.Source, "reason": reason})
				}
				for _, r := range results {
					urls = append(urls, r.Value...)
				}
			}

			cheapestLimit := top(topN)
			if cheapestLimit > 3 {
				cheapestLimit = 3
			}
			if len(urls) > cheapestLimit {
				urls = urls[:cheapestLimit]
			}
			cheapest, cerrs := cliutil.FanoutRun(cmd.Context(), urls, func(s string) string { return s }, func(ctx context.Context, u string) (map[string]any, error) {
				legCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				ch, err := computeCheapest(legCtx, u, cheapestParams{Checkin: checkin, Checkout: checkout, Guests: guests, SearchBackend: backend, MaxDirectResults: 1})
				if err != nil {
					return nil, err
				}
				pt, _ := firstPlatformTotals(ch)
				dt := cheapestDirectTotal(ch)
				platformURL := platformURLForRecommendation(u, ch)
				// PATCH: Plan output must keep a citable platform link and only claim savings when both totals are verified.
				result := map[string]any{
					"platform_url":        platformURL,
					"recommendation_url":  platformURL,
					"direct_url":          directURL(ch),
					"savings":             nil,
					"platform_total":      nullableFloat(pt),
					"direct_total":        nullableFloat(dt),
					"availability_status": listingStringField(ch, "availability_status"),
					"available":           listingBoolField(ch, "available"),
					"cheapest":            ch.Cheapest,
					"listing":             ch.Listing,
				}
				if !listingBoolField(ch, "available") {
					result["filtered_out"] = true
					result["filter_reason"] = "date_unavailable"
				}
				if pt > 0 && dt > 0 {
					result["savings"] = pt - dt
				}
				if maxTotalPrice > 0 && pt > 0 && pt > maxTotalPrice {
					result["filtered_out"] = true
					result["filter_reason"] = "exceeds_max_total_price"
				}
				return result, nil
			}, cliutil.WithConcurrency(3))
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), cerrs)
			for _, e := range cerrs {
				failures = append(failures, map[string]any{"source": e.Source, "reason": e.Err.Error()})
			}
			out := make([]map[string]any, 0, len(cheapest))
			for _, r := range cheapest {
				if filteredOut, _ := r.Value["filtered_out"].(bool); filteredOut {
					continue
				}
				out = append(out, r.Value)
			}
			sortBySavings(out)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"city": city, "results": out, "failures": failures}, flags)
		},
	}
	cmd.Flags().StringVar(&checkin, "checkin", "", "Arrival date YYYY-MM-DD")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Departure date YYYY-MM-DD")
	cmd.Flags().IntVar(&guests, "guests", 1, "Guest count")
	cmd.Flags().Float64Var(&budget, "budget", 0, "Maximum platform total (legacy, use --max-total-price)")
	cmd.Flags().Float64Var(&maxTotalPrice, "max-total-price", 0, "Maximum total price including fees; filters out results above this threshold")
	cmd.Flags().IntVar(&topN, "top-n", 5, "Top listings per platform")
	cmd.Flags().StringVar(&backend, "search-backend", "", "Search backend")
	cmd.Flags().BoolVar(&localFirst, "local-first", false, "Try local SQLite store first before hitting the network")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path for local-first mode")
	return cmd
}

func top(n int) int {
	if n <= 0 {
		return 5
	}
	return n
}

func directURL(ch *cheapestOutput) string {
	if m, ok := ch.Cheapest.(map[string]any); ok {
		if source, _ := m["source"].(string); source != "direct" {
			return ""
		}
		if s, ok := m["url"].(string); ok {
			return s
		}
	}
	return ""
}

func platformURLForRecommendation(fallback string, ch *cheapestOutput) string {
	for _, key := range []string{"booking_url", "url"} {
		if s := listingStringField(ch, key); s != "" {
			return s
		}
	}
	return fallback
}

func listingStringField(ch *cheapestOutput, key string) string {
	if ch == nil || ch.Listing == nil {
		return ""
	}
	if s, ok := ch.Listing[key].(string); ok {
		return s
	}
	return ""
}

func listingBoolField(ch *cheapestOutput, key string) bool {
	if ch == nil || ch.Listing == nil {
		return false
	}
	if b, ok := ch.Listing[key].(bool); ok {
		return b
	}
	return false
}
