# Transit Printed CLI

Use `transit-pp-cli` for Berlin VBB/BVG real-time departures, nearby stops,
journey planning, latest leave time, vehicle radar, and trip inspection.

## Common Commands

```bash
transit-pp-cli config init
transit-pp-cli config set-home --address "Examplestr. 1, 10115 Berlin"
transit-pp-cli config set-home --lat 52.520000 --lon 13.405000 --label home
transit-pp-cli nearby --from home --radius 1000
transit-pp-cli board --from home --minutes 20
transit-pp-cli board --stop "Berlin Hauptbahnhof" --minutes 20
transit-pp-cli watch --from home --minutes 20 --every 30s
transit-pp-cli route --from home --to "Berlin Hauptbahnhof" --arrive-by "09:30"
transit-pp-cli leave --to "Berlin Hauptbahnhof" --arrive-by "09:30" --buffer 7
transit-pp-cli radar --from home --radius 1500
transit-pp-cli trip --id "<tripId>"
```

Use `--json` for machine-readable output and `--debug` to print request URLs and
raw provider errors to stderr.

Home config lives outside the repo at:

```text
~/.config/printing-press/transit/config.yaml
```
