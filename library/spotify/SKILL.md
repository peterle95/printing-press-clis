# Spotify Printing Press CLI

Use this skill when the user wants to organize their Spotify library with the
local `spotify-pp-cli` module.

## Rules

- Use only the official Spotify Web API.
- Do not scrape Spotify or use browser automation.
- Do not use Audio Features, Audio Analysis, Recommendations, Related Artists,
  Featured Playlists, Category Playlists, or Spotify-owned editorial playlists.
- Treat copy and move differently:
  - copy adds to playlists and leaves Liked Songs unchanged.
  - move adds to playlists, verifies playlist membership, then removes from
    Liked Songs only after explicit confirmation.
- Prefer dry-run before mutations.

## Setup

```bash
export PATH="$HOME/printing-press/library/spotify/bin:$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd $HOME/printing-press/library/spotify
go build -o ./bin/spotify-pp-cli ./cmd/spotify-pp-cli
```

Spotify credentials come from environment variables or config:

```bash
export SPOTIFY_CLIENT_ID="..."
export SPOTIFY_REDIRECT_URI="http://127.0.0.1:43683/callback"
```

## Core Workflow

```bash
spotify-pp-cli auth login
spotify-pp-cli liked scan
spotify-pp-cli rules init
spotify-pp-cli classify liked
spotify-pp-cli liked plan
spotify-pp-cli liked copy --confirm
```

Use move only when the user explicitly asks:

```bash
spotify-pp-cli liked move --dry-run
spotify-pp-cli liked move --confirm
```

## Useful Commands

```bash
spotify-pp-cli auth status
spotify-pp-cli liked list
spotify-pp-cli liked export --format md
spotify-pp-cli rules explain <track>
spotify-pp-cli track copy <track> --to "Techno" --confirm
spotify-pp-cli playlists list
spotify-pp-cli playlist scan "Techno"
spotify-pp-cli ops list
spotify-pp-cli ops undo <operation-id> --dry-run
```

## Data Locations

- Config: `~/.config/printing-press/spotify/config.yaml`
- Rules: `~/.config/printing-press/spotify/rules.yaml`
- Token fallback: `~/.config/printing-press/spotify/token.json`
- SQLite DB: `~/.local/share/printing-press/spotify/spotify.db`
