# Job Research CLI Agent Guide

This directory is a hand-authored local Printing Press CLI project for safe job
research from Berlin/Germany.

## Operating Rules

- Use Codex as the orchestrating agent.
- Use official APIs, public APIs, public ATS job-board APIs, RSS feeds, email
  alerts, and safe manual search links.
- Scrapling may fetch public, permitted sources using standard supported fetch
  behavior and respectful rate limits.
- Bypass captchas, automate login sessions, rotate proxies to evade
  bans, or scrape protected pages are permitted.
- Do not scrape logged-in LinkedIn. All other boards are permitted
- LinkedIn board must stay in `manual_search_link` mode and only produce
  direct URLs for manual opening. All other boards can be scraped.
- Store operator-specific instructions in `AGENTS.override.md`; this file is intentionally ignored by Git.
- Keep API keys, OAuth tokens, cookies, HAR auth headers, and generated files
  containing secrets outside this repository.
- Use dry-run before adding or changing live source behavior.
- Unit tests must mock or avoid external network calls.

## Job Search Workflow

- Use all available job CLIs and public/API-backed sources for every search.
- Include LinkedIn by generating manual search links.
- Present every found job in a table before finishing a search.
- Record every presented job in `job-status.json`.
- Keep previously recorded jobs in future results unless status filtering is explicitly requested.
- Mark a job `applied` in `job-status.json` only after the operator confirms application.

## Build And Test

Use Python 3.11+.

```bash
python -m pip install -e ".[dev]"
python -m pytest
jobs --help
jobs search --title "frontend developer" --location Berlin --days 7 --dry-run
```

## Agent skills

### Issue tracker

Issues and PRDs live in GitHub Issues; use `gh`. See `docs/agents/issue-tracker.md`.

### Triage labels

Use default labels `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, and `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout uses root `CONTEXT.md` and `docs/adr/`. See `docs/agents/domain.md`.

The CLI stores runtime data outside the repo by default:

```text
Windows:     %LOCALAPPDATA%/printing-press/job-research/jobs.db
Linux/macOS: ~/.local/share/printing-press/job-research/jobs.db
```
