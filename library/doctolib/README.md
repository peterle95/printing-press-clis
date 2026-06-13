# Doctolib CLI

Public website endpoints used by Doctolib's patient search pages.

Learn more at [Doctolib](https://www.doctolib.de).

## Install

The recommended path installs both the `doctolib-pp-cli` binary and the `pp-doctolib` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install doctolib
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install doctolib --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/doctolib-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-doctolib --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-doctolib --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-doctolib skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-doctolib. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
doctolib-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
doctolib-pp-cli find-doctors --reason hausarzt --location berlin --visit-reason akut --within-days 7
```

## Usage

Run `doctolib-pp-cli --help` for the full command reference and flag list.

## Commands

### find-doctors

Find doctors or practices with real appointment slots returned by Doctolib.

```bash
# Search by common specialty/reason and city slug
doctolib-pp-cli find-doctors --reason hausarzt --location berlin --within-days 7

# Narrow to a visit motive/reason shown by Doctolib
doctolib-pp-cli find-doctors --reason allgemeinmedizin --location berlin --visit-reason akut --limit 5 --json

# Filter around coordinates
doctolib-pp-cli find-doctors --url https://www.doctolib.de/allgemeinmedizin/berlin --lat 52.52 --lng 13.405 --radius-km 10 --json
```

`--reason` accepts Doctolib slugs such as `allgemeinmedizin` and a few common aliases such as `hausarzt`. For exact control, pass a copied Doctolib search URL with `--url`.

Booking is not implemented. This command is read-only and only returns profile links, visit motive IDs, agenda IDs, and slot times that can be used for a future booking workflow.

### doctolib-website-search-search

Manage doctolib website search search

- **`doctolib-pp-cli doctolib-website-search-search get-availabilities`** - Returns available appointment slots for a visit motive, agendas, practice, insurance sector, and start date.

### patient-health-search

Manage patient health search

- **`doctolib-pp-cli patient-health-search get-patient-search-filters`** - Fetch available patient-search filters
- **`doctolib-pp-cli patient-health-search get-search-qualification-step`** - Fetch search qualification options
- **`doctolib-pp-cli patient-health-search search-healthcare-providers`** - Patient health search endpoint used by Doctolib's frontend for specialty/symptom navigation.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
doctolib-pp-cli doctolib-website-search-search --start-date-time 2026-01-15 --limit 50

# JSON for scripting and agents
doctolib-pp-cli doctolib-website-search-search --start-date-time 2026-01-15 --limit 50 --json

# Filter to specific fields
doctolib-pp-cli doctolib-website-search-search --start-date-time 2026-01-15 --limit 50 --json --select id,name,status

# Dry run — show the request without sending
doctolib-pp-cli doctolib-website-search-search --start-date-time 2026-01-15 --limit 50 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
doctolib-pp-cli doctolib-website-search-search --start-date-time 2026-01-15 --limit 50 --agent
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
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-doctolib -g
```

Then invoke `/pp-doctolib <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add doctolib doctolib-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/doctolib-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "doctolib": {
      "command": "doctolib-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
doctolib-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/doctolib-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
