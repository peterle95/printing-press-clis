# Job Research CLI Agent Guide

This directory is a hand-authored local Printing Press CLI project for safe job
research from Berlin/Germany.

## Operating Rules

- Use Codex as the orchestrating agent.
- Use official APIs, public APIs, public ATS job-board APIs, RSS feeds, email
  alerts, and safe manual search links.
- Do not bypass captchas, automate login sessions, rotate proxies to evade
  bans, or scrape protected pages.
- Do not scrape logged-in LinkedIn, XING, Indeed, StepStone, Glassdoor, Monster,
  Kununu, or similar pages unless a future adapter documents explicit permission.
- Restricted job boards must stay in `manual_search_link` mode and only produce
  direct URLs for manual opening.
- Keep API keys, OAuth tokens, cookies, HAR auth headers, and generated files
  containing secrets outside this repository.
- Use dry-run before adding or changing live source behavior.
- Unit tests must mock or avoid external network calls.

## Build And Test

Use Python 3.11+.

```bash
python -m pip install -e ".[dev]"
python -m pytest
jobs --help
jobs search --title "frontend developer" --location Berlin --days 7 --dry-run
```

The CLI stores runtime data outside the repo by default:

```text
Windows:     %LOCALAPPDATA%/printing-press/job-research/jobs.db
Linux/macOS: ~/.local/share/printing-press/job-research/jobs.db
```
