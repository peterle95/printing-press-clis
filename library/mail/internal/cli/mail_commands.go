package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"mail-pp-cli/internal/accounts"
	ppmail "mail-pp-cli/internal/mail"
)

type messageAggregate struct {
	Messages []ppmail.Message `json:"messages"`
	Errors   []accountError   `json:"errors,omitempty"`
}

func newInboxCmd(flags *rootFlags) *cobra.Command {
	var accountRef string
	var all bool
	var limit int
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "List recent inbox messages",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			result, err := runMessageList(cmd, app, accountRef, all, limit, func(provider ppmail.MailProvider) ([]ppmail.Message, error) {
				ctx, cancel := commandContext(flags)
				defer cancel()
				return provider.Inbox(ctx, limit)
			})
			outErr := outputValue(flags, result, func() error {
				printMessages(cmd.OutOrStdout(), result.Messages)
				warnErrors(result.Errors)
				return nil
			})
			if outErr != nil {
				return outErr
			}
			return err
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Account name or address")
	cmd.Flags().BoolVar(&all, "all", false, "Search all configured accounts")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum messages per account")
	return cmd
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var accountRef string
	var all bool
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search normalized messages across Gmail or Proton Bridge",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			result, err := runMessageList(cmd, app, accountRef, all, limit, func(provider ppmail.MailProvider) ([]ppmail.Message, error) {
				ctx, cancel := commandContext(flags)
				defer cancel()
				return provider.Search(ctx, query, limit)
			})
			outErr := outputValue(flags, result, func() error {
				printMessages(cmd.OutOrStdout(), result.Messages)
				warnErrors(result.Errors)
				return nil
			})
			if outErr != nil {
				return outErr
			}
			return err
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Account name or address")
	cmd.Flags().BoolVar(&all, "all", false, "Search all configured accounts")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum messages per account")
	return cmd
}

func newReadCmd(flags *rootFlags) *cobra.Command {
	var accountRef string
	var id string
	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read a message by provider-prefixed ID",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			msg, err := readMessage(app, accountRef, id)
			if err != nil {
				return err
			}
			return outputValue(flags, msg, func() error {
				printMessage(cmd.OutOrStdout(), msg)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Account name or address")
	cmd.Flags().StringVar(&id, "id", "", "Message ID returned by inbox/search")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newDraftCmd(flags *rootFlags) *cobra.Command {
	var accountRef, subject, body, bodyFile, replyTo string
	var to, cc, bcc []string
	cmd := &cobra.Command{
		Use:   "draft",
		Short: "Create a draft message; never sends mail",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			account, provider, err := app.resolveProvider(accountRef)
			if err != nil {
				return err
			}
			outbound, err := outboundFromFlags(account.Address, to, cc, bcc, subject, body, bodyFile)
			if err != nil {
				return err
			}
			if err := applyReplyContext(app, accountRef, replyTo, &outbound); err != nil {
				return err
			}
			if err := validateOutboundBasics(outbound); err != nil {
				return err
			}
			ctx, cancel := commandContext(flags)
			defer cancel()
			result, err := provider.Draft(ctx, outbound)
			if err != nil {
				return err
			}
			return outputValue(flags, result, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "Draft created: %s\n", result.ID)
				return nil
			})
		},
	}
	addOutboundFlags(cmd, &accountRef, &to, &cc, &bcc, &subject, &body, &bodyFile)
	cmd.Flags().StringVar(&replyTo, "reply-to", "", "Message ID to reply to; keeps supported providers in the same thread")
	return cmd
}

func newSendCmd(flags *rootFlags) *cobra.Command {
	var accountRef, subject, body, bodyFile, replyTo string
	var confirmSend bool
	var allowCompactBody bool
	var to, cc, bcc []string
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Preview an email; sends only with explicit confirmation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			account, provider, err := app.resolveProvider(accountRef)
			if err != nil {
				return err
			}
			outbound, err := outboundFromFlags(account.Address, to, cc, bcc, subject, body, bodyFile)
			if err != nil {
				return err
			}
			if err := applyReplyContext(app, accountRef, replyTo, &outbound); err != nil {
				return err
			}
			if err := validateOutboundBasics(outbound); err != nil {
				return err
			}
			if err := validateReadableBody(outbound.Body, allowCompactBody); err != nil {
				return err
			}
			if !confirmSend {
				preview := map[string]any{
					"sent":                  false,
					"requires_confirmation": true,
					"instruction":           "No email was sent. Show this preview to the user and rerun with --confirm-send only after explicit approval.",
					"message":               outbound,
				}
				outErr := outputValue(flags, preview, func() error {
					printOutboundPreview(cmd.OutOrStdout(), outbound)
					fmt.Fprintln(cmd.OutOrStdout())
					fmt.Fprintln(cmd.OutOrStdout(), "No email sent. Show this preview to the user, ask for explicit approval, then rerun with --confirm-send.")
					return nil
				})
				if outErr != nil {
					return outErr
				}
				return fmt.Errorf("send requires --confirm-send after user approval; no email sent")
			}
			ctx, cancel := commandContext(flags)
			defer cancel()
			result, err := provider.Send(ctx, outbound)
			if err != nil {
				return err
			}
			return outputValue(flags, result, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "Sent: %s\n", result.ID)
				return nil
			})
		},
	}
	addOutboundFlags(cmd, &accountRef, &to, &cc, &bcc, &subject, &body, &bodyFile)
	cmd.Flags().StringVar(&replyTo, "reply-to", "", "Message ID to reply to; keeps supported providers in the same thread")
	cmd.Flags().BoolVar(&confirmSend, "confirm-send", false, "Actually send after the user has approved the preview")
	cmd.Flags().BoolVar(&allowCompactBody, "allow-compact-body", false, "Allow a long single-line body")
	return cmd
}

func newSummarizeCmd(flags *rootFlags) *cobra.Command {
	var accountRef string
	var id string
	var summarizer string
	cmd := &cobra.Command{
		Use:   "summarize",
		Short: "Summarize a normalized message without requiring a cloud LLM",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			msg, err := readMessage(app, accountRef, id)
			if err != nil {
				return err
			}
			ctx, cancel := commandContext(flags)
			defer cancel()
			summary, err := ppmail.Summarize(ctx, *msg, ppmail.SummarizeOptions{SummarizerCommand: summarizer})
			if err != nil {
				return err
			}
			return outputValue(flags, summary, func() error {
				printSummary(cmd.OutOrStdout(), summary)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Account name or address")
	cmd.Flags().StringVar(&id, "id", "", "Message ID returned by inbox/search")
	cmd.Flags().StringVar(&summarizer, "summarizer", "", "Local summarizer command, for example: ollama run llama3.2")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newWriteReplyCmd(flags *rootFlags) *cobra.Command {
	var accountRef, id, body, bodyFile string
	var createDraft bool
	cmd := &cobra.Command{
		Use:   "write-reply",
		Short: "Generate reply text or create a draft; never sends",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			msg, err := readMessage(app, accountRef, id)
			if err != nil {
				return err
			}
			replyBody, err := resolveBody(body, bodyFile)
			if err != nil {
				return err
			}
			if strings.TrimSpace(replyBody) == "" {
				replyBody = defaultReplyBody(msg)
			}
			subject := replySubject(msg.Subject)
			outbound := ppmail.OutboundMessage{
				To:      []string{msg.From},
				Subject: subject,
				Body:    normalizeBodyText(replyBody),
				ReplyTo: msg.ID,
			}
			applyMessageReplyContext(msg, &outbound)
			account, provider, err := app.resolveProvider(firstNonEmpty(accountRef, msg.Account))
			if err != nil {
				return err
			}
			outbound.From = account.Address
			if err := validateOutboundBasics(outbound); err != nil {
				return err
			}
			if !createDraft {
				return outputValue(flags, outbound, func() error {
					printOutboundPreview(cmd.OutOrStdout(), outbound)
					return nil
				})
			}
			ctx, cancel := commandContext(flags)
			defer cancel()
			result, err := provider.Draft(ctx, outbound)
			if err != nil {
				return err
			}
			return outputValue(flags, result, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "Reply draft created: %s\n", result.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Account name or address")
	cmd.Flags().StringVar(&id, "id", "", "Message ID returned by inbox/search")
	cmd.Flags().StringVar(&body, "body", "", "Reply body text")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Read reply body from a file or '-' for stdin")
	cmd.Flags().BoolVar(&createDraft, "create-draft", false, "Create a draft instead of printing reply text")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newArchiveCmd(flags *rootFlags) *cobra.Command {
	var accountRef, id string
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Archive a Gmail message by removing INBOX when gmail.modify scope is available",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			_, provider, err := app.resolveProvider(accountRef)
			if err != nil {
				return err
			}
			archiver, ok := provider.(ppmail.ArchiveProvider)
			if !ok {
				return fmt.Errorf("provider %s does not support archive", provider.ProviderName())
			}
			ctx, cancel := commandContext(flags)
			defer cancel()
			if err := archiver.Archive(ctx, id); err != nil {
				return err
			}
			return outputValue(flags, map[string]any{"archived": true, "id": id}, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "Archived %s\n", id)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Account name or address")
	cmd.Flags().StringVar(&id, "id", "", "Message ID returned by inbox/search")
	_ = cmd.MarkFlagRequired("account")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newLabelCmd(flags *rootFlags) *cobra.Command {
	var accountRef, id string
	var add, remove []string
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Add or remove Gmail labels when gmail.modify scope is available",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := loadApp(flags)
			if err != nil {
				return err
			}
			_, provider, err := app.resolveProvider(accountRef)
			if err != nil {
				return err
			}
			labeler, ok := provider.(ppmail.LabelProvider)
			if !ok {
				return fmt.Errorf("provider %s does not support labels", provider.ProviderName())
			}
			ctx, cancel := commandContext(flags)
			defer cancel()
			result, err := labeler.Label(ctx, id, parseAddRemove(add), parseAddRemove(remove))
			if err != nil {
				return err
			}
			return outputValue(flags, result, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "Updated labels on %s\n", result.ID)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&accountRef, "account", "", "Account name or address")
	cmd.Flags().StringVar(&id, "id", "", "Message ID returned by inbox/search")
	cmd.Flags().StringSliceVar(&add, "add", nil, "Label IDs to add")
	cmd.Flags().StringSliceVar(&remove, "remove", nil, "Label IDs to remove")
	_ = cmd.MarkFlagRequired("account")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func addOutboundFlags(cmd *cobra.Command, accountRef *string, to, cc, bcc *[]string, subject, body, bodyFile *string) {
	cmd.Flags().StringVar(accountRef, "account", "", "Account name or address")
	cmd.Flags().StringSliceVar(to, "to", nil, "Recipient email address; repeat or comma-separate")
	cmd.Flags().StringSliceVar(cc, "cc", nil, "Cc email address; repeat or comma-separate")
	cmd.Flags().StringSliceVar(bcc, "bcc", nil, "Bcc email address; repeat or comma-separate")
	cmd.Flags().StringVar(subject, "subject", "", "Message subject")
	cmd.Flags().StringVar(body, "body", "", "Message body text")
	cmd.Flags().StringVar(bodyFile, "body-file", "", "Read body from a file or '-' for stdin")
	_ = cmd.MarkFlagRequired("account")
}

func runMessageList(cmd *cobra.Command, app *app, accountRef string, all bool, limit int, fn func(provider ppmail.MailProvider) ([]ppmail.Message, error)) (messageAggregate, error) {
	var accountsToUse []accounts.Account
	if all {
		accountsToUse = app.config.List()
	} else {
		if accountRef == "" {
			return messageAggregate{}, fmt.Errorf("pass --account or --all")
		}
		account, err := app.config.Resolve(accountRef)
		if err != nil {
			return messageAggregate{}, err
		}
		accountsToUse = append(accountsToUse, account)
	}
	aggregate := messageAggregate{Messages: []ppmail.Message{}}
	for _, account := range accountsToUse {
		if err := maybeAutoRefreshProtonExport(cmd, app, account); err != nil {
			aggregate.Errors = append(aggregate.Errors, accountError{Account: account.Name, Error: err.Error()})
			continue
		}
		provider, err := app.provider(account)
		if err != nil {
			aggregate.Errors = append(aggregate.Errors, accountError{Account: account.Name, Error: err.Error()})
			continue
		}
		messages, err := fn(provider)
		if err != nil {
			aggregate.Errors = append(aggregate.Errors, accountError{Account: account.Name, Error: err.Error()})
			continue
		}
		aggregate.Messages = append(aggregate.Messages, messages...)
	}
	if len(aggregate.Errors) > 0 {
		return aggregate, fmt.Errorf("one or more accounts failed")
	}
	return aggregate, nil
}

func readMessage(app *app, accountRef, id string) (*ppmail.Message, error) {
	var account accounts.Account
	var err error
	if accountRef != "" {
		account, err = app.config.Resolve(accountRef)
	} else {
		account, err = app.config.ResolveByMessageID(id)
	}
	if err != nil {
		return nil, err
	}
	provider, err := app.provider(account)
	if err != nil {
		return nil, err
	}
	ctx, cancel := commandContext(app.flags)
	defer cancel()
	return provider.Read(ctx, id)
}

func applyReplyContext(app *app, accountRef, replyID string, outbound *ppmail.OutboundMessage) error {
	if strings.TrimSpace(replyID) == "" {
		return nil
	}
	msg, err := readMessage(app, accountRef, replyID)
	if err != nil {
		return err
	}
	if len(outbound.To) == 0 && len(outbound.Cc) == 0 && len(outbound.Bcc) == 0 && strings.TrimSpace(msg.From) != "" {
		outbound.To = []string{msg.From}
	}
	if strings.TrimSpace(outbound.Subject) == "" {
		outbound.Subject = replySubject(msg.Subject)
	}
	outbound.ReplyTo = msg.ID
	applyMessageReplyContext(msg, outbound)
	return nil
}

func applyMessageReplyContext(msg *ppmail.Message, outbound *ppmail.OutboundMessage) {
	if msg == nil || outbound == nil {
		return
	}
	outbound.ReplyThreadID = msg.ThreadID
	outbound.ReplyMessageID = msg.MessageID
	outbound.References = msg.References
}

func validateOutboundBasics(outbound ppmail.OutboundMessage) error {
	if len(outbound.To) == 0 && len(outbound.Cc) == 0 && len(outbound.Bcc) == 0 {
		return fmt.Errorf("at least one recipient is required; for replies, pass --reply-to <message-id> or use write-reply --create-draft")
	}
	if strings.TrimSpace(outbound.Subject) == "" {
		return fmt.Errorf("subject is required; for replies, pass --reply-to <message-id> to reuse the original subject")
	}
	if strings.TrimSpace(outbound.Body) == "" {
		return fmt.Errorf("body is required")
	}
	return nil
}

func validateReadableBody(body string, allowCompact bool) error {
	if allowCompact {
		return nil
	}
	trimmed := strings.TrimSpace(body)
	if len(trimmed) < 90 || strings.Contains(trimmed, "\n") {
		return nil
	}
	lower := strings.ToLower(trimmed)
	looksLikeLetter := strings.HasPrefix(lower, "hallo ") ||
		strings.HasPrefix(lower, "hallo,") ||
		strings.HasPrefix(lower, "hi ") ||
		strings.HasPrefix(lower, "dear ") ||
		strings.Contains(lower, "beste grüße") ||
		strings.Contains(lower, "best regards") ||
		strings.Contains(lower, "viele grüße")
	if looksLikeLetter || len(strings.Fields(trimmed)) > 16 {
		return fmt.Errorf("send body looks like a long single-line email; add paragraph breaks or pass --body-file, then preview again")
	}
	return nil
}

func replySubject(subject string) string {
	subject = strings.TrimSpace(subject)
	if strings.HasPrefix(strings.ToLower(subject), "re:") {
		return subject
	}
	return "Re: " + subject
}

func defaultReplyBody(msg *ppmail.Message) string {
	var b strings.Builder
	b.WriteString("Hi,\n\n")
	b.WriteString("\n\n")
	if !msg.Date.IsZero() {
		fmt.Fprintf(&b, "On %s, %s wrote:\n", msg.Date.Format("2006-01-02 15:04"), msg.From)
	} else {
		fmt.Fprintf(&b, "On an earlier message, %s wrote:\n", msg.From)
	}
	for _, line := range strings.Split(msg.Body, "\n") {
		fmt.Fprintf(&b, "> %s\n", line)
	}
	return b.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
