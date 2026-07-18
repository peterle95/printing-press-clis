# Security model

## Current capability boundary

The Go binary can read local files, write its local SQLite database, run the
configured `pytr` process, and run `pdftotext`. It has no Trade Republic order
adapter and exposes no live `order execute` command.

The `pytr` command is invoked directly with a fixed argument vector. No shell is
involved, and user research text never becomes an argument. Interactive login
inherits the terminal so that second-factor values go directly to `pytr`.
Those values are not accepted, recorded, or logged by `tr`.

## Private data

SQLite, statement PDFs, normalized exports, and research reports can contain
sensitive financial or personal data. Runtime directories are created with
mode `0700`, configuration and databases with mode `0600`, and public-tree
audits reject database and session artifacts. SQLite is not encrypted; use
full-disk or directory encryption when that threat matters.

PDF extraction never stores extracted full text. The importer retains a hash,
vetted local path, recognized metadata, parser version, and review status.
Unknown, encrypted, scanned, oversized, or unrecognized statements fail closed.

## Execution design

The paper workflow is:

```text
structured research
        ↓
deterministic risk evaluation
        ↓
short-lived preview + canonical hash
        ↓
preview-specific typed approval
        ↓
paper export only
```

The engine checks an exact ISIN whitelist, limit order, per-order and per-day
caps, current reserved exposure, current price and balance age, buy balance,
idempotency, preview expiry, paper mode, and the global kill switch. Audit
events form a SHA-256 hash chain. This is tamper-evident local logging, not
compliance-grade immutable storage.

Adding live execution requires a separate security review and ideally a
separate process with separate credentials. It must re-check every control
immediately before submission, persist a `submitting` state before the network
call, treat timeouts as unknown rather than retrying, and reconcile against the
broker before another attempt.

