from __future__ import annotations

from typing import Any

from ..models import JobPosting
from .base import JobSource, first_value, location_matches, remote_mode_from_payload, title_matches, within_days


class LeverSource(JobSource):
    name = "lever"

    def search(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[JobPosting]:
        slugs = [str(slug) for slug in self.settings.get("company_slugs", []) if str(slug).strip()]
        postings: list[JobPosting] = []
        for slug in slugs:
            payload = self.get_json(self._endpoint(slug), params={"mode": "json"})
            jobs = payload if isinstance(payload, list) else []
            for raw in jobs:
                if not isinstance(raw, dict):
                    continue
                posting = self._map_job(raw, title, slug)
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
                    return postings
        return postings

    def dry_run_urls(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[str]:
        return [self.build_url(self._endpoint(str(slug)), {"mode": "json"}) for slug in self.settings.get("company_slugs", [])]

    def _endpoint(self, slug: str) -> str:
        return f"{self.settings['base_url'].rstrip('/')}/{slug}"

    def _map_job(self, raw: dict[str, Any], search_term: str, slug: str) -> JobPosting:
        categories = raw.get("categories") if isinstance(raw.get("categories"), dict) else {}
        location = first_value(categories, "location", "team") if isinstance(categories, dict) else None
        remote_mode = remote_mode_from_payload(raw, location, raw.get("descriptionPlain"), raw.get("text"))
        return JobPosting(
            job_id=str(first_value(raw, "id")) if first_value(raw, "id") else None,
            title=str(first_value(raw, "text", "title") or "Untitled job"),
            company=slug,
            location=str(location) if location else None,
            date_of_posting=first_value(raw, "createdAt", "updatedAt"),
            source_website=self.name,
            url=str(first_value(raw, "hostedUrl", "applyUrl") or f"https://jobs.lever.co/{slug}"),
            search_term=search_term,
            remote_mode=remote_mode,
            raw_payload=raw,
        )
