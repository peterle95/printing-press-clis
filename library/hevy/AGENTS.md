# Hevy Printed CLI Agent Guide

This is a hand-authored Go CLI for importing Hevy CSV exports, maintaining
local workout plans, and using the public Hevy website through Playwright.

## Safety rules

- Never call, inspect, or reproduce private Hevy APIs.
- Browser interaction must use visible UI controls only; never bypass login,
  CAPTCHA, two-factor authentication, rate limits, or account limits.
- Do not store passwords, cookies, tokens, browser profiles, CSV exports,
  databases, screenshots, or debug HTML in Git.
- Run routine changes with `--dry-run` first. Live routine mutations require
  explicit confirmation and must stop on an unexpected UI state.
- Store operator-specific instructions in `AGENTS.override.md`; this file is intentionally ignored by Git.
- Keep real-site tests opt-in with `HEVY_E2E_REAL=1`.

## Build and test

```bash
cd $HOME/printing-press/library/hevy
go fmt ./...
go vet ./...
go test ./...
go build -o ./bin/hevy ./cmd/hevy
```
