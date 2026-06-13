package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	ppmail "mail-pp-cli/internal/mail"
)

func printMessages(w io.Writer, messages []ppmail.Message) {
	for i, msg := range messages {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "ID: %s\n", msg.ID)
		fmt.Fprintf(w, "Account: %s (%s)\n", msg.Account, msg.Provider)
		if !msg.Date.IsZero() {
			fmt.Fprintf(w, "Date: %s\n", msg.Date.Format("2006-01-02 15:04 MST"))
		}
		fmt.Fprintf(w, "From: %s\n", msg.From)
		if len(msg.To) > 0 {
			fmt.Fprintf(w, "To: %s\n", strings.Join(msg.To, ", "))
		}
		fmt.Fprintf(w, "Subject: %s\n", msg.Subject)
		if msg.Snippet != "" {
			fmt.Fprintf(w, "Snippet: %s\n", msg.Snippet)
		}
		if len(msg.Attachments) > 0 {
			fmt.Fprintf(w, "Attachments: %d\n", len(msg.Attachments))
		}
	}
}

func printMessage(w io.Writer, msg *ppmail.Message) {
	fmt.Fprintf(w, "ID: %s\n", msg.ID)
	fmt.Fprintf(w, "Account: %s (%s)\n", msg.Account, msg.Provider)
	if msg.ThreadID != "" {
		fmt.Fprintf(w, "Thread: %s\n", msg.ThreadID)
	}
	if msg.MessageID != "" {
		fmt.Fprintf(w, "Message-ID: %s\n", msg.MessageID)
	}
	if !msg.Date.IsZero() {
		fmt.Fprintf(w, "Date: %s\n", msg.Date.Format("2006-01-02 15:04 MST"))
	}
	fmt.Fprintf(w, "From: %s\n", msg.From)
	if len(msg.To) > 0 {
		fmt.Fprintf(w, "To: %s\n", strings.Join(msg.To, ", "))
	}
	if len(msg.Cc) > 0 {
		fmt.Fprintf(w, "Cc: %s\n", strings.Join(msg.Cc, ", "))
	}
	fmt.Fprintf(w, "Subject: %s\n", msg.Subject)
	if len(msg.Labels) > 0 {
		fmt.Fprintf(w, "Labels: %s\n", strings.Join(msg.Labels, ", "))
	}
	if len(msg.Attachments) > 0 {
		fmt.Fprintln(w, "Attachments:")
		for _, attachment := range msg.Attachments {
			fmt.Fprintf(w, "  - %s (%s, %d bytes)\n", attachment.Filename, attachment.MimeType, attachment.Size)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, msg.Body)
}

func printOutboundPreview(w io.Writer, msg ppmail.OutboundMessage) {
	fmt.Fprintln(w, "Email preview")
	if msg.ReplyTo != "" {
		fmt.Fprintf(w, "Reply-To Message: %s\n", msg.ReplyTo)
	}
	if msg.ReplyThreadID != "" {
		fmt.Fprintf(w, "Thread: %s\n", msg.ReplyThreadID)
	}
	fmt.Fprintf(w, "From: %s\n", msg.From)
	if len(msg.To) > 0 {
		fmt.Fprintf(w, "To: %s\n", strings.Join(msg.To, ", "))
	}
	if len(msg.Cc) > 0 {
		fmt.Fprintf(w, "Cc: %s\n", strings.Join(msg.Cc, ", "))
	}
	if len(msg.Bcc) > 0 {
		fmt.Fprintf(w, "Bcc: %s\n", strings.Join(msg.Bcc, ", "))
	}
	fmt.Fprintf(w, "Subject: %s\n", msg.Subject)
	fmt.Fprintln(w)
	fmt.Fprint(w, msg.Body)
	if !strings.HasSuffix(msg.Body, "\n") {
		fmt.Fprintln(w)
	}
}

func printSummary(w io.Writer, summary ppmail.Summary) {
	fmt.Fprintf(w, "Message: %s\n", summary.MessageID)
	fmt.Fprintf(w, "From: %s\n", summary.From)
	fmt.Fprintf(w, "Subject: %s\n", summary.Subject)
	if !summary.Date.IsZero() {
		fmt.Fprintf(w, "Date: %s\n", summary.Date.Format("2006-01-02 15:04 MST"))
	}
	if summary.LLMOutput != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, summary.LLMOutput)
		return
	}
	if summary.Excerpt != "" {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Excerpt: %s\n", summary.Excerpt)
	}
	printStringList(w, "Action items", summary.ActionItems)
	printStringList(w, "Dates", summary.Dates)
	printStringList(w, "Links", summary.Links)
	if len(summary.Attachments) > 0 {
		fmt.Fprintf(w, "Attachments: %d\n", len(summary.Attachments))
	}
}

func printStringList(w io.Writer, title string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(w, "%s:\n", title)
	for _, value := range values {
		fmt.Fprintf(w, "  - %s\n", value)
	}
}

func warnErrors(errors []accountError) {
	for _, err := range errors {
		fmt.Fprintf(os.Stderr, "%s: %s\n", err.Account, err.Error)
	}
}

type accountError struct {
	Account string `json:"account"`
	Error   string `json:"error"`
}
