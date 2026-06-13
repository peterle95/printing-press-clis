package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"mail-pp-cli/internal/accounts"
	ppmail "mail-pp-cli/internal/mail"
	"mail-pp-cli/internal/providers/eml"
	"mail-pp-cli/internal/providers/gmail"
	"mail-pp-cli/internal/providers/imap"
)

const (
	ExitOK           = 0
	ExitRuntimeError = 1
)

var version = "0.1.0"

type rootFlags struct {
	asJSON          bool
	agent           bool
	configPath      string
	credentialsPath string
	timeout         time.Duration
	noAutoRefresh   bool
}

type app struct {
	flags  *rootFlags
	config *accounts.Config
}

func Execute() error {
	return RootCmd().Execute()
}

func RootCmd() *cobra.Command {
	flags := &rootFlags{}
	root := &cobra.Command{
		Use:           "mail-pp-cli",
		Short:         "Unified Printing Press mail CLI for Gmail and Proton Bridge",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.SetVersionTemplate("mail-pp-cli {{ .Version }}\n")
	root.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "Output stable JSON")
	root.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Agent-friendly output (same as --json)")
	root.PersistentFlags().StringVar(&flags.configPath, "config", "", "Account config path")
	root.PersistentFlags().StringVar(&flags.credentialsPath, "google-credentials", gmail.DefaultCredentialsPath, "Google OAuth client JSON path")
	root.PersistentFlags().DurationVar(&flags.timeout, "timeout", 30*time.Second, "Network timeout")
	root.PersistentFlags().BoolVar(&flags.noAutoRefresh, "no-auto-refresh", false, "Disable configured Proton export auto-refresh")

	root.AddCommand(newAccountsCmd(flags))
	root.AddCommand(newAuthCmd(flags))
	root.AddCommand(newProtonCmd(flags))
	root.AddCommand(newInboxCmd(flags))
	root.AddCommand(newSearchCmd(flags))
	root.AddCommand(newReadCmd(flags))
	root.AddCommand(newDraftCmd(flags))
	root.AddCommand(newSendCmd(flags))
	root.AddCommand(newSummarizeCmd(flags))
	root.AddCommand(newWriteReplyCmd(flags))
	root.AddCommand(newArchiveCmd(flags))
	root.AddCommand(newLabelCmd(flags))
	return root
}

func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	return ExitRuntimeError
}

func loadApp(flags *rootFlags) (*app, error) {
	cfg, err := accounts.Load(flags.configPath)
	if err != nil {
		return nil, err
	}
	return &app{flags: flags, config: cfg}, nil
}

func commandContext(flags *rootFlags) (context.Context, context.CancelFunc) {
	timeout := flags.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return context.WithTimeout(context.Background(), timeout)
}

func (a *app) provider(account accounts.Account) (ppmail.MailProvider, error) {
	switch accounts.NormalizeProvider(account.Provider) {
	case accounts.ProviderGmail:
		return gmail.NewProvider(account, a.flags.credentialsPath, a.flags.timeout), nil
	case accounts.ProviderProton:
		return imap.NewProvider(account, a.flags.timeout), nil
	case accounts.ProviderProtonExport:
		return eml.NewProvider(account), nil
	default:
		return nil, fmt.Errorf("account %s has unsupported provider %q", account.Name, account.Provider)
	}
}

func (a *app) resolveProvider(identifier string) (accounts.Account, ppmail.MailProvider, error) {
	account, err := a.config.Resolve(identifier)
	if err != nil {
		return accounts.Account{}, nil, err
	}
	provider, err := a.provider(account)
	if err != nil {
		return accounts.Account{}, nil, err
	}
	return account, provider, nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func outputValue(flags *rootFlags, v any, text func() error) error {
	if flags.asJSON || flags.agent {
		return printJSON(v)
	}
	if text != nil {
		return text()
	}
	fmt.Println(v)
	return nil
}
