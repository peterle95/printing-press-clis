# `pytr` adapter contract

The adapter targets the documented `pytr` 0.4.10 command surface. It uses only
read-only commands and normalizes their file outputs immediately.

Portfolio synchronization invokes the equivalent of:

```text
pytr portfolio --lang en --no-decimal-localization --output <private>/portfolio.csv
```

The adapter strictly parses the semicolon columns `Name`, `ISIN`, `quantity`,
`price`, `avgCost`, and `netValue`. The file does not contain a cash balance,
and `tr` does not scrape one from human-readable stdout.

Timeline synchronization invokes the equivalent of:

```text
pytr export_transactions --lang en --date-with-time \
  --no-decimal-localization --sort --export-format json \
  --outputdir <private>
```

`account_transactions.json` is JSON Lines. `pytr` removes timezone information
from exported timestamps, so the configured `account_timezone` is attached and
recorded as an import assumption before conversion to UTC. `--since` becomes a
conservative `--last_days` request, followed by an exact local time filter and
transactional deduplication.

With `--documents`, the adapter uses `dl_docs` in the configured private
documents directory and ingests its transaction export plus PDF metadata.

The adapter rejects missing columns, malformed decimals, bad ISINs, oversized
files, duplicate inconsistent records, and command failures. Diagnostics are
bounded and redacted. It never enables `pytr` debug logging.

`pytr` is unofficial, uses a private API, and can break when Trade Republic
changes its service. A failed adapter leaves the previous SQLite snapshot
intact and records a failed sync run without sensitive process output.
