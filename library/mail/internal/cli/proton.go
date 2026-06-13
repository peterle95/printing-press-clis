package cli

import (
	"fmt"
	"io"
	"net"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"mail-pp-cli/internal/accounts"
)

func newProtonCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "proton", Short: "Manage Proton Bridge accounts"}
	bridge := &cobra.Command{Use: "bridge", Short: "Inspect or configure Proton Bridge"}
	bridge.AddCommand(newProtonBridgeStatusCmd(flags))
	bridge.AddCommand(newProtonBridgeConfigureCmd(flags))
	cmd.AddCommand(bridge)
	export := &cobra.Command{Use: "export", Short: "Use Proton Mail Export Tool archives for Proton Free"}
	export.AddCommand(newProtonExportStatusCmd(flags))
	export.AddCommand(newProtonExportConfigureCmd(flags))
	export.AddCommand(newProtonExportRefreshCmd(flags))
	export.AddCommand(newProtonExportAutomateCmd(flags))
	export.AddCommand(newProtonExportPasswordCmd(flags))
	cmd.AddCommand(export)
	return cmd
}

func newProtonBridgeStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check Proton Bridge IMAP/SMTP reachability",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			var reports []map[string]any
			for _, account := range app.config.List() {
				if accounts.NormalizeProvider(account.Provider) != accounts.ProviderProton {
					continue
				}
				reports = append(reports, protonBridgeReport(account, flags.timeout))
			}
			return outputValue(flags, map[string]any{"accounts": reports}, func() error {
				for _, report := range reports {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", report["account"], report["address"])
					fmt.Fprintf(cmd.OutOrStdout(), "  IMAP %s reachable=%v\n", report["imap_host"], report["imap_reachable"])
					fmt.Fprintf(cmd.OutOrStdout(), "  SMTP %s reachable=%v\n", report["smtp_host"], report["smtp_reachable"])
					fmt.Fprintf(cmd.OutOrStdout(), "  password_command configured=%v\n", report["password_command_configured"])
				}
				return nil
			})
		},
	}
}

func newProtonBridgeConfigureCmd(flags *rootFlags) *cobra.Command {
	var accountRef string
	var username string
	var imapHost string
	var smtpHost string
	var passwordCommand string
	var noIMAPStartTLS bool
	var noSMTPStartTLS bool
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Update Proton Bridge settings in the unified account config",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			account, err := app.config.Resolve(accountRef)
			if err != nil {
				return err
			}
			account.Provider = accounts.ProviderProton
			if username != "" {
				account.Username = username
			}
			if imapHost != "" {
				account.IMAPHost = imapHost
			}
			if smtpHost != "" {
				account.SMTPHost = smtpHost
			}
			if passwordCommand != "" {
				account.PasswordCommand = passwordCommand
			}
			if noIMAPStartTLS {
				value := false
				account.IMAPStartTLS = &value
			}
			if noSMTPStartTLS {
				value := false
				account.SMTPStartTLS = &value
			}
			app.config.Accounts[account.Name] = account
			if err := app.config.Save(app.config.Path); err != nil {
				return err
			}
			return outputValue(flags, map[string]any{"updated": true, "account": account, "path": app.config.Path}, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "Updated %s in %s\n", account.Name, app.config.Path)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Proton account name or address")
	cmd.Flags().StringVar(&username, "username", "", "Proton Bridge username")
	cmd.Flags().StringVar(&imapHost, "imap", "", "Proton Bridge IMAP host:port")
	cmd.Flags().StringVar(&smtpHost, "smtp", "", "Proton Bridge SMTP host:port")
	cmd.Flags().StringVar(&passwordCommand, "password-command", "", "Command that prints the Proton Bridge password")
	cmd.Flags().BoolVar(&noIMAPStartTLS, "no-imap-starttls", false, "Disable IMAP STARTTLS")
	cmd.Flags().BoolVar(&noSMTPStartTLS, "no-smtp-starttls", false, "Disable SMTP STARTTLS")
	_ = cmd.MarkFlagRequired("account")
	return cmd
}

func newProtonExportStatusCmd(flags *rootFlags) *cobra.Command {
	var accountRef string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check Proton Export Tool archive readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			var reports []map[string]any
			if accountRef != "" {
				account, err := app.config.Resolve(accountRef)
				if err != nil {
					return err
				}
				reports = append(reports, protonExportReport(account))
			} else {
				for _, account := range app.config.List() {
					if accounts.NormalizeProvider(account.Provider) == accounts.ProviderProtonExport {
						reports = append(reports, protonExportReport(account))
					}
				}
			}
			return outputValue(flags, map[string]any{"accounts": reports}, func() error {
				for _, report := range reports {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", report["account"], report["address"])
					fmt.Fprintf(cmd.OutOrStdout(), "  archive: %s\n", report["archive_dir"])
					fmt.Fprintf(cmd.OutOrStdout(), "  exists=%v eml_count=%v\n", report["exists"], report["eml_count"])
					fmt.Fprintf(cmd.OutOrStdout(), "  draft_dir: %s\n", report["draft_dir"])
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Proton account name or address")
	return cmd
}

func newProtonExportConfigureCmd(flags *rootFlags) *cobra.Command {
	var accountRef string
	var archiveDir string
	var draftDir string
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Configure a Proton Free account to read local EML exports",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			account, err := app.config.Resolve(accountRef)
			if err != nil {
				return err
			}
			account.Provider = accounts.ProviderProtonExport
			account.IMAPHost = ""
			account.SMTPHost = ""
			account.PasswordCommand = ""
			account.IMAPStartTLS = nil
			account.SMTPStartTLS = nil
			if archiveDir != "" {
				account.ArchiveDir = archiveDir
			}
			if draftDir != "" {
				account.DraftDir = draftDir
			}
			if account.ArchiveDir == "" {
				account.ArchiveDir = "~/.local/share/printing-press/mail-private/proton-export/" + strings.TrimSuffix(account.Address, "@proton.me")
			}
			if account.DraftDir == "" {
				account.DraftDir = "~/.local/share/printing-press/mail-private/proton-drafts"
			}
			if account.ExportTool == "" {
				account.ExportTool = defaultProtonExportToolPath()
			}
			app.config.Accounts[account.Name] = account
			if err := app.config.Save(app.config.Path); err != nil {
				return err
			}
			expandedArchive, _ := account.ExpandedArchiveDir()
			expandedDraft, _ := account.ExpandedDraftDir()
			_ = os.MkdirAll(expandedArchive, 0o700)
			_ = os.MkdirAll(expandedDraft, 0o700)
			return outputValue(flags, map[string]any{"updated": true, "account": account, "path": app.config.Path}, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "Configured %s for Proton Export Tool archive mode.\n", account.Name)
				fmt.Fprintf(cmd.OutOrStdout(), "Export Proton EML files to: %s\n", expandedArchive)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Proton account name or address")
	cmd.Flags().StringVar(&archiveDir, "archive-dir", "", "Directory containing Proton Export Tool .eml files")
	cmd.Flags().StringVar(&draftDir, "draft-dir", "", "Directory where local Proton .eml drafts are written")
	_ = cmd.MarkFlagRequired("account")
	return cmd
}

func newProtonExportRefreshCmd(flags *rootFlags) *cobra.Command {
	var accountRef string
	var days int
	var toolPath string
	var passwordCommand string
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh a Proton Free local archive using the official Proton Mail Export Tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			account, err := app.config.Resolve(accountRef)
			if err != nil {
				return err
			}
			if accounts.NormalizeProvider(account.Provider) != accounts.ProviderProtonExport {
				return fmt.Errorf("account %s is not configured for proton-export-eml", account.Name)
			}
			if days <= 0 {
				days = 21
			}
			archiveDir, err := account.ExpandedArchiveDir()
			if err != nil {
				return err
			}
			if toolPath == "" {
				toolPath = firstNonEmpty(account.ExportTool, defaultProtonExportToolPath())
			}
			password, err := protonExportPassword(cmd, firstNonEmpty(passwordCommand, account.PasswordCommand), account.Name)
			if err != nil {
				return err
			}
			result, err := runProtonExportRefresh(cmd, account, archiveDir, toolPath, password, days)
			if err != nil {
				return err
			}
			return outputValue(flags, result, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "Refreshed %s\n", account.Name)
				fmt.Fprintf(cmd.OutOrStdout(), "Archive: %s\n", archiveDir)
				fmt.Fprintf(cmd.OutOrStdout(), "Kept %d of %d exported EML files from the last %d days\n", result["kept_eml"], result["exported_eml"], days)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Proton account name or address")
	cmd.Flags().IntVar(&days, "days", 21, "Keep only exported messages dated within this many days")
	cmd.Flags().StringVar(&toolPath, "tool", "", "Path to proton-mail-export-cli")
	cmd.Flags().StringVar(&passwordCommand, "password-command", "", "Optional command that prints the Proton password")
	_ = cmd.MarkFlagRequired("account")
	return cmd
}

func newProtonExportAutomateCmd(flags *rootFlags) *cobra.Command {
	var accountRef string
	var days int
	var toolPath string
	cmd := &cobra.Command{
		Use:   "automate",
		Short: "Store a local encrypted Proton export password and enable auto-refresh",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			account, err := app.config.Resolve(accountRef)
			if err != nil {
				return err
			}
			if accounts.NormalizeProvider(account.Provider) != accounts.ProviderProtonExport {
				return fmt.Errorf("account %s is not configured for proton-export-eml", account.Name)
			}
			if days <= 0 {
				days = 21
			}
			if toolPath == "" {
				toolPath = firstNonEmpty(account.ExportTool, defaultProtonExportToolPath())
			}
			password, err := promptPassword(cmd, fmt.Sprintf("Proton password for %s: ", account.Name))
			if err != nil {
				return err
			}
			secretPath, err := storeLocalSecret(account.Name, password)
			if err != nil {
				return err
			}
			account.PasswordCommand = localSecretPasswordCommand(account.Name)
			account.AutoRefreshDays = days
			account.ExportTool = toolPath
			app.config.Accounts[account.Name] = account
			if err := app.config.Save(app.config.Path); err != nil {
				return err
			}
			result := map[string]any{
				"account":           account.Name,
				"address":           account.Address,
				"auto_refresh_days": account.AutoRefreshDays,
				"export_tool":       account.ExportTool,
				"secret_path":       secretPath,
				"config_path":       app.config.Path,
			}
			return outputValue(flags, result, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "Configured automated Proton export refresh for %s.\n", account.Name)
				fmt.Fprintf(cmd.OutOrStdout(), "Auto-refresh days: %d\n", account.AutoRefreshDays)
				fmt.Fprintf(cmd.OutOrStdout(), "Encrypted local secret: %s\n", secretPath)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Proton account name or address")
	cmd.Flags().IntVar(&days, "days", 21, "Auto-refresh retention window in days")
	cmd.Flags().StringVar(&toolPath, "tool", "", "Path to proton-mail-export-cli")
	_ = cmd.MarkFlagRequired("account")
	return cmd
}

func newProtonExportPasswordCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "password", Short: "Manage the local encrypted Proton export password"}
	cmd.AddCommand(newProtonExportPasswordStatusCmd(flags))
	cmd.AddCommand(newProtonExportPasswordDeleteCmd(flags))
	cmd.AddCommand(newProtonExportPasswordGetCmd(flags))
	return cmd
}

func newProtonExportPasswordStatusCmd(flags *rootFlags) *cobra.Command {
	var accountRef string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check whether a local encrypted Proton export password is stored",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			account, err := app.config.Resolve(accountRef)
			if err != nil {
				return err
			}
			status, err := localSecretStatus(account.Name)
			if err != nil {
				return err
			}
			status["account"] = account.Name
			status["password_command_configured"] = strings.TrimSpace(account.PasswordCommand) != ""
			status["auto_refresh_days"] = account.AutoRefreshDays
			return outputValue(flags, status, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\tstored=%v\n", account.Name, status["stored"])
				fmt.Fprintf(cmd.OutOrStdout(), "  secret: %s\n", status["path"])
				fmt.Fprintf(cmd.OutOrStdout(), "  password_command configured=%v\n", status["password_command_configured"])
				fmt.Fprintf(cmd.OutOrStdout(), "  auto_refresh_days=%v\n", status["auto_refresh_days"])
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Proton account name or address")
	_ = cmd.MarkFlagRequired("account")
	return cmd
}

func newProtonExportPasswordDeleteCmd(flags *rootFlags) *cobra.Command {
	var accountRef string
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete the local encrypted Proton export password and disable auto-refresh",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			account, err := app.config.Resolve(accountRef)
			if err != nil {
				return err
			}
			status, err := deleteLocalSecret(account.Name)
			if err != nil {
				return err
			}
			account.PasswordCommand = ""
			account.AutoRefreshDays = 0
			app.config.Accounts[account.Name] = account
			if err := app.config.Save(app.config.Path); err != nil {
				return err
			}
			status["account"] = account.Name
			return outputValue(flags, status, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "Deleted local Proton export password for %s and disabled auto-refresh.\n", account.Name)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Proton account name or address")
	_ = cmd.MarkFlagRequired("account")
	return cmd
}

func newProtonExportPasswordGetCmd(flags *rootFlags) *cobra.Command {
	var accountRef string
	cmd := &cobra.Command{
		Use:    "get",
		Short:  "Print the local Proton export password for password_command use",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			account, err := app.config.Resolve(accountRef)
			if err != nil {
				return err
			}
			password, err := readLocalSecret(account.Name)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), password)
			return err
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Proton account name or address")
	_ = cmd.MarkFlagRequired("account")
	return cmd
}

func protonBridgeReport(account accounts.Account, timeout time.Duration) map[string]any {
	return map[string]any{
		"account":                     account.Name,
		"address":                     account.Address,
		"provider":                    "proton",
		"imap_host":                   account.IMAPHost,
		"smtp_host":                   account.SMTPHost,
		"imap_starttls":               account.IMAPStartTLSEnabled(),
		"smtp_starttls":               account.SMTPStartTLSEnabled(),
		"password_command_configured": account.PasswordCommand != "",
		"imap_reachable":              tcpReachable(account.IMAPHost, timeout),
		"smtp_reachable":              tcpReachable(account.SMTPHost, timeout),
	}
}

func tcpReachable(addr string, timeout time.Duration) bool {
	if timeout <= 0 || timeout > 5*time.Second {
		timeout = 5 * time.Second
	}
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func protonExportReport(account accounts.Account) map[string]any {
	archiveDir, archiveErr := account.ExpandedArchiveDir()
	draftDir, _ := account.ExpandedDraftDir()
	report := map[string]any{
		"account":                     account.Name,
		"address":                     account.Address,
		"provider":                    "proton-export-eml",
		"archive_dir":                 archiveDir,
		"draft_dir":                   draftDir,
		"export_tool":                 firstNonEmpty(account.ExportTool, defaultProtonExportToolPath()),
		"auto_refresh_days":           account.AutoRefreshDays,
		"password_command_configured": strings.TrimSpace(account.PasswordCommand) != "",
		"exists":                      false,
		"eml_count":                   0,
	}
	if archiveErr != nil {
		report["error"] = archiveErr.Error()
		return report
	}
	info, err := os.Stat(archiveDir)
	if err != nil {
		report["error"] = err.Error()
		return report
	}
	report["exists"] = info.IsDir()
	count := 0
	_ = filepath.WalkDir(archiveDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".eml") {
			count++
		}
		return nil
	})
	report["eml_count"] = count
	return report
}

func protonExportPassword(cmd *cobra.Command, passwordCommand, accountName string) (string, error) {
	if strings.TrimSpace(passwordCommand) != "" {
		out, err := exec.Command("bash", "-lc", passwordCommand).Output()
		if err != nil {
			return "", fmt.Errorf("password command failed: %w", err)
		}
		password := strings.TrimRight(string(out), "\r\n")
		if password == "" {
			return "", fmt.Errorf("password command produced no password")
		}
		return password, nil
	}
	return promptPassword(cmd, fmt.Sprintf("Proton password for %s: ", accountName))
}

func promptPassword(cmd *cobra.Command, prompt string) (string, error) {
	fmt.Fprint(cmd.ErrOrStderr(), prompt)
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(cmd.ErrOrStderr())
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	password := string(passwordBytes)
	if password == "" {
		return "", fmt.Errorf("empty password")
	}
	return password, nil
}

func maybeAutoRefreshProtonExport(cmd *cobra.Command, app *app, account accounts.Account) error {
	if app.flags.noAutoRefresh || accounts.NormalizeProvider(account.Provider) != accounts.ProviderProtonExport || account.AutoRefreshDays <= 0 {
		return nil
	}
	archiveDir, err := account.ExpandedArchiveDir()
	if err != nil {
		return err
	}
	toolPath := firstNonEmpty(account.ExportTool, defaultProtonExportToolPath())
	password, err := protonExportPassword(cmd, account.PasswordCommand, account.Name)
	if err != nil {
		return err
	}
	_, err = runProtonExportRefresh(cmd, account, archiveDir, toolPath, password, account.AutoRefreshDays)
	return err
}

func defaultProtonExportToolPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "proton-mail-export-cli"
	}
	return filepath.Join(home, ".local", "share", "printing-press", "mail-private", "tools", "proton-mail-export", "proton-mail-export-cli")
}

func localSecretPasswordCommand(accountName string) string {
	return "mail-pp-cli proton export password get --account " + shellQuote(accountName)
}

func runProtonExportRefresh(cmd *cobra.Command, account accounts.Account, archiveDir, toolPath, password string, days int) (map[string]any, error) {
	if _, err := os.Stat(toolPath); err != nil {
		return nil, fmt.Errorf("Proton Export Tool not found at %s: %w", toolPath, err)
	}
	parent := filepath.Dir(archiveDir)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return nil, err
	}
	refreshRoot, err := os.MkdirTemp(parent, ".refresh-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(refreshRoot)
	rawDir := filepath.Join(refreshRoot, "raw")
	filteredDir := filepath.Join(refreshRoot, "filtered")
	if err := os.MkdirAll(rawDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filteredDir, 0o700); err != nil {
		return nil, err
	}
	exportCmd := exec.Command(toolPath, "--operation", "backup", "--dir", rawDir, "--user", account.Address, "--telemetry")
	exportCmd.Dir = filepath.Dir(toolPath)
	exportCmd.Env = append(os.Environ(),
		"ET_OPERATION=backup",
		"ET_DIR="+rawDir,
		"ET_USER_EMAIL="+account.Address,
		"ET_USER_PASSWORD="+password,
		"ET_TELEMETRY_OFF=1",
	)
	exportCmd.Stdin = os.Stdin
	exportCmd.Stdout = cmd.ErrOrStderr()
	exportCmd.Stderr = cmd.ErrOrStderr()
	if err := exportCmd.Run(); err != nil {
		return nil, fmt.Errorf("Proton Export Tool failed: %w", err)
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	exported, kept, err := filterRecentEML(rawDir, filteredDir, cutoff)
	if err != nil {
		return nil, err
	}
	if err := replaceDir(archiveDir, filteredDir); err != nil {
		return nil, err
	}
	return map[string]any{
		"account":      account.Name,
		"address":      account.Address,
		"archive_dir":  archiveDir,
		"days":         days,
		"cutoff":       cutoff.Format(time.RFC3339),
		"exported_eml": exported,
		"kept_eml":     kept,
	}, nil
}

func filterRecentEML(srcRoot, dstRoot string, cutoff time.Time) (int, int, error) {
	exported := 0
	kept := 0
	err := filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".eml") {
			return nil
		}
		exported++
		date, err := emlDate(path)
		if err != nil || date.Before(cutoff) {
			return nil
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dstRoot, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := copyFile(path, target); err != nil {
			return err
		}
		kept++
		jsonPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".json"
		if _, err := os.Stat(jsonPath); err == nil {
			jsonTarget := strings.TrimSuffix(target, filepath.Ext(target)) + ".json"
			_ = copyFile(jsonPath, jsonTarget)
		}
		return nil
	})
	return exported, kept, err
}

func emlDate(path string) (time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()
	msg, err := mail.ReadMessage(f)
	if err != nil {
		return time.Time{}, err
	}
	if value := msg.Header.Get("Date"); value != "" {
		if date, err := mail.ParseDate(value); err == nil {
			return date, nil
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func replaceDir(target, replacement string) error {
	if strings.TrimSpace(target) == "" || target == "/" {
		return fmt.Errorf("refusing to replace unsafe archive directory %q", target)
	}
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	backup := filepath.Join(parent, "."+filepath.Base(target)+".old")
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(replacement, target); err != nil {
		if _, statErr := os.Stat(backup); statErr == nil {
			_ = os.Rename(backup, target)
		}
		return err
	}
	_ = os.RemoveAll(backup)
	return nil
}
