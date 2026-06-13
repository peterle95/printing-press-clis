# Transit Printed CLI Agent Guide

This directory is a hand-authored `transit-pp-cli` project for Berlin VBB/BVG
journey data through `https://v6.vbb.transport.rest`.

## Operating Rules

- Do not scrape BVG or VBB journey-planner pages.
- Keep home addresses and coordinates in local config only.
- Use fake addresses and coordinates in tests and documentation.
- Keep request frequency within the built-in limits.
- Present line-specific queries in separate direction tables.
- Keep personal stop and walking-time defaults in `AGENTS.override.md`.

## Build And Test

```bash
go test ./...
go vet ./...
go build ./...
```
