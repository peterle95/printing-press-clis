# Flight Printing Press CLI

Use `flight-pp-cli` when the user wants to compare flight options across safe
provider adapters without direct scraping.

## Safety

- Do not scrape Google Flights, Skyscanner, Expedia, KAYAK, airline sites, or
  booking pages.
- Do not add bot-detection evasion, proxy rotation, captcha bypass, account
  abuse, or credential stuffing.
- Use official APIs, affiliate APIs, or deep links.
- Prices must be verified on the booking page before purchase.

## Common Commands

```bash
flight-pp-cli config init
flight-pp-cli providers status
flight-pp-cli search --from BER --to CPH --depart 2026-07-10 --return 2026-07-14 --currency EUR --providers amadeus,google
flight-pp-cli cheapest --from BER --to anywhere --month 2026-07 --max-price 100 --providers kiwi
flight-pp-cli watch --from BER --to CPH --depart 2026-07-10 --return 2026-07-14 --max-price 80
flight-pp-cli cache clear
```

Use `--json` or `--agent` for scripting.

## Credentials

Prefer environment variables or OS config outside the repository:

```bash
AMADEUS_CLIENT_ID
AMADEUS_CLIENT_SECRET
KIWI_API_KEY
SERPAPI_KEY
SEARCHAPI_KEY
TRAVELPAYOUTS_TOKEN
```

Config lives at `%APPDATA%/printing-press/flight/config.yaml` on Windows and
`~/.config/printing-press/flight/config.yaml` on Linux/macOS.
