from __future__ import annotations

from typing import Any

from ..models import JobPosting
from .base import JobSource, first_value, location_matches, remote_mode_from_payload, title_matches, within_days


class GreenhouseSource(JobSource):
    name = "greenhouse"

    def search(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[JobPosting]:
        board_tokens = [str(token) for token in self.settings.get("board_tokens", []) if str(token).strip()]
        postings: list[JobPosting] = []
        for token in board_tokens:
            payload = self.get_json(self._endpoint(token), params={"content": "true"})
            jobs = payload.get("jobs", []) if isinstance(payload, dict) else []
            for raw in jobs:
                if not isinstance(raw, dict):
                    continue
                posting = self._map_job(raw, title, token)
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
        return [self.build_url(self._endpoint(str(token)), {"content": "true"}) for token in self.settings.get("board_tokens", [])]

    def _endpoint(self, token: str) -> str:
        return f"{self.settings['base_url'].rstrip('/')}/{token}/jobs"

    def _map_job(self, raw: dict[str, Any], search_term: str, token: str) -> JobPosting:
        offices = raw.get("offices") if isinstance(raw.get("offices"), list) else []
        office_names = [first_value(office, "location", "name") for office in offices if isinstance(office, dict)]
        location = ", ".join(str(item) for item in office_names if item) or None
        remote_mode = remote_mode_from_payload(raw, location, raw.get("content"), raw.get("title"))
        return JobPosting(
            job_id=str(first_value(raw, "id", "internal_job_id")) if first_value(raw, "id", "internal_job_id") else None,
            title=str(first_value(raw, "title") or "Untitled job"),
            company=str(first_value(raw, "company_name") or token),
            location=location,
            date_of_posting=first_value(raw, "updated_at", "published_at"),
            source_website=self.name,
            url=str(first_value(raw, "absolute_url", "url") or f"https://boards.greenhouse.io/{token}"),
            search_term=search_term,
            remote_mode=remote_mode,
            raw_payload=raw,
        )
