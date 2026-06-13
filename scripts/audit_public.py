#!/usr/bin/env python3
"""Fail when the candidate public tree contains private or generated artifacts."""

from __future__ import annotations

import os
from pathlib import Path
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[1]
MAX_FILE_SIZE = 5 * 1024 * 1024
ALLOWED_LARGE: set[str] = set()
FORBIDDEN_NAMES = {
    ".env",
    "local.properties",
    "accounts.yaml",
}
FORBIDDEN_SUFFIXES = (
    ".db",
    ".db-shm",
    ".db-wal",
    ".sqlite",
    ".sqlite-shm",
    ".sqlite-wal",
    ".session",
)
FORBIDDEN_DIRS = {
    ".gradle",
    ".idea",
    ".kotlin",
    ".mail",
    ".google",
    ".runstate",
    ".venv",
    "__pycache__",
    "bin",
    "build",
    "debug",
    "dist",
    "manuscripts",
    "node_modules",
}
PRIVATE_PATTERNS = {
    "Windows user path": re.compile(r"(?i)[A-Z]:[\\/]Users[\\/][^\\/\s]+"),
    "WSL Windows user path": re.compile(r"/mnt/c/Users/[^/\s]+"),
    "machine-specific WSL home": re.compile(r"/home/ubuntu"),
}


def candidate_files() -> list[Path]:
    if (ROOT / ".git").exists():
        result = subprocess.run(
            ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
            cwd=ROOT,
            check=True,
            capture_output=True,
        )
        return [ROOT / item.decode() for item in result.stdout.split(b"\0") if item]
    return [path for path in ROOT.rglob("*") if path.is_file()]


def is_binary(data: bytes) -> bool:
    return data.startswith(b"\x7fELF") or data.startswith(b"MZ") or data.startswith(b"\xfe\xed\xfa")


def main() -> int:
    failures: list[str] = []
    files = candidate_files()
    for path in files:
        if not path.is_file():
            continue
        relative = path.relative_to(ROOT).as_posix()
        parts = set(path.relative_to(ROOT).parts)
        lower = path.name.lower()
        if parts & FORBIDDEN_DIRS:
            failures.append(f"forbidden directory: {relative}")
            continue
        if path.name in FORBIDDEN_NAMES or lower.endswith(FORBIDDEN_SUFFIXES):
            failures.append(f"private runtime file: {relative}")
            continue
        if ("token" in lower or "credential" in lower or "cookie" in lower) and lower.endswith(".json"):
            failures.append(f"credential-like JSON file: {relative}")
            continue
        size = path.stat().st_size
        if size > MAX_FILE_SIZE and relative not in ALLOWED_LARGE:
            failures.append(f"file exceeds 5 MiB: {relative} ({size} bytes)")
        data = path.read_bytes()
        if is_binary(data) and relative not in ALLOWED_LARGE:
            failures.append(f"compiled binary: {relative}")
            continue
        if relative != "scripts/audit_public.py":
            text = data.decode("utf-8", errors="ignore")
            for label, pattern in PRIVATE_PATTERNS.items():
                for match in pattern.finditer(text):
                    line = text.count("\n", 0, match.start()) + 1
                    failures.append(f"{label}: {relative}:{line}")

    workspace = ROOT / "workspace.yaml"
    if workspace.exists():
        import json

        projects = json.loads(workspace.read_text(encoding="utf-8")).get("projects", [])
        declared = {project["path"] for project in projects}
        actual = {
            f"library/{path.name}"
            for path in (ROOT / "library").iterdir()
            if path.is_dir() and not path.name.startswith(".")
        }
        if len(declared) != 23 or declared != actual:
            failures.append(
                f"workspace inventory mismatch: declared={len(declared)} actual={len(actual)} "
                f"missing={sorted(actual - declared)} extra={sorted(declared - actual)}"
            )

    if failures:
        print("Public-tree audit failed:", file=sys.stderr)
        for failure in sorted(set(failures)):
            print(f"  - {failure}", file=sys.stderr)
        return 1
    print(f"Public-tree audit passed for {len(files)} candidate files.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
