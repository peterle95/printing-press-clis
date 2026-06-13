// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.
package windy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"windy-weather-pp-cli/internal/config"
	"windy-weather-pp-cli/internal/weather"
)

type NetworkResponse struct {
	URL         string `json:"url"`
	Method      string `json:"method"`
	Status      int    `json:"status"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Body        string `json:"body,omitempty"`
	Preview     string `json:"preview,omitempty"`
}

type Result struct {
	Success         bool              `json:"success"`
	Error           string            `json:"error,omitempty"`
	Responses       []NetworkResponse `json:"responses"`
	EndpointsUsed   []string          `json:"endpoints_used"`
	ScreenshotSaved bool              `json:"screenshot_saved"`
	DebugLogSaved   bool              `json:"debug_log_saved"`
	DebugLogPath    string            `json:"debug_log_path,omitempty"`
	ScreenshotPath  string            `json:"screenshot_path,omitempty"`
}

type Options struct {
	DebugNetwork   bool
	ScreenshotPath string
	Debug          bool
}

type ScrapeError struct {
	Code    string
	Message string
}

func (e ScrapeError) Error() string {
	return e.Message
}

func Run(ctx context.Context, cfg *config.Config, opts Options) (*Result, error) {
	script, err := findScraper()
	if err != nil {
		return nil, ScrapeError{Code: "browser_dependency_missing", Message: err.Error()}
	}

	args := []string{
		script,
		"--url", cfg.DefaultLocation.WindyURL,
		"--lat", strconv.FormatFloat(cfg.DefaultLocation.Latitude, 'f', 6, 64),
		"--lon", strconv.FormatFloat(cfg.DefaultLocation.Longitude, 'f', 6, 64),
		"--timeout", strconv.Itoa(cfg.Browser.TimeoutMs),
	}
	if !cfg.Browser.Headless {
		args = append(args, "--non-headless")
	}
	if opts.DebugNetwork {
		args = append(args, "--debug-network")
	}
	if opts.ScreenshotPath != "" {
		args = append(args, "--screenshot", opts.ScreenshotPath)
	}

	cmd := exec.CommandContext(ctx, "node", args...)
	cmd.Dir = filepath.Dir(script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return nil, classifyRunError(msg)
	}

	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, ScrapeError{Code: "malformed_scraper_output", Message: fmt.Sprintf("parsing scraper output: %v", err)}
	}
	if !result.Success {
		return &result, classifyRunError(result.Error)
	}
	return &result, nil
}

func findScraper() (string, error) {
	candidates := []string{}
	if env := os.Getenv("WINDY_WEATHER_SCRAPER"); env != "" {
		candidates = append(candidates, env)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "scraper.js"))
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "scraper.js"),
			filepath.Join(exeDir, "..", "scraper.js"),
			filepath.Join(exeDir, "..", "..", "scraper.js"),
		)
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "printing-press", "library", "windy-weather-pp-cli", "scraper.js"))
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			abs, err := filepath.Abs(candidate)
			if err != nil {
				return candidate, nil
			}
			return abs, nil
		}
	}
	return "", errors.New("scraper.js not found; set WINDY_WEATHER_SCRAPER or run from the CLI directory")
}

func classifyRunError(message string) ScrapeError {
	code := "windy_page_failed"
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "executable doesn't exist"),
		strings.Contains(lower, "browsertype.launch"),
		strings.Contains(lower, "cannot find module 'playwright'"),
		strings.Contains(lower, "cannot find module \"playwright\""):
		code = "browser_dependency_missing"
	case strings.Contains(lower, "timeout"):
		code = "network_timeout"
	case strings.Contains(lower, "net::"):
		code = "windy_page_failed"
	}
	return ScrapeError{Code: code, Message: message}
}

func ErrorItem(err error) weather.ErrorItem {
	var scrapeErr ScrapeError
	if errors.As(err, &scrapeErr) {
		return weather.ErrorItem{Code: scrapeErr.Code, Message: scrapeErr.Message}
	}
	return weather.ErrorItem{Code: "scrape_failed", Message: err.Error()}
}
