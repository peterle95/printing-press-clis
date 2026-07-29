package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
)

func main() {
	profile := os.ExpandEnv("$HOME/.local/share/hevy-cli/browser-profile")

	pw, err := playwright.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "playwright run: %v\n", err)
		os.Exit(1)
	}
	defer pw.Stop()

	b, err := pw.Chromium.LaunchPersistentContext(profile, playwright.BrowserTypeLaunchPersistentContextOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "launch: %v\n", err)
		os.Exit(1)
	}
	defer b.Close()

	p := b.Pages()[0]
	if _, err = p.Goto("https://hevy.com/create-routine"); err != nil {
		fmt.Fprintf(os.Stderr, "goto: %v\n", err)
		os.Exit(1)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		body, err := p.Locator("body").InnerText()
		if err == nil && !strings.Contains(body, "Loading...") {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	body, err := p.Locator("body").InnerText()
	if err != nil {
		fmt.Fprintf(os.Stderr, "body text: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== BODY TEXT ===")
	fmt.Println(body)
	fmt.Println("=== END BODY TEXT ===")

	// Try to find the exercise library list items
	items, err := p.Locator(`li, [role="listitem"], .exercise-item, [class*="exercise"]`).All()
	if err == nil {
		fmt.Printf("\n=== FOUND %d LIST/EXERCISE ITEMS ===\n", len(items))
		for i, item := range items {
			txt, _ := item.InnerText()
			fmt.Printf("[%d] %q\n", i, strings.TrimSpace(txt))
		}
	}

	// Get all button text
	buttons, err := p.Locator("button").All()
	if err == nil {
		fmt.Printf("\n=== FOUND %d BUTTONS ===\n", len(buttons))
		for i, b := range buttons {
			txt, _ := b.InnerText()
			fmt.Printf("[%d] %q\n", i, strings.TrimSpace(txt))
		}
	}

	// Get all links
	links, err := p.Locator("a").All()
	if err == nil {
		fmt.Printf("\n=== FOUND %d LINKS ===\n", len(links))
		for i, l := range links {
			txt, _ := l.InnerText()
			href, _ := l.GetAttribute("href")
			fmt.Printf("[%d] %q href=%s\n", i, strings.TrimSpace(txt), href)
		}
	}

	// Get all inputs
	inputs, err := p.Locator("input").All()
	if err == nil {
		fmt.Printf("\n=== FOUND %d INPUTS ===\n", len(inputs))
		for i, in := range inputs {
			ph, _ := in.GetAttribute("placeholder")
			typ, _ := in.GetAttribute("type")
			fmt.Printf("[%d] type=%s placeholder=%q\n", i, typ, ph)
		}
	}

	// Try common exercise library selectors
	for _, sel := range []string{
		`[class*="exercise-bank"]`,
		`[class*="exercise-search"]`,
		`[class*="exercise-list"]`,
		`[class*="library"]`,
		`[data-testid*="exercise"]`,
		`div[role="listbox"]`,
		`[role="list"]`,
	} {
		els, err := p.Locator(sel).All()
		if err == nil {
			for _, el := range els {
				txt, _ := el.InnerText()
				if strings.TrimSpace(txt) != "" {
					fmt.Printf("\n=== SELECTOR %q ===\n%s\n", sel, strings.TrimSpace(txt))
				}
			}
		}
	}
}
