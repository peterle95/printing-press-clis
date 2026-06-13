# mail-pp-cli

Unified Printing Press mail CLI for multiple mail providers behind one common
message abstraction.

## Providers

- `gmail-api`: official Gmail API with one OAuth token per account.
- `proton-bridge-imap-smtp`: Proton Mail Bridge through local IMAP/SMTP.
- `proton-export-eml`: Proton Free workaround that reads local `.eml` archives
  produced by the official Proton Mail Export Tool.

This CLI never scrapes Gmail or Proton webmail and does not bypass paid Proton
Bridge/SMTP access. `send` is the only command that sends mail, and it previews
the exact message first. Nothing is sent unless `--confirm-send` is supplied
after user approval. `draft`, `summarize`, and `write-reply` never send. Proton
Free archive mode cannot send through Proton; it writes local `.eml` draft files
for manual sending.

## Build

```bash
make build
```

The binary is written to:

```text
bin/mail-pp-cli
```

## Account Config

Default path:

```text
~/.local/share/printing-press/mail-private/accounts.yaml
```

Create the default config:

```bash
./bin/mail-pp-cli accounts init
```

Default accounts:

```yaml
accounts:
  gmail-main:
    address: user@example.com
    provider: gmail-api
    token: ~/.config/printing-press/google-private/tokens/gmail-moelzerpeter-token.json
  gmail-alt:
    address: alternate@example.com
    provider: gmail-api
    token: ~/.config/printing-press/google-private/tokens/gmail-molzerpeter-token.json
  proton-free:
    address: user@proton.me
    provider: proton-export-eml
    username: moelzerpeter
    password_command: $HOME/go/bin/mail-pp-cli proton export password get --account 'proton-free'
    archive_dir: ~/.local/share/printing-press/mail-private/proton-export/user
    draft_dir: ~/.local/share/printing-press/mail-private/proton-drafts
    export_tool: ~/.local/share/printing-press/mail-private/tools/proton-mail-export/proton-mail-export-cli
    auto_refresh_days: 21
```

For a paid Proton plan, Bridge can still be configured. Set a Proton Bridge
password command instead of storing a plaintext password:

```bash
./bin/mail-pp-cli proton bridge configure \
  --account proton-free \
  --password-command 'secret-tool lookup service proton-bridge account moelzerpeter'
```

For Proton Free, configure archive mode:

```bash
./bin/mail-pp-cli proton export configure --account proton-free
./bin/mail-pp-cli proton export automate --account proton-free --days 21
./bin/mail-pp-cli proton export status --account proton-free
```

`automate` prompts once for the Proton password, stores it outside this repo in
Windows' local DPAPI encrypted store for the current user, and updates
`accounts.yaml` with a `password_command`. It does not write the password into
the repository or the account config.

Refresh the archive before reading current Proton mail:

```bash
./bin/mail-pp-cli proton export refresh --account proton-free --days 21
```

The command prompts for the Proton password and does not store it. Proton's
official export tool has no date/since flag, so `mail-pp-cli` exports into a
temporary directory, filters the `.eml` files to the last 21 days, replaces the
older local archive, and removes the temporary files.

With `auto_refresh_days: 21`, `inbox` and `search` auto-refresh before listing
Proton messages:

```bash
./bin/mail-pp-cli inbox --account proton-free --json
./bin/mail-pp-cli search --account proton-free "from:someone" --json
```

Use `--no-auto-refresh` when you want to inspect the current local archive
without contacting Proton. `read` and `summarize` do not auto-refresh because a
refresh can replace the archive and change local EML IDs.

After the export, these commands work over the local archive:

```bash
./bin/mail-pp-cli inbox --account proton-free --json
./bin/mail-pp-cli search --account proton-free "from:someone" --json
./bin/mail-pp-cli read --account proton-free --id proton:proton-free:eml:<id> --json
./bin/mail-pp-cli summarize --account proton-free --id proton:proton-free:eml:<id>
```

## Google OAuth

Save the shared Gmail OAuth Desktop client JSON at:

```text
~/.config/printing-press/google-private/credentials/printing-press-gmail-cli.json
```

Then log in one Gmail account at a time:

```bash
./bin/mail-pp-cli auth login gmail --account user@example.com
./bin/mail-pp-cli auth login gmail --account alternate@example.com
```

The CLI requests these scopes by default:

- `https://www.googleapis.com/auth/gmail.readonly`
- `https://www.googleapis.com/auth/gmail.compose`
- `https://www.googleapis.com/auth/gmail.send`
- `https://www.googleapis.com/auth/gmail.modify`

Commands fail clearly when the stored token is missing a needed scope.

## Examples

```bash
./bin/mail-pp-cli accounts list
./bin/mail-pp-cli proton bridge status
./bin/mail-pp-cli proton export status
./bin/mail-pp-cli proton export automate --account proton-free --days 21
./bin/mail-pp-cli inbox --all
./bin/mail-pp-cli inbox --account gmail-main
./bin/mail-pp-cli search --all "from:conny"
./bin/mail-pp-cli read --account gmail-main --id gmail:gmail-main:<gmail-message-id>
./bin/mail-pp-cli summarize --account gmail-main --id gmail:gmail-main:<gmail-message-id>
./bin/mail-pp-cli draft --account gmail-main --to someone@example.com --subject "Test" --body "Hello"
./bin/mail-pp-cli send --account gmail-main --to someone@example.com --subject "Test" --body-file message.txt
```

Add `--json` or `--agent` to read/search/list commands for stable machine
output.

## Safe Replies And Sending

For replies, use a message ID from `inbox`, `search`, or `read`. Gmail replies
preserve the original thread using Gmail `threadId` and the original RFC
`Message-ID`.

```bash
./bin/mail-pp-cli write-reply \
  --account gmail-main \
  --id gmail:gmail-main:<message-id> \
  --body-file reply.txt

./bin/mail-pp-cli write-reply \
  --account gmail-main \
  --id gmail:gmail-main:<message-id> \
  --body-file reply.txt \
  --create-draft
```

`send` is a two-step safety flow. First run it without `--confirm-send`; it
prints the exact preview and exits without sending. Only after the user approves
that preview should the same command be rerun with `--confirm-send`.

```bash
./bin/mail-pp-cli send \
  --account gmail-main \
  --reply-to gmail:gmail-main:<message-id> \
  --body-file reply.txt

./bin/mail-pp-cli send \
  --account gmail-main \
  --reply-to gmail:gmail-main:<message-id> \
  --body-file reply.txt \
  --confirm-send
```

Use `--body-file` or escaped `\n` line breaks for formatted messages. Long
single-line send bodies are rejected by default.
