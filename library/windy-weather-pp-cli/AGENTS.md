# Windy Weather Printed CLI Agent Guide

This directory is a hand-authored CLI that reads the public Windy.com web
interface through Playwright.

## Operating Rules

- Do not bypass login, paywalls, anti-bot protections, or captchas.
- Keep request frequency low and prefer the built-in cache.
- Never log cookies, auth headers, session values, or tracking identifiers.
- The CLI returns calendar advice but does not create calendar events.
- Every report must include model confidence, agreement percentage, and model
  divergence when forecasts disagree.
- Keep the operator's location in local configuration or `AGENTS.override.md`.

## Build And Test

```bash
npm ci
go test ./...
go vet ./...
go build ./...
```
