package mail

import "time"

type Message struct {
	ID          string       `json:"id"`
	Provider    string       `json:"provider"`
	Account     string       `json:"account"`
	ThreadID    string       `json:"thread_id,omitempty"`
	MessageID   string       `json:"message_id,omitempty"`
	References  []string     `json:"references,omitempty"`
	From        string       `json:"from"`
	To          []string     `json:"to,omitempty"`
	Cc          []string     `json:"cc,omitempty"`
	Bcc         []string     `json:"bcc,omitempty"`
	Subject     string       `json:"subject"`
	Date        time.Time    `json:"date,omitempty"`
	Snippet     string       `json:"snippet,omitempty"`
	Body        string       `json:"body,omitempty"`
	Labels      []string     `json:"labels,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

type OutboundMessage struct {
	From           string   `json:"from,omitempty"`
	To             []string `json:"to,omitempty"`
	Cc             []string `json:"cc,omitempty"`
	Bcc            []string `json:"bcc,omitempty"`
	Subject        string   `json:"subject"`
	Body           string   `json:"body"`
	ReplyTo        string   `json:"reply_to,omitempty"`
	ReplyThreadID  string   `json:"reply_thread_id,omitempty"`
	ReplyMessageID string   `json:"reply_message_id,omitempty"`
	References     []string `json:"references,omitempty"`
}

type DraftResult struct {
	ID        string `json:"id"`
	Account   string `json:"account"`
	Provider  string `json:"provider"`
	MessageID string `json:"message_id,omitempty"`
	ThreadID  string `json:"thread_id,omitempty"`
}

type SendResult struct {
	ID       string `json:"id"`
	Account  string `json:"account"`
	Provider string `json:"provider"`
	ThreadID string `json:"thread_id,omitempty"`
}

type LabelResult struct {
	ID      string   `json:"id"`
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

type Summary struct {
	MessageID    string       `json:"message_id"`
	Account      string       `json:"account"`
	Provider     string       `json:"provider"`
	From         string       `json:"from"`
	To           []string     `json:"to,omitempty"`
	Cc           []string     `json:"cc,omitempty"`
	Subject      string       `json:"subject"`
	Date         time.Time    `json:"date,omitempty"`
	Excerpt      string       `json:"excerpt,omitempty"`
	ActionItems  []string     `json:"action_items,omitempty"`
	Dates        []string     `json:"dates,omitempty"`
	Links        []string     `json:"links,omitempty"`
	Attachments  []Attachment `json:"attachments,omitempty"`
	LLMOutput    string       `json:"llm_output,omitempty"`
	Summarizer   string       `json:"summarizer,omitempty"`
	FallbackUsed bool         `json:"fallback_used"`
}
