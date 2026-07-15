---
name: pp-trello-calendar
description: "Printing Press CLI for Trello Calendar. Trello REST API surface with Trello-to-Google-Calendar scheduling workflows."
author: "Peter Moelzer"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - trello-calendar-pp-cli
---

# Trello Calendar — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `trello-calendar-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install trello-calendar --cli-only
   ```
2. Verify: `trello-calendar-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Trello REST API surface with Trello-to-Google-Calendar scheduling workflows.

## Planner Safety Contract

Authenticate and inspect health before planning:

```bash
trello-calendar-pp-cli auth google --no-browser
trello-calendar-pp-cli doctor --json
trello-calendar-pp-cli cards --agent
```

Use `preview` or `schedule --dry-run` before live scheduling. A live agent or
JSON invocation must include `--yes`; never infer permission to add Trello
comments unless `--comment-on-card` was explicitly requested.

Before live scheduling, inspect `schedule --dry-run` output for cards in the
"Doing" list flagged for re-schedule. Ask operator if each is completed — if
yes, move card to "Done" list or archive before running live schedule.

```bash
trello-calendar-pp-cli preview --agent
trello-calendar-pp-cli schedule --dry-run --agent
trello-calendar-pp-cli schedule --yes --agent
```

`cards` and `preview` are read-only. `schedule` creates Google Calendar events,
and `--comment-on-card` additionally mutates Trello. Deterministic event IDs and
private extended properties make repeated scheduling idempotent.

## Command Reference

**actions** — Manage actions

- `trello-calendar-pp-cli actions delete-id` — Delete a specific action. Only comment actions can be deleted.
- `trello-calendar-pp-cli actions get-id` — Get an Action
- `trello-calendar-pp-cli actions get-id-field` — Get a specific property of an action
- `trello-calendar-pp-cli actions put-id` — Update a specific Action. Only comment actions can be updated. Used to edit the content of a comment.

**applications** — Manage applications


**batch** — Manage batch

- `trello-calendar-pp-cli batch` — Make up to 10 GET requests in a single, batched API call.

**boards** — Manage boards

- `trello-calendar-pp-cli boards delete-id` — Delete a board.
- `trello-calendar-pp-cli boards get-id` — Request a single board.
- `trello-calendar-pp-cli boards get-id-field` — Get a single, specific field on a board
- `trello-calendar-pp-cli boards post` — Create a new board.
- `trello-calendar-pp-cli boards put-id` — Update an existing board by id

**cards** — Manage cards

- `trello-calendar-pp-cli cards delete-id` — Delete a Card
- `trello-calendar-pp-cli cards get-id` — Get a card by its ID
- `trello-calendar-pp-cli cards get-id-field` — Get a specific property of a card
- `trello-calendar-pp-cli cards post` — Create a new card. Query parameters may also be replaced with a JSON request body instead.
- `trello-calendar-pp-cli cards put-id` — Update a card. Query parameters may also be replaced with a JSON request body instead.

**checklists** — Manage checklists

- `trello-calendar-pp-cli checklists delete-id` — Delete a checklist
- `trello-calendar-pp-cli checklists get-id` — Get a Checklist
- `trello-calendar-pp-cli checklists get-id-field` — Get field on a Checklist
- `trello-calendar-pp-cli checklists post` — Create a Checklist
- `trello-calendar-pp-cli checklists put-checlists-id` — Update an existing checklist.
- `trello-calendar-pp-cli checklists put-id-field` — Update field on a Checklist

**custom-fields** — Manage custom fields

- `trello-calendar-pp-cli custom-fields delete-id` — Delete a Custom Field from a board.
- `trello-calendar-pp-cli custom-fields get-id` — Get a Custom Field
- `trello-calendar-pp-cli custom-fields post` — Create a new Custom Field on a board.
- `trello-calendar-pp-cli custom-fields put-id` — Update a Custom Field definition.

**emoji** — Manage emoji

- `trello-calendar-pp-cli emoji` — List available Emoji

**enterprises** — Manage enterprises

- `trello-calendar-pp-cli enterprises get-id` — Get an enterprise by its ID.
- `trello-calendar-pp-cli enterprises put-id-join-request-bulk` — Decline enterpriseJoinRequests from one organization or a bulk list of organizations.

**labels** — Manage labels

- `trello-calendar-pp-cli labels delete-id` — Delete a label by ID.
- `trello-calendar-pp-cli labels get-id` — Get information about a single Label.
- `trello-calendar-pp-cli labels post` — Create a new Label on a Board.
- `trello-calendar-pp-cli labels put-id` — Update a label by ID.
- `trello-calendar-pp-cli labels put-id-field` — Update a field on a label.

**lists** — Manage lists

- `trello-calendar-pp-cli lists get-id` — Get information about a List
- `trello-calendar-pp-cli lists post` — Create a new List on a Board
- `trello-calendar-pp-cli lists put-id` — Update the properties of a List
- `trello-calendar-pp-cli lists put-id-field` — Update a field on a List

**members** — Manage members

- `trello-calendar-pp-cli members get-id` — Get a member
- `trello-calendar-pp-cli members get-id-field` — Get a particular property of a member
- `trello-calendar-pp-cli members put-id` — Update a Member

**notifications** — Manage notifications

- `trello-calendar-pp-cli notifications get-id` — Get a Notification
- `trello-calendar-pp-cli notifications get-id-field` — Get a specific property of a notification
- `trello-calendar-pp-cli notifications post-all-read` — Mark all notifications as read
- `trello-calendar-pp-cli notifications put-id` — Update the read status of a notification

**organizations** — Manage organizations

- `trello-calendar-pp-cli organizations delete-id` — Delete an Organization
- `trello-calendar-pp-cli organizations get-id` — Get an Organization
- `trello-calendar-pp-cli organizations get-id-field` — Get field on Organization
- `trello-calendar-pp-cli organizations post` — Create a new Organization
- `trello-calendar-pp-cli organizations put-id` — Update an organization

**plugins** — Manage plugins

- `trello-calendar-pp-cli plugins get-id` — Get a Plugin
- `trello-calendar-pp-cli plugins put-id` — Update a Plugin

**tokens** — Manage tokens

- `trello-calendar-pp-cli tokens delete` — Delete a token.
- `trello-calendar-pp-cli tokens get` — Retrieve information about a token.

**trello-calendar-search** — Manage trello calendar search

- `trello-calendar-pp-cli trello-calendar-search get` — Find what you're looking for in Trello
- `trello-calendar-pp-cli trello-calendar-search get-members` — Search for Trello members.

**webhooks** — Manage webhooks

- `trello-calendar-pp-cli webhooks delete-id` — Delete a webhook by ID.
- `trello-calendar-pp-cli webhooks get-id` — Get a webhook by ID. You must use the token query parameter and pass in the token the webhook was created under, or...
- `trello-calendar-pp-cli webhooks post` — Create a new webhook.
- `trello-calendar-pp-cli webhooks put-id` — Update a webhook by ID.
- `trello-calendar-pp-cli webhooks webhooksidfield` — Get a field on a Webhook


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
trello-calendar-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

This CLI uses a browser session. Log in to  in Chrome, then:

```bash
trello-calendar-pp-cli auth login --chrome
```

Requires a cookie extraction tool (`pycookiecheat` via pip, or `cookies` via Homebrew).

Run `trello-calendar-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  trello-calendar-pp-cli batch --urls https://example.com/resource --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
trello-calendar-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
trello-calendar-pp-cli feedback --stdin < notes.txt
trello-calendar-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.trello-calendar-pp-cli/feedback.jsonl`. They are never POSTed unless `TRELLO_CALENDAR_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TRELLO_CALENDAR_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
trello-calendar-pp-cli profile save briefing --json
trello-calendar-pp-cli --profile briefing batch --urls https://example.com/resource
trello-calendar-pp-cli profile list --json
trello-calendar-pp-cli profile show briefing
trello-calendar-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `trello-calendar-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add trello-calendar-pp-mcp -- trello-calendar-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which trello-calendar-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   trello-calendar-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `trello-calendar-pp-cli <command> --help`.
