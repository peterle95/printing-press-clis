package mail

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"unicode"
)

type SummarizeOptions struct {
	SummarizerCommand string
}

func Summarize(ctx context.Context, msg Message, opts SummarizeOptions) (Summary, error) {
	s := fallbackSummary(msg)
	if strings.TrimSpace(opts.SummarizerCommand) == "" {
		s.FallbackUsed = true
		return s, nil
	}
	text := renderMessageForSummarizer(msg)
	cmd := shellCommand(ctx, opts.SummarizerCommand)
	cmd.Stdin = strings.NewReader(text)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return s, fmt.Errorf("summarizer command failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	s.LLMOutput = strings.TrimSpace(stdout.String())
	s.Summarizer = opts.SummarizerCommand
	s.FallbackUsed = false
	return s, nil
}

func fallbackSummary(msg Message) Summary {
	body := normalizeSpace(msg.Body)
	return Summary{
		MessageID:    msg.ID,
		Account:      msg.Account,
		Provider:     msg.Provider,
		From:         msg.From,
		To:           msg.To,
		Cc:           msg.Cc,
		Subject:      msg.Subject,
		Date:         msg.Date,
		Excerpt:      excerpt(body, 900),
		ActionItems:  detectActionItems(msg.Body),
		Dates:        uniqueMatches(datePattern, msg.Body),
		Links:        uniqueMatches(linkPattern, msg.Body),
		Attachments:  msg.Attachments,
		FallbackUsed: true,
	}
}

func renderMessageForSummarizer(msg Message) string {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\n", msg.From)
	fmt.Fprintf(&b, "To: %s\n", strings.Join(msg.To, ", "))
	if len(msg.Cc) > 0 {
		fmt.Fprintf(&b, "Cc: %s\n", strings.Join(msg.Cc, ", "))
	}
	fmt.Fprintf(&b, "Subject: %s\n", msg.Subject)
	if !msg.Date.IsZero() {
		fmt.Fprintf(&b, "Date: %s\n", msg.Date.Format("2006-01-02 15:04 MST"))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, msg.Body)
	return b.String()
}

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "bash", "-lc", command)
}

func excerpt(text string, limit int) string {
	text = normalizeSpace(text)
	if len(text) <= limit {
		return text
	}
	return strings.TrimSpace(text[:limit]) + "..."
}

func normalizeSpace(text string) string {
	return strings.Join(strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r)
	}), " ")
}

func detectActionItems(body string) []string {
	lines := strings.Split(body, "\n")
	var out []string
	seen := map[string]bool{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || len(trimmed) > 280 {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "please") ||
			strings.Contains(lower, "can you") ||
			strings.Contains(lower, "could you") ||
			strings.Contains(lower, "need to") ||
			strings.Contains(lower, "action") ||
			strings.Contains(lower, "todo") ||
			strings.Contains(lower, "required") {
			if !seen[trimmed] {
				seen[trimmed] = true
				out = append(out, trimmed)
			}
		}
	}
	return out
}

func uniqueMatches(re *regexp.Regexp, text string) []string {
	matches := re.FindAllString(text, -1)
	out := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		match = strings.Trim(match, ".,;:)]}>\"'")
		if match != "" && !seen[match] {
			seen[match] = true
			out = append(out, match)
		}
	}
	return out
}

var (
	linkPattern = regexp.MustCompile(`https?://[^\s<>"']+`)
	datePattern = regexp.MustCompile(`(?i)\b(?:\d{4}-\d{2}-\d{2}|\d{1,2}[./-]\d{1,2}[./-]\d{2,4}|(?:jan|feb|mar|apr|may|jun|jul|aug|sep|sept|oct|nov|dec)[a-z]*\s+\d{1,2}(?:,\s*\d{4})?)\b`)
)
