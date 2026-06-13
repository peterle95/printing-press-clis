#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

mode=--all
base=
projects=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --all)
      mode=--all
      shift
      ;;
    --changed)
      mode=--changed
      base=${2:?missing base revision}
      shift 2
      ;;
    --project)
      projects+=("${2:?missing project name}")
      shift 2
      ;;
    *)
      echo "usage: $0 [--all | --changed BASE] [--project NAME ...]" >&2
      exit 2
      ;;
  esac
done

args=(--verify)
if [[ $mode == --changed ]]; then
  args+=(--changed "$base")
fi
for project in "${projects[@]}"; do
  args+=(--project "$project")
done

python3 scripts/workspace.py "${args[@]}"
