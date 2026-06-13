#!/usr/bin/env python3
"""Bootstrap, verify, and install projects declared in workspace.yaml."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[1]
WORKSPACE_FILE = ROOT / "workspace.yaml"


def run(command: list[str], cwd: Path, env: dict[str, str] | None = None) -> None:
    print(f"\n==> {cwd.relative_to(ROOT)}: {' '.join(command)}", flush=True)
    subprocess.run(command, cwd=cwd, env=env, check=True)


def load_projects() -> list[dict[str, object]]:
    data = json.loads(WORKSPACE_FILE.read_text(encoding="utf-8"))
    projects = data.get("projects", [])
    if len(projects) != 23:
        raise SystemExit(f"workspace.yaml must declare exactly 23 projects, found {len(projects)}")
    return projects


def changed_paths(base: str) -> list[str] | None:
    if not base or set(base) == {"0"}:
        return None
    result = subprocess.run(
        ["git", "diff", "--name-only", f"{base}...HEAD"],
        cwd=ROOT,
        text=True,
        capture_output=True,
    )
    if result.returncode != 0:
        print(f"warning: cannot diff from {base}; verifying all projects", file=sys.stderr)
        return None
    return [line for line in result.stdout.splitlines() if line]


def select_projects(projects: list[dict[str, object]], base: str | None) -> list[dict[str, object]]:
    if base is None:
        return projects
    paths = changed_paths(base)
    if paths is None:
        return projects
    root_files = {"workspace.yaml", "scripts/workspace.py", "scripts/verify.sh", "scripts/bootstrap.sh"}
    if any(path in root_files or path.startswith(".github/") for path in paths):
        return projects
    selected = []
    for project in projects:
        prefix = str(project["path"]).rstrip("/") + "/"
        if any(path == str(project["path"]) or path.startswith(prefix) for path in paths):
            selected.append(project)
    return selected


def filter_named_projects(projects: list[dict[str, object]], names: list[str]) -> list[dict[str, object]]:
    if not names:
        return projects
    requested = set(names)
    selected = [project for project in projects if project["name"] in requested]
    missing = requested - {str(project["name"]) for project in selected}
    if missing:
        raise SystemExit("unknown project(s): " + ", ".join(sorted(missing)))
    return selected


def ensure_tools(runtimes: set[str]) -> None:
    required = {"go": "go", "node": "npm", "python": "python3"}
    missing = [binary for runtime, binary in required.items() if runtime in runtimes and shutil.which(binary) is None]
    if missing:
        raise SystemExit("missing required tools: " + ", ".join(sorted(missing)))


def python_environment(project_dir: Path) -> tuple[Path, Path]:
    venv = project_dir / ".venv"
    python = venv / "bin" / "python"
    pip = venv / "bin" / "pip"

    compatible = False
    if python.exists():
        result = subprocess.run(
            [str(python), "-c", "import sys; raise SystemExit(sys.version_info < (3, 11))"],
            cwd=project_dir,
        )
        compatible = result.returncode == 0

    if not compatible:
        if sys.version_info >= (3, 11):
            run(["python3", "-m", "venv", "--clear", str(venv)], project_dir)
        elif shutil.which("uv"):
            run(["uv", "venv", "--python", "3.12", "--seed", "--clear", str(venv)], project_dir)
        else:
            raise SystemExit(
                "job-research-cli requires Python 3.11+; install a compatible Python or uv"
            )
    return python, pip


def bootstrap(project: dict[str, object]) -> None:
    project_dir = ROOT / str(project["path"])
    runtimes = set(project["runtimes"])
    if "go" in runtimes:
        run(["go", "mod", "download"], project_dir, {**os.environ, "GOWORK": "off"})
    if "node" in runtimes:
        run(["npm", "ci"], project_dir)
    if "python" in runtimes:
        _, pip = python_environment(project_dir)
        run([str(pip), "install", "-e", ".[dev]"], project_dir)


def verify(project: dict[str, object]) -> None:
    project_dir = ROOT / str(project["path"])
    runtimes = set(project["runtimes"])
    if "go" in runtimes:
        env = {**os.environ, "GOWORK": "off"}
        run(["go", "test", "./..."], project_dir, env)
        run(["go", "vet", "./..."], project_dir, env)
        run(["go", "build", "./..."], project_dir, env)
    if "node" in runtimes:
        run(["npm", "ci"], project_dir)
        run(["npm", "run", "build", "--if-present"], project_dir)
        run(["npm", "test", "--if-present"], project_dir)
    if "python" in runtimes:
        python, pip = python_environment(project_dir)
        run([str(pip), "install", "-e", ".[dev]"], project_dir)
        run([str(python), "-m", "pytest"], project_dir)
def install(project: dict[str, object]) -> None:
    project_dir = ROOT / str(project["path"])
    runtimes = set(project["runtimes"])
    if "go" in runtimes:
        destination = Path.home() / "go" / "bin"
        destination.mkdir(parents=True, exist_ok=True)
        for command_dir in sorted((project_dir / "cmd").glob("*")):
            if command_dir.is_dir():
                run(
                    ["go", "build", "-o", str(destination / command_dir.name), f"./cmd/{command_dir.name}"],
                    project_dir,
                    {**os.environ, "GOWORK": "off"},
                )
    if "node" in runtimes and project["name"] == "kleinanzeigen-pp-cli":
        run(
            ["npm", "install", "--global", "--prefix", str(Path.home() / ".local"), "."],
            project_dir,
        )
    if "python" in runtimes:
        _, pip = python_environment(project_dir)
        run([str(pip), "install", "-e", "."], project_dir)


def main() -> int:
    parser = argparse.ArgumentParser()
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--list", action="store_true")
    mode.add_argument("--bootstrap", action="store_true")
    mode.add_argument("--verify", action="store_true")
    mode.add_argument("--install", action="store_true")
    parser.add_argument("--changed", metavar="BASE")
    parser.add_argument("--project", action="append", default=[])
    args = parser.parse_args()

    projects = load_projects()
    selected = filter_named_projects(select_projects(projects, args.changed), args.project)
    if args.list:
        for project in selected:
            print(project["name"])
        return 0
    if not selected:
        print("No CLI projects changed.")
        return 0

    runtimes = {runtime for project in selected for runtime in project["runtimes"]}
    ensure_tools(runtimes)

    operation = bootstrap if args.bootstrap else install if args.install else None
    for project in selected:
        print(f"\n##### {project['name']} #####", flush=True)
        if operation:
            operation(project)
        else:
            verify(project)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
