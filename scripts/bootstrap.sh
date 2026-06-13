#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

install=false
if [[ ${1:-} == "--install" ]]; then
  install=true
elif [[ $# -gt 0 ]]; then
  echo "usage: $0 [--install]" >&2
  exit 2
fi

git submodule update --init --recursive
python3 scripts/workspace.py --bootstrap

if $install; then
  python3 scripts/workspace.py --install
fi

echo "Bootstrap complete."
