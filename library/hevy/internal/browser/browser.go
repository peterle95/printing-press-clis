// Package browser contains the allowed-origin, visible-UI Playwright adapter.
package browser

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
	"hevy-pp-cli/internal/plans"
)

const loginURL = "https://hevy.com/login"
const routinesURL = "https://hevy.com/routines"
const createRoutineURL = "https://hevy.com/create-routine"

var setSummary = regexp.MustCompile(`^(\d+) sets?(?: · (\d+) reps)?$`)

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
	if o.Timeout <= 0 {
		o.Timeout = 5 * time.Minute
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
	b, err := browserType.LaunchPersistentContext(o.Profile, playwright.BrowserTypeLaunchPersistentContextOptions{Headless: playwright.Bool(false), Args: []string{"--start-maximized"}})
	if err != nil {
		return err
	}
	defer b.Close()
	p := b.Pages()[0]
	if _, err = p.Goto(loginURL); err != nil {
		return err
	}
	if err := waitForPage(context.Background(), p, 30*time.Second, func(body string) bool { return !strings.Contains(body, "Loading...") }); err != nil {
		return fmt.Errorf("login page did not finish loading: %w", err)
	}
	fmt.Fprintln(os.Stderr, "Complete Hevy login in the browser. Credentials are never read by this CLI.")
	deadline := time.Now().Add(o.Timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		body, err := p.Locator("body").InnerText()
		if err == nil && !isLoginPage(body) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("login timed out; finish the visible browser flow and retry")
}

func isLoginPage(body string) bool {
	return strings.Contains(body, "Log In") && strings.Contains(body, "Sign Up")
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
	if err := waitForPage(context.Background(), p, 15*time.Second, func(body string) bool { return !strings.Contains(body, "Loading...") }); err != nil {
		st.Status = "unknown"
		st.Detail = "page did not finish loading"
		return st
	}
	body, err := p.Locator("body").InnerText()
	if err != nil {
		st.Status = "unknown"
		st.Detail = "could not read page"
		return st
	}
	if isLoginPage(body) {
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
	if err := ensureURL(routinesURL); err != nil {
		return nil, err
	}
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("start Playwright: %w", err)
	}
	defer pw.Stop()
	if o.Browser != "chromium" {
		return nil, fmt.Errorf("unsupported browser %q; only chromium is currently supported", o.Browser)
	}
	b, err := pw.Chromium.LaunchPersistentContext(o.Profile, playwright.BrowserTypeLaunchPersistentContextOptions{Headless: playwright.Bool(!o.Headed)})
	if err != nil {
		return nil, err
	}
	defer b.Close()
	p := b.Pages()[0]
	if _, err = p.Goto(routinesURL); err != nil {
		return nil, err
	}
	if err := waitForPage(ctx, p, o.Timeout, func(body string) bool { return !strings.Contains(body, "Loading...") }); err != nil {
		return nil, fmt.Errorf("load routines page: %w", err)
	}
	links, err := p.Locator(`a[href^="/routine/"]`).All()
	if err != nil {
		return nil, err
	}
	hrefs := make([]string, 0, len(links))
	seen := map[string]bool{}
	for _, link := range links {
		href, err := link.GetAttribute("href")
		if err != nil {
			return nil, err
		}
		if href != "" && !seen[href] {
			seen[href] = true
			hrefs = append(hrefs, href)
		}
	}
	routines := make([]plans.Routine, 0, len(hrefs))
	for _, href := range hrefs {
		detailURL := "https://hevy.com" + href
		if err := ensureURL(detailURL); err != nil {
			return nil, err
		}
		if _, err = p.Goto(detailURL); err != nil {
			return nil, err
		}
		if err := waitForPage(ctx, p, o.Timeout, func(body string) bool { return !strings.Contains(body, "Loading...") }); err != nil {
			return nil, fmt.Errorf("load routine %q: %w", href, err)
		}
		title, err := p.Locator("h2").First().InnerText()
		if err != nil {
			return nil, fmt.Errorf("read routine %q title: %w", href, err)
		}
		body, err := p.Locator("body").InnerText()
		if err != nil {
			return nil, fmt.Errorf("read routine %q: %w", title, err)
		}
		routine, err := parseRoutineText(title, body)
		if err != nil {
			return nil, fmt.Errorf("parse routine %q: %w", title, err)
		}
		routines = append(routines, routine)
	}
	return routines, nil
}

func CreateRoutine(ctx context.Context, o Options, name string, exercises []plans.Exercise) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("routine name must not be empty")
	}
	if len(exercises) == 0 {
		return fmt.Errorf("routine requires at least one exercise")
	}
	st := AuthStatus(ctx, o)
	if st.Status != "authenticated" {
		return fmt.Errorf("authentication is %s", st.Status)
	}
	if err := ensureURL(createRoutineURL); err != nil {
		return err
	}
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("start Playwright: %w", err)
	}
	defer pw.Stop()
	if o.Browser != "chromium" {
		return fmt.Errorf("unsupported browser %q; only chromium is currently supported", o.Browser)
	}
	b, err := pw.Chromium.LaunchPersistentContext(o.Profile, playwright.BrowserTypeLaunchPersistentContextOptions{Headless: playwright.Bool(!o.Headed)})
	if err != nil {
		return err
	}
	defer b.Close()
	p := b.Pages()[0]
	if _, err = p.Goto(createRoutineURL); err != nil {
		return err
	}
	if err := waitForPage(ctx, p, o.Timeout, func(body string) bool { return !strings.Contains(body, "Loading...") }); err != nil {
		return fmt.Errorf("load routine creation page: %w", err)
	}
	title := p.Locator(`input[placeholder="Workout Routine Title"]`)
	if err := title.Fill(name); err != nil {
		return fmt.Errorf("fill routine title: %w", err)
	}
	for _, exercise := range exercises {
		if strings.TrimSpace(exercise.Name) == "" {
			return fmt.Errorf("exercise name must not be empty")
		}
		item := p.GetByText(exercise.Name, playwright.PageGetByTextOptions{Exact: playwright.Bool(true)}).Last()
		count, err := item.Count()
		if err != nil || count == 0 {
			return fmt.Errorf("exercise not found in visible library: %q", exercise.Name)
		}
		if err := item.Click(); err != nil {
			return fmt.Errorf("add exercise %q: %w", exercise.Name, err)
		}
		sets := len(exercise.Sets)
		if sets == 0 {
			sets = 1
		}
		for i := 1; i < sets; i++ {
			if err := p.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Add Set", Exact: playwright.Bool(true)}).Last().Click(); err != nil {
				return fmt.Errorf("add set to %q: %w", exercise.Name, err)
			}
		}
		if len(exercise.Sets) == 0 || exercise.Sets[0].TargetReps == nil {
			continue
		}
		reps := exercise.Sets[0].TargetReps.Min
		if reps < 1 {
			continue
		}
		decimals := p.Locator(`input[inputmode="decimal"]`)
		decimalCount, err := decimals.Count()
		if err != nil || decimalCount < sets*2 {
			return fmt.Errorf("could not locate rep inputs for %q", exercise.Name)
		}
		start := decimalCount - sets*2
		for i := 0; i < sets; i++ {
			if err := decimals.Nth(start + i*2 + 1).Fill(strconv.Itoa(reps)); err != nil {
				return fmt.Errorf("set reps for %q: %w", exercise.Name, err)
			}
		}
	}
	if err := p.Keyboard().Press("Escape"); err != nil {
		return fmt.Errorf("close exercise library: %w", err)
	}
	if err := p.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Save Routine", Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("save routine: %w", err)
	}
	if err := waitForPage(ctx, p, o.Timeout, func(body string) bool {
		return strings.HasPrefix(p.URL(), "https://hevy.com/routine/") || (!strings.Contains(p.URL(), "/create-routine") && strings.Contains(body, name))
	}); err != nil {
		return fmt.Errorf("verify routine creation: %w", err)
	}
	return nil
}

func waitForPage(ctx context.Context, p playwright.Page, timeout time.Duration, ready func(string) bool) error {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		body, err := p.Locator("body").InnerText()
		if err == nil && ready(body) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out after %s", timeout)
}

func parseRoutineText(title, body string) (plans.Routine, error) {
	lines := make([]string, 0)
	for _, line := range strings.Split(body, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	start := -1
	for i, line := range lines {
		if line == title {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return plans.Routine{}, fmt.Errorf("title not found in visible page")
	}
	routine := plans.Routine{Name: title}
	for i := start; i+1 < len(lines) && lines[i] != "Created by"; {
		match := setSummary.FindStringSubmatch(lines[i+1])
		if match == nil {
			i++
			continue
		}
		count, _ := strconv.Atoi(match[1])
		exercise := plans.Exercise{Name: lines[i], Sets: make([]plans.Set, count)}
		for set := range exercise.Sets {
			exercise.Sets[set].Type = "normal"
			if match[2] != "" {
				reps, _ := strconv.Atoi(match[2])
				exercise.Sets[set].TargetReps = &plans.Reps{Min: reps, Max: reps}
			}
		}
		routine.Exercises = append(routine.Exercises, exercise)
		i += 2
	}
	if len(routine.Exercises) == 0 {
		return plans.Routine{}, fmt.Errorf("no visible exercises found")
	}
	return routine, nil
}
func DeleteRoutine(ctx context.Context, o Options, name string) error {
	st := AuthStatus(ctx, o)
	if st.Status != "authenticated" {
		return fmt.Errorf("authentication is %s", st.Status)
	}
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("start Playwright: %w", err)
	}
	defer pw.Stop()
	if o.Browser != "chromium" {
		return fmt.Errorf("unsupported browser %q; only chromium is currently supported", o.Browser)
	}
	b, err := pw.Chromium.LaunchPersistentContext(o.Profile, playwright.BrowserTypeLaunchPersistentContextOptions{Headless: playwright.Bool(!o.Headed)})
	if err != nil {
		return err
	}
	defer b.Close()
	p := b.Pages()[0]

	getURL := func() string {
		links := p.Locator(`a[href^="/routine/"]`)
		c, _ := links.Count()
		for i := 0; i < c; i++ {
			text, _ := links.Nth(i).InnerText()
			if strings.HasPrefix(text, name) {
				href, _ := links.Nth(i).GetAttribute("href")
				return "https://hevy.com" + href
			}
		}
		return ""
	}

	if _, err = p.Goto("https://hevy.com/routines"); err != nil {
		return err
	}
	if err := waitForPage(ctx, p, o.Timeout, func(body string) bool { return !strings.Contains(body, "Loading...") }); err != nil {
		return fmt.Errorf("load routines page: %w", err)
	}
	routineURL := getURL()
	if routineURL == "" {
		return fmt.Errorf("routine not found: %q", name)
	}

	if _, err = p.Goto(routineURL); err != nil {
		return err
	}
	if err := waitForPage(ctx, p, o.Timeout, func(body string) bool { return !strings.Contains(body, "Loading...") }); err != nil {
		return fmt.Errorf("load routine page: %w", err)
	}
	dots := p.Locator(`[type="vertical-dots"]`).Last()
	if err := dots.Click(); err != nil {
		return fmt.Errorf("open menu: %w", err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := p.GetByText("Delete Routine").First().Click(); err != nil {
		return fmt.Errorf("click delete in menu: %w", err)
	}
	time.Sleep(500 * time.Millisecond)
	confirm := p.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Delete Routine", Exact: playwright.Bool(true)}).Last()
	if err := confirm.Click(); err != nil {
		return fmt.Errorf("confirm delete: %w", err)
	}
	if err := waitForPage(ctx, p, o.Timeout, func(body string) bool {
		return strings.Contains(body, "My Routines") || strings.Contains(body, "New Routine")
	}); err != nil {
		return fmt.Errorf("verify deletion: %w", err)
	}
	return nil
}

func Logout(profile string) error {
	if profile == "" || filepath.Base(profile) != "browser-profile" {
		return fmt.Errorf("refusing to remove unexpected browser profile path")
	}
	return os.RemoveAll(profile)
}
