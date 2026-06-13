# Spotify Printing Press CLI

`spotify-pp-cli` is a personal local CLI for organizing a Spotify library with
the official Spotify Web API. The main workflow is routing tracks from Liked
Songs into genre-specific or manually selected playlists. Copy is the default
safe behavior; move only removes a track from Liked Songs after playlist add
verification and explicit confirmation.

## Spotify Developer Setup

1. Create an app at <https://developer.spotify.com/dashboard>.
2. Add a redirect URI for the local callback, for example:

   ```text
   http://127.0.0.1:43683/callback
   ```

   Spotify requires the redirect URI used during login to match the app
   allowlist exactly. If you do not configure one, the CLI uses a dynamic local
   port and prints the exact URI to add.

3. Set credentials:

   ```bash
   export SPOTIFY_CLIENT_ID="..."
   # Optional for PKCE desktop flow:
   export SPOTIFY_CLIENT_SECRET="..."
   export SPOTIFY_REDIRECT_URI="http://127.0.0.1:43683/callback"
   ```

The CLI requests these scopes for the MVP:

```text
user-library-read
user-library-modify
playlist-read-private
playlist-read-collaborative
playlist-modify-private
playlist-modify-public
user-read-private
```

## Config

Config lives at:

```text
~/.config/printing-press/spotify/config.yaml
```

Example:

```yaml
client_id: "..."
redirect_uri: "http://127.0.0.1:43683/callback"
market: "DE"
default_mode: "dry-run"
liked_move_remove_after_add: false
```

Commands:

```bash
spotify-pp-cli config show
spotify-pp-cli config set market DE
spotify-pp-cli config set default_mode dry-run
```

## Authenticate

```bash
spotify-pp-cli auth login
spotify-pp-cli auth status
spotify-pp-cli auth logout
```

The login flow uses Authorization Code with PKCE and a localhost callback. It
prints the authorization URL for WSL/headless use and opens the browser when an
opener is available. Tokens are stored in the OS keychain when `secret-tool` or
macOS `security` is available, otherwise in:

```text
~/.config/printing-press/spotify/token.json
```

## Scan Liked Songs

```bash
spotify-pp-cli liked scan
spotify-pp-cli liked scan --limit 100
spotify-pp-cli liked list
```

The local SQLite cache defaults to:

```text
~/.local/share/printing-press/spotify/spotify.db
```

## Rules

Initialize editable routing rules:

```bash
spotify-pp-cli rules init
spotify-pp-cli rules path
spotify-pp-cli rules validate
```

Rules live at:

```text
~/.config/printing-press/spotify/rules.yaml
```

Classification uses cached track artists, artist genres when present, normalized
genre strings, priority-ordered YAML rules, and a fallback playlist such as
`Needs Review`.

Explain one track:

```bash
spotify-pp-cli rules explain "Windowlicker"
spotify-pp-cli classify liked
spotify-pp-cli classify track spotify:track:...
```

## Copy Liked Songs

Plan first:

```bash
spotify-pp-cli liked plan
spotify-pp-cli liked copy --dry-run
```

Apply copy:

```bash
spotify-pp-cli liked copy --confirm
```

Copy creates missing target playlists when the rules allow it, skips tracks that
already exist in the target playlist, records an operation, and leaves Liked
Songs unchanged.

## Move Liked Songs Safely

```bash
spotify-pp-cli liked move --dry-run
spotify-pp-cli liked move --confirm
spotify-pp-cli liked move --only genre=techno --confirm
```

Move is guarded:

1. Add to target playlist.
2. Re-scan the target playlist.
3. Verify the track exists in the target playlist.
4. Remove from Liked Songs only after verification.

The CLI asks for the exact phrase `move liked songs` unless `--yes` is used.

## Manual Commands

```bash
spotify-pp-cli track copy "song query" --to "Techno" --confirm
spotify-pp-cli track move spotify:track:... --to "Techno" --confirm
spotify-pp-cli track unlike spotify:track:... --confirm
```

## Playlist Cleanup

```bash
spotify-pp-cli playlists list
spotify-pp-cli playlist scan "Techno"
spotify-pp-cli playlist dedupe "Techno" --dry-run
spotify-pp-cli playlist sort "Techno" --by artist --dry-run
spotify-pp-cli playlist rename "old name" "new name" --confirm
```

Deduplication is intentionally dry-run-only in this MVP unless exact-position
removal is added. This avoids accidentally removing every copy of a duplicate
URI.

## Undo

```bash
spotify-pp-cli ops list
spotify-pp-cli ops show <operation-id>
spotify-pp-cli ops undo <operation-id> --dry-run
spotify-pp-cli ops undo <operation-id> --confirm
```

Undo re-saves tracks removed from Liked Songs and removes tracks added to
playlists where possible. Playlist ordering may not be perfectly restored.

## Limitations

- Spotify does not expose playlist folders.
- Spotify does not expose reliable track-level genres.
- Artist genres may be missing or marked deprecated by Spotify.
- Audio Features, Audio Analysis, Recommendations, Related Artists, Featured
  Playlists, Category Playlists, and Spotify-owned editorial playlist workflows
  are intentionally not used.
- Playlist delete is not a real Web API concept.
- Local files cannot be added to playlists via the Web API.
- This is a personal local CLI, not a public SaaS product.

## Development

```bash
go test ./...
go build -o ./bin/spotify-pp-cli ./cmd/spotify-pp-cli
```

Optional live integration tests are gated:

```bash
SPOTIFY_INTEGRATION_TESTS=1 go test ./internal/client -run Integration
```
