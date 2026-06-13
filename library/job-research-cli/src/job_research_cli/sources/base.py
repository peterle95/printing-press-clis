from __future__ import annotations

from abc import ABC, abstractmethod
from datetime import date, timedelta
import logging
from typing import Any

from ..http_client import PoliteHttpClient
from ..models import JobPosting
from ..normalizer import infer_remote_mode, normalize_location, normalize_text, parse_date_value

LOGGER = logging.getLogger(__name__)

GENERIC_TITLE_WORDS = {
    "developer",
    "engineer",
    "software",
    "junior",
    "senior",
    "full",
    "stack",
}


class SourceAdapterError(RuntimeError):
    pass


class JobSource(ABC):
    name: str
    source_type = "api"

    def __init__(self, settings: dict[str, Any], http: PoliteHttpClient) -> None:
        self.settings = settings
        self.http = http

    @abstractmethod
    def search(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[JobPosting]:
        raise NotImplementedError

    def dry_run_urls(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[str]:
        return []

    @property
    def rate_limit_per_minute(self) -> int:
        return int(self.settings.get("rate_limit_per_minute") or 20)

    @property
    def cooldown_seconds(self) -> float:
        return float(self.settings.get("cooldown_seconds") or 0)

    def get_json(
        self,
        url: str,
        *,
        params: dict[str, Any] | None = None,
        headers: dict[str, str] | None = None,
    ) -> Any:
        return self.http.get_json(
            self.name,
            url,
            params=params,
            headers=headers,
            rate_limit_per_minute=self.rate_limit_per_minute,
            cooldown_seconds=self.cooldown_seconds,
        )

    def build_url(self, url: str, params: dict[str, Any] | None = None) -> str:
        return self.http.build_url(url, params=params)

    def is_configured(self) -> bool:
        return True


def title_matches(raw_title: str | None, search_term: str) -> bool:
    haystack = normalize_text(raw_title)
    needle = normalize_text(search_term)
    if not needle:
        return True
    if needle in haystack:
        return True
    tokens = [token for token in needle.split() if len(token) >= 3 and token not in GENERIC_TITLE_WORDS]
    if not tokens:
        tokens = [token for token in needle.split() if len(token) >= 3]
    overlap = sum(1 for token in tokens if token in haystack)
    return overlap >= max(1, min(2, len(tokens)))


def location_matches(raw_location: str | None, requested_location: str, remote_mode: str | None = None) -> bool:
    requested = normalize_location(requested_location)
    candidate = normalize_location(raw_location)
    if not requested:
        return True
    if "remote" in requested:
        return remote_mode in {"remote", "hybrid"} or "remote" in candidate or "home office" in candidate
    if requested == "germany":
        return not candidate or "germany" in candidate or "deutschland" in candidate or "berlin" in candidate
    if not candidate:
        return True
    return requested in candidate


def within_days(posted: object, days: int) -> bool:
    parsed = parse_date_value(posted)
    if parsed is None or days <= 0:
        return True
    return parsed >= date.today() - timedelta(days=days)


def first_value(raw: dict[str, Any], *keys: str) -> Any:
    for key in keys:
        value = raw.get(key)
        if value not in (None, ""):
            return value
    return None


def remote_mode_from_payload(raw: dict[str, Any], *extra_values: object) -> str | None:
    for key in ("remote", "is_remote", "homeoffice", "homeOffice", "hybrid", "workplace_type"):
        value = raw.get(key)
        if value is True:
            return "hybrid" if "hybrid" in key.lower() else "remote"
        if isinstance(value, str):
            inferred = infer_remote_mode(value)
            if inferred:
                return inferred
    return infer_remote_mode(*extra_values)
