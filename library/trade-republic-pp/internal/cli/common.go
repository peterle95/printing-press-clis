package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"trade-republic-pp-cli/config"
)

func loadConfig(f *flags) (config.Config, string, error) {
	cfg, path, err := config.Load(f.Config)
	if err != nil {
		return config.Config{}, path, err
	}
	if f.Database != "" {
		cfg.DatabasePath = f.Database
	}
	return cfg, path, cfg.Validate()
}

func commandContext(cmdContext context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(cmdContext)
	}
	return context.WithTimeout(cmdContext, timeout)
}

func parseDate(value string, location *time.Location) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return &parsed, nil
	}
	if location == nil {
		location = time.UTC
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, location)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q (use YYYY-MM-DD or RFC3339)", value)
	}
	return &parsed, nil
}

func periodStart(period string, now time.Time, location *time.Location) (time.Time, error) {
	if location == nil {
		location = time.UTC
	}
	now = now.In(location)
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "", "all":
		return time.Time{}, nil
	case "ytd":
		return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, location), nil
	case "mtd":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location), nil
	case "1y":
		return now.AddDate(-1, 0, 0), nil
	default:
		parsed, err := parseDate(period, location)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid period %q (use all, ytd, mtd, 1y, or a date)", period)
		}
		return *parsed, nil
	}
}
