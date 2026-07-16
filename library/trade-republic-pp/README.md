# Trade Republic Printing Press CLI

`tr` is a local-first Go CLI for normalizing Trade Republic portfolio exports,
timeline activity, and statement documents into SQLite. It keeps broker access,
research, reporting, and future order execution behind separate boundaries.

This project is not affiliated with Trade Republic Bank GmbH. The optional
live adapter invokes the unofficial [`pytr`](https://github.com/pytr-org/pytr)
CLI; it does not reproduce Trade Republic's private protocol in Go.

## What ships in v0.1

- Optional interactive web login and read-only synchronization through `pytr`.
- Deterministic offline synchronization through versioned FinanceJSON.
- SQLite storage keyed by ISIN with idempotent transaction and document imports.
- Local portfolio, allocation, ledger, dividend, fee, tax, and deliberately
  limited P&L reports.
- Citation-bearing ETF and company research imported through FinanceJSON.
- A deterministic, paper-only order preview, typed approval, and export flow.
- No live order submission endpoint.

The CLI never sends a broker order. `order preview`, `order approve`, and
`order export` create auditable local artifacts only.

## Build

```bash
go test ./...
go build -o ./bin/tr ./cmd/tr
./bin/tr --help
```

Runtime data defaults to the platform data directory, under
`printing-press/trade-republic`. Databases, PDFs, staging exports, credentials,
and audit records do not belong in this repository.

## Optional dependencies

- `pytr` 0.4.10 for live, read-only synchronization. Install and pin it outside
  this Go module; configure `pytr_command` if the executable is not `pytr`.
- Poppler's `pdftotext` for statement text extraction. Unknown or low-confidence
  statement layouts are retained as documents and marked for review.

Run `tr doctor` to see which optional dependencies are available.

## Safe first run

```bash
tr config init
tr doctor
tr sync --provider financejson --input ./example.finance.json --dry-run
tr sync --provider financejson --input ./example.finance.json
tr portfolio
```

For a live read-only sync:

```bash
tr auth login
tr sync --dry-run
tr sync
tr sync --documents --since 2026-01-01
```

`tr auth login` delegates the interactive four-digit web-login challenge to
`pytr`. `tr` deliberately has no phone-number, PIN, OTP, cookie, or WAF-token
flags, and it does not place those values in SQLite or its audit log.

## Portfolio and reports

```bash
tr portfolio
tr portfolio --json
tr position IE00B4L5Y983
tr allocation --group sector
tr report daily
tr report monthly
tr report pnl --period ytd
tr report dividends --year 2026
tr report fees
tr report taxes
```

All monetary JSON values are decimal strings. The CLI refuses to aggregate
different currencies without an explicit FX history. P&L currently reports
cost basis and unrealized P&L only; realized P&L remains unavailable until lot
matching, corporate actions, and timestamped FX conversion are complete.

## Research

```bash
tr research import ./research.finance.json
tr search "MSCI World"
tr research ASML
tr research --isin IE00B4L5Y983
tr compare VWCE EUNL SPYI
tr news ASML
```

FinanceJSON research reports must include retrieval dates and source citations.
ETF reports cover fund identity, index, TER, size, domicile, replication,
distribution policy, currencies, holdings, country/sector exposure, tracking
difference, liquidity, spread, and current-portfolio overlap. Company reports
cover the requested business, financial, valuation, filing, activity, risk,
catalyst, and portfolio-exposure fields.

Research is read-only data. The research package cannot import the execution
package and cannot create an executable order.

## Paper-only order workflow

```bash
tr order preview --buy IE00B4L5Y983 --amount 100 --limit-price 98.50
tr order approve <preview-id>
tr order export <preview-id> --format json
```

The risk policy defaults to a global kill switch, paper trading, an empty ISIN
whitelist, limit orders, value caps, and short price/balance freshness windows.
Approval requires typing a preview-specific phrase, not `y`. A typed phrase is
still not a separate trust domain; any future live executor must use an
OS-mediated or cryptographically signed approval outside the research agent.

See [docs/security.md](docs/security.md), [docs/financejson.md](docs/financejson.md),
and [docs/pytr.md](docs/pytr.md) for the data and safety contracts.

## License

Apache-2.0, under the repository's root `LICENSE`.
