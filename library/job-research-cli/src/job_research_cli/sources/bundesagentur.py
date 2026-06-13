from __future__ import annotations

import os
from typing import Any
from urllib.parse import quote

from ..models import JobPosting
from ..normalizer import infer_remote_mode
from .base import JobSource, first_value, location_matches, remote_mode_from_payload, title_matches, within_days


class BundesagenturSource(JobSource):
    name = "bundesagentur"

    def search(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[JobPosting]:
        endpoint = f"{self.settings['base_url'].rstrip('/')}/pc/v6/jobs"
        params = self._params(title, location, remote, days, limit)
        payload = self.get_json(endpoint, params=params, headers={"X-API-Key": self._api_key()})
        jobs = _job_items(payload)
        postings: list[JobPosting] = []
        for raw in jobs:
            posting = self._map_job(raw, title)
            if not title_matches(posting.title, title):
                continue
            if not within_days(posting.date_of_posting, days):
                continue
            if not location_matches(posting.location, location, posting.remote_mode):
                continue
            postings.append(posting)
            if len(postings) >= limit:
                break
        return postings

    def dry_run_urls(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[str]:
        endpoint = f"{self.settings['base_url'].rstrip('/')}/pc/v6/jobs"
        return [self.build_url(endpoint, self._params(title, location, remote, days, limit))]

    def _params(self, title: str, location: str, remote: bool, days: int, limit: int) -> dict[str, Any]:
        query_title = f"{title} remote" if remote and "remote" not in title.lower() else title
        params: dict[str, Any] = {
            "was": query_title,
            "wo": location,
            "page": 1,
            "size": min(max(limit, 1), 100),
            "angebotsart": 1,
        }
        if days > 0:
            params["veroeffentlichtseit"] = days
        return params

    def _api_key(self) -> str:
        env_name = self.settings.get("api_key_env", "JOB_RESEARCH_BUNDESAGENTUR_API_KEY")
        return os.environ.get(env_name) or self.settings.get("default_api_key") or "jobboerse-jobsuche"

    def _map_job(self, raw: dict[str, Any], search_term: str) -> JobPosting:
        refnr = first_value(raw, "referenznummer", "refnr", "id")
        title = first_value(raw, "stellenangebotsTitel", "titel", "beruf", "stellenbezeichnung", "jobtitel") or "Untitled job"
        company = _company(raw)
        location = _location(raw)
        remote_mode = remote_mode_from_payload(raw, location, title)
        date_value = first_value(
            raw,
            "aktuelleVeroeffentlichungsdatum",
            "veroeffentlichungsdatum",
            "veroeffentlicht",
            "datumErsteVeroeffentlichung",
            "aenderungsdatum",
            "publicationDate",
        ) or _period_start(raw.get("veroeffentlichungszeitraum"))
        url = first_value(raw, "externeUrl", "url")
        if not url and refnr:
            url = f"https://www.arbeitsagentur.de/jobsuche/jobdetail/{quote(str(refnr), safe='')}"
        return JobPosting(
            job_id=str(refnr) if refnr else None,
            title=str(title),
            company=str(company) if company else None,
            location=location,
            date_of_posting=date_value,
            source_website=self.name,
            url=str(url or "https://www.arbeitsagentur.de/jobsuche/"),
            search_term=search_term,
            remote_mode=remote_mode,
            raw_payload=raw,
        )


def _job_items(payload: Any) -> list[dict[str, Any]]:
    if isinstance(payload, list):
        return [item for item in payload if isinstance(item, dict)]
    if not isinstance(payload, dict):
        return []
    for key in ("ergebnisliste", "stellenangebote", "jobs", "results", "items"):
        value = payload.get(key)
        if isinstance(value, list):
            return [item for item in value if isinstance(item, dict)]
    embedded = payload.get("_embedded")
    if isinstance(embedded, dict):
        for value in embedded.values():
            if isinstance(value, list):
                return [item for item in value if isinstance(item, dict)]
    return []


def _company(raw: dict[str, Any]) -> str | None:
    arbeitgeber = raw.get("arbeitgeber")
    if isinstance(arbeitgeber, dict):
        return first_value(arbeitgeber, "name", "arbeitgeberName", "firma")
    return first_value(raw, "arbeitgeber", "firma", "unternehmen")


def _location(raw: dict[str, Any]) -> str | None:
    value = raw.get("arbeitsort") or raw.get("arbeitsOrt")
    if isinstance(value, dict):
        parts = [value.get(key) for key in ("ort", "region", "land")]
        return ", ".join(str(part) for part in parts if part)
    if isinstance(value, str):
        return value
    values = raw.get("arbeitsorte")
    if isinstance(values, list) and values:
        first = values[0]
        if isinstance(first, dict):
            parts = [first.get(key) for key in ("ort", "region", "land")]
            return ", ".join(str(part) for part in parts if part)
        return str(first)
    values = raw.get("stellenlokationen")
    if isinstance(values, list) and values:
        first = values[0]
        if isinstance(first, dict):
            address = first.get("adresse")
            if isinstance(address, dict):
                parts = [address.get(key) for key in ("ort", "region", "land")]
                return ", ".join(str(part) for part in parts if part)
            parts = [first.get(key) for key in ("ort", "region", "land")]
            return ", ".join(str(part) for part in parts if part)
    return first_value(raw, "ort", "location", "arbeitsortLabel")


def _period_start(value: Any) -> Any:
    if isinstance(value, dict):
        return first_value(value, "von", "from", "start")
    return None
