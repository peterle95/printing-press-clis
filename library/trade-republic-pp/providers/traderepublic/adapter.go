package traderepublic

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"trade-republic-pp-cli/internal/instruments"
)

const (
	ProviderName         = "trade_republic"
	AdapterName          = "pytr"
	SupportedPytrVersion = "0.4.10"
)

var versionPattern = regexp.MustCompile(`\b[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?\b`)

// Options contains only local adapter settings. Authentication secrets are
// deliberately absent: pytr obtains them through its interactive web-login
// flow and its own private credential store.
type Options struct {
	Command             []string
	PDFToTextCommand    []string
	CredentialDirectory string
	StagingDirectory    string
	DocumentsDirectory  string
	AccountTimezone     string
	BaseCurrency        string
	Timeout             time.Duration
	Runner              Runner
	Stdin               io.Reader
	Stdout              io.Writer
	Stderr              io.Writer
	Now                 func() time.Time
}

type Adapter struct {
	command             []string
	pdfToTextCommand    []string
	credentialDirectory string
	stagingDirectory    string
	documentsDirectory  string
	location            *time.Location
	baseCurrency        string
	timeout             time.Duration
	runner              Runner
	stdin               io.Reader
	stdout              io.Writer
	stderr              io.Writer
	now                 func() time.Time
}

var _ Provider = (*Adapter)(nil)

func New(options Options) (*Adapter, error) {
	if len(options.Command) == 0 {
		options.Command = []string{"pytr"}
	}
	if strings.TrimSpace(options.Command[0]) == "" {
		return nil, fmt.Errorf("pytr command must contain an executable")
	}
	if containsCredentialArgument(options.Command) {
		return nil, fmt.Errorf("pytr command must not contain phone or PIN arguments")
	}
	if options.AccountTimezone == "" {
		options.AccountTimezone = "Europe/Berlin"
	}
	location, err := time.LoadLocation(options.AccountTimezone)
	if err != nil {
		return nil, fmt.Errorf("account timezone %q: %w", options.AccountTimezone, err)
	}
	if options.BaseCurrency == "" {
		options.BaseCurrency = "EUR"
	}
	options.BaseCurrency = strings.ToUpper(strings.TrimSpace(options.BaseCurrency))
	if len(options.BaseCurrency) != 3 {
		return nil, fmt.Errorf("base currency must be a three-letter ISO 4217 code")
	}
	if options.StagingDirectory == "" {
		options.StagingDirectory = filepath.Join(os.TempDir(), "trade-republic-pp-staging")
	}
	if options.Timeout == 0 {
		options.Timeout = 5 * time.Minute
	}
	if options.Timeout < 0 {
		return nil, fmt.Errorf("timeout must not be negative")
	}
	if options.Runner == nil {
		options.Runner = ExecRunner{}
	}
	if options.Stdin == nil {
		options.Stdin = os.Stdin
	}
	if options.Stdout == nil {
		options.Stdout = os.Stdout
	}
	if options.Stderr == nil {
		options.Stderr = os.Stderr
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Adapter{
		command:             append([]string(nil), options.Command...),
		pdfToTextCommand:    append([]string(nil), options.PDFToTextCommand...),
		credentialDirectory: options.CredentialDirectory,
		stagingDirectory:    options.StagingDirectory,
		documentsDirectory:  options.DocumentsDirectory,
		location:            location,
		baseCurrency:        options.BaseCurrency,
		timeout:             options.Timeout,
		runner:              options.Runner,
		stdin:               options.Stdin,
		stdout:              options.Stdout,
		stderr:              options.Stderr,
		now:                 options.Now,
	}, nil
}

func (a *Adapter) Status(ctx context.Context) Status {
	version, err := a.version(ctx)
	if err != nil {
		return Status{Available: false, Detail: "pytr unavailable: " + err.Error()}
	}
	if version != SupportedPytrVersion {
		return Status{
			Available: false,
			Version:   version,
			Detail:    fmt.Sprintf("adapter is validated for pytr %s; found %s", SupportedPytrVersion, version),
		}
	}
	return Status{Available: true, Version: version, Detail: "supported pytr version"}
}

// Login starts pytr's interactive web login. No phone number, PIN, or second
// factor is accepted by this API, which keeps secrets out of argv and logs.
func (a *Adapter) Login(ctx context.Context) error {
	credentialDirectory, err := a.prepareCredentialDirectory()
	if err != nil {
		return err
	}
	ctx, cancel := a.commandContext(ctx)
	defer cancel()
	if err := a.runner.Run(ctx, RunRequest{
		Argv:   a.argv("login", "--store_credentials"),
		Stdin:  a.stdin,
		Stdout: a.stdout,
		Stderr: a.stderr,
	}); err != nil {
		return fmt.Errorf("pytr login failed: %w", err)
	}
	if err := secureCredentialFiles(credentialDirectory); err != nil {
		return fmt.Errorf("secure pytr credential store: %w", err)
	}
	return nil
}

func (a *Adapter) Sync(ctx context.Context, request SyncRequest) (Snapshot, error) {
	ctx, cancel := a.commandContext(ctx)
	defer cancel()

	version, err := a.version(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("check pytr version: %w", err)
	}
	if version != SupportedPytrVersion {
		return Snapshot{}, fmt.Errorf("unsupported pytr version %s; adapter requires %s", version, SupportedPytrVersion)
	}

	now := a.now()
	lastDays := conservativeLastDays(now, request.Since)
	stage, err := a.newStage()
	if err != nil {
		return Snapshot{}, err
	}
	defer os.RemoveAll(stage)

	portfolioPath := filepath.Join(stage, "portfolio.csv")
	if err := createPrivateFile(portfolioPath); err != nil {
		return Snapshot{}, err
	}
	if err := a.runHelper(ctx, stage, a.argv(
		"portfolio",
		"--lang", "en",
		"--no-decimal-localization",
		"--no-include-watchlist",
		"--sort-by-column", "ISIN",
		"--sort-ascending",
		"--output", portfolioPath,
	)); err != nil {
		return Snapshot{}, fmt.Errorf("pytr portfolio export failed: %w", err)
	}

	transactionDir := filepath.Join(stage, "transactions")
	if err := ensurePrivateDir(transactionDir); err != nil {
		return Snapshot{}, err
	}
	transactionPath := filepath.Join(transactionDir, "account_transactions.json")
	if err := createPrivateFile(transactionPath); err != nil {
		return Snapshot{}, err
	}
	if err := a.runHelper(ctx, transactionDir, a.argv(
		"export_transactions",
		"--lang", "en",
		"--date-with-time",
		"--no-decimal-localization",
		"--sort",
		"--last_days", strconv.Itoa(lastDays),
		"--no-store-event-database",
		"--no-scan-for-duplicates",
		"--no-dump-raw-data",
		"--format", "json",
		"--outputdir", transactionDir,
	)); err != nil {
		return Snapshot{}, fmt.Errorf("pytr transaction export failed: %w", err)
	}

	asOf := a.now().UTC()
	parsedInstruments, positions, err := parsePortfolioCSV(portfolioPath, a.baseCurrency, asOf)
	if err != nil {
		return Snapshot{}, fmt.Errorf("parse pytr portfolio CSV: %w", err)
	}
	parsedTransactions, err := parseTransactionJSONL(transactionPath, a.baseCurrency, a.location, request.Since)
	if err != nil {
		return Snapshot{}, fmt.Errorf("parse pytr transaction JSONL: %w", err)
	}

	documents := []DocumentAlias{}
	warnings := []string{"cash balance omitted: pytr portfolio CSV does not contain cash and human stdout is never parsed"}
	if request.IncludeDocuments {
		destination := request.DocumentsDir
		if destination == "" {
			destination = a.documentsDirectory
		}
		if destination == "" {
			return Snapshot{}, fmt.Errorf("documents directory is required when document sync is requested")
		}
		var documentWarnings []string
		documents, documentWarnings, err = a.syncDocuments(ctx, stage, destination, lastDays, request.Since, asOf)
		if err != nil {
			return Snapshot{}, err
		}
		warnings = append(warnings, documentWarnings...)
	}

	instrumentMap := make(map[string]instruments.Instrument, len(parsedInstruments)+len(parsedTransactions))
	for _, instrument := range parsedInstruments {
		instrumentMap[instrument.ISIN] = instrument
	}
	for _, transaction := range parsedTransactions {
		if transaction.ISIN == "" {
			continue
		}
		if _, ok := instrumentMap[transaction.ISIN]; !ok {
			instrumentMap[transaction.ISIN] = placeholderInstrument(transaction.ISIN, transaction.Note, a.baseCurrency, asOf)
		}
	}
	for _, document := range documents {
		if document.ISIN == "" {
			continue
		}
		if _, ok := instrumentMap[document.ISIN]; !ok {
			instrumentMap[document.ISIN] = placeholderInstrument(document.ISIN, "", a.baseCurrency, asOf)
		}
	}
	normalizedInstruments := make([]instruments.Instrument, 0, len(instrumentMap))
	for _, instrument := range instrumentMap {
		normalizedInstruments = append(normalizedInstruments, instrument)
	}
	sort.Slice(normalizedInstruments, func(i, j int) bool { return normalizedInstruments[i].ISIN < normalizedInstruments[j].ISIN })

	normalizedDocuments := makeDocuments(documents)
	return Snapshot{
		Provider:       ProviderName,
		Adapter:        AdapterName,
		AdapterVersion: version,
		AsOf:           asOf,
		Instruments:    normalizedInstruments,
		Positions:      positions,
		CashBalances:   nil,
		Transactions:   parsedTransactions,
		Documents:      normalizedDocuments,
		Warnings:       warnings,
	}, nil
}

func (a *Adapter) version(ctx context.Context) (string, error) {
	ctx, cancel := a.commandContext(ctx)
	defer cancel()
	stdout := newBoundedBuffer(16 << 10)
	stderr := newBoundedBuffer(16 << 10)
	if err := a.runner.Run(ctx, RunRequest{Argv: a.argv("--version"), Stdout: stdout, Stderr: stderr}); err != nil {
		return "", err
	}
	match := versionPattern.FindString(stdout.String())
	if match == "" {
		return "", fmt.Errorf("pytr returned no parseable version")
	}
	return match, nil
}

func (a *Adapter) commandContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, a.timeout)
}

func (a *Adapter) argv(arguments ...string) []string {
	argv := make([]string, 0, len(a.command)+len(arguments))
	argv = append(argv, a.command...)
	return append(argv, arguments...)
}

func (a *Adapter) runHelper(ctx context.Context, directory string, argv []string) error {
	// pytr's human-readable stdout includes a cash line. It is routed to the
	// diagnostic stream and never parsed or mixed with structured CLI output.
	return a.runner.Run(ctx, RunRequest{Argv: argv, Dir: directory, Stdout: a.stderr, Stderr: a.stderr})
}

func (a *Adapter) newStage() (string, error) {
	if err := ensurePrivateDir(a.stagingDirectory); err != nil {
		return "", fmt.Errorf("create staging directory: %w", err)
	}
	stage, err := os.MkdirTemp(a.stagingDirectory, ".sync-*")
	if err != nil {
		return "", fmt.Errorf("create sync staging directory: %w", err)
	}
	if err := os.Chmod(stage, 0o700); err != nil {
		_ = os.RemoveAll(stage)
		return "", err
	}
	return stage, nil
}

func ensurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return os.Chmod(path, 0o700)
}

func createPrivateFile(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	return file.Close()
}

func conservativeLastDays(now time.Time, since *time.Time) int {
	if since == nil {
		return 0
	}
	if !since.Before(now) {
		return 1
	}
	days := int(math.Ceil(now.Sub(*since).Hours()/24)) + 2
	if days < 1 {
		return 1
	}
	return days
}

func placeholderInstrument(isin, name, currency string, asOf time.Time) instruments.Instrument {
	name = strings.TrimSpace(name)
	if name == "" {
		name = isin
	}
	return instruments.Instrument{
		ISIN:            isin,
		Name:            name,
		BaseCurrency:    currency,
		TradingCurrency: currency,
		UpdatedAt:       asOf,
	}
}

func containsCredentialArgument(command []string) bool {
	for _, argument := range command {
		key := strings.ToLower(strings.TrimSpace(argument))
		if key == "-p" || key == "--pin" || strings.HasPrefix(key, "--pin=") ||
			key == "-n" || key == "--phone_no" || key == "--phone-no" ||
			strings.HasPrefix(key, "--phone_no=") || strings.HasPrefix(key, "--phone-no=") {
			return true
		}
	}
	return false
}

func (a *Adapter) prepareCredentialDirectory() (string, error) {
	directory := a.credentialDirectory
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve pytr credential directory: %w", err)
		}
		directory = filepath.Join(home, ".pytr")
	}
	directory, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("resolve pytr credential directory: %w", err)
	}
	if info, err := os.Lstat(directory); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf("pytr credential directory must be a real directory")
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect pytr credential directory: %w", err)
	} else if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create pytr credential directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return "", fmt.Errorf("secure pytr credential directory: %w", err)
	}
	return directory, nil
}

func secureCredentialFiles(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	credentialsFound := false
	cookiesFound := false
	for _, entry := range entries {
		name := entry.Name()
		isCredentials := name == "credentials"
		isCookies := strings.HasPrefix(name, "cookies.") && strings.HasSuffix(name, ".txt")
		if !isCredentials && !isCookies {
			continue
		}
		path := filepath.Join(directory, name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("pytr credential artifacts must be regular files")
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return err
		}
		credentialsFound = credentialsFound || isCredentials
		cookiesFound = cookiesFound || isCookies
	}
	if !credentialsFound || !cookiesFound {
		return fmt.Errorf("pytr did not persist the expected credentials and web-session cookie")
	}
	return nil
}
