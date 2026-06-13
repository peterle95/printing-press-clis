package cli

import (
	"fmt"
	"io"
	"net/mail"
	"os"
	"strings"

	ppmail "mail-pp-cli/internal/mail"
)

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func outboundFromFlags(accountAddress string, to, cc, bcc []string, subject, body, bodyFile string) (ppmail.OutboundMessage, error) {
	resolvedBody, err := resolveBody(body, bodyFile)
	if err != nil {
		return ppmail.OutboundMessage{}, err
	}
	resolvedBody = normalizeBodyText(resolvedBody)
	recipients := append([]string{}, to...)
	recipients = append(recipients, cc...)
	recipients = append(recipients, bcc...)
	for _, recipient := range recipients {
		if _, err := mail.ParseAddress(recipient); err != nil {
			return ppmail.OutboundMessage{}, fmt.Errorf("invalid recipient %q: %w", recipient, err)
		}
	}
	return ppmail.OutboundMessage{
		From:    accountAddress,
		To:      to,
		Cc:      cc,
		Bcc:     bcc,
		Subject: subject,
		Body:    resolvedBody,
	}, nil
}

func resolveBody(body, bodyFile string) (string, error) {
	if bodyFile == "" {
		return body, nil
	}
	var data []byte
	var err error
	if bodyFile == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(bodyFile)
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(body) != "" {
		return body + "\n\n" + string(data), nil
	}
	return string(data), nil
}

func normalizeBodyText(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	body = strings.ReplaceAll(body, `\n`, "\n")
	body = strings.TrimRight(body, " \t\r\n")
	if body != "" {
		body += "\n"
	}
	return body
}

func splitScopes(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseAddRemove(values []string) []string {
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}
