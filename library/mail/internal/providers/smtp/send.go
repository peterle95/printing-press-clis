package smtp

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	netsmtp "net/smtp"
	"strings"
	"time"

	"mail-pp-cli/internal/accounts"
	ppmail "mail-pp-cli/internal/mail"
)

func SendBridge(ctx context.Context, account accounts.Account, msg ppmail.OutboundMessage) (*ppmail.SendResult, error) {
	raw, err := BuildMessage(account, msg)
	if err != nil {
		return nil, err
	}
	password, err := account.BridgePassword(ctx)
	if err != nil {
		return nil, err
	}
	host, _, err := net.SplitHostPort(account.SMTPHost)
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP host %q: %w", account.SMTPHost, err)
	}
	dialer := net.Dialer{Timeout: 30 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", account.SMTPHost)
	if err != nil {
		return nil, fmt.Errorf("connecting to Proton Bridge SMTP at %s: %w", account.SMTPHost, err)
	}
	client, err := netsmtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	defer client.Close()
	if account.SMTPStartTLSEnabled() {
		if err := client.StartTLS(&tls.Config{ServerName: host, InsecureSkipVerify: true}); err != nil {
			return nil, fmt.Errorf("starting SMTP TLS with Proton Bridge: %w", err)
		}
	}
	if err := client.Auth(netsmtp.PlainAuth("", account.Username, password, host)); err != nil {
		return nil, fmt.Errorf("authenticating to Proton Bridge SMTP as %s: %w", account.Username, err)
	}
	from := msg.From
	if strings.TrimSpace(from) == "" {
		from = account.Address
	}
	if parsed, err := mail.ParseAddress(from); err == nil {
		from = parsed.Address
	}
	if err := client.Mail(from); err != nil {
		return nil, err
	}
	for _, rcpt := range allRecipients(msg) {
		if parsed, err := mail.ParseAddress(rcpt); err == nil {
			rcpt = parsed.Address
		}
		if err := client.Rcpt(rcpt); err != nil {
			return nil, err
		}
	}
	w, err := client.Data()
	if err != nil {
		return nil, err
	}
	if _, err := w.Write([]byte(raw)); err != nil {
		_ = w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if err := client.Quit(); err != nil {
		return nil, err
	}
	return &ppmail.SendResult{
		ID:       fmt.Sprintf("proton:%s:sent:%d", account.Name, time.Now().Unix()),
		Account:  account.Name,
		Provider: "proton",
	}, nil
}

func BuildMessage(account accounts.Account, msg ppmail.OutboundMessage) (string, error) {
	if len(allRecipients(msg)) == 0 {
		return "", fmt.Errorf("at least one recipient is required")
	}
	from := strings.TrimSpace(msg.From)
	if from == "" {
		from = account.Address
	}
	var b bytes.Buffer
	writeHeader(&b, "From", normalizeAddressHeader(from))
	writeHeader(&b, "To", strings.Join(msg.To, ", "))
	writeHeader(&b, "Cc", strings.Join(msg.Cc, ", "))
	writeHeader(&b, "Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	writeHeader(&b, "Date", time.Now().Format(time.RFC1123Z))
	if replyMessageID := replyHeaderMessageID(msg); replyMessageID != "" {
		writeHeader(&b, "In-Reply-To", replyMessageID)
		writeHeader(&b, "References", strings.Join(replyReferences(msg, replyMessageID), " "))
	}
	writeHeader(&b, "MIME-Version", "1.0")
	writeHeader(&b, "Content-Type", `text/plain; charset="UTF-8"`)
	writeHeader(&b, "Content-Transfer-Encoding", "8bit")
	b.WriteString("\r\n")
	body := strings.ReplaceAll(msg.Body, "\n", "\r\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\r\n") {
		b.WriteString("\r\n")
	}
	return b.String(), nil
}

func replyHeaderMessageID(msg ppmail.OutboundMessage) string {
	if strings.TrimSpace(msg.ReplyMessageID) != "" {
		return strings.TrimSpace(msg.ReplyMessageID)
	}
	if looksLikeRFCMessageID(msg.ReplyTo) {
		return strings.TrimSpace(msg.ReplyTo)
	}
	return ""
}

func replyReferences(msg ppmail.OutboundMessage, replyMessageID string) []string {
	seen := map[string]bool{}
	var out []string
	for _, ref := range msg.References {
		ref = strings.TrimSpace(ref)
		if ref != "" && !seen[ref] {
			seen[ref] = true
			out = append(out, ref)
		}
	}
	if replyMessageID != "" && !seen[replyMessageID] {
		out = append(out, replyMessageID)
	}
	return out
}

func looksLikeRFCMessageID(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">") && strings.Contains(value, "@")
}

func allRecipients(msg ppmail.OutboundMessage) []string {
	out := append([]string{}, msg.To...)
	out = append(out, msg.Cc...)
	out = append(out, msg.Bcc...)
	return out
}

func writeHeader(b *bytes.Buffer, name, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	fmt.Fprintf(b, "%s: %s\r\n", name, value)
}

func normalizeAddressHeader(value string) string {
	if parsed, err := mail.ParseAddress(value); err == nil {
		return parsed.String()
	}
	return fmt.Sprintf("<%s>", value)
}
