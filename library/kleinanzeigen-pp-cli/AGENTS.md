# Kleinanzeigen Printed CLI Agent Guide

This is a hand-authored local Printing Press CLI named `kleinanzeigen-pp-cli`.
It is built with TypeScript, Node.js, Playwright, YAML config, and a local
SQLite-format cache through `sql.js`.

## Operating Rules

- Use only the user's own Kleinanzeigen account and local browser profile.
- Do not create accounts, bypass CAPTCHA, evade bot checks, rotate proxies,
  spoof user agents for evasion, or bypass login protections.
- Prefer URL/cache/local commands. `search` and `watch run` must not open a
  browser unless the user explicitly asks for browser use and the command uses
  both `--open-browser` and `--browser-ok USER_REQUESTED_BROWSER`.
- Do not scrape aggressively. Browser-backed searches are visible-browser,
  user-initiated, sequential, and capped.
- Do not add bulk messaging features. There must never be a "message all"
  command.
- Treat messages as draft-first. Sending requires the user to pass `--live` when
  dry-run defaults are enabled and then type exactly `SEND`.
- Never ask for or store a Kleinanzeigen password. Login happens only in the
  visible browser profile.
- Do not log cookies, local storage, session tokens, auth headers, passwords, or
  private message content beyond explicit local drafts/sent-message records.

## Build And Test

```bash
cd $HOME/printing-press/library/kleinanzeigen-pp-cli
npm install
npx playwright install chromium
npm run build
npm test
```

Install the local executable into the WSL user bin:

```bash
npm link
```

## Useful Commands

```bash
kleinanzeigen-pp-cli config init
kleinanzeigen-pp-cli --agent search "standing desk"
kleinanzeigen-pp-cli search "monitor" --radius-km 10
kleinanzeigen-pp-cli search "monitor" --radius-km 10 --open-browser --browser-ok USER_REQUESTED_BROWSER
kleinanzeigen-pp-cli watch add "standing desk" --radius-km 5 --max-price 80
kleinanzeigen-pp-cli watch run
kleinanzeigen-pp-cli watch run --open-browser --browser-ok USER_REQUESTED_BROWSER
kleinanzeigen-pp-cli auth login
kleinanzeigen-pp-cli message draft <listing-id> --template availability
kleinanzeigen-pp-cli message send <listing-id> --template availability --live
```
