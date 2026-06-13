#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

python3 scripts/audit_public.py
git diff --check

if command -v gitleaks >/dev/null 2>&1; then
  gitleaks dir . --no-banner --redact
else
  echo "warning: gitleaks is not installed; structural and privacy audits completed" >&2
fi
