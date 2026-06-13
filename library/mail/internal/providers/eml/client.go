package eml

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	msgmail "github.com/emersion/go-message/mail"

	"mail-pp-cli/internal/accounts"
	ppmail "mail-pp-cli/internal/mail"
	"mail-pp-cli/internal/providers/smtp"
)

type Provider struct {
	account accounts.Account
}

func NewProvider(account accounts.Account) *Provider {
	return &Provider{account: account}
}

func (p *Provider) AccountName() string {
	return p.account.Name
}

func (p *Provider) ProviderName() string {
	return "proton"
}

func (p *Provider) Inbox(ctx context.Context, limit int) ([]ppmail.Message, error) {
	return p.Search(ctx, "", limit)
}

func (p *Provider) Search(ctx context.Context, query string, limit int) ([]ppmail.Message, error) {
	root, err := p.archiveRoot()
	if err != nil {
		return nil, err
	}
	files, err := emlFiles(root)
	if err != nil {
		return nil, err
	}
	var out []ppmail.Message
	for _, file := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		msg, err := p.parseFile(root, file, false)
		if err != nil {
			continue
		}
		if matchMessage(msg, query) {
			out = append(out, msg)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.After(out[j].Date) })
	if limit <= 0 {
		limit = 10
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (p *Provider) Read(ctx context.Context, id string) (*ppmail.Message, error) {
	root, err := p.archiveRoot()
	if err != nil {
		return nil, err
	}
	rawID, err := ppmail.RawMessageID("proton", p.account.Name, id)
	if err != nil {
		return nil, err
	}
	hash := strings.TrimPrefix(rawID, "eml:")
	file, err := findFileByHash(root, hash)
	if err != nil {
		return nil, err
	}
	msg, err := p.parseFile(root, file, true)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func (p *Provider) Draft(ctx context.Context, draft ppmail.OutboundMessage) (*ppmail.DraftResult, error) {
	raw, err := smtp.BuildMessage(p.account, draft)
	if err != nil {
		return nil, err
	}
	dir, err := p.account.ExpandedDraftDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	stamp := time.Now().Format("20060102-150405")
	name := stamp + "-" + safeFilename(draft.Subject) + ".eml"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		return nil, err
	}
	return &ppmail.DraftResult{
		ID:        "proton:" + p.account.Name + ":local-draft:" + filepath.Base(path),
		Account:   p.account.Name,
		Provider:  "proton",
		MessageID: path,
	}, nil
}

func (p *Provider) Send(ctx context.Context, msg ppmail.OutboundMessage) (*ppmail.SendResult, error) {
	return nil, fmt.Errorf("Proton Free cannot send through CLI because Proton SMTP/Bridge requires a paid plan; use `mail-pp-cli draft --account %s ...` to create a local .eml draft, then send manually in Proton Mail", p.account.Name)
}

func (p *Provider) archiveRoot() (string, error) {
	root, err := p.account.ExpandedArchiveDir()
	if err != nil {
		return "", err
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("Proton export archive not found at %s; run the official Proton Mail Export Tool and export EML files there", root)
		}
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("Proton export archive path is not a directory: %s", root)
	}
	return root, nil
}

func emlFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".eml") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func (p *Provider) parseFile(root, path string, includeBody bool) (ppmail.Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return ppmail.Message{}, err
	}
	defer f.Close()
	mr, err := msgmail.CreateReader(f)
	if err != nil {
		return ppmail.Message{}, err
	}
	rel, _ := filepath.Rel(root, path)
	rel = filepath.ToSlash(rel)
	msg := ppmail.Message{
		ID:       ppmail.ProviderMessageID("proton", p.account.Name, "eml:"+fileHash(rel)),
		Provider: "proton",
		Account:  p.account.Name,
		Labels:   []string{"LOCAL_EXPORT"},
	}
	msg.Subject, _ = mr.Header.Subject()
	msg.MessageID = mr.Header.Get("Message-ID")
	msg.References = parseReferences(mr.Header.Get("References"))
	if date, err := mr.Header.Date(); err == nil {
		msg.Date = date
	} else if info, statErr := os.Stat(path); statErr == nil {
		msg.Date = info.ModTime()
	}
	if from, err := mr.Header.AddressList("From"); err == nil && len(from) > 0 {
		msg.From = from[0].String()
	}
	if to, err := mr.Header.AddressList("To"); err == nil {
		msg.To = addressStrings(to)
	}
	if cc, err := mr.Header.AddressList("Cc"); err == nil {
		msg.Cc = addressStrings(cc)
	}
	if bcc, err := mr.Header.AddressList("Bcc"); err == nil {
		msg.Bcc = addressStrings(bcc)
	}
	body, attachments, _ := readParts(mr, includeBody)
	if includeBody {
		msg.Body = strings.TrimSpace(body)
		msg.Snippet = snippet(msg.Body)
		msg.Attachments = attachments
	} else {
		msg.Snippet = snippet(body)
	}
	return msg, nil
}

func parseReferences(value string) []string {
	var out []string
	for _, part := range strings.Fields(value) {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func readParts(mr *msgmail.Reader, includeBody bool) (string, []ppmail.Attachment, error) {
	var bodyParts []string
	var attachments []ppmail.Attachment
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return strings.Join(bodyParts, "\n\n"), attachments, err
		}
		switch header := part.Header.(type) {
		case *msgmail.InlineHeader:
			contentType, _, _ := header.ContentType()
			data, _ := io.ReadAll(part.Body)
			text := string(data)
			if strings.EqualFold(contentType, "text/html") {
				text = htmlToText(text)
			}
			if includeBody || len(bodyParts) == 0 {
				bodyParts = append(bodyParts, text)
			}
		case *msgmail.AttachmentHeader:
			filename, _ := header.Filename()
			contentType, _, _ := header.ContentType()
			n, _ := io.Copy(io.Discard, part.Body)
			attachments = append(attachments, ppmail.Attachment{
				Filename: filename,
				MimeType: contentType,
				Size:     n,
			})
		}
	}
	return strings.Join(bodyParts, "\n\n"), attachments, nil
}

func findFileByHash(root, hash string) (string, error) {
	files, err := emlFiles(root)
	if err != nil {
		return "", err
	}
	for _, file := range files {
		rel, _ := filepath.Rel(root, file)
		if fileHash(filepath.ToSlash(rel)) == hash {
			return file, nil
		}
	}
	return "", fmt.Errorf("message %s not found in Proton export archive", hash)
}

func fileHash(rel string) string {
	sum := sha1.Sum([]byte(rel))
	return hex.EncodeToString(sum[:10])
}

func matchMessage(msg ppmail.Message, query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return true
	}
	lower := strings.ToLower(query)
	switch {
	case strings.HasPrefix(lower, "from:"):
		return strings.Contains(strings.ToLower(msg.From), strings.TrimSpace(lower[5:]))
	case strings.HasPrefix(lower, "to:"):
		return strings.Contains(strings.ToLower(strings.Join(msg.To, " ")), strings.TrimSpace(lower[3:]))
	case strings.HasPrefix(lower, "subject:"):
		needle := strings.Trim(strings.TrimSpace(query[8:]), `"`)
		return strings.Contains(strings.ToLower(msg.Subject), strings.ToLower(needle))
	default:
		haystack := strings.ToLower(strings.Join([]string{msg.From, strings.Join(msg.To, " "), msg.Subject, msg.Snippet, msg.Body}, " "))
		return strings.Contains(haystack, strings.ToLower(query))
	}
}

func addressStrings(values []*mail.Address) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != nil {
			out = append(out, value.String())
		}
	}
	return out
}

func htmlToText(value string) string {
	value = htmlTagPattern.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	return strings.Join(strings.Fields(value), " ")
}

func snippet(body string) string {
	body = strings.Join(strings.Fields(body), " ")
	if len(body) > 220 {
		return body[:220] + "..."
	}
	return body
}

func safeFilename(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "draft"
	}
	value = filenamePattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if len(value) > 60 {
		value = value[:60]
	}
	return value
}

var (
	htmlTagPattern  = regexp.MustCompile(`(?s)<[^>]*>`)
	filenamePattern = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
)
