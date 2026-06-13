from __future__ import annotations

from typing import Any

from ..models import JobPosting
from .base import JobSource, first_value, location_matches, remote_mode_from_payload, title_matches, within_days


class ArbeitnowSource(JobSource):
    name = "arbeitnow"

    def __init__(self, settings: dict[str, Any], http) -> None:
        super().__init__(settings, http)
        self._page_cache: dict[int, list[dict[str, Any]]] = {}

    def search(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[JobPosting]:
        postings: list[JobPosting] = []
        for raw in self._load_pages():
            posting = self._map_job(raw, title)
            if not title_matches(posting.title, title):
                continue
            if remote and posting.remote_mode not in {"remote", "hybrid"}:
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
        base_url = self.settings["base_url"]
        max_pages = int(self.settings.get("max_pages") or 1)
        return [self.build_url(base_url, {"page": page}) for page in range(1, max_pages + 1)]

    def _load_pages(self) -> list[dict[str, Any]]:
        all_jobs: list[dict[str, Any]] = []
        max_pages = int(self.settings.get("max_pages") or 1)
        for page in range(1, max_pages + 1):
            if page not in self._page_cache:
                payload = self.get_json(self.settings["base_url"], params={"page": page})
                data = payload.get("data", []) if isinstance(payload, dict) else []
                self._page_cache[page] = [item for item in data if isinstance(item, dict)]
            all_jobs.extend(self._page_cache[page])
        return all_jobs

    def _map_job(self, raw: dict[str, Any], search_term: str) -> JobPosting:
        title = first_value(raw, "title", "position") or "Untitled job"
        company = first_value(raw, "company_name", "company")
        location = first_value(raw, "location", "city")
        remote_mode = remote_mode_from_payload(raw, location, title, raw.get("tags"))
        url = first_value(raw, "url", "job_url")
        return JobPosting(
            job_id=str(first_value(raw, "slug", "id")) if first_value(raw, "slug", "id") else None,
            title=str(title),
            company=str(company) if company else None,
            location=str(location) if location else None,
            date_of_posting=first_value(raw, "created_at", "published_at"),
            source_website=self.name,
            url=str(url or "https://www.arbeitnow.com/jobs"),
            search_term=search_term,
            remote_mode=remote_mode,
            raw_payload=raw,
        )
