package traderepublic

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestImportDocumentsDoesNotInvokePytr(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "synthetic.pdf"), []byte("%PDF-1.4\nsynthetic document\n%%EOF\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{handler: func(_ context.Context, request RunRequest) error {
		if request.Argv[0] != "pdftotext-shim" {
			t.Fatalf("offline import invoked non-pdftotext command: %#v", request.Argv)
		}
		fixture, err := os.ReadFile("testdata/fixtures/statement.txt")
		if err != nil {
			return err
		}
		_, err = request.Stdout.Write(fixture)
		return err
	}}
	adapter, err := New(Options{
		Command:            []string{"pytr-must-not-run"},
		PDFToTextCommand:   []string{"pdftotext-shim", "-layout"},
		Runner:             runner,
		StagingDirectory:   filepath.Join(root, "staging"),
		DocumentsDirectory: destination,
		AccountTimezone:    "Europe/Berlin",
		Stderr:             io.Discard,
		Now:                func() time.Time { return time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.ImportDocuments(context.Background(), DocumentImportRequest{SourceDirectory: source})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Documents) != 1 || result.Documents[0].Source != "local:statement_pdf" || result.Documents[0].DocumentType != "trade_confirmation" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(runner.calls) != 1 || runner.calls[0].Argv[0] != "pdftotext-shim" {
		t.Fatalf("unexpected runner calls: %#v", runner.calls)
	}
}
