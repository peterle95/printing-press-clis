package imap

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	emimap "github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
	msgmail "github.com/emersion/go-message/mail"

	"mail-pp-cli/internal/accounts"
	ppmail "mail-pp-cli/internal/mail"
	"mail-pp-cli/internal/providers/smtp"
)

type Provider struct {
	account accounts.Account
	timeout time.Duration
}

func NewProvider(account accounts.Account, timeout time.Duration) *Provider {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Provider{account: account, timeout: timeout}
}

func (p *Provider) AccountName() string {
	return p.account.Name
}

func (p *Provider) ProviderName() string {
	return "proton"
}

func (p *Provider) connect(ctx context.Context) (*imapclient.Client, error) {
	password, err := p.account.BridgePassword(ctx)
	if err != nil {
		return nil, err
	}
	c, err := imapclient.Dial(p.account.IMAPHost)
	if err != nil {
		return nil, fmt.Errorf("connecting to Proton Bridge IMAP at %s: %w", p.account.IMAPHost, err)
	}
	if p.account.IMAPStartTLSEnabled() {
		host, _, _ := net.SplitHostPort(p.account.IMAPHost)
		if err := c.StartTLS(&tls.Config{ServerName: host, InsecureSkipVerify: true}); err != nil {
			_ = c.Logout()
			return nil, fmt.Errorf("starting IMAP TLS with Proton Bridge: %w", err)
		}
	}
	if err := c.Login(p.account.Username, password); err != nil {
		_ = c.Logout()
		return nil, fmt.Errorf("logging into Proton Bridge IMAP as %s: %w", p.account.Username, err)
	}
	return c, nil
}

func (p *Provider) Inbox(ctx context.Context, limit int) ([]ppmail.Message, error) {
	return p.search(ctx, "", limit)
}

func (p *Provider) Search(ctx context.Context, query string, limit int) ([]ppmail.Message, error) {
	return p.search(ctx, query, limit)
}

func (p *Provider) search(ctx context.Context, query string, limit int) ([]ppmail.Message, error) {
	if limit <= 0 {
		limit = 10
	}
	c, err := p.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Logout()
	if _, err := c.Select("INBOX", true); err != nil {
		return nil, fmt.Errorf("selecting INBOX: %w", err)
	}
	criteria := searchCriteria(query)
	uids, err := c.UidSearch(criteria)
	if err != nil {
		return nil, fmt.Errorf("searching Proton Bridge IMAP: %w", err)
	}
	if len(uids) == 0 {
		return nil, nil
	}
	sort.Slice(uids, func(i, j int) bool { return uids[i] < uids[j] })
	if len(uids) > limit {
		uids = uids[len(uids)-limit:]
	}
	return p.fetchMetadata(c, uids)
}

func (p *Provider) fetchMetadata(c *imapclient.Client, uids []uint32) ([]ppmail.Message, error) {
	seqset := new(emimap.SeqSet)
	seqset.AddNum(uids...)
	messages := make(chan *emimap.Message, len(uids))
	done := make(chan error, 1)
	go func() {
		done <- c.UidFetch(seqset, []emimap.FetchItem{emimap.FetchEnvelope, emimap.FetchUid, emimap.FetchFlags}, messages)
	}()
	var out []ppmail.Message
	for msg := range messages {
		out = append(out, p.messageFromEnvelope(msg))
	}
	if err := <-done; err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.After(out[j].Date) })
	return out, nil
}

func (p *Provider) Read(ctx context.Context, id string) (*ppmail.Message, error) {
	rawID, err := ppmail.RawMessageID("proton", p.account.Name, id)
	if err != nil {
		return nil, err
	}
	uid64, err := strconv.ParseUint(rawID, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("Proton message id must be an IMAP UID: %w", err)
	}
	c, err := p.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Logout()
	if _, err := c.Select("INBOX", true); err != nil {
		return nil, fmt.Errorf("selecting INBOX: %w", err)
	}
	seqset := new(emimap.SeqSet)
	seqset.AddNum(uint32(uid64))
	section := &emimap.BodySectionName{Peek: true}
	messages := make(chan *emimap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- c.UidFetch(seqset, []emimap.FetchItem{emimap.FetchEnvelope, emimap.FetchUid, emimap.FetchFlags, section.FetchItem()}, messages)
	}()
	var fetched *emimap.Message
	for msg := range messages {
		fetched = msg
	}
	if err := <-done; err != nil {
		return nil, err
	}
	if fetched == nil {
		return nil, fmt.Errorf("message %s not found in Proton INBOX", id)
	}
	r := fetched.GetBody(section)
	if r == nil {
		msg := p.messageFromEnvelope(fetched)
		return &msg, nil
	}
	msg, err := p.parseRawMessage(fetched, r)
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
	c, err := p.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Logout()
	reader := strings.NewReader(raw)
	if err := c.Append("Drafts", []string{emimap.DraftFlag}, time.Now(), reader); err != nil {
		reader.Seek(0, io.SeekStart)
		if fallbackErr := c.Append("Draft", []string{emimap.DraftFlag}, time.Now(), reader); fallbackErr != nil {
			return nil, fmt.Errorf("appending Proton draft: %w", err)
		}
	}
	id := fmt.Sprintf("proton:%s:draft:%d", p.account.Name, time.Now().Unix())
	return &ppmail.DraftResult{ID: id, Account: p.account.Name, Provider: "proton"}, nil
}

func (p *Provider) Send(ctx context.Context, msg ppmail.OutboundMessage) (*ppmail.SendResult, error) {
	return smtp.SendBridge(ctx, p.account, msg)
}

func (p *Provider) messageFromEnvelope(msg *emimap.Message) ppmail.Message {
	var date time.Time
	subject := ""
	var from string
	var to, cc, bcc []string
	if msg != nil && msg.Envelope != nil {
		date = msg.Envelope.Date
		subject = msg.Envelope.Subject
		from = firstAddress(msg.Envelope.From)
		to = addresses(msg.Envelope.To)
		cc = addresses(msg.Envelope.Cc)
		bcc = addresses(msg.Envelope.Bcc)
	}
	uid := uint32(0)
	if msg != nil {
		uid = msg.Uid
	}
	return ppmail.Message{
		ID:       ppmail.ProviderMessageID("proton", p.account.Name, strconv.FormatUint(uint64(uid), 10)),
		Provider: "proton",
		Account:  p.account.Name,
		From:     from,
		To:       to,
		Cc:       cc,
		Bcc:      bcc,
		Subject:  subject,
		Date:     date,
		Labels:   msgFlags(msg),
	}
}

func (p *Provider) parseRawMessage(msg *emimap.Message, r io.Reader) (ppmail.Message, error) {
	out := p.messageFromEnvelope(msg)
	mr, err := msgmail.CreateReader(r)
	if err != nil {
		return out, fmt.Errorf("parsing MIME message: %w", err)
	}
	if subject, err := mr.Header.Subject(); err == nil && subject != "" {
		out.Subject = subject
	}
	out.MessageID = mr.Header.Get("Message-ID")
	out.References = parseReferences(mr.Header.Get("References"))
	if date, err := mr.Header.Date(); err == nil {
		out.Date = date
	}
	if from, err := mr.Header.AddressList("From"); err == nil && len(from) > 0 {
		out.From = from[0].String()
	}
	if to, err := mr.Header.AddressList("To"); err == nil {
		out.To = addrStrings(to)
	}
	if cc, err := mr.Header.AddressList("Cc"); err == nil {
		out.Cc = addrStrings(cc)
	}
	var bodyParts []string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, err
		}
		switch header := part.Header.(type) {
		case *msgmail.InlineHeader:
			contentType, _, _ := header.ContentType()
			data, _ := io.ReadAll(part.Body)
			if strings.EqualFold(contentType, "text/plain") {
				bodyParts = append(bodyParts, string(data))
			}
		case *msgmail.AttachmentHeader:
			filename, _ := header.Filename()
			contentType, _, _ := header.ContentType()
			var buf bytes.Buffer
			n, _ := io.Copy(&buf, part.Body)
			out.Attachments = append(out.Attachments, ppmail.Attachment{
				Filename: filename,
				MimeType: contentType,
				Size:     n,
			})
		}
	}
	out.Body = strings.TrimSpace(strings.Join(bodyParts, "\n\n"))
	out.Snippet = snippet(out.Body)
	return out, nil
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

func searchCriteria(query string) *emimap.SearchCriteria {
	criteria := emimap.NewSearchCriteria()
	query = strings.TrimSpace(query)
	if query == "" {
		return criteria
	}
	lower := strings.ToLower(query)
	switch {
	case strings.HasPrefix(lower, "from:"):
		criteria.Header.Add("From", strings.TrimSpace(query[5:]))
	case strings.HasPrefix(lower, "to:"):
		criteria.Header.Add("To", strings.TrimSpace(query[3:]))
	case strings.HasPrefix(lower, "subject:"):
		criteria.Header.Add("Subject", strings.Trim(strings.TrimSpace(query[8:]), `"`))
	default:
		criteria.Text = []string{query}
	}
	return criteria
}

func firstAddress(values []*emimap.Address) string {
	if len(values) == 0 {
		return ""
	}
	return addressString(values[0])
}

func addresses(values []*emimap.Address) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if s := addressString(value); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func addressString(value *emimap.Address) string {
	if value == nil {
		return ""
	}
	addr := value.MailboxName
	if value.HostName != "" {
		addr += "@" + value.HostName
	}
	if value.PersonalName != "" {
		return fmt.Sprintf("%s <%s>", value.PersonalName, addr)
	}
	return addr
}

func addrStrings(values []*msgmail.Address) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != nil {
			out = append(out, value.String())
		}
	}
	return out
}

func msgFlags(msg *emimap.Message) []string {
	if msg == nil {
		return nil
	}
	return append([]string(nil), msg.Flags...)
}

func snippet(body string) string {
	body = strings.Join(strings.Fields(body), " ")
	if len(body) > 220 {
		return body[:220] + "..."
	}
	return body
}
