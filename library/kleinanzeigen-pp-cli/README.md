# kleinanzeigen-pp-cli

`kleinanzeigen-pp-cli` is a personal, lightweight CLI for cautious
Kleinanzeigen searches near Berlin 12045. By default it builds normal
Kleinanzeigen navigation URLs and works from the local cache. It opens a visible
Playwright browser only when a command explicitly needs one, such as manual
login, opening a listing, browser-backed result caching, or confirmed sending.

The tool is for personal use only. It does not bypass Kleinanzeigen protections,
does not create accounts, does not use proxy rotation, does not send bulk
messages, and does not scrape hidden APIs. Kleinanzeigen' current
[robots.txt](https://www.kleinanzeigen.de/robots.txt) disallows automated access
to paths such as `/search`, `/api`, and `/messages`; the
[terms](https://themen.kleinanzeigen.de/nutzungsbedingungen/) describe normal
location/radius search and sorting by distance/date/price. This CLI is therefore
modeled as a slow, visible, user-initiated helper.

## Installation

```bash
cd $HOME/printing-press/library/kleinanzeigen-pp-cli
npm install
npx playwright install chromium
npm run build
npm link
```

Initialize config:

```bash
kleinanzeigen-pp-cli config init
kleinanzeigen-pp-cli config show
```

Default config path:

```text
~/.config/kleinanzeigen-pp-cli/config.yaml
```

Default cache path:

```text
~/.local/share/kleinanzeigen-pp-cli/kleinanzeigen.sqlite
```

## Configuration

```bash
kleinanzeigen-pp-cli config set-location --postal-code 12045 --city Berlin --radius-km 5
kleinanzeigen-pp-cli config set-browser-profile ~/.local/share/kleinanzeigen-pp-cli/browser-profile
```

Default config:

```yaml
location:
  postal_code: "12045"
  city: Berlin
  radius_km: 5
search:
  default_sort: distance
  max_pages: 2
  min_delay_ms: 3000
  max_delay_ms: 9000
safety:
  require_send_confirmation: true
  allow_bulk_messaging: false
  max_messages_per_session: 5
  dry_run_default: true
```

## First Login

```bash
kleinanzeigen-pp-cli auth login
kleinanzeigen-pp-cli auth status
```

Login opens a visible browser. You manually enter credentials and handle any
2FA/CAPTCHA. The CLI never asks for your password and never stores it. It only
reuses the local browser profile after you log in.

## Search Examples

```bash
kleinanzeigen-pp-cli search "standing desk"
kleinanzeigen-pp-cli search "monitor" --radius-km 10
kleinanzeigen-pp-cli search "3d printer" --max-price 150
kleinanzeigen-pp-cli search "ikea kallax" --sort distance
kleinanzeigen-pp-cli search "bike" --json
kleinanzeigen-pp-cli search "bike" --markdown
kleinanzeigen-pp-cli search "monitor" --open-browser --browser-ok USER_REQUESTED_BROWSER
kleinanzeigen-pp-cli --agent search "standing desk"
```

By default, `search` prints a user-facing search URL and does not open a
browser. Use `--open-browser` only when you want the CLI to open a visible
browser, parse visible result cards, save them to the local SQLite cache, and
deduplicate by listing ID/URL. Browser-backed search pages are capped at five,
with the default at two.

Use `--agent` for compact machine-readable output that keeps agent context
small. It does not open a browser unless combined with `--open-browser`.
Agents must not use browser-backed search unless the user explicitly asks for
browser use. Browser-backed search requires both `--open-browser` and
`--browser-ok USER_REQUESTED_BROWSER`.

## Listing Examples

```bash
kleinanzeigen-pp-cli listing show <listing-id>
kleinanzeigen-pp-cli listing open <listing-id>
kleinanzeigen-pp-cli listing notes <listing-id> "Looks good, ask if still available"
```

`listing open` uses the cached URL and opens the visible browser. `listing show`
does not refetch remote listing details.

## Watch Examples

```bash
kleinanzeigen-pp-cli watch add "standing desk" --radius-km 5 --max-price 80
kleinanzeigen-pp-cli watch list
kleinanzeigen-pp-cli watch run
kleinanzeigen-pp-cli watch run --open-browser --browser-ok USER_REQUESTED_BROWSER
kleinanzeigen-pp-cli watch remove <watch-id>
```

`watch run` prints one URL per active watch rule by default. It does not create a
daemon or cron job. Use `watch run --open-browser` for a single manual
browser-backed scan that caches visible results and prints only listings that
are new for each watch rule. Browser-backed watch runs also require
`--browser-ok USER_REQUESTED_BROWSER`.

## Message Drafting

```bash
kleinanzeigen-pp-cli message draft <listing-id> --template availability
kleinanzeigen-pp-cli message draft <listing-id> --template polite_offer --offer-price 50
kleinanzeigen-pp-cli message draft <listing-id> --text "Hallo, ist der Artikel noch verfügbar?"
kleinanzeigen-pp-cli message templates list
kleinanzeigen-pp-cli message templates add availability
```

Default German templates:

```text
availability:
Hallo, ist der Artikel noch verfügbar? Ich könnte ihn in Berlin abholen. Viele Grüße

polite_offer:
Hallo, ist der Artikel noch verfügbar? Wären Sie mit {offer_price} € einverstanden? Ich könnte ihn zeitnah abholen. Viele Grüße

pickup_question:
Hallo, ist der Artikel noch verfügbar? Wann wäre eine Abholung ungefähr möglich? Viele Grüße
```

Drafts are stored locally in the SQLite cache. Nothing is sent by draft
commands.

## Confirmed Sending

```bash
kleinanzeigen-pp-cli message send <listing-id> --template availability
```

By default this is a dry run because `safety.dry_run_default` is `true`.

To allow an actual send, use:

```bash
kleinanzeigen-pp-cli message send <listing-id> --template availability --live
```

The CLI opens the listing in a visible browser, fills the message box, and then
shows this terminal confirmation:

```text
About to send this message to listing:
Title: ...
URL: ...
Message:
...

Type SEND to confirm:
```

Only the exact input `SEND` clicks the send button. Anything else cancels.
Every confirmed send is logged locally with timestamp, listing ID, listing URL,
message text, and confirmation method.

## Safety Rules

- Use only your own Kleinanzeigen account.
- Do not create fake accounts.
- Do not bypass CAPTCHA, bot checks, login protections, rate limits, or paywalls.
- Do not send bulk messages.
- Never send a message without explicit interactive confirmation.
- Use saved searches, visible browser session, manual login, exported links, and
  human-approved actions where possible.
- Prefer URL/cache/local commands. Open the browser only when you explicitly ask
  for browser use.
- Keep requests slow and sequential. There is no parallel crawling.
- Stop when Kleinanzeigen blocks or challenges the session.
- Do not store passwords, cookies, tokens, auth headers, or browser session data
  in this repository.

## Limitations

- Result parsing depends on the visible Kleinanzeigen page and may need selector
  updates if the site changes.
- Search URL parameters are based on current user-facing search behavior and may
  change.
- Inbox/thread commands are intentionally not implemented yet.
- No daemon, cron, systemd integration, bulk export, or bulk messaging exists.
- Auth status is a local profile check; it does not reveal session tokens.
