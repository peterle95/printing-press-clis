// Package browser contains the allowed-origin, visible-UI Playwright adapter.
package browser

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
	"hevy-pp-cli/internal/plans"
)

const loginURL = "https://hevy.com/login"

var allowedHosts = map[string]bool{"www.hevyapp.com": true, "hevyapp.com": true, "hevy.com": true, "www.hevy.com": true}

type Status struct {
	Status    string `json:"status"`
	CheckedAt string `json:"checkedAt"`
	Detail    string `json:"detail,omitempty"`
}
type Options struct {
	Profile, Browser string
	Headed, Debug    bool
	Timeout          time.Duration
}

func ensureURL(raw string) error {
	u, e := url.Parse(raw)
	if e != nil {
		return e
	}
	if u.Scheme != "https" || !allowedHosts[strings.ToLower(u.Hostname())] {
		return fmt.Errorf("refusing navigation to unexpected origin %q", u.Host)
	}
	return nil
}
func Login(ctx context.Context, o Options) error {
	if err := ensureURL(loginURL); err != nil {
		return err
	}
	if err := os.MkdirAll(o.Profile, 0o700); err != nil {
		return err
	}
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("start Playwright (run 'go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6100.0 install %s'): %w", o.Browser, err)
	}
	defer pw.Stop()
	browserType := pw.Chromium
	if o.Browser != "chromium" {
		return fmt.Errorf("unsupported browser %q; only chromium is currently supported", o.Browser)
	}
	b, err := browserType.LaunchPersistentContext(o.Profile, playwright.BrowserTypeLaunchPersistentContextOptions{Headless: playwright.Bool(false)})
	if err != nil {
		return err
	}
	defer b.Close()
	p := b.Pages()[0]
	if _, err = p.Goto(loginURL); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Complete Hevy login in the browser. Credentials are never read by this CLI.")
	deadline := time.Now().Add(o.Timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		cur := p.URL()
		if !strings.Contains(cur, "/login") && !strings.Contains(cur, "/wp-login.php") {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("login timed out; finish the visible browser flow and retry")
}
func AuthStatus(ctx context.Context, o Options) Status {
	st := Status{Status: "missing", CheckedAt: time.Now().UTC().Format(time.RFC3339)}
	if _, err := os.Stat(o.Profile); err != nil {
		return st
	}
	pw, err := playwright.Run()
	if err != nil {
		st.Status = "unknown"
		st.Detail = "Playwright unavailable"
		return st
	}
	defer pw.Stop()
	if o.Browser != "chromium" {
		st.Status = "unknown"
		st.Detail = "unsupported browser"
		return st
	}
	b, err := pw.Chromium.LaunchPersistentContext(o.Profile, playwright.BrowserTypeLaunchPersistentContextOptions{Headless: playwright.Bool(!o.Headed)})
	if err != nil {
		st.Status = "unknown"
		st.Detail = "profile unavailable"
		return st
	}
	defer b.Close()
	p := b.Pages()[0]
	if _, err = p.Goto("https://hevy.com/"); err != nil {
		st.Status = "unknown"
		st.Detail = "could not reach Hevy"
		return st
	}
	cur := p.URL()
	if strings.Contains(cur, "/login") || strings.Contains(cur, "/wp-login.php") {
		st.Status = "expired"
	} else {
		st.Status = "authenticated"
	}
	return st
}
func Inspect(ctx context.Context, o Options) ([]plans.Routine, error) {
	st := AuthStatus(ctx, o)
	if st.Status != "authenticated" {
		return nil, fmt.Errorf("authentication is %s", st.Status)
	}
	return nil, fmt.Errorf("routine inspection is unsupported until selectors have been validated against the current Hevy public UI; no changes were made")
}
func Logout(profile string) error {
	if profile == "" || filepath.Base(profile) != "browser-profile" {
		return fmt.Errorf("refusing to remove unexpected browser profile path")
	}
	return os.RemoveAll(profile)
}
