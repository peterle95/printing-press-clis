package traderepublic

import (
	"os"
	"testing"
	"time"
)

func TestParseStatementMetadataFromSyntheticText(t *testing.T) {
	text, err := os.ReadFile("testdata/fixtures/statement.txt")
	if err != nil {
		t.Fatal(err)
	}
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	metadata := ParseStatementMetadata(string(text), location)
	if metadata.DocumentType != "trade_confirmation" || metadata.ISIN != "IE00B4L5Y983" {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
	want := time.Date(2026, 7, 15, 0, 0, 0, 0, location)
	if !metadata.OccurredAt.Equal(want) {
		t.Fatalf("occurred_at = %s, want %s", metadata.OccurredAt, want)
	}
}

func TestParseStatementMetadataRejectsInvalidISINCandidate(t *testing.T) {
	metadata := ParseStatementMetadata("Dividend statement\nISIN: IE00B4L5Y980\nDate: 2026-07-15", time.UTC)
	if metadata.DocumentType != "dividend_statement" || metadata.ISIN != "" {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
}
