# Trello Calendar Printing Press CLI

`trello-calendar-pp-cli` reads open cards from a configured Trello weekly-planner list and schedules as many as possible into the next calendar week without overloading or overlapping Google Calendar days. It also retains the complete generated Trello REST command surface and ships a companion MCP server.

## Features

- Go 1.26.5 or newer, Cobra, strict typed decoding, table/JSON/agent output, retries, and typed exit behavior.
- Full generated Trello REST surface from the archived official OpenAPI schema.
- Secure Google OAuth 2.0 PKCE loopback login and permission-restricted refresh-token storage.
- Monday-to-Sunday planning in `Europe/Berlin`, weekdays by default.
- Counts timed and all-day events, ignores cancelled events, and never overlaps an active event.
- At most one Trello card per day and no day that already has the configured event maximum.
- Due-date-first card ordering followed by Trello list position.
- Deterministic Google event IDs and private extended properties for cross-run idempotency.
- Complete planning before mutation, confirmation before writes, and write-free previews/dry-runs.
- Optional Trello comment after successful scheduling.
- Human CLI and MCP surfaces; `cards` and `preview` are annotated read-only.

## Install

```bash
cd ~/printing-press/library/trello-calendar
go install ./cmd/trello-calendar-pp-cli
go install ./cmd/trello-calendar-pp-mcp
```

Or build local binaries:

```bash
make build-all
./bin/trello-calendar-pp-cli --help
```

## Trello credentials

1. Create or select a Trello Power-Up at <https://trello.com/power-ups/admin>.
2. Generate an API key and a user token with permission to read the planner and add comments if `--comment-on-card` will be used.
3. Find the board ID and the ID of its `Doing` list.
4. Export the credentials and IDs:

```bash
export TRELLO_API_KEY='...'
export TRELLO_TOKEN='...'
export TRELLO_BOARD_ID='...'
export TRELLO_LIST_ID='...'
```

The key and token are sent in Trello's OAuth `Authorization` header. They are never stored in `config.toml` or printed.

## Google Cloud and OAuth setup

1. Create a project in the [Google Cloud Console](https://console.cloud.google.com/).
2. Enable **Google Calendar API** under APIs & Services.
3. Configure the OAuth consent screen. For an external app in testing, add the operator account as a test user.
4. Create an OAuth 2.0 **Desktop app** client.
5. Register a loopback redirect such as `http://127.0.0.1:8765/oauth2/callback` if the client configuration requires an explicit URI.
6. Export:

```bash
export GOOGLE_CLIENT_ID='...apps.googleusercontent.com'
export GOOGLE_CLIENT_SECRET='...'
export GOOGLE_REDIRECT_URI='http://127.0.0.1:8765/oauth2/callback'
export GOOGLE_CALENDAR_ID='primary'
```

Complete the initial login:

```bash
trello-calendar-pp-cli auth google
# WSL/headless:
trello-calendar-pp-cli auth google --no-browser
```

The command always prints the authorization URL. It requests only `calendar.events`, requires a refresh token, and writes it atomically to:

```text
~/.config/trello-calendar-pp-cli/google-token.json
```

The directory is `0700` and the file is `0600` on Unix-like systems.

## Configuration

The default file is `~/.config/trello-calendar-pp-cli/config.toml`. Copy [`config.example.toml`](config.example.toml) and edit only non-secret settings:

```toml
timezone = "Europe/Berlin"
trello_board_id = "BOARD_ID"
trello_list_id = "LIST_ID"
google_calendar_id = "primary"
duration_minutes = 60
preferred_time = "10:00"
day_start = "09:00"
day_end = "18:00"
max_events_per_day = 3
include_weekends = false
title_prefix = ""
```

Precedence is command flags, then environment IDs, then TOML, then defaults. Unknown TOML keys and invalid scheduling windows are rejected. Credentials remain in environment variables and the Google token file.

## Commands

```bash
trello-calendar-pp-cli auth google
trello-calendar-pp-cli cards
trello-calendar-pp-cli preview
trello-calendar-pp-cli schedule
trello-calendar-pp-cli schedule --yes
trello-calendar-pp-cli schedule --dry-run
trello-calendar-pp-cli doctor
```

The generated Trello API commands remain below their resource groups:

```bash
trello-calendar-pp-cli cards get-id --help
trello-calendar-pp-cli boards --help
trello-calendar-pp-cli which "get a Trello card" --json
```

### Planner options

`preview` and `schedule` accept:

```text
--duration 60
--day-start 09:00
--day-end 18:00
--preferred-time 10:00
--include-weekends
--max-events-per-day 3
--title-prefix "[Trello] "
```

`schedule` additionally accepts `--comment-on-card`. Global Printing Press flags include `--json`, `--agent`, `--yes`, `--no-input`, `--verbose`, `--timeout`, and `--rate-limit`.

Examples:

```bash
trello-calendar-pp-cli cards --json
trello-calendar-pp-cli preview --include-weekends
trello-calendar-pp-cli schedule --duration 90 --title-prefix '[Trello] '
trello-calendar-pp-cli schedule --yes --comment-on-card
trello-calendar-pp-cli schedule --dry-run --json
trello-calendar-pp-cli schedule --agent --dry-run
```

Live `schedule --json` requires `--yes` so stdout remains one valid JSON document. Logs and verbose diagnostics go to stderr.

## Planning and dry-run behavior

The next week is the next Monday after today through the following Sunday. A run on Sunday, 2026-07-12 therefore plans 2026-07-13 through 2026-07-19; a run on Monday plans the following Monday.

The planner:

1. Validates the configured board/list and fetches open list cards.
2. Marks cards already represented in the target Calendar.
3. Sorts due cards by due instant, then undated cards by Trello position.
4. Expands and groups all non-cancelled Calendar events by local day.
5. Tries the preferred time, then 30-minute increments between the day boundaries.
6. Assigns at most one card to each eligible day and validates the complete plan.
7. On a confirmed live run, rechecks the duplicate, capacity, source event, and exact slot immediately before each insert.

All-day events block each covered date. Timed events block regardless of Calendar transparency. Boundary-touching events do not overlap.

`preview` and `schedule --dry-run` perform the same live GET requests and return the same plan. They never insert Calendar events, add Trello comments, or save refreshed OAuth tokens.

## Duplicate prevention

Each inserted event receives a deterministic ID based on calendar, board, and card IDs plus:

```json
{
  "private": {
    "trelloCardId": "CARD_ID",
    "trelloBoardId": "BOARD_ID",
    "source": "trello-calendar-cli"
  }
}
```

The CLI checks both the deterministic ID and `trelloCardId` extended property. An active, cancelled, deleted, or tombstoned deterministic event is treated as already scheduled. HTTP `409` insertion conflicts are reconciled instead of duplicated. Existing active source events also enforce one Trello card per day across repeated runs.

## Calendar event format

The card name is the title unless `--title-prefix` is supplied. The description contains the card URL and ID, followed by labels and due date when present:

```text
Scheduled from Trello.

Card: https://trello.com/c/example
Trello card ID: abc123
Labels: backend, priority
Due date: 2026-07-15
```

## Doctor and troubleshooting

```bash
trello-calendar-pp-cli doctor
trello-calendar-pp-cli doctor --json
trello-calendar-pp-cli doctor --fail-on error
```

Doctor independently checks configuration, IDs, timezone data, Trello environment credentials, board/list access, Google OAuth inputs, token permissions/refresh, and Calendar writer access.

- **No Google refresh token:** run `auth google`; if Google omits it, revoke the app's existing consent and authenticate again.
- **OAuth callback timeout:** confirm `GOOGLE_REDIRECT_URI` is loopback HTTP, its port is free, and the browser uses the printed URL.
- **List not named Doing:** the configured list ID remains authoritative; doctor emits a warning.
- **Rate limits or temporary failures:** requests retry three times with exponential backoff and `Retry-After` support.
- **Partial schedule:** successful events remain; failures are summarized and the process exits nonzero. Re-run safely—duplicate checks prevent repeated events.
- **No slot:** inspect all-day events and the configured day window/duration.

## Security

Never commit `.env`, the OAuth token, normal config containing personal IDs, exported Calendar/Trello data, or browser/session material. Redirect stdout and stderr carefully in automation because diagnostics may contain card names and URLs, though never credentials. See [`SECURITY.md`](SECURITY.md).

## Development

```bash
make fmt
make test
make test-race
make vet
make lint
make build-all
```

Live read-only integration checks are opt-in:

```bash
TRELLO_CALENDAR_INTEGRATION_READ=1 go test -tags=integration ./integration
```

Default tests mock every external API. No real Calendar writes occur in the integration layer.

## Project layout

```text
cmd/
  trello-calendar-pp-cli/
  trello-calendar-pp-mcp/
internal/
  cli/                 Cobra commands and output
  client/              generated Trello HTTP client
  config/              strict TOML/environment configuration
  googlecalendar/      OAuth, token storage, Calendar REST adapter
  scheduling/          pure week, ordering, capacity, and slot logic
  trello/              typed planner adapter over the generated client
  workflow/            read/plan/confirm/execute orchestration
integration/           opt-in real-credential read checks
spec.json              normalized archived official Trello OpenAPI
```

## Assumptions and future work

- The list ID is authoritative; a non-`Doing` name warns but does not block.
- `dueComplete` does not exclude an otherwise open card.
- Cancelled events do not consume capacity, but a cancelled matching event permanently prevents rescheduling that card.
- Cards are not moved or archived. The workflow interface leaves room for a future post-scheduling action.
- A future week policy may support different week starts or rolling windows without changing API adapters.
