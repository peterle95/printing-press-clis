---
name: pp-youtube
description: "Printing Press CLI for Youtube. The YouTube Data API v3 is an API that provides access to YouTube data, such as videos, playlists, and channels."
author: "Peter Moelzer"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - youtube-pp-cli
---

# Youtube — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `youtube-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install youtube --cli-only
   ```
2. Verify: `youtube-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The YouTube Data API v3 is an API that provides access to YouTube data, such as videos, playlists, and channels.

## Command Reference

**workflow** — Compound workflows

- `youtube-pp-cli workflow recent-subscription-uploads` — Find recent YouTube notifications-style uploads from subscribed channels and optionally add them to a playlist.
- `youtube-pp-cli workflow notifications` — Find recent uploads from channels whose YouTube web bell menu is set to All and optionally add them to a playlist.

In this workspace, a prompt that says **notifications** means channels whose
YouTube web bell menu is set to **All**. Use `workflow notifications`, not the
broad subscription scan. YouTube Data API v3 does not expose the web
notifications tray or per-channel bell state, so this command requires a
channel-ID file collected from the web UI:

```bash
youtube-pp-cli workflow notifications --agent --since 24h
youtube-pp-cli workflow notifications --agent --since 24h --playlist-id PL...
youtube-pp-cli workflow notifications --agent --since 24h --playlist-title "notifications" --create-playlist --dry-run
```

The default channel file is `~/.config/youtube-pp-cli/all-bell-channel-ids.txt`.
Agents can override it with `YOUTUBE_PP_ALL_BELL_CHANNEL_ID_FILE` or
`--channel-id-file`.

Do not use `subscription.contentDetails.activityType=all` as a substitute for
the web bell setting. It describes subscription activity scope, not the
All/Personalized/None bell menu, and may match almost every subscription.

For broader "all subscribed channels" requests that do not mention
notifications, use:

```bash
youtube-pp-cli workflow recent-subscription-uploads --agent --since 24h
```

Notification mode intentionally errors without `--channel-id-file`, because the
Data API cannot discover All/Personalized/None bell state.

**youtube** — Manage youtube

- `youtube-pp-cli youtube abuse-reports-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube activities-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube captions-delete` — Deletes a resource.
- `youtube-pp-cli youtube captions-download` — Downloads a caption track.
- `youtube-pp-cli youtube captions-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube captions-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube captions-update` — Updates an existing resource.
- `youtube-pp-cli youtube channel-banners-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube channel-sections-delete` — Deletes a resource.
- `youtube-pp-cli youtube channel-sections-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube channel-sections-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube channel-sections-update` — Updates an existing resource.
- `youtube-pp-cli youtube channels-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube channels-update` — Updates an existing resource.
- `youtube-pp-cli youtube comment-threads-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube comment-threads-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube comments-delete` — Deletes a resource.
- `youtube-pp-cli youtube comments-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube comments-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube comments-mark-as-spam` — Expresses the caller's opinion that one or more comments should be flagged as spam.
- `youtube-pp-cli youtube comments-set-moderation-status` — Sets the moderation status of one or more comments.
- `youtube-pp-cli youtube comments-update` — Updates an existing resource.
- `youtube-pp-cli youtube i18n-languages-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube i18n-regions-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube live-broadcasts-bind` — Bind a broadcast to a stream.
- `youtube-pp-cli youtube live-broadcasts-delete` — Delete a given broadcast.
- `youtube-pp-cli youtube live-broadcasts-insert` — Inserts a new stream for the authenticated user.
- `youtube-pp-cli youtube live-broadcasts-insert-cuepoint` — Insert cuepoints in a broadcast
- `youtube-pp-cli youtube live-broadcasts-list` — Retrieve the list of broadcasts associated with the given channel.
- `youtube-pp-cli youtube live-broadcasts-transition` — Transition a broadcast to a given status.
- `youtube-pp-cli youtube live-broadcasts-update` — Updates an existing broadcast for the authenticated user.
- `youtube-pp-cli youtube live-chat-bans-delete` — Deletes a chat ban.
- `youtube-pp-cli youtube live-chat-bans-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube live-chat-messages-delete` — Deletes a chat message.
- `youtube-pp-cli youtube live-chat-messages-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube live-chat-messages-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube live-chat-moderators-delete` — Deletes a chat moderator.
- `youtube-pp-cli youtube live-chat-moderators-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube live-chat-moderators-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube live-streams-delete` — Deletes an existing stream for the authenticated user.
- `youtube-pp-cli youtube live-streams-insert` — Inserts a new stream for the authenticated user.
- `youtube-pp-cli youtube live-streams-list` — Retrieve the list of streams associated with the given channel. --
- `youtube-pp-cli youtube live-streams-update` — Updates an existing stream for the authenticated user.
- `youtube-pp-cli youtube members-list` — Retrieves a list of members that match the request criteria for a channel.
- `youtube-pp-cli youtube memberships-levels-list` — Retrieves a list of all pricing levels offered by a creator to the fans.
- `youtube-pp-cli youtube playlist-items-delete` — Deletes a resource.
- `youtube-pp-cli youtube playlist-items-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube playlist-items-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube playlist-items-update` — Updates an existing resource.
- `youtube-pp-cli youtube playlists-delete` — Deletes a resource.
- `youtube-pp-cli youtube playlists-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube playlists-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube playlists-update` — Updates an existing resource.
- `youtube-pp-cli youtube search-list` — Retrieves a list of search resources
- `youtube-pp-cli youtube subscriptions-delete` — Deletes a resource.
- `youtube-pp-cli youtube subscriptions-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube subscriptions-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube super-chat-events-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube tests-insert` — POST method.
- `youtube-pp-cli youtube third-party-links-delete` — Deletes a resource.
- `youtube-pp-cli youtube third-party-links-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube third-party-links-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube third-party-links-update` — Updates an existing resource.
- `youtube-pp-cli youtube thumbnails-set` — As this is not an insert in a strict sense (it supports uploading/setting of a thumbnail for multiple videos, which...
- `youtube-pp-cli youtube update-comment-threads` — Updates an existing resource.
- `youtube-pp-cli youtube video-abuse-report-reasons-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube video-categories-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube videos-delete` — Deletes a resource.
- `youtube-pp-cli youtube videos-get-rating` — Retrieves the ratings that the authorized user gave to a list of specified videos.
- `youtube-pp-cli youtube videos-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube videos-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube videos-rate` — Adds a like or dislike rating to a video or removes a rating from a video.
- `youtube-pp-cli youtube videos-report-abuse` — Report abuse for a video.
- `youtube-pp-cli youtube videos-update` — Updates an existing resource.
- `youtube-pp-cli youtube watermarks-set` — Allows upload of watermark image and setting it for a channel.
- `youtube-pp-cli youtube watermarks-unset` — Allows removal of channel watermark.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
youtube-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `youtube-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
youtube-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `YOUTUBE_DATA_OAUTH2C` as an environment variable.

Run `youtube-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  youtube-pp-cli youtube abuse-reports-insert --part example-value --agent --select id,name,status
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
youtube-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
youtube-pp-cli feedback --stdin < notes.txt
youtube-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.youtube-pp-cli/feedback.jsonl`. They are never POSTed unless `YOUTUBE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `YOUTUBE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
youtube-pp-cli profile save briefing --json
youtube-pp-cli --profile briefing youtube abuse-reports-insert --part example-value
youtube-pp-cli profile list --json
youtube-pp-cli profile show briefing
youtube-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `youtube-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add youtube-pp-mcp -- youtube-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which youtube-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   youtube-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `youtube-pp-cli <command> --help`.
