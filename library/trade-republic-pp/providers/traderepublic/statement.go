package traderepublic

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"trade-republic-pp-cli/internal/instruments"
)

const StatementParserVersion = "tr-statement-text-v1"

var (
	statementISINPattern    = regexp.MustCompile(`\b[A-Z]{2}[A-Z0-9]{9}[0-9]\b`)
	statementISODatePattern = regexp.MustCompile(`\b(20[0-9]{2})[-/.]([01]?[0-9])[-/.]([0-3]?[0-9])\b`)
	statementEUDatePattern  = regexp.MustCompile(`\b([0-3]?[0-9])[./-]([01]?[0-9])[./-](20[0-9]{2})\b`)
)

type StatementMetadata struct {
	DocumentType string
	OccurredAt   time.Time
	ISIN         string
}

// ParseStatementMetadata interprets already-extracted statement text. Keeping
// this pure parser separate from pdftotext makes it deterministic and safe to
// test with synthetic, non-account fixtures.
func ParseStatementMetadata(text string, location *time.Location) StatementMetadata {
	if location == nil {
		location = time.UTC
	}
	upper := strings.ToUpper(text)
	metadata := StatementMetadata{DocumentType: classifyStatementText(upper)}
	for _, candidate := range statementISINPattern.FindAllString(upper, -1) {
		candidate = instruments.NormalizeISIN(candidate)
		if instruments.ValidateISIN(candidate) == nil {
			metadata.ISIN = candidate
			break
		}
	}
	metadata.OccurredAt = firstStatementDate(text, location)
	return metadata
}

func classifyStatementText(upper string) string {
	switch {
	case strings.Contains(upper, "DIVIDEND") || strings.Contains(upper, "DIVIDENDE") || strings.Contains(upper, "ERTRAGSAUSSCHÜTTUNG"):
		return "dividend_statement"
	case strings.Contains(upper, "TAX STATEMENT") || strings.Contains(upper, "STEUERABRECHNUNG") || strings.Contains(upper, "STEUERBESCHEINIGUNG"):
		return "tax_statement"
	case strings.Contains(upper, "SECURITIES SETTLEMENT") || strings.Contains(upper, "TRADE CONFIRMATION") || strings.Contains(upper, "WERTPAPIERABRECHNUNG") || strings.Contains(upper, "ABRECHNUNG AUSFÜHRUNG"):
		return "trade_confirmation"
	case strings.Contains(upper, "ACCOUNT STATEMENT") || strings.Contains(upper, "KONTOAUSZUG"):
		return "account_statement"
	case strings.Contains(upper, "COST INFORMATION") || strings.Contains(upper, "KOSTENINFORMATION"):
		return "cost_information"
	default:
		return "unknown"
	}
}

func firstStatementDate(text string, location *time.Location) time.Time {
	lines := strings.Split(text, "\n")
	preferred := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		upper := strings.ToUpper(line)
		if strings.Contains(upper, "DATE") || strings.Contains(upper, "DATUM") || strings.Contains(upper, "EXECUTION") || strings.Contains(upper, "AUSFÜHR") || strings.Contains(upper, "VALUTA") {
			preferred = append(preferred, line)
		}
	}
	preferred = append(preferred, text)
	for _, value := range preferred {
		if match := statementISODatePattern.FindStringSubmatch(value); len(match) == 4 {
			if parsed := dateFromParts(match[1], match[2], match[3], location); !parsed.IsZero() {
				return parsed
			}
		}
		if match := statementEUDatePattern.FindStringSubmatch(value); len(match) == 4 {
			if parsed := dateFromParts(match[3], match[2], match[1], location); !parsed.IsZero() {
				return parsed
			}
		}
	}
	return time.Time{}
}

func dateFromParts(yearText, monthText, dayText string, location *time.Location) time.Time {
	year, yearErr := strconv.Atoi(yearText)
	month, monthErr := strconv.Atoi(monthText)
	day, dayErr := strconv.Atoi(dayText)
	if yearErr != nil || monthErr != nil || dayErr != nil || month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}
	}
	parsed := time.Date(year, time.Month(month), day, 0, 0, 0, 0, location)
	if parsed.Year() != year || int(parsed.Month()) != month || parsed.Day() != day {
		return time.Time{}
	}
	return parsed
}

func extractPDFText(ctx context.Context, runner Runner, command []string, path string) (string, error) {
	if len(command) == 0 {
		command = []string{"pdftotext", "-layout"}
	}
	if strings.TrimSpace(command[0]) == "" {
		return "", fmt.Errorf("pdftotext command is not configured")
	}
	stdout := newBoundedBuffer(8 << 20)
	stderr := newBoundedBuffer(16 << 10)
	argv := append(append([]string(nil), command...), path, "-")
	if err := runner.Run(ctx, RunRequest{Argv: argv, Stdout: stdout, Stderr: stderr}); err != nil {
		return "", fmt.Errorf("pdftotext failed: %w", err)
	}
	if stdout.truncated {
		return "", fmt.Errorf("pdftotext output exceeds 8 MiB")
	}
	return stdout.String(), nil
}

func inferDocumentTypeFromFilename(filename string) string {
	return classifyStatementText(strings.ToUpper(strings.NewReplacer("_", " ", "-", " ").Replace(filename)))
}
