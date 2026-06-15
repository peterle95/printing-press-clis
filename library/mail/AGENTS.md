# Mail Printed CLI Agent Guide

This directory is a hand-authored `mail-pp-cli` project for Gmail API, Proton
Bridge, and official Proton Mail Export Tool archives.

## Operating Rules

- Never scrape or automate Gmail or Proton webmail.
- Never bypass Proton Bridge's paid IMAP/SMTP gating.
- Keep OAuth tokens, passwords, account files, exports, and drafts outside Git.
- Never send mail until the user has reviewed the exact preview and explicitly
  approved it. `--confirm-send` is reserved for that second step.
- Generated summaries and replies must never auto-send.
- Use reply-aware draft commands so providers preserve message threads.
- Put account aliases and mailbox-specific defaults in the external accounts
  file or `AGENTS.override.md`.
- When the user asks to check emails, only show messages with the `UNREAD`
  label in the output.

Private runtime data defaults to:

```text
~/.config/printing-press/google-private
~/.local/share/printing-press/mail-private
```

## Build And Test

```bash
go test ./...
go vet ./...
go build ./...
```
