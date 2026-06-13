# windy-weather-pp-cli

`windy-weather-pp-cli` checks rain and weather risk near a configured location using Windy.com. It opens the public Windy page with
Playwright, records relevant network responses, fetches the public point
forecast JSON that the page uses, and normalizes the result into stable output
for agents.

The CLI does not use paid APIs, does not log cookies or headers, and does not
create Google Calendar events. It only returns calendar advice JSON.

## Location

- Name: Berlin city center
- Latitude: `52.520`
- Longitude: `13.405`
- Timezone: `Europe/Berlin`
- Windy URL: `https://www.windy.com/-Rain-thunder-rain?rain,52.520,13.405,10,p:cities,m:e6Fagxs`

## Build

```bash
cd $HOME/printing-press/library/windy-weather-pp-cli
npm install
npx playwright install chromium
go test ./...
go build -o ./bin/windy-weather-pp-cli ./cmd/windy-weather-pp-cli
```

Install into the WSL Go bin directory:

```bash
install -m 0755 ./bin/windy-weather-pp-cli $HOME/go/bin/windy-weather-pp-cli
```

## Configuration

The default config is created at:

```text
~/.config/windy-weather-pp-cli/config.json
```

You can also pass `--config /path/to/config.json` or set
`WINDY_WEATHER_CONFIG`. See `config.example.json`.

Default cache TTL is 20 minutes. Use `--no-cache` for a fresh Windy run.

## Examples

```bash
windy-weather-pp-cli now
windy-weather-pp-cli now --json
windy-weather-pp-cli rain --hours 6
windy-weather-pp-cli rain --hours 6 --json
windy-weather-pp-cli today
windy-weather-pp-cli tomorrow
windy-weather-pp-cli calendar-advice --json
```

Agent mode:

```bash
windy-weather-pp-cli --agent calendar-advice
```

## Debugging

Capture relevant network metadata:

```bash
windy-weather-pp-cli debug-network
```

This writes:

```text
debug/windy-network-log.json
```

The debug log includes sanitized request URLs, method, status, content type,
response size, and short previews for JSON/text responses. It does not include
cookies, tokens, auth headers, or raw browser headers.

Save a troubleshooting screenshot:

```bash
windy-weather-pp-cli debug-screenshot
windy-weather-pp-cli debug-screenshot --output debug/windy-home.png
```

## Weather Data Notes

Windy currently exposes useful public point data at endpoints shaped like:

```text
https://node.windy.com/forecast/point/now/ecmwf/v1.0/{lat}/{lon}?refTime={YYYYMMDDHH}
https://node.windy.com/forecast/point/ecmwf/v2.9/{lat}/{lon}?refTime={YYYYMMDDHH}&step=3&interpolate=true
```

The `refTime` is discovered from the public ECMWF-HRES minifest loaded by the
Windy page. Values are model data. Exact precipitation probability is not
available, so rain and thunderstorm risk are best-effort low/medium/high
classifications with a confidence field and notes in `raw_observations`.

## Calendar Advice

`calendar-advice --json` returns a decision object for another agent:

```json
{
  "should_create_event": true,
  "event_type": "weather_reminder",
  "title": "Check rain before going out",
  "suggested_time": "2026-05-20T16:00:00+02:00",
  "reason": "Windy rain layer indicates medium or higher rain risk near home in the next configured period.",
  "confidence": "medium",
  "source": "windy.com",
  "weather": {
    "rain_risk": "medium",
    "thunderstorm_risk": "low",
    "temperature_c": null,
    "wind_speed_kmh": null
  }
}
```

Decision logic:

- Medium or high rain risk recommends a weather reminder.
- Medium or high thunderstorm risk recommends a stronger weather warning.
- Strong wind at or above the configured threshold recommends a wind warning.
- Otherwise `should_create_event` is false.

## Error Handling

JSON output stays predictable. When Windy changes internals or the browser fails,
the CLI returns null values where needed, low confidence, and an `errors` array
with codes such as `network_timeout`, `browser_dependency_missing`,
`no_usable_weather_endpoint`, or `malformed_response`.
