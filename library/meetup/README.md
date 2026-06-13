# Meetup CLI

Read-only wrapper for Meetup's public GraphQL search endpoint, focused on finding software-development networking events.

## Install

The recommended path installs both the `meetup-pp-cli` binary and the `pp-meetup` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install meetup
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install meetup --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/meetup-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-meetup --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-meetup --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-meetup skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-meetup. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
meetup-pp-cli doctor
```

This checks configuration and connectivity. Public event search works without a token.

### 3. Try Your First Command

```bash
meetup-pp-cli find-events
```

## Usage

Run `meetup-pp-cli --help` for the full command reference and flag list.

## Commands

### find-events

Find public Meetup events for software-development networking. Defaults are
tuned for Berlin, Germany:

```bash
# Find in-person developer networking events in Berlin
meetup-pp-cli find-events

# Use the same Meetup Find URL shape from the website
meetup-pp-cli find-events --url 'https://www.meetup.com/find/?keywords=developer&location=de--Berlin--Berlin&source=EVENTS' --limit 5 --json

# Agent-friendly output with only the fields you need
meetup-pp-cli find-events --agent --select results.title,results.date_time,results.rsvp_count,results.url

# Change the topic while keeping Berlin defaults
meetup-pp-cli find-events --query devrel --networking-only --limit 5
```

Useful filters:

- `--query` defaults to `developer`.
- `--city`, `--country`, `--lat`, `--lon`, and `--radius-km` default to Berlin.
- `--event-type` defaults to `physical`; use `online`, `hybrid`, or `all`.
- `--within-days` defaults to `90`; use `0` to disable the upper date bound.
- `--min-rsvps` defaults to `5` so tiny placeholder events do not dominate.
- `--sort` defaults to `networking`; use `date` or `relevance` when needed.

### gql-ext

Raw GraphQL escape hatch for advanced reads.

- **`meetup-pp-cli gql-ext execute-graph-ql`** - Executes a GraphQL query against Meetup's current API endpoint. The
endpoint supports anonymous read-only eventSearch requests for public
discovery and bearer-token authenticated requests for account-specific
data.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
meetup-pp-cli find-events

# JSON for scripting and agents
meetup-pp-cli find-events --json

# Filter to specific fields
meetup-pp-cli find-events --json --select results.title,results.date_time,results.url

# Dry run - show the request without sending
meetup-pp-cli find-events --dry-run

# Agent mode - JSON + compact + no prompts in one flag
meetup-pp-cli find-events --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as an MCP Server

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register `meetup-pp-mcp` with your MCP client. The `MEETUP_GRAPHQL_SEARCH_BEARER_AUTH` environment variable is optional and only needed for account-specific GraphQL fields.

## Health Check

```bash
meetup-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/meetup-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Optional environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MEETUP_GRAPHQL_SEARCH_BEARER_AUTH` | per_call | No | Optional bearer token for account-specific GraphQL fields. Public `find-events` searches do not need it. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Public `find-events` search does not require credentials.
- If you are using raw account-specific GraphQL fields, run `meetup-pp-cli doctor` and verify the optional token env var: `echo $MEETUP_GRAPHQL_SEARCH_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
