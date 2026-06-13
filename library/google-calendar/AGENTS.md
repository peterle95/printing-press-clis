# Google Calendar Printed CLI Agent Guide

This directory is a generated `google-calendar-pp-cli` project with local OAuth
portability patches.

## Operating Rules

- Keep OAuth client JSON and tokens outside the repository.
- The default private credential directory is
  `~/.config/printing-press/google-private`.
- Prefer `auth login --manual` or `auth login --no-browser` in headless WSL.
- Use `auth set-token --token-json <file>` for non-interactive token import.
- Inspect event mutations and use dry-run behavior where available.
- Keep personal event defaults in `AGENTS.override.md`.

## Build And Test

```bash
go test ./...
go vet ./...
go build ./...
```
