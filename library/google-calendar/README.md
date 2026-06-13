# Google Calendar CLI

Manipulates events and other calendar data.

Learn more at [Google Calendar](https://google.com).

## Install

The recommended path installs both the `google-calendar-pp-cli` binary and the `pp-google-calendar` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install google-calendar
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install google-calendar --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-calendar-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-google-calendar --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-google-calendar --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-google-calendar skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-google-calendar. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Create a Google OAuth Desktop app in Google Cloud Console and save the client
JSON here:

```text
$HOME/printing-press/library/.google/credentials/calendar-tasks-oauth-client.json
```

Then authenticate:

```bash
google-calendar-pp-cli auth login
```

WSL/headless-friendly alternatives:

```bash
google-calendar-pp-cli auth login --no-browser
google-calendar-pp-cli auth login --manual
```

`--no-browser` prints the Google OAuth URL and waits for the localhost callback.
`--manual` prints the URL, asks you to open it yourself, then asks you to paste
the full redirected localhost URL so the CLI can extract the `code` parameter.

If you already have a Google OAuth token JSON file, store it without hand-writing
TOML:

```bash
google-calendar-pp-cli auth set-token --token-json token.json
cat token.json | google-calendar-pp-cli auth set-token --token-json -
```

You can also set a short-lived access token via environment variable:

```bash
export CALENDAR_OAUTH2C="ya29..."
```

### 3. Verify Setup

```bash
google-calendar-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
google-calendar-pp-cli calendars get mock-value
```

## Usage

Run `google-calendar-pp-cli --help` for the full command reference and flag list.

## Commands

### calendars

Manage calendars

- **`google-calendar-pp-cli calendars delete`** - Deletes a secondary calendar. Use calendars.clear for clearing all events on primary calendars.
- **`google-calendar-pp-cli calendars get`** - Returns metadata for a calendar.
- **`google-calendar-pp-cli calendars insert`** - Creates a secondary calendar.
- **`google-calendar-pp-cli calendars patch`** - Updates metadata for a calendar. This method supports patch semantics.
- **`google-calendar-pp-cli calendars update`** - Updates metadata for a calendar.

### channels

Manage channels

- **`google-calendar-pp-cli channels stop`** - Stop watching resources through this channel

### colors

Manage colors

- **`google-calendar-pp-cli colors get`** - Returns the color definitions for calendars and events.

### free-busy

Manage free busy

- **`google-calendar-pp-cli free-busy calendar-query`** - Returns free/busy information for a set of calendars.

### users

Manage users

- **`google-calendar-pp-cli users calendar-calendar-list-delete`** - Removes a calendar from the user's calendar list.
- **`google-calendar-pp-cli users calendar-calendar-list-get`** - Returns a calendar from the user's calendar list.
- **`google-calendar-pp-cli users calendar-calendar-list-insert`** - Inserts an existing calendar into the user's calendar list.
- **`google-calendar-pp-cli users calendar-calendar-list-list`** - Returns the calendars on the user's calendar list.
- **`google-calendar-pp-cli users calendar-calendar-list-patch`** - Updates an existing calendar on the user's calendar list. This method supports patch semantics.
- **`google-calendar-pp-cli users calendar-calendar-list-update`** - Updates an existing calendar on the user's calendar list.
- **`google-calendar-pp-cli users calendar-calendar-list-watch`** - Watch for changes to CalendarList resources.
- **`google-calendar-pp-cli users calendar-settings-get`** - Returns a single user setting.
- **`google-calendar-pp-cli users calendar-settings-list`** - Returns all user settings for the authenticated user.
- **`google-calendar-pp-cli users calendar-settings-watch`** - Watch for changes to Settings resources.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
google-calendar-pp-cli calendars get mock-value

# JSON for scripting and agents
google-calendar-pp-cli calendars get mock-value --json

# Filter to specific fields
google-calendar-pp-cli calendars get mock-value --json --select id,name,status

# Dry run — show the request without sending
google-calendar-pp-cli calendars get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
google-calendar-pp-cli calendars get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-google-calendar -g
```

Then invoke `/pp-google-calendar <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add google-calendar google-calendar-pp-mcp -e CALENDAR_OAUTH2C=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-calendar-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CALENDAR_OAUTH2C` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "google-calendar": {
      "command": "google-calendar-pp-mcp",
      "env": {
        "CALENDAR_OAUTH2C": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
google-calendar-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/google-calendar-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CALENDAR_OAUTH2C` | per_call | No | Optional short-lived OAuth access token override. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `google-calendar-pp-cli auth status` and `google-calendar-pp-cli doctor`
- In WSL, prefer `google-calendar-pp-cli auth login --manual`
- Use `google-calendar-pp-cli auth set-token --token-json token.json` instead of manually editing TOML
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
