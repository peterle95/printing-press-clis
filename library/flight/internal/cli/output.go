package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"flight-pp-cli/internal/flight"
	"flight-pp-cli/internal/ui"
)

func printResults(w io.Writer, results []flight.FlightSearchResult) error {
	if len(results) == 0 {
		fmt.Fprintln(w, "No flight results.")
		return nil
	}
	table := ui.NewTable(w)
	table.Row("Provider", "Price", "Route", "Depart", "Arrive", "Airlines", "Stops", "Duration", "Bags", "Risk", "Link")
	for _, result := range results {
		table.Row(
			result.Provider,
			formatPrice(result),
			route(result),
			shortTime(result.DepartAt),
			shortTime(result.ArriveAt),
			joinOrDash(result.Airlines),
			result.Stops,
			formatDuration(result.DurationMinutes),
			formatBaggage(result.Baggage),
			formatRisk(result),
			firstNonEmpty(result.BookingLink, result.DeepLink, "-"),
		)
	}
	return table.Flush()
}

func printProviderErrors(errors []flight.ProviderError) {
	for _, item := range errors {
		fmt.Fprintf(os.Stderr, "%s failed: %s\n", title(item.Provider), item.Error)
	}
}

func printStatuses(w io.Writer, statuses []flight.ProviderStatus) error {
	table := ui.NewTable(w)
	table.Row("Provider", "Enabled", "Available", "Mode", "Missing", "Warnings")
	for _, status := range statuses {
		table.Row(
			status.Name,
			status.Enabled,
			status.Available,
			firstNonEmpty(status.Mode, "-"),
			joinOrDash(status.Missing),
			joinOrDash(status.Warnings),
		)
	}
	return table.Flush()
}

func formatPrice(result flight.FlightSearchResult) string {
	if !result.PriceAvailable {
		return "open manually"
	}
	if result.Currency == "" {
		return strconv.FormatFloat(result.TotalPrice, 'f', 2, 64)
	}
	return fmt.Sprintf("%.2f %s", result.TotalPrice, result.Currency)
}

func route(result flight.FlightSearchResult) string {
	if result.Origin == "" && result.Destination == "" {
		return "-"
	}
	return result.Origin + "-" + result.Destination
}

func shortTime(value string) string {
	if value == "" {
		return "-"
	}
	value = strings.ReplaceAll(value, "T", " ")
	if len(value) > 16 {
		return value[:16]
	}
	return value
}

func formatDuration(minutes int) string {
	if minutes <= 0 {
		return "-"
	}
	return fmt.Sprintf("%dh%02dm", minutes/60, minutes%60)
}

func formatBaggage(bag flight.Baggage) string {
	parts := []string{}
	if bag.PersonalItemIncluded != nil {
		parts = append(parts, "personal:"+yesNo(*bag.PersonalItemIncluded))
	}
	if bag.CabinBagIncluded != nil {
		parts = append(parts, "cabin:"+yesNo(*bag.CabinBagIncluded))
	}
	if bag.CheckedBagIncluded != nil {
		parts = append(parts, "checked:"+yesNo(*bag.CheckedBagIncluded))
	}
	if len(parts) == 0 && bag.Notes == "" {
		return "unknown"
	}
	if bag.Notes != "" {
		parts = append(parts, bag.Notes)
	}
	return strings.Join(parts, "; ")
}

func formatRisk(result flight.FlightSearchResult) string {
	parts := append([]string{}, result.Risks...)
	parts = append(parts, result.Warnings...)
	return joinOrDash(parts)
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func joinOrDash(values []string) string {
	clean := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			clean = append(clean, value)
		}
	}
	if len(clean) == 0 {
		return "-"
	}
	return strings.Join(clean, ", ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func title(value string) string {
	if value == "" {
		return "Provider"
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
