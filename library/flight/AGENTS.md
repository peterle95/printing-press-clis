# Flight Printed CLI Agent Guide

This directory is a hand-authored local `flight-pp-cli` module for the Printing
Press library.

## Operating Rules

- Use Codex as the orchestrating agent.
- Use official APIs, affiliate APIs, or safe deep links only.
- Do not scrape Google Flights, Skyscanner, Expedia, KAYAK, or airline sites.
- Do not implement bot-detection evasion, rotating proxies, captcha bypass,
  account abuse, or credential stuffing.
- Keep API keys, OAuth tokens, cookies, and affiliate credentials outside the
  repository.
- Treat flight prices as volatile. Every result must be verified on the
  provider or booking page before purchase.
- When presenting any flight result, always include a precise clickable link
  to the tickets (Google Flights, Skyscanner, Kiwi, airline site, or deep
  link). Never show a result without its link.
- Prefer `--no-cache` only when the user needs a fresh provider request.

## Build And Test

```bash
go fmt ./...
go test ./...
go build -o ./bin/flight-pp-cli ./cmd/flight-pp-cli
```

Live provider checks must stay manual and credential-gated. Unit tests must mock
all external API calls.
