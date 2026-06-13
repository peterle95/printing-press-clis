# transit-pp-cli

`transit-pp-cli` is a local Printing Press CLI for Berlin VBB/BVG transit. It
uses `https://v6.vbb.transport.rest` to show nearby U-Bahn, S-Bahn, bus, tram,
and regional departures, plan routes, calculate when to leave home, and
optionally show moving vehicles near a configured origin.

## Setup

Build from this workspace:

```bash
cd $HOME/printing-press/transit
go fmt ./...
go test ./...
go build -o ./bin/transit-pp-cli ./cmd/transit-pp-cli
```

Create local config:

```bash
transit-pp-cli config init
transit-pp-cli config set-home --address "Examplestr. 1, 10115 Berlin"
```

The config is stored outside the repo:

```text
~/.config/printing-press/transit/config.yaml
```

If an address is present but coordinates are missing, the first command that
needs home calls `/locations` and saves the selected latitude and longitude
locally. When VBB returns multiple address matches, interactive runs ask which
one to use; non-interactive runs can pass `--yes` to accept the first match.

## Examples

```bash
transit-pp-cli nearby --from home --radius 1000
transit-pp-cli board --from home --minutes 20
transit-pp-cli board --stop "Berlin Hauptbahnhof" --minutes 20
transit-pp-cli board --from home --minutes 20 --subway --line U7
transit-pp-cli watch --from home --minutes 20 --every 30s
transit-pp-cli route --from home --to "Berlin Hauptbahnhof" --arrive-by "09:30"
transit-pp-cli leave --to "Berlin Hauptbahnhof" --arrive-by "09:30" --buffer 7
transit-pp-cli radar --from home --radius 1500
transit-pp-cli trip --id "<tripId>"
```

All commands support `--json`. Use `--debug` to print request URLs and provider
errors to stderr.

## Board Output

```text
Departures near home, next 20 min

STATUS      LINE  MODE  DEPARTS  DELAY  MIN  STOP              DIRECTION          PLAT  REMARKS
LEAVE NOW   M41   bus   08:42     +2m    4    Erkstr.           Hauptbahnhof       -     BVG
CATCHABLE   U7    U     08:48     0m     10   Rathaus Neukoelln Rathaus Spandau    2     Bicycle conveyance
```

Departures that are physically impossible to catch from home are hidden by
default. Pass `--show-too-late` to include them. The watch command includes all
statuses so `TOO LATE` remains visible while refreshing.

## Config

See `config.example.yaml` for a fake-address template. The default config:

```yaml
provider: vbb_transport_rest
base_url: https://v6.vbb.transport.rest
home:
  label: home
  address: ""
  latitude: null
  longitude: null
defaults:
  radius_meters: 1000
  departure_window_minutes: 20
  refresh_seconds: 30
  walking_speed: normal
  safety_buffer_minutes: 5
  modes:
    suburban: true
    subway: true
    tram: true
    bus: true
    ferry: false
    express: false
    regional: true
```

## Reliability

- HTTP timeout defaults to 10 seconds.
- Transient `429` and `5xx` responses are retried with exponential backoff.
- A process-local limiter keeps requests under 100 per minute.
- Nearby stops are cached for 24 hours.
- Address geocoding is cached for 30 days.
- Live departures are cached for at most 20 seconds.
- Human-readable times are shown in `Europe/Berlin`.
