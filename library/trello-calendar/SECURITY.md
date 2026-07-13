# Security Policy

## Credential handling

Trello credentials and Google OAuth client credentials are accepted only from environment variables. They are not valid `config.toml` fields. Google access and refresh tokens are stored separately at `~/.config/trello-calendar-pp-cli/google-token.json` using atomic replacement, a `0700` parent directory, and a `0600` non-symlink regular file on Unix-like systems.

The CLI never prints API keys, tokens, client secrets, refresh tokens, or Authorization headers. Printing Press dry-run output uses the planner's mutation gate rather than rendering credential-bearing requests.

Do not commit:

- `.env` files
- `config.toml` containing operator-specific IDs
- `google-token.json` or other token/credential JSON
- browser profiles, cookies, sessions, databases, Calendar exports, or Trello exports
- compiled binaries and MCP bundles

The repository `.gitignore` covers the common local names, but operators remain responsible for reviewing staged files.

## OAuth security

The Google flow uses PKCE, a cryptographically random state value, an HTTP loopback callback, a three-minute timeout, offline access, and the minimal `calendar.events` scope. Non-loopback redirect URIs are rejected. Revoke access in the Google Account security console if a token may have leaked.

## Reporting vulnerabilities

Report security issues privately to the repository maintainer. Do not include live credentials, tokens, card contents, Calendar contents, or exploit details in a public issue.

## Operational guidance

Use `preview` or `schedule --dry-run` before every unfamiliar mutation. Use a dedicated Google Calendar for integration testing. `--comment-on-card` is a Trello write and should be enabled only after the event plan is confirmed.

