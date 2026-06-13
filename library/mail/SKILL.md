# Mail Printing Press CLI

Use `mail-pp-cli` to work with Gmail and Proton Bridge mail accounts through one
normalized interface.

## Safety

- Never send mail unless the user has seen the exact preview and explicitly
  approved it. `send` exits without sending unless `--confirm-send` is passed.
  Agents must not pass `--confirm-send` until after user approval.
- Use `write-reply --create-draft --id <message-id>` or `draft --reply-to
  <message-id>` for generated replies by default. These preserve Gmail thread
  context through `thread_id` and RFC `message_id`.
- Use `--body-file` or escaped `\n` line breaks for real email bodies. Long
  single-line `send` bodies are rejected by default.
- Do not scrape Gmail or Proton webmail.
- Do not bypass Proton Bridge's paid IMAP/SMTP gating. For Proton Free, use
  official Proton Mail Export Tool `.eml` archives.
- Do not store Proton Bridge passwords in this repository.

## Common Commands

```bash
cd $HOME/printing-press/library/mail
make build
./bin/mail-pp-cli accounts list
./bin/mail-pp-cli auth login gmail --account user@example.com
./bin/mail-pp-cli auth login gmail --account alternate@example.com
./bin/mail-pp-cli proton bridge status
./bin/mail-pp-cli proton export configure --account proton-free
./bin/mail-pp-cli proton export automate --account proton-free --days 21
./bin/mail-pp-cli proton export status --account proton-free
./bin/mail-pp-cli proton export refresh --account proton-free --days 21
./bin/mail-pp-cli inbox --all --json
./bin/mail-pp-cli search --all "from:conny" --json
./bin/mail-pp-cli read --account gmail-main --id '<message-id>' --json
./bin/mail-pp-cli summarize --account gmail-main --id '<message-id>' --json
./bin/mail-pp-cli draft --account gmail-main --to someone@example.com --subject "Hello" --body "Text"
./bin/mail-pp-cli write-reply --account gmail-main --id '<message-id>' --body-file reply.txt
./bin/mail-pp-cli write-reply --account gmail-main --id '<message-id>' --body-file reply.txt --create-draft
```

Use provider-prefixed IDs returned by `inbox` and `search` directly with
`read`, for example:

```text
gmail:gmail-main:18f...
proton:proton-free:12345
proton:proton-free:eml:<export-message-id>
```

To send after approval:

```bash
./bin/mail-pp-cli send --account gmail-main --reply-to '<message-id>' --body-file reply.txt
./bin/mail-pp-cli send --account gmail-main --reply-to '<message-id>' --body-file reply.txt --confirm-send
```

The first command only previews and returns `requires_confirmation`. The second
command is allowed only after the user approves the preview.

## Summarization

Local fallback mode requires no LLM and extracts sender, recipients, subject,
date, body excerpt, action items, dates, links, and attachments.

For current Proton Free mail, `proton-free` is set to auto-refresh on `inbox`
and `search`. Manual refresh is still available:

```bash
./bin/mail-pp-cli proton export refresh --account proton-free --days 21
```

The password is retrieved from the local Windows DPAPI encrypted store through
`password_command`; do not place it in command arguments. The refresh replaces
older local exports and keeps only messages dated within the requested local
window. Use fresh IDs from `inbox` or `search` before `read` or `summarize`.

Optional local summarizer command:

```bash
./bin/mail-pp-cli summarize --account gmail-main --id '<id>' --summarizer 'ollama run llama3.2'
```
