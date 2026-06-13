# Ba Jobsuche CLI

Official Bundesagentur fuer Arbeit Jobsuche endpoint.

## Install

The recommended path installs both the `ba-jobsuche-pp-cli` binary and the `pp-ba-jobsuche` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install ba-jobsuche
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install ba-jobsuche --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ba-jobsuche-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ba-jobsuche --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ba-jobsuche --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-ba-jobsuche skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-ba-jobsuche. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
ba-jobsuche-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
ba-jobsuche-pp-cli ba-jobsuche-jobs
```

## Usage

Run `ba-jobsuche-pp-cli --help` for the full command reference and flag list.

## Commands

### ba-jobsuche-jobs

Manage ba jobsuche jobs

- **`ba-jobsuche-pp-cli ba-jobsuche-jobs search`** - Search BA Jobsuche jobs


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ba-jobsuche-pp-cli ba-jobsuche-jobs

# JSON for scripting and agents
ba-jobsuche-pp-cli ba-jobsuche-jobs --json

# Filter to specific fields
ba-jobsuche-pp-cli ba-jobsuche-jobs --json --select id,name,status

# Dry run — show the request without sending
ba-jobsuche-pp-cli ba-jobsuche-jobs --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ba-jobsuche-pp-cli ba-jobsuche-jobs --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-ba-jobsuche -g
```

Then invoke `/pp-ba-jobsuche <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add ba-jobsuche ba-jobsuche-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ba-jobsuche-current).
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
    "ba-jobsuche": {
      "command": "ba-jobsuche-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
ba-jobsuche-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/ba-jobsuche-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
