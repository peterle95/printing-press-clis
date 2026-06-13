# Spotify Printed CLI Agent Guide

This directory is a hand-authored local `spotify-pp-cli` module for the Printing
Press library.

## Operating Rules

- Use Codex as the orchestrating agent.
- Use only the official Spotify Web API.
- Do not scrape Spotify, use browser automation, or call unofficial endpoints.
- Do not add Audio Features, Audio Analysis, Recommendations, Related Artists,
  Featured Playlists, Category Playlists, or editorial playlist workflows.
- Never remove tracks from Liked Songs unless a move command was explicitly
  confirmed and playlist membership was verified after the add.
- Prefer dry-run before every Spotify mutation.

## Build And Test

```bash
cd $HOME/printing-press/library/spotify
go fmt ./...
go test ./...
go build -o ./bin/spotify-pp-cli ./cmd/spotify-pp-cli
```

Live tests must stay gated:

```bash
SPOTIFY_INTEGRATION_TESTS=1 go test ./internal/client -run Integration
```
