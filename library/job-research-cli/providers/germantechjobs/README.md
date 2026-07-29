# Germantechjobs CLI

Public GermanTechJobs HTML and RSS search surfaces.

## Install

The recommended path installs both the `germantechjobs-pp-cli` binary and the `pp-germantechjobs` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install germantechjobs
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install germantechjobs --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/germantechjobs-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-germantechjobs --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-germantechjobs --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-germantechjobs skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-germantechjobs. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
germantechjobs-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
germantechjobs-pp-cli en mock-value mock-value
```

## Usage

Run `germantechjobs-pp-cli --help` for the full command reference and flag list.

## Commands

### en

Manage en

- **`germantechjobs-pp-cli en search-jobs`** - Fetch GermanTechJobs search results

### rss

Manage rss

- **`germantechjobs-pp-cli rss rss`** - Fetch GermanTechJobs RSS feed


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
germantechjobs-pp-cli en mock-value mock-value

# JSON for scripting and agents
germantechjobs-pp-cli en mock-value mock-value --json

# Filter to specific fields
germantechjobs-pp-cli en mock-value mock-value --json --select id,name,status

# Dry run — show the request without sending
germantechjobs-pp-cli en mock-value mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
germantechjobs-pp-cli en mock-value mock-value --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-germantechjobs -g
```

Then invoke `/pp-germantechjobs <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add germantechjobs germantechjobs-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/germantechjobs-current).
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
    "germantechjobs": {
      "command": "germantechjobs-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
germantechjobs-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/germantechjobs-public-search-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
