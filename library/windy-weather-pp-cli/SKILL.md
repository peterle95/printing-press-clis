# Windy Weather CLI

Use `windy-weather-pp-cli` when an agent needs a stable, machine-readable rain
or weather-risk check near the configured Berlin home area before deciding
whether to call another tool such as Google Calendar.

## Commands

```bash
windy-weather-pp-cli now
windy-weather-pp-cli now --json
windy-weather-pp-cli rain --hours 6
windy-weather-pp-cli rain --hours 6 --json
windy-weather-pp-cli today
windy-weather-pp-cli tomorrow
windy-weather-pp-cli calendar-advice --json
windy-weather-pp-cli debug-network
windy-weather-pp-cli debug-screenshot
```

Use `--agent` when another agent is calling the CLI. It implies JSON output.
Use `--no-cache` only when fresh Windy data is explicitly needed.

## Calendar Flow

Call:

```bash
windy-weather-pp-cli calendar-advice --json
```

If `should_create_event` is true, another agent may create the event with the
Google Calendar CLI. This CLI never writes to calendars itself.
