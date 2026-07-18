package traderepublic

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	calls   []RunRequest
	handler func(context.Context, RunRequest) error
}

func (f *fakeRunner) Run(ctx context.Context, request RunRequest) error {
	request.Argv = append([]string(nil), request.Argv...)
	f.calls = append(f.calls, request)
	if f.handler == nil {
		return nil
	}
	return f.handler(ctx, request)
}

func TestStatusAndInteractiveLoginUseDirectArgv(t *testing.T) {
	stdin := strings.NewReader("synthetic interactive input\n")
	var stdout, stderr bytes.Buffer
	credentialDir := t.TempDir()
	runner := &fakeRunner{}
	runner.handler = func(_ context.Context, request RunRequest) error {
		if hasArgument(request.Argv, "--version") {
			_, _ = io.WriteString(request.Stdout, "pytr 0.4.10\n")
		}
		if hasArgument(request.Argv, "--store_credentials") {
			if err := os.WriteFile(filepath.Join(credentialDir, "credentials"), []byte("{}"), 0o644); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(credentialDir, "cookies.example.txt"), []byte("cookie"), 0o644); err != nil {
				return err
			}
		}
		return nil
	}
	adapter, err := New(Options{
		Command:             []string{"uvx", "pytr@0.4.10"},
		Runner:              runner,
		Stdin:               stdin,
		Stdout:              &stdout,
		Stderr:              &stderr,
		StagingDirectory:    t.TempDir(),
		CredentialDirectory: credentialDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	status := adapter.Status(context.Background())
	if !status.Available || status.Version != SupportedPytrVersion {
		t.Fatalf("unexpected status: %#v", status)
	}
	if err := adapter.Login(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(runner.calls))
	}
	if got, want := runner.calls[0].Argv, []string{"uvx", "pytr@0.4.10", "--version"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("version argv = %#v, want %#v", got, want)
	}
	if got, want := runner.calls[1].Argv, []string{"uvx", "pytr@0.4.10", "login", "--store_credentials"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("login argv = %#v, want %#v", got, want)
	}
	if runner.calls[1].Stdin != stdin || runner.calls[1].Stdout != &stdout || runner.calls[1].Stderr != &stderr {
		t.Fatal("interactive streams were not forwarded to login")
	}
	for _, name := range []string{"credentials", "cookies.example.txt"} {
		info, err := os.Stat(filepath.Join(credentialDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("%s mode = %s, want 0600", name, got)
		}
	}
	assertNoShellInvocation(t, runner.calls)
}

func TestStatusRejectsUnvalidatedVersion(t *testing.T) {
	runner := &fakeRunner{handler: func(_ context.Context, request RunRequest) error {
		_, _ = io.WriteString(request.Stdout, "0.5.0\n")
		return nil
	}}
	adapter, err := New(Options{Runner: runner, StagingDirectory: t.TempDir(), Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	status := adapter.Status(context.Background())
	if status.Available || status.Version != "0.5.0" || !strings.Contains(status.Detail, SupportedPytrVersion) {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestSyncNormalizesExportsAndNeverParsesHumanCash(t *testing.T) {
	fixedNow := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	since := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	runner := newExportRunner(t, false)
	adapter, err := New(Options{
		Command:          []string{"pytr-shim", "--pinned-0.4.10"},
		Runner:           runner,
		StagingDirectory: filepath.Join(t.TempDir(), "staging"),
		AccountTimezone:  "Europe/Berlin",
		BaseCurrency:     "EUR",
		Stderr:           io.Discard,
		Now:              func() time.Time { return fixedNow },
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := adapter.Sync(context.Background(), SyncRequest{Since: &since})
	if err != nil {
		t.Fatal(err)
	}
	second, err := adapter.Sync(context.Background(), SyncRequest{Since: &since})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.CashBalances) != 0 {
		t.Fatalf("cash balances were parsed from human output: %#v", first.CashBalances)
	}
	if len(first.Positions) != 1 || first.Positions[0].Quantity.String() != "1.234567" || first.Positions[0].AverageCost.String() != "99.8765" {
		t.Fatalf("unexpected positions: %#v", first.Positions)
	}
	if len(first.Transactions) != 2 {
		t.Fatalf("transactions = %d, want 2: %#v", len(first.Transactions), first.Transactions)
	}
	buy := first.Transactions[0]
	if buy.Amount.String() != "-123.45678901" || buy.Quantity.String() != "1.23456789" || buy.Fees.String() != "1.25" {
		t.Fatalf("unexpected exact decimals: %#v", buy)
	}
	if got, want := buy.OccurredAt.UTC(), time.Date(2026, 7, 15, 8, 30, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("occurred_at = %s, want %s", got, want)
	}
	if buy.TimezoneAssumption != "Europe/Berlin" {
		t.Fatalf("timezone assumption = %q", buy.TimezoneAssumption)
	}
	if got := first.Transactions[1].Amount.String(); got != "12" {
		t.Fatalf("exponent amount = %s, want 12", got)
	}
	if first.Transactions[0].Fingerprint == "" || first.Transactions[0].Fingerprint != second.Transactions[0].Fingerprint {
		t.Fatalf("fingerprint is not idempotent: %q / %q", first.Transactions[0].Fingerprint, second.Transactions[0].Fingerprint)
	}
	if first.Provider != ProviderName || first.Adapter != AdapterName || first.AdapterVersion != SupportedPytrVersion {
		t.Fatalf("unexpected adapter identity: %#v", first)
	}
	portfolioCall := findCommandCall(t, runner.calls, "portfolio")
	assertArguments(t, portfolioCall.Argv,
		"--lang", "en", "--no-decimal-localization", "--no-include-watchlist",
		"--sort-by-column", "ISIN", "--sort-ascending", "--output",
	)
	transactionCall := findCommandCall(t, runner.calls, "export_transactions")
	assertArguments(t, transactionCall.Argv,
		"--lang", "en", "--date-with-time", "--no-decimal-localization", "--sort",
		"--last_days", "4", "--no-store-event-database", "--no-scan-for-duplicates",
		"--no-dump-raw-data", "--format", "json", "--outputdir",
	)
	assertNoShellInvocation(t, runner.calls)
}

func TestSyncDocumentsHashesPersistsAndParsesMetadata(t *testing.T) {
	fixedNow := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	documentsDir := filepath.Join(root, "documents")
	runner := newExportRunner(t, true)
	adapter, err := New(Options{
		Command:            []string{"pytr-shim"},
		PDFToTextCommand:   []string{"pdftotext-shim", "-layout"},
		Runner:             runner,
		StagingDirectory:   filepath.Join(root, "staging"),
		DocumentsDirectory: documentsDir,
		AccountTimezone:    "Europe/Berlin",
		Stderr:             io.Discard,
		Now:                func() time.Time { return fixedNow },
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := adapter.Sync(context.Background(), SyncRequest{IncludeDocuments: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Documents) != 1 {
		t.Fatalf("documents = %#v", snapshot.Documents)
	}
	document := snapshot.Documents[0]
	content := []byte("%PDF-1.4\nsynthetic document\n%%EOF\n")
	hash := sha256.Sum256(content)
	wantHash := hex.EncodeToString(hash[:])
	if document.ID != wantHash || document.SHA256 != wantHash || document.DocumentType != "trade_confirmation" || document.ISIN != "IE00B4L5Y983" {
		t.Fatalf("unexpected document: %#v", document)
	}
	if got, want := document.OccurredAt.Location().String(), "Europe/Berlin"; got != want {
		t.Fatalf("document location = %s, want %s", got, want)
	}
	if info, err := os.Stat(document.Path); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o600 {
		t.Fatalf("document mode = %o, want 600", info.Mode().Perm())
	}
	if info, err := os.Stat(documentsDir); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o700 {
		t.Fatalf("documents directory mode = %o, want 700", info.Mode().Perm())
	}
	downloadCall := findCommandCall(t, runner.calls, "dl_docs")
	assertArguments(t, downloadCall.Argv,
		"--lang", "en", "--date-with-time", "--no-decimal-localization", "--sort",
		"--last_days", "0", "--universal", "--no-store-event-database",
		"--no-dump-raw-data", "--no-export-transactions", "--flat",
	)
	pdfCall := findExecutableCall(t, runner.calls, "pdftotext-shim")
	if len(pdfCall.Argv) < 4 || pdfCall.Argv[len(pdfCall.Argv)-1] != "-" || pdfCall.Argv[1] != "-layout" {
		t.Fatalf("unexpected pdftotext argv: %#v", pdfCall.Argv)
	}
}

func newExportRunner(t *testing.T, withDocuments bool) *fakeRunner {
	t.Helper()
	runner := &fakeRunner{}
	runner.handler = func(_ context.Context, request RunRequest) error {
		switch {
		case hasArgument(request.Argv, "--version"):
			_, _ = io.WriteString(request.Stdout, "0.4.10\n")
			return nil
		case hasArgument(request.Argv, "portfolio"):
			if request.Stdout != nil {
				_, _ = io.WriteString(request.Stdout, "Cash EUR 999.99\n")
			}
			return copyFixture("testdata/fixtures/portfolio.csv", argumentValue(t, request.Argv, "--output"))
		case hasArgument(request.Argv, "export_transactions"):
			dir := argumentValue(t, request.Argv, "--outputdir")
			return copyFixture("testdata/fixtures/transactions.jsonl", filepath.Join(dir, "account_transactions.json"))
		case hasArgument(request.Argv, "dl_docs"):
			if !withDocuments {
				return fmt.Errorf("unexpected dl_docs call")
			}
			destination := request.Argv[len(request.Argv)-1]
			if err := os.MkdirAll(destination, 0o700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(destination, "synthetic-trade-confirmation.pdf"), []byte("%PDF-1.4\nsynthetic document\n%%EOF\n"), 0o600)
		case request.Argv[0] == "pdftotext-shim":
			fixture, err := os.ReadFile("testdata/fixtures/statement.txt")
			if err != nil {
				return err
			}
			_, err = request.Stdout.Write(fixture)
			return err
		default:
			return fmt.Errorf("unexpected argv: %#v", request.Argv)
		}
	}
	return runner
}

func copyFixture(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o600)
}

func findCommandCall(t *testing.T, calls []RunRequest, command string) RunRequest {
	t.Helper()
	for _, call := range calls {
		if hasArgument(call.Argv, command) {
			return call
		}
	}
	t.Fatalf("command %q not called; calls=%#v", command, calls)
	return RunRequest{}
}

func findExecutableCall(t *testing.T, calls []RunRequest, executable string) RunRequest {
	t.Helper()
	for _, call := range calls {
		if len(call.Argv) > 0 && call.Argv[0] == executable {
			return call
		}
	}
	t.Fatalf("executable %q not called", executable)
	return RunRequest{}
}

func hasArgument(argv []string, value string) bool {
	for _, argument := range argv {
		if argument == value {
			return true
		}
	}
	return false
}

func argumentValue(t *testing.T, argv []string, flag string) string {
	t.Helper()
	for index, argument := range argv {
		if argument == flag && index+1 < len(argv) {
			return argv[index+1]
		}
	}
	t.Fatalf("flag %q has no value in %#v", flag, argv)
	return ""
}

func assertArguments(t *testing.T, argv []string, expected ...string) {
	t.Helper()
	for _, value := range expected {
		if !hasArgument(argv, value) {
			t.Errorf("argv missing %q: %#v", value, argv)
		}
	}
}

func assertNoShellInvocation(t *testing.T, calls []RunRequest) {
	t.Helper()
	for _, call := range calls {
		if len(call.Argv) == 0 {
			t.Fatal("empty argv")
		}
		base := strings.ToLower(filepath.Base(call.Argv[0]))
		if base == "sh" || base == "bash" || base == "cmd" || base == "cmd.exe" || base == "powershell" || base == "powershell.exe" {
			t.Fatalf("shell invocation found: %#v", call.Argv)
		}
		if hasArgument(call.Argv, "-c") || hasArgument(call.Argv, "/c") {
			t.Fatalf("shell command flag found: %#v", call.Argv)
		}
	}
}
