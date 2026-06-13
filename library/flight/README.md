# Flight Printing Press CLI

`flight-pp-cli` compares flight options across provider adapters and normalizes
them into one result table. The MVP uses official APIs, affiliate/third-party
APIs, and safe deep links only.

## What It Does

- Searches configured providers with a common request model.
- Normalizes price, timing, duration, stops, baggage notes, warnings, and risk
  flags.
- Keeps going when one provider fails and prints provider errors separately.
- Ranks results by `price`, `duration`, or `best`.
- Caches provider responses for a short TTL.
- Saves local price-watch searches without requiring a daemon.

## What It Does Not Do

- It does not scrape Google Flights, Skyscanner, Expedia, KAYAK, airline sites,
  or booking pages.
- It does not implement bot-detection evasion, rotating proxies, captcha bypass,
  or account abuse.
- It does not purchase tickets or guarantee that a price is still available.
- It does not make deep-link-only providers comparable unless an API returns
  price data.

Flight prices change quickly. Always verify the final price, baggage rules,
fare restrictions, and booking page before purchase.

## Config

Create the default config:

```bash
flight-pp-cli config init
```

Config paths:

```text
Windows:     %APPDATA%/printing-press/flight/config.yaml
Linux/macOS: ~/.config/printing-press/flight/config.yaml
```

Cache paths:

```text
Windows:     %LOCALAPPDATA%/printing-press/flight/cache
Linux/macOS: ~/.cache/printing-press/flight
```

Default config:

```yaml
providers:
  amadeus:
    enabled: true
    client_id: ""
    client_secret: ""
    environment: "test"
  kiwi:
    enabled: false
    api_key: ""
  google:
    enabled: true
    mode: "deeplink"
    serpapi_key: ""
    searchapi_key: ""
  skyscanner:
    enabled: false
    api_key: ""
  expedia:
    enabled: false
    api_key: ""
    mode: "deeplink"
  kayak:
    enabled: false
    api_key: ""
    mode: "deeplink"
  travelpayouts:
    enabled: false
    token: ""
defaults:
  currency: "EUR"
  adults: 1
  cabin: "economy"
  home_airport: "BER"
  cache_ttl_minutes: 15
```

Credentials can also be supplied with environment variables:

```bash
export AMADEUS_CLIENT_ID="..."
export AMADEUS_CLIENT_SECRET="..."
export KIWI_API_KEY="..."
export SERPAPI_KEY="..."
export SEARCHAPI_KEY="..."
export TRAVELPAYOUTS_TOKEN="..."
```

Check provider readiness:

```bash
flight-pp-cli providers status
flight-pp-cli providers status --json
```

## Providers

### Amadeus

Uses Amadeus Flight Offers Search with OAuth client credentials. Configure:

```yaml
providers:
  amadeus:
    enabled: true
    client_id: ""
    client_secret: ""
    environment: "test" # test or production
```

or use `AMADEUS_CLIENT_ID` and `AMADEUS_CLIENT_SECRET`. OAuth access tokens are
cached in the CLI cache directory and refreshed automatically.

### Kiwi / Tequila

Uses the Kiwi Tequila API when enabled and `KIWI_API_KEY` is configured. Kiwi is
also the MVP flexible-date provider for `cheapest`, including `--to anywhere`.
Self-transfer, virtual interlining, baggage recheck, and separate-ticket signals
are marked as risks when returned by the API.

### Google Flights

Google is `deeplink` by default. The CLI builds a Google Flights URL and marks
the result as `open manually`, with no comparable price data.

Optional API modes:

```yaml
providers:
  google:
    enabled: true
    mode: "serpapi"   # or searchapi
    serpapi_key: ""
    searchapi_key: ""
```

These modes call third-party Google Flights APIs if the user provides a key.
The CLI never scrapes `google.com`.

### Future Providers

- Skyscanner: official Travel API partner access only. Direct scraping is not
  implemented.
- Expedia: deep-link mode in the MVP; Rapid API/Developer Hub can be added
  later.
- KAYAK: deep-link mode in the MVP; affiliate API can be added later.
- Travelpayouts: reserved for cached/stale price trend data and should be
  clearly labeled as such when implemented.

## Search

```bash
flight-pp-cli search \
  --from BER \
  --to CPH \
  --depart 2026-07-10 \
  --return 2026-07-14 \
  --adults 1 \
  --currency EUR \
  --providers amadeus,kiwi,google \
  --sort best \
  --limit 10
```

Common options:

```text
--from IATA
--to IATA|anywhere
--depart YYYY-MM-DD
--return YYYY-MM-DD
--one-way
--adults N
--children N
--infants N
--currency EUR
--cabin economy|premium_economy|business|first
--max-stops N
--direct-only
--include-self-transfer
--bags none|personal_item|cabin|checked
--providers amadeus,kiwi,google,expedia,kayak
--sort price|duration|best
--limit N
--json
--open-best
--no-cache
```

## Cheapest

Flexible-date search:

```bash
flight-pp-cli cheapest --from BER --to CPH --month 2026-07 --providers kiwi,google
flight-pp-cli cheapest --from BER --to anywhere --month 2026-07 --max-price 100 --providers kiwi
```

In the MVP, Kiwi can return comparable flexible-date prices. Google returns a
manual deep link. Providers without flexible-date support report a provider
error without failing the whole command.

## Price Watches

`watch` stores the search locally and prints a manual command you can schedule:

```bash
flight-pp-cli watch \
  --from BER \
  --to CPH \
  --depart 2026-07-10 \
  --return 2026-07-14 \
  --max-price 80 \
  --notify telegram
```

The MVP does not run a daemon and does not send notifications itself.

Cron example:

```cron
0 */6 * * * $HOME/go/bin/flight-pp-cli search --from BER --to CPH --depart 2026-07-10 --return 2026-07-14 --sort price --limit 10 --json
```

Windows Task Scheduler action example:

```powershell
powershell.exe -ExecutionPolicy Bypass -File \\wsl.localhost\Ubuntu\home\<wsl-user>\printing-press\flight-pp-cli.ps1 search --from BER --to CPH --depart 2026-07-10 --return 2026-07-14 --sort price --limit 10 --json
```

## Risk Flags

Risk and warning labels are deliberately conservative:

- `self-transfer / virtual interlining`: separate check-in or missed-connection
  risk may apply.
- `separate-ticket itinerary`: segments may not be protected as one ticket.
- `baggage recheck required`: you may need to reclaim and recheck bags.
- `airport change during connection`: connection may require ground transfer.
- `overnight layover`: connection crosses a date boundary or is very long.
- `baggage data missing`: provider did not return enough baggage detail.
- `open manually`: a deep-link-only provider did not return price data.

## Cache

Provider responses are cached by request hash. Default TTL is 15 minutes:

```bash
flight-pp-cli cache clear
flight-pp-cli search ... --no-cache
```

## Development

```bash
go fmt ./...
go test ./...
go build -o ./bin/flight-pp-cli ./cmd/flight-pp-cli
```

All external API calls in tests are mocked.
