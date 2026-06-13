from __future__ import annotations

from copy import deepcopy
from pathlib import Path
import os
import platform
from typing import Any

import yaml

DEFAULT_TITLES_CONFIG: dict[str, Any] = {
    "titles": [
        "Frontend Developer",
        "Junior Frontend Developer",
        "Full Stack Developer",
        "Junior Full Stack Developer",
        "React Developer",
        "JavaScript Developer",
        "TypeScript Developer",
        "Web Developer",
        "Software Engineer",
        "Technical Sourcer",
        "Technical Recruiter",
        "Talent Acquisition Specialist",
    ],
    "locations": ["Berlin", "Germany", "Remote Germany", "Remote Europe"],
}

DEFAULT_SOURCES_CONFIG: dict[str, Any] = {
    "sources": {
        "bundesagentur": {
            "enabled": True,
            "type": "api",
            "base_url": "https://rest.arbeitsagentur.de/jobboerse/jobsuche-service",
            "api_key_env": "JOB_RESEARCH_BUNDESAGENTUR_API_KEY",
            "default_api_key": "jobboerse-jobsuche",
            "rate_limit_per_minute": 30,
            "cooldown_seconds": 2,
        },
        "arbeitnow": {
            "enabled": True,
            "type": "api",
            "base_url": "https://www.arbeitnow.com/api/job-board-api",
            "rate_limit_per_minute": 20,
            "cooldown_seconds": 3,
        },
        "adzuna": {
            "enabled": False,
            "type": "api",
            "requires_api_key": True,
            "app_id_env": "ADZUNA_APP_ID",
            "app_key_env": "ADZUNA_APP_KEY",
            "country": "de",
            "base_url": "https://api.adzuna.com/v1/api",
            "rate_limit_per_minute": 20,
            "cooldown_seconds": 3,
        },
        "themuse": {
            "enabled": True,
            "type": "api",
            "base_url": "https://www.themuse.com/api/public/jobs",
            "rate_limit_per_minute": 20,
            "cooldown_seconds": 3,
        },
        "remoteok": {
            "enabled": False,
            "type": "api",
            "base_url": "https://remoteok.com/api",
            "rate_limit_per_minute": 10,
            "cooldown_seconds": 6,
        },
        "greenhouse": {
            "enabled": False,
            "type": "api",
            "board_tokens": [],
            "base_url": "https://boards-api.greenhouse.io/v1/boards",
            "rate_limit_per_minute": 20,
            "cooldown_seconds": 3,
        },
        "lever": {
            "enabled": False,
            "type": "api",
            "company_slugs": [],
            "base_url": "https://api.lever.co/v0/postings",
            "rate_limit_per_minute": 20,
            "cooldown_seconds": 3,
        },
        "linkedin": {"enabled": True, "type": "manual_search_link"},
        "xing": {"enabled": True, "type": "manual_search_link"},
        "indeed": {"enabled": True, "type": "manual_search_link"},
        "stepstone": {"enabled": True, "type": "manual_search_link"},
        "glassdoor": {"enabled": True, "type": "manual_search_link"},
        "monster": {"enabled": True, "type": "manual_search_link"},
        "google_jobs": {"enabled": True, "type": "manual_search_link"},
        "kununu": {"enabled": True, "type": "manual_search_link"},
        "wellfound": {"enabled": False, "type": "manual_search_link"},
        "github_jobs": {"enabled": False, "type": "manual_search_link"},
    }
}


def default_config_dir() -> Path:
    if platform.system() == "Windows":
        root = os.environ.get("APPDATA")
        if root:
            return Path(root) / "printing-press" / "job-research"
    return Path.home() / ".config" / "printing-press" / "job-research"


def default_data_dir() -> Path:
    if platform.system() == "Windows":
        root = os.environ.get("LOCALAPPDATA")
        if root:
            return Path(root) / "printing-press" / "job-research"
    return Path.home() / ".local" / "share" / "printing-press" / "job-research"


def default_db_path() -> Path:
    return default_data_dir() / "jobs.db"


def load_runtime_config(config_dir: Path | None = None) -> tuple[dict[str, Any], dict[str, Any], Path]:
    config_dir = config_dir or default_config_dir()
    load_env_file(config_dir / ".env")
    sources = _load_default_or_project("sources.yaml", DEFAULT_SOURCES_CONFIG)
    titles = _load_default_or_project("titles.yaml", DEFAULT_TITLES_CONFIG)

    user_sources = _read_yaml(config_dir / "sources.yaml")
    user_titles = _read_yaml(config_dir / "titles.yaml")
    return deep_merge(sources, user_sources), deep_merge(titles, user_titles), config_dir


def write_default_config(config_dir: Path | None = None, *, overwrite: bool = False) -> list[Path]:
    config_dir = config_dir or default_config_dir()
    config_dir.mkdir(parents=True, exist_ok=True)
    written: list[Path] = []
    for filename, data in {
        "sources.yaml": DEFAULT_SOURCES_CONFIG,
        "titles.yaml": DEFAULT_TITLES_CONFIG,
    }.items():
        path = config_dir / filename
        if path.exists() and not overwrite:
            continue
        path.write_text(yaml.safe_dump(data, sort_keys=False, allow_unicode=False), encoding="utf-8")
        written.append(path)
    default_data_dir().mkdir(parents=True, exist_ok=True)
    return written


def append_env_values(config_dir: Path, values: dict[str, str]) -> Path | None:
    clean = {key: value for key, value in values.items() if value}
    if not clean:
        return None
    config_dir.mkdir(parents=True, exist_ok=True)
    env_path = config_dir / ".env"
    existing = _parse_env_file(env_path) if env_path.exists() else {}
    existing.update(clean)
    lines = [f"{key}={value}" for key, value in existing.items()]
    env_path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return env_path


def load_titles_file(path: Path) -> list[str]:
    if not path.exists():
        raise FileNotFoundError(path)
    if path.suffix.lower() in {".yaml", ".yml"}:
        data = _read_yaml(path)
        if isinstance(data, dict):
            values = data.get("titles", [])
        else:
            values = data
        return _dedupe_strings(values or [])
    return _dedupe_strings(line.strip() for line in path.read_text(encoding="utf-8").splitlines())


def deep_merge(base: dict[str, Any], override: dict[str, Any]) -> dict[str, Any]:
    merged = deepcopy(base)
    for key, value in override.items():
        if isinstance(value, dict) and isinstance(merged.get(key), dict):
            merged[key] = deep_merge(merged[key], value)
        else:
            merged[key] = deepcopy(value)
    return merged


def load_env_file(path: Path) -> None:
    if not path.exists():
        return
    for key, value in _parse_env_file(path).items():
        os.environ.setdefault(key, value)


def _parse_env_file(path: Path) -> dict[str, str]:
    parsed: dict[str, str] = {}
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        parsed[key.strip()] = value.strip().strip('"').strip("'")
    return parsed


def _load_default_or_project(filename: str, fallback: dict[str, Any]) -> dict[str, Any]:
    project_config = Path(__file__).resolve().parents[2] / "config" / filename
    if project_config.exists():
        return deep_merge(fallback, _read_yaml(project_config))
    return deepcopy(fallback)


def _read_yaml(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {}
    data = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
    if not isinstance(data, dict):
        return {"values": data}
    return data


def _dedupe_strings(values: object) -> list[str]:
    deduped: list[str] = []
    seen: set[str] = set()
    if not isinstance(values, list):
        values = list(values) if values is not None else []
    for value in values:
        text = str(value).strip()
        key = text.lower()
        if text and key not in seen:
            deduped.append(text)
            seen.add(key)
    return deduped
