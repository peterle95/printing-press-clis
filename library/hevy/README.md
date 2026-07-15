# Hevy Printing Press CLI

A local-first Go CLI for Hevy CSV exports and portable workout plans. It does not
call private Hevy APIs. Browser access uses a persistent Playwright profile and
visible public UI only.

## Build

```bash
go run ./cmd/hevy --help
go build -o ./bin/hevy ./cmd/hevy
```

## Basic workflow

```bash
hevy sync ~/Downloads/hevy-export.csv
hevy plans import ./push-pull-legs.yaml
hevy plans validate ./push-pull-legs.yaml
hevy login
hevy auth status
hevy routines apply "Push Pull Legs" --dry-run
```

See `docs/` for CSV, plans, browser, and routine safety details.

