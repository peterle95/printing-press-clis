from __future__ import annotations

from typing import Any

from ..models import JobPosting
from .base import JobSource, first_value, location_matches, remote_mode_from_payload, title_matches, within_days


class RemoteOKSource(JobSource):
    name = "remoteok"

    def search(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[JobPosting]:
        payload = self.get_json(self.settings["base_url"])
        items = payload if isinstance(payload, list) else []
        postings: list[JobPosting] = []
        for raw in items:
            if not isinstance(raw, dict) or not raw.get("position"):
                continue
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
        return [self.build_url(self.settings["base_url"])]

    def _map_job(self, raw: dict[str, Any], search_term: str) -> JobPosting:
        location = first_value(raw, "location", "region")
        remote_mode = remote_mode_from_payload(raw, location, "remote")
        return JobPosting(
            job_id=str(first_value(raw, "id", "slug")) if first_value(raw, "id", "slug") else None,
            title=str(first_value(raw, "position", "title") or "Untitled job"),
            company=str(first_value(raw, "company")) if first_value(raw, "company") else None,
            location=str(location) if location else "Remote",
            date_of_posting=first_value(raw, "date", "epoch"),
            source_website=self.name,
            url=str(first_value(raw, "url", "apply_url") or "https://remoteok.com/"),
            search_term=search_term,
            remote_mode=remote_mode or "remote",
            raw_payload=raw,
        )
