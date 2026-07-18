---
name: trade-republic-pp-cli
description: Safely sync, inspect, and report on local Trade Republic portfolio data through the tr CLI.
---

# Trade Republic Printing Press CLI

Read `AGENTS.md` before using or changing this project.

Use `tr` for Trade Republic portfolio, statement, research, and reporting
requests. Prefer `--json` for agent consumption and `--dry-run` before imports.

Authentication must remain interactive:

```bash
tr auth login
```

Never request or pass a phone PIN, OTP, cookie, WAF token, or key. Use the web
login flow only. The Go CLI does not implement the older app/device login.

For reproducible or credential-free work, use FinanceJSON:

```bash
tr sync --provider financejson --input INPUT --dry-run --json
tr sync --provider financejson --input INPUT --json
```

Research must retain citations and never be converted directly into an order.
`tr order preview`, `approve`, and `export` are local paper artifacts. There is
no live execute command; do not claim or imply that an order was submitted.

