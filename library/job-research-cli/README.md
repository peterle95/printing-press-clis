# job-research-cli

`job-research-cli` is a safe Printing Press CLI for job research. It searches
API-first sources, deduplicates postings, stores structured results in SQLite,
exports Markdown/CSV/JSON/SQLite, and generates manual search links for
restricted job boards instead of scraping them.

The default research profile is tuned for Berlin/Germany software roles:
frontend, full-stack, React, JavaScript, TypeScript, web/software engineering,
and related recruiting roles across Berlin, Germany, Remote Germany, and Remote
Europe.

## What It Does

- Searches compliant public/API-backed sources through adapters.
- Generates direct manual search URLs for restricted sites.
- Deduplicates by canonical URL, then company + normalized title + location,
  then source-specific ID.
- Stores structured job postings in SQLite.
- Exports Markdown, CSV, JSON, or a copy of the SQLite database.
- Keeps going when one source fails and reports source errors at the end.
- Uses timeouts, retries with exponential backoff, per-source cooldowns, and
  conservative rate limits.

## What It Does Not Do

- It does not bypass captchas.
- It does not use proxy rotation to evade bans.
- It does not automate login sessions.
- It does not scrape logged-in LinkedIn, XING, Indeed, StepStone, Glassdoor,
  Kununu, Monster, or similar protected pages.
- It does not hammer websites.

Restricted boards run in `manual_search_link` mode: the CLI creates URLs that
you can open manually, but it does not parse those pages. This keeps job
research useful without risking account or IP bans.

## Install

Use Python 3.11+.

```bash
cd $HOME/printing-press/library/job-research-cli
python -m pip install -e .
```

For development:

```bash
python -m pip install -e ".[dev]"
python -m pytest
```

The console entry point is:

```bash
jobs --help
```

## Initialize Config

```bash
jobs init
```

This creates user config outside the repository:

```text
Windows:     %APPDATA%/printing-press/job-research/
Linux/macOS: ~/.config/printing-press/job-research/
```

Runtime data is stored in SQLite outside the repository:

```text
Windows:     %LOCALAPPDATA%/printing-press/job-research/jobs.db
Linux/macOS: ~/.local/share/printing-press/job-research/jobs.db
```

`jobs init` can optionally store Adzuna credentials in the user config `.env`
file. You can also set them in your shell:

```bash
export ADZUNA_APP_ID="..."
export ADZUNA_APP_KEY="..."
```

Bundesagentur defaults to the public `jobboerse-jobsuche` API key. Override it
only if the API changes:

```bash
export JOB_RESEARCH_BUNDESAGENTUR_API_KEY="jobboerse-jobsuche"
```

## Sources

API adapters:

| Source | Default | Notes |
|---|---:|---|
| `bundesagentur` | enabled | Bundesagentur Jobsuche public API. |
| `arbeitnow` | enabled | Public Arbeitnow job board API; locally filtered. |
| `adzuna` | disabled | Requires `ADZUNA_APP_ID` and `ADZUNA_APP_KEY`. |
| `themuse` | enabled | Public The Muse jobs endpoint; locally filtered. |
| `remoteok` | disabled | Public JSON endpoint if you choose to enable it. |
| `greenhouse` | disabled | Public Greenhouse Job Board API for configured board tokens. |
| `lever` | disabled | Public Lever postings API for configured company slugs. |

Manual/search-link sources:

| Source | Mode |
|---|---|
| `linkedin` | `manual_search_link` |
| `xing` | `manual_search_link` |
| `indeed` | `manual_search_link` |
| `stepstone` | `manual_search_link` |
| `glassdoor` | `manual_search_link` |
| `monster` | `manual_search_link` |
| `google_jobs` | `manual_search_link` |
| `kununu` | `manual_search_link` |
| `wellfound` | disabled `manual_search_link` |
| `github_jobs` | disabled `manual_search_link`; no current official GitHub Jobs API is configured |

To enable Greenhouse or Lever, add company board identifiers to your user
`sources.yaml`:

```yaml
sources:
  greenhouse:
    enabled: true
    board_tokens:
      - examplecompany
  lever:
    enabled: true
    company_slugs:
      - examplecompany
```

## Search Examples

Preview what would be queried without making requests:

```bash
jobs search --title "frontend developer" --location Berlin --days 7 --dry-run
```

Search Berlin frontend jobs and print a table:

```bash
jobs search --title "frontend developer" --location Berlin --days 7 --format table
```

Use an isolated SQLite database for a one-off run:

```bash
jobs search --title "frontend developer" --location Berlin --db /tmp/job-search.db
```

Use the default title list, prioritize remote jobs in Germany, and write
Markdown:

```bash
jobs search --titles config/titles.yaml --location Germany --remote --out results/jobs.md
```

Search selected API sources and export CSV:

```bash
jobs search --source bundesagentur,arbeitnow,adzuna --title "React Developer" --format csv --out jobs.csv
```

Open the latest stored structured job links:

```bash
jobs open --latest 10
```

Deduplicate old result files:

```bash
jobs dedupe results/jobs.md --out results/jobs-deduped.md
```

Export stored results:

```bash
jobs export --format markdown --out results/latest.md
jobs export --format csv --out results/latest.csv
jobs export --format json --out results/latest.json
jobs export --format sqlite --out results/jobs.db
```

## Output

Terminal table:

```text
| # | Title | Company | Location | Posted | Website | Link |
|---|-------|---------|----------|--------|---------|------|
```

Markdown exports include:

- `# Job Research Results`
- Search parameters
- `## Structured results`
- `## Manual search links`
- `## Source errors` when a source fails

Manual links are included in CSV/JSON exports with
`source_type=manual_search_link`.

## Config Files

Project example files live in:

```text
config/sources.yaml
config/titles.yaml
```

`jobs init` writes user-editable copies to the config directory. Edit those
copies for real use so secrets and personal preferences stay outside the repo.

## Development

```bash
python -m pip install -e ".[dev]"
python -m pytest
jobs search --title "frontend developer" --location Berlin --dry-run
```

All tests avoid live network calls.
