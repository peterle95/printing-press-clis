#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)

windows_installer=$(wslpath -w "$repo_root/scripts/windows/install.ps1")
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$windows_installer"

echo "Installed Windows launchers under %LOCALAPPDATA%\\Programs\\printing-press\\bin"
