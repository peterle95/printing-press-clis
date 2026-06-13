package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFilterRecentEMLKeepsOnlyMessagesAfterCutoff(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	if err := os.MkdirAll(src, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		t.Fatal(err)
	}

	recentDate := time.Now().AddDate(0, 0, -2).Format(time.RFC1123Z)
	oldDate := time.Now().AddDate(0, 0, -40).Format(time.RFC1123Z)
	writeTestEML(t, filepath.Join(src, "recent.eml"), recentDate)
	writeTestEML(t, filepath.Join(src, "old.eml"), oldDate)

	exported, kept, err := filterRecentEML(src, dst, time.Now().AddDate(0, 0, -21))
	if err != nil {
		t.Fatal(err)
	}
	if exported != 2 {
		t.Fatalf("exported = %d, want 2", exported)
	}
	if kept != 1 {
		t.Fatalf("kept = %d, want 1", kept)
	}
	if _, err := os.Stat(filepath.Join(dst, "recent.eml")); err != nil {
		t.Fatalf("recent message not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "old.eml")); !os.IsNotExist(err) {
		t.Fatalf("old message should not be copied, stat err = %v", err)
	}
}

func writeTestEML(t *testing.T, path, date string) {
	t.Helper()
	body := "Date: " + date + "\r\nSubject: Test\r\nFrom: sender@example.com\r\nTo: receiver@example.com\r\n\r\nHello\r\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
