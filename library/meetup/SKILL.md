---
name: pp-meetup
description: "Printing Press CLI for Meetup. Read-only wrapper for Meetup's public GraphQL search endpoint, focused on finding software-development networking..."
author: "Peter Moelzer"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - meetup-pp-cli
---

# Meetup — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `meetup-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install meetup --cli-only
   ```
2. Verify: `meetup-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read-only wrapper for Meetup's public GraphQL search endpoint, focused on finding software-development networking events.

## Command Reference

**find-events** - Find public Meetup events for software-development networking.

- `meetup-pp-cli find-events --agent` - Find in-person developer networking events in Berlin, Germany.
- `meetup-pp-cli find-events --url 'https://www.meetup.com/find/?keywords=developer&location=de--Berlin--Berlin&source=EVENTS' --limit 5 --agent`
- `meetup-pp-cli find-events --query devrel --networking-only --select results.title,results.date_time,results.rsvp_count,results.url --agent`

Defaults are tuned for the user's stated workflow: `--query developer`, Berlin
coordinates, `--event-type physical`, `--within-days 90`, `--min-rsvps 5`, and
`--sort networking`.

**gql-ext** - Raw GraphQL escape hatch for advanced reads.

- `meetup-pp-cli gql-ext` - Executes a GraphQL query against Meetup's current API endpoint. The endpoint supports anonymous read-only event search and bearer-token authenticated account-specific fields.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
meetup-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match - fall back to `--help` or use a narrower query.

## Auth Setup

Public `find-events` searches do not require authentication. A bearer token is
only needed for raw account-specific GraphQL fields. If you have a token, store
it:

```bash
meetup-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `MEETUP_GRAPHQL_SEARCH_BEARER_AUTH` as an environment variable.

Run `meetup-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** - JSON on stdout, errors on stderr
- **Filterable** - `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  meetup-pp-cli find-events --agent --select results.title,results.date_time,results.rsvp_count,results.url
  ```
- **Previewable** - `--dry-run` shows the request without sending
- **Non-interactive** - never prompts, every input is a flag
- **Explicit retries** - use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
meetup-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
meetup-pp-cli feedback --stdin < notes.txt
meetup-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.meetup-pp-cli/feedback.jsonl`. They are never POSTed unless `MEETUP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MEETUP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
meetup-pp-cli profile save briefing --json
meetup-pp-cli --profile briefing find-events
meetup-pp-cli profile list --json
meetup-pp-cli profile show briefing
meetup-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** -> show `meetup-pp-cli --help` output
2. **Starts with `install`** -> ends with `mcp` -> MCP installation; otherwise -> see Prerequisites above
3. **Anything else** -> Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it with your MCP client:

```bash
meetup-pp-mcp
```

## Direct Use

1. Check if installed: `which meetup-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Prefer `find-events` for user asks about Berlin software-development networking events.
3. Execute with the `--agent` flag:
   ```bash
   meetup-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `meetup-pp-cli <command> --help`.
