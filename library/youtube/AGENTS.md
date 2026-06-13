# Youtube Printed CLI Agent Guide

This directory is a generated `youtube-pp-cli` printed CLI. It was produced by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), so treat systemic fixes as upstream Printing Press fixes first. Keep local edits narrow and document why a generated-tree patch belongs here.

## Local Operating Contract

Start by asking the generated CLI for current runtime truth:

```bash
youtube-pp-cli doctor --json
youtube-pp-cli agent-context --pretty
```

Use runtime discovery instead of relying on a copied command list:

```bash
youtube-pp-cli which "<capability>" --json
youtube-pp-cli <command> --help
```

Add `--agent` to command invocations for JSON, compact output, non-interactive defaults, no color, and confirmation-safe scripting:

```bash
youtube-pp-cli <command> --agent
```

Before running an unfamiliar command that may mutate remote state, inspect its help and prefer a dry run:

```bash
youtube-pp-cli <command> --help
youtube-pp-cli <command> --dry-run --agent
```

Use `--yes --no-input` only after the target, arguments, and side effects are clear.

For install, auth, examples, and longer product guidance, read `README.md` and `SKILL.md`. This file intentionally stays small so repo-local agents get invariant local guidance without duplicating the generated docs.

## Recent Subscription Uploads

In this workspace, a prompt containing **notifications** means channels whose
YouTube web bell menu is set to **All**. Use the fail-closed notification alias:

```bash
youtube-pp-cli workflow notifications --agent --since 24h --channel-id-file <path>
```

If `--channel-id-file` is omitted, the workflow uses
`~/.config/youtube-pp-cli/all-bell-channel-ids.txt` or the
`YOUTUBE_PP_ALL_BELL_CHANNEL_ID_FILE` environment variable.

The YouTube Data API does not expose the web notifications tray or per-channel
bell state. This workflow finds recent uploads from subscribed channels by
listing subscriptions, batching channel lookups, and checking upload playlists
in parallel. To add results to a playlist, pass `--playlist-id` or
`--playlist-title`; use `--dry-run` first for playlist mutations.

Do not treat all subscriptions as equivalent to notifications. The workflow
intentionally errors without a channel-ID file collected from the web UI.
Also do not substitute `subscription.contentDetails.activityType=all` for the
web bell setting; in this account it appears on nearly every subscription and
does not distinguish the YouTube All/Personalized/None bell menu.

## Local Customizations

If you modify this CLI beyond what the generator produced, record each customization so it isn't lost on the next regen and is visible to the next reader.

1. **Mark every changed site** in source with a comment summarizing the deviation:

    ```
    // PATCH: <one-line summary>
    ```

    Include an upstream reference inline when there is one (e.g. `// PATCH(upstream cli-printing-press#<issue>): ...`). `grep -rn 'PATCH' .` from this directory then surfaces every customization.

2. **Catalog the change** in a `.printing-press-patches.json` at this CLI's root (parallel to `.printing-press.json`). Minimum shape:

    ```json
    {
      "schema_version": 1,
      "applied_at": "YYYY-MM-DD",
      "base_run_id": "<copy from .printing-press.json>",
      "base_printing_press_version": "<copy from .printing-press.json>",
      "patches": [
        {
          "id": "short-identifier",
          "summary": "What changed (one sentence).",
          "reason": "Why this customization was needed (one or two sentences).",
          "files": ["internal/cli/foo.go"],
          "validated_outcome": "Optional: non-obvious test result that confirms the fix.",
          "upstream_issue": "Optional: https://github.com/mvanhorn/cli-printing-press/issues/<n>"
        }
      ]
    }
    ```

This file is an **index of customizations**, not a second copy of the diff. Diffs live in `git`; code lives in the source files; the inline `// PATCH:` comment carries the local semantics. Keep `summary` and `reason` short -- if you find yourself writing tables of field renames or code transformations, that detail belongs in the source comment or commit message, not here.

## Playlist Operations

When moving a video from one playlist to another, you must perform a two-step operation:
1. Insert the video into the destination playlist using the relevant `playlist-items-insert` command.
2. Remove the video from the original playlist using the corresponding `playlist-items-delete` command.

Always ensure both operations succeed to complete the move.
