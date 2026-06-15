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

## Output Format

When reporting weather, use `windy-weather-pp-cli today --json` (or
`tomorrow --json`) and format the result as a table with one row per hour.
Include a model agreement summary at the bottom.

### Table columns

| Hour | Weather | Temp | Rain | Clouds |
|---|---|---|---|---|

- **Hour** — local time extracted from the `time` field.
- **Weather** — the `summary` field (e.g. "Rain nearby").
- **Temp** — `temperature_c` with the degree suffix.
- **Rain** — `precipitation_mm` value; if >0 show the amount, otherwise "—".
- **Clouds** — `cloud_cover_pct` as a percentage.

Append a summary line showing the models used (ECMWF, GFS, ICON) and
`model_agreement_pct` from `raw_observations.model_agreement_pct`.

## Build And Test

```bash
npm ci
go test ./...
go vet ./...
go build ./...
```
