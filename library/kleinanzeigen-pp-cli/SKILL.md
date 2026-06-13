# kleinanzeigen-pp-cli

Use this local CLI for cautious personal Kleinanzeigen searches near Berlin
12045, local caching, watch rules, browser-profile login, and confirmed message
drafting/sending.

## Safety First

- Search and watch commands are URL/cache/local-first.
- Browser-backed search/watch scans require explicit `--open-browser` plus
  `--browser-ok USER_REQUESTED_BROWSER`, and agents must only use those flags
  when the user explicitly asks for browser use.
- Watches run manually once; there is no daemon.
- Messaging is local draft-first and never bulk.
- Sending requires exact terminal confirmation: `SEND`.
- The tool refuses unsafe messaging config such as disabled send confirmation or
  enabled bulk messaging.

## Commands

```bash
kleinanzeigen-pp-cli config init
kleinanzeigen-pp-cli config show
kleinanzeigen-pp-cli config set-location --postal-code 12045 --city Berlin --radius-km 5
kleinanzeigen-pp-cli config set-browser-profile ~/.local/share/kleinanzeigen-pp-cli/browser-profile
```

```bash
kleinanzeigen-pp-cli search "standing desk"
kleinanzeigen-pp-cli search "monitor" --radius-km 10
kleinanzeigen-pp-cli search "3d printer" --max-price 150
kleinanzeigen-pp-cli search "ikea kallax" --sort distance
kleinanzeigen-pp-cli search "bike" --json
kleinanzeigen-pp-cli --agent search "standing desk"
kleinanzeigen-pp-cli search "monitor" --open-browser --browser-ok USER_REQUESTED_BROWSER
```

```bash
kleinanzeigen-pp-cli listing open <listing-id>
kleinanzeigen-pp-cli listing show <listing-id>
kleinanzeigen-pp-cli listing notes <listing-id> "Looks good, ask if still available"
```

```bash
kleinanzeigen-pp-cli watch add "standing desk" --radius-km 5 --max-price 80
kleinanzeigen-pp-cli watch list
kleinanzeigen-pp-cli watch run
kleinanzeigen-pp-cli watch run --open-browser --browser-ok USER_REQUESTED_BROWSER
kleinanzeigen-pp-cli watch remove <watch-id>
```

```bash
kleinanzeigen-pp-cli auth login
kleinanzeigen-pp-cli auth status
kleinanzeigen-pp-cli auth logout
```

```bash
kleinanzeigen-pp-cli message draft <listing-id> --template availability
kleinanzeigen-pp-cli message draft <listing-id> --text "Hallo, ist der Artikel noch verfügbar?"
kleinanzeigen-pp-cli message templates list
kleinanzeigen-pp-cli message templates add availability
kleinanzeigen-pp-cli message send <listing-id> --template availability --live
```
