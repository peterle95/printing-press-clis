---
name: pp-doctolib
description: "Printing Press CLI for Doctolib. Public website endpoints used by Doctolib's patient search pages."
author: "Peter Moelzer"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - doctolib-pp-cli
---

# Doctolib — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `doctolib-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install doctolib --cli-only
   ```
2. Verify: `doctolib-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Public website endpoints used by Doctolib's patient search pages.

## Command Reference

**find-doctors** — Find doctors or practices with real appointment slots.

- `doctolib-pp-cli find-doctors --reason hausarzt --location berlin --within-days 7 --agent`
- Add `--visit-reason akut` to match a specific visit motive/reason.
- Add `--lat <value> --lng <value> --radius-km <value>` to filter around the user's coordinates.
- Use `--url <doctolib-search-url>` when the user provides or copied a Doctolib search results page.

Booking is not implemented. Keep this workflow read-only unless the user explicitly asks to design the future authenticated booking path.

**doctolib-website-search-search** — Manage doctolib website search search

- `doctolib-pp-cli doctolib-website-search-search` — Returns available appointment slots for a visit motive, agendas, practice, insurance sector, and start date.

**patient-health-search** — Manage patient health search

- `doctolib-pp-cli patient-health-search get-patient-search-filters` — Fetch available patient-search filters
- `doctolib-pp-cli patient-health-search get-search-qualification-step` — Fetch search qualification options
- `doctolib-pp-cli patient-health-search search-healthcare-providers` — Patient health search endpoint used by Doctolib's frontend for specialty/symptom navigation.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
doctolib-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `doctolib-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  doctolib-pp-cli find-doctors --reason hausarzt --location berlin --within-days 7 --agent --select results.name,results.first_available
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
doctolib-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
doctolib-pp-cli feedback --stdin < notes.txt
doctolib-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.doctolib-pp-cli/feedback.jsonl`. They are never POSTed unless `DOCTOLIB_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DOCTOLIB_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
doctolib-pp-cli profile save briefing --json
doctolib-pp-cli --profile briefing find-doctors --reason hausarzt --location berlin --within-days 7
doctolib-pp-cli profile list --json
doctolib-pp-cli profile show briefing
doctolib-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `doctolib-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add doctolib-pp-mcp -- doctolib-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which doctolib-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
   Prefer `find-doctors` for user asks about finding available doctors near a place or coordinate.
3. Execute with the `--agent` flag:
   ```bash
   doctolib-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `doctolib-pp-cli <command> --help`.
