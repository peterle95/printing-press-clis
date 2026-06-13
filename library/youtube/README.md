# Youtube CLI

The YouTube Data API v3 is an API that provides access to YouTube data, such as videos, playlists, and channels.

Learn more at [Youtube](https://google.com).

## Install

The recommended path installs both the `youtube-pp-cli` binary and the `pp-youtube` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install youtube
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install youtube --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/youtube-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-youtube --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-youtube --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-youtube skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-youtube. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Google Credentials

Create a Google OAuth Desktop client JSON and place it in the shared Printing Press credential directory. See [Google credentials](#google-credentials).

```bash
youtube-pp-cli auth login
youtube-pp-cli auth status
```

### 3. Verify Setup

```bash
youtube-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
youtube-pp-cli youtube abuse-reports-insert --part example-value
```

## Google credentials

This CLI uses the official YouTube Data API and the official Google OAuth 2.0 installed/Desktop app flow. It does not scrape YouTube, automate YouTube web pages, or use service accounts.

Shared Printing Press credential layout:

```text
$HOME/printing-press/library/.google/
  credentials/
    youtube-oauth-client.json
  tokens/
    youtube-token.json
```

Setup:

```bash
mkdir -p $HOME/printing-press/library/.google/credentials
mkdir -p $HOME/printing-press/library/.google/tokens
```

Put downloaded OAuth Desktop client JSON here:

```text
$HOME/printing-press/library/.google/credentials/youtube-oauth-client.json
```

Then run:

```bash
make build
./bin/youtube-pp-cli auth login
# WSL-friendly: print the URL and do not try to open a browser
./bin/youtube-pp-cli auth login --no-browser
./bin/youtube-pp-cli auth status
./bin/youtube-pp-cli doctor
```

Default credential paths can be overridden with `YOUTUBE_PP_CREDENTIALS` and `YOUTUBE_PP_TOKEN`. Never commit OAuth client JSON files, tokens, cookies, or other credentials.

Default scopes requested by `auth login`:

- `https://www.googleapis.com/auth/youtube`
- `https://www.googleapis.com/auth/youtube.upload`

Some generated commands may require broader YouTube scopes. The imported official YouTube Data API spec lists these as acceptable scopes on moderation, delete, comment, caption, partner, or content-owner operations; they are documented here but are not requested by default:

- `https://www.googleapis.com/auth/youtube.force-ssl` should only be used when explicitly enabling broad moderation, delete, comment, or caption permissions.
- `https://www.googleapis.com/auth/youtubepartner` should only be used with a YouTube CMS/content-owner account.
- `https://www.googleapis.com/auth/youtubepartner-channel-audit` appears in the source spec for partner/channel audit flows.

## Usage

Run `youtube-pp-cli --help` for the full command reference and flag list.

## Recent Subscription Uploads

In this workspace, a prompt that says **notifications** means channels whose
YouTube web bell menu is set to **All**. YouTube Data API v3 does not expose
the web notifications tray or per-channel bell setting, so this mode requires
a file containing those All-bell channel IDs. The default file is
`~/.config/youtube-pp-cli/all-bell-channel-ids.txt`, or set
`YOUTUBE_PP_ALL_BELL_CHANNEL_ID_FILE`:

```bash
youtube-pp-cli workflow notifications --agent --since 24h
```

You can override the file per run:

```bash
youtube-pp-cli workflow notifications --agent --since 24h --channel-id-file all-bell-channel-ids.txt
```

Without a channel file, the CLI refuses notification mode instead of silently
treating every subscription as an All-bell channel.

Do not use `subscription.contentDetails.activityType=all` as a substitute for
the web bell setting. It describes subscription activity scope, not the
All/Personalized/None bell menu, and may match almost every subscription.

For broader "what did all my subscribed channels upload recently?" work, use:

```bash
youtube-pp-cli workflow recent-subscription-uploads --agent --since 24h
```

To add the discovered videos to a playlist:

```bash
youtube-pp-cli workflow notifications --agent --since 24h --playlist-id PL...
```

To preview playlist changes without mutating YouTube:

```bash
youtube-pp-cli workflow notifications --agent --since 24h --playlist-title "notifications" --create-playlist --dry-run
```

The workflow lists subscriptions, batches channel lookups to get upload
playlists, checks only the newest `--per-channel` uploads from each channel in
parallel, sorts by publish time, and skips videos already present in the
destination playlist.

## Commands

### workflow

Compound workflows

- **`youtube-pp-cli workflow notifications`** - Find recent uploads from channels whose YouTube web bell menu is set to All and optionally add them to a playlist.
- **`youtube-pp-cli workflow recent-subscription-uploads`** - Find recent uploads from all subscribed channels and optionally add them to a playlist.

### youtube

Manage youtube

- **`youtube-pp-cli youtube abuse-reports-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube activities-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube captions-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube captions-download`** - Downloads a caption track.
- **`youtube-pp-cli youtube captions-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube captions-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube captions-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube channel-banners-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube channel-sections-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube channel-sections-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube channel-sections-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube channel-sections-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube channels-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube channels-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube comment-threads-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube comment-threads-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube comments-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube comments-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube comments-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube comments-mark-as-spam`** - Expresses the caller's opinion that one or more comments should be flagged as spam.
- **`youtube-pp-cli youtube comments-set-moderation-status`** - Sets the moderation status of one or more comments.
- **`youtube-pp-cli youtube comments-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube i18n-languages-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube i18n-regions-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube live-broadcasts-bind`** - Bind a broadcast to a stream.
- **`youtube-pp-cli youtube live-broadcasts-delete`** - Delete a given broadcast.
- **`youtube-pp-cli youtube live-broadcasts-insert`** - Inserts a new stream for the authenticated user.
- **`youtube-pp-cli youtube live-broadcasts-insert-cuepoint`** - Insert cuepoints in a broadcast
- **`youtube-pp-cli youtube live-broadcasts-list`** - Retrieve the list of broadcasts associated with the given channel.
- **`youtube-pp-cli youtube live-broadcasts-transition`** - Transition a broadcast to a given status.
- **`youtube-pp-cli youtube live-broadcasts-update`** - Updates an existing broadcast for the authenticated user.
- **`youtube-pp-cli youtube live-chat-bans-delete`** - Deletes a chat ban.
- **`youtube-pp-cli youtube live-chat-bans-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube live-chat-messages-delete`** - Deletes a chat message.
- **`youtube-pp-cli youtube live-chat-messages-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube live-chat-messages-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube live-chat-moderators-delete`** - Deletes a chat moderator.
- **`youtube-pp-cli youtube live-chat-moderators-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube live-chat-moderators-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube live-streams-delete`** - Deletes an existing stream for the authenticated user.
- **`youtube-pp-cli youtube live-streams-insert`** - Inserts a new stream for the authenticated user.
- **`youtube-pp-cli youtube live-streams-list`** - Retrieve the list of streams associated with the given channel. --
- **`youtube-pp-cli youtube live-streams-update`** - Updates an existing stream for the authenticated user.
- **`youtube-pp-cli youtube members-list`** - Retrieves a list of members that match the request criteria for a channel.
- **`youtube-pp-cli youtube memberships-levels-list`** - Retrieves a list of all pricing levels offered by a creator to the fans.
- **`youtube-pp-cli youtube playlist-items-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube playlist-items-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube playlist-items-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube playlist-items-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube playlists-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube playlists-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube playlists-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube playlists-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube search-list`** - Retrieves a list of search resources
- **`youtube-pp-cli youtube subscriptions-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube subscriptions-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube subscriptions-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube super-chat-events-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube tests-insert`** - POST method.
- **`youtube-pp-cli youtube third-party-links-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube third-party-links-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube third-party-links-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube third-party-links-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube thumbnails-set`** - As this is not an insert in a strict sense (it supports uploading/setting of a thumbnail for multiple videos, which doesn't result in creation of a single resource), I use a custom verb here.
- **`youtube-pp-cli youtube update-comment-threads`** - Updates an existing resource.
- **`youtube-pp-cli youtube video-abuse-report-reasons-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube video-categories-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube videos-delete`** - Deletes a resource.
- **`youtube-pp-cli youtube videos-get-rating`** - Retrieves the ratings that the authorized user gave to a list of specified videos.
- **`youtube-pp-cli youtube videos-insert`** - Inserts a new resource into this collection.
- **`youtube-pp-cli youtube videos-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube videos-rate`** - Adds a like or dislike rating to a video or removes a rating from a video.
- **`youtube-pp-cli youtube videos-report-abuse`** - Report abuse for a video.
- **`youtube-pp-cli youtube videos-update`** - Updates an existing resource.
- **`youtube-pp-cli youtube watermarks-set`** - Allows upload of watermark image and setting it for a channel.
- **`youtube-pp-cli youtube watermarks-unset`** - Allows removal of channel watermark.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
youtube-pp-cli youtube abuse-reports-insert --part example-value

# JSON for scripting and agents
youtube-pp-cli youtube abuse-reports-insert --part example-value --json

# Filter to specific fields
youtube-pp-cli youtube abuse-reports-insert --part example-value --json --select id,name,status

# Dry run — show the request without sending
youtube-pp-cli youtube abuse-reports-insert --part example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
youtube-pp-cli youtube abuse-reports-insert --part example-value --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-youtube -g
```

Then invoke `/pp-youtube <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add youtube youtube-pp-mcp -e YOUTUBE_DATA_OAUTH2C=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/youtube-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `YOUTUBE_DATA_OAUTH2C` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "youtube": {
      "command": "youtube-pp-mcp",
      "env": {
        "YOUTUBE_DATA_OAUTH2C": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
youtube-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/youtube-data-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `YOUTUBE_PP_CREDENTIALS` | path override | No | OAuth Desktop client JSON path. Defaults to the shared Printing Press `.google/credentials/youtube-oauth-client.json`. |
| `YOUTUBE_PP_TOKEN` | path override | No | OAuth token JSON path. Defaults to the shared Printing Press `.google/tokens/youtube-token.json`. |
| `YOUTUBE_DATA_OAUTH2C` | legacy bearer token | No | Existing direct bearer-token override. Prefer `auth login` and the shared token file. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `youtube-pp-cli doctor` to check credentials
- Run `youtube-pp-cli auth status` and verify the credential/token paths exist
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
