# Trade Republic Printing Press CLI Agent Guide

This directory is a hand-authored, local-first Go CLI for normalizing Trade
Republic portfolio data, timeline exports, and statement PDFs into SQLite.

## Operating Rules

- Treat `pytr` as an isolated compatibility adapter for Trade Republic's
  private API. Do not reproduce, extend, or directly call the private protocol
  from Go.
- Use `pytr`'s web login flow by default. Never put a phone PIN, four-digit
  second-factor code, cookies, tokens, or private keys in command arguments,
  logs, configuration examples, fixtures, or Git.
- The older app-login/device-registration flow is out of scope because it can
  displace the user's mobile app session.
- Keep databases, downloaded PDFs, raw exports, credentials, and audit data in
  the operator's external data directory. Never add them to the repository.
- Store operator-specific instructions in `AGENTS.override.md`; this file is intentionally ignored by Git.
- Synchronization is read-only at the broker. Remote order placement is not
  implemented. Do not add a live execution path without a separate review of
  the deterministic risk engine, typed approval challenge, idempotency,
  freshness checks, audit chain, and kill switch.
- Research providers may only produce structured research. They must not
  import or call the execution package.
- Preserve ISIN as the canonical instrument identifier. Tickers and names are
  aliases only.
- Import financial decimals without binary floating-point arithmetic. Keep
  imports idempotent and retain source provenance.
- Use dry-run before persisting imports when inspecting unfamiliar exports.
- Real-account tests are opt-in and must never run in the default test suite.

## Build And Test

```bash
cd $HOME/printing-press/library/trade-republic-pp
go fmt ./...
go test ./...
go vet ./...
go build -o ./bin/tr ./cmd/tr
```

