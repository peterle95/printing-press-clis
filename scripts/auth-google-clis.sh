#!/usr/bin/env bash
set -euo pipefail

# Authenticate Calendar, Gmail, and Trello-calendar sequentially.
# OAuth client JSON and resulting tokens stay in their existing private stores.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MAIL_ACCOUNT="${MAIL_ACCOUNT:-${1:-}}"
SECONDARY_MAIL_ACCOUNT="${SECONDARY_MAIL_ACCOUNT:-${2:-}}"
LILUVASIL_MAIL_ACCOUNT="${LILUVASIL_MAIL_ACCOUNT:-liluvasil-gmail}"
GOOGLE_CREDENTIALS="${GOOGLE_CREDENTIALS:-$HOME/.config/printing-press/google-private/credentials/calendar-tasks-oauth-client.json}"
GMAIL_CREDENTIALS="${GMAIL_CREDENTIALS:-$HOME/.config/printing-press/google-private/credentials/printing-press-gmail-cli.json}"
CALENDAR_CONFIG="${CALENDAR_CONFIG:-$HOME/.config/google-calendar-pp-cli/config.toml}"
LILUVASIL_CALENDAR_CONFIG="${LILUVASIL_CALENDAR_CONFIG:-$HOME/.config/google-calendar-pp-cli/config-liluvasil.toml}"
TRELLO_TOKEN="${TRELLO_TOKEN_FILE:-$HOME/.config/trello-calendar-pp-cli/google-token.json}"
MAIN_GMAIL_TOKEN="${MAIN_GMAIL_TOKEN:-$ROOT_DIR/library/.google/tokens/gmail-molzerpeter-token.json}"
SECONDARY_GMAIL_TOKEN="${SECONDARY_GMAIL_TOKEN:-$ROOT_DIR/library/.google/tokens/gmail-alt-moelzerpeter-token.json}"
LILUVASIL_GMAIL_TOKEN="${LILUVASIL_GMAIL_TOKEN:-$ROOT_DIR/library/.google/tokens/gmail-liluvasil-token.json}"

if [[ -z "$MAIL_ACCOUNT" || -z "$SECONDARY_MAIL_ACCOUNT" ]]; then
  printf 'Usage: MAIL_ACCOUNT=main@gmail.com SECONDARY_MAIL_ACCOUNT=secondary@gmail.com %s\n' "${BASH_SOURCE[0]}" >&2
  exit 2
fi

find_bin() {
  local name=$1 local_path=$2
  if [[ -x "$local_path" ]]; then
    printf '%s\n' "$local_path"
  elif command -v "$name" >/dev/null 2>&1; then
    command -v "$name"
  else
    printf 'Missing executable: %s (build it or add it to PATH)\n' "$name" >&2
    exit 1
  fi
}

calendar_bin="${CALENDAR_BIN:-$(find_bin google-calendar-pp-cli "$ROOT_DIR/library/google-calendar/bin/google-calendar-pp-cli")}"
mail_bin="${MAIL_BIN:-$(find_bin mail-pp-cli "$ROOT_DIR/library/mail/bin/mail-pp-cli")}"
trello_bin="${TRELLO_CALENDAR_BIN:-$(find_bin trello-calendar-pp-cli "$ROOT_DIR/library/trello-calendar/bin/trello-calendar-pp-cli")}"

if [[ ! -f "$GOOGLE_CREDENTIALS" ]]; then
  printf 'Missing Calendar OAuth client JSON: %s\n' "$GOOGLE_CREDENTIALS" >&2
  exit 1
fi
if [[ ! -f "$GMAIL_CREDENTIALS" ]]; then
  printf 'Missing Gmail OAuth client JSON: %s\n' "$GMAIL_CREDENTIALS" >&2
  exit 1
fi

red=$'\033[1;31m'
reset=$'\033[0m'

run_flow() {
  local label=$1 token_file=$2
  shift 2
  if [[ -f "$token_file" ]]; then
  printf '%s%s: existing token found; skipping%s\n' "$red" "$label" "$reset"
    return
  fi
  printf '%s%s%s\n' "$red" "$label" "$reset"
  "$@"
}

run_flow 'Main account Google Calendar OAuth link' "$CALENDAR_CONFIG" \
  "$calendar_bin" auth login --credentials "$GOOGLE_CREDENTIALS"
run_flow 'Liluvasil account Google Calendar OAuth link' "$LILUVASIL_CALENDAR_CONFIG" \
  "$calendar_bin" auth login --credentials "$GOOGLE_CREDENTIALS" --config "$LILUVASIL_CALENDAR_CONFIG"
run_flow 'Main account Gmail OAuth link' "$MAIN_GMAIL_TOKEN" \
  "$mail_bin" --google-credentials "$GMAIL_CREDENTIALS" auth login gmail --account "$MAIL_ACCOUNT"
run_flow 'Secondary account Gmail OAuth link' "$SECONDARY_GMAIL_TOKEN" \
  "$mail_bin" --google-credentials "$GMAIL_CREDENTIALS" auth login gmail --account "$SECONDARY_MAIL_ACCOUNT"
run_flow 'Liluvasil account Gmail OAuth link' "$LILUVASIL_GMAIL_TOKEN" \
  "$mail_bin" --google-credentials "$GMAIL_CREDENTIALS" auth login gmail --account "$LILUVASIL_MAIL_ACCOUNT"
if [[ -f "$TRELLO_TOKEN" ]]; then
  printf '%sMain account Trello-calendar Google OAuth link: existing token found; skipping%s\n' "$red" "$reset"
else
  printf '%sMain account Trello-calendar Google OAuth link:%s\n' "$red" "$reset"
  source "$ROOT_DIR/library/trello-calendar/.env"
  "$trello_bin" auth google
fi

printf 'Authentication check complete.\n'
