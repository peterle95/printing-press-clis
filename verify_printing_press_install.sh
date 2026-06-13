#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
cd "$repo_root"

export PATH="${HOME}/.local/go/bin:${HOME}/go/bin:${HOME}/.local/bin:${PATH}"

echo "Go:"
go version

echo "Printing Press:"
command -v printing-press
printing-press --version

echo "Repository:"
printf 'root: %s\n' "$repo_root"
git submodule status -- external/cli-printing-press
test "$(git -C external/cli-printing-press rev-parse HEAD)" = \
  "b8b1b6a4d4b6a8f037b8dc3f78625a1b676ea402"

echo "Workspace:"
project_count=$(python3 scripts/workspace.py --list | wc -l)
printf 'projects: %s\n' "$project_count"
test "$project_count" -eq 23

echo "Public-tree hygiene:"
./scripts/audit-public.sh
git diff --check

echo "Installed representative CLIs:"
for binary in flight-pp-cli transit-pp-cli youtube-pp-cli; do
  command -v "$binary"
  "$binary" --help >/dev/null
done

echo "Printing Press installation verified."
