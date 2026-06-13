from __future__ import annotations

from typing import Any

from ..models import JobPosting
from .base import JobSource, first_value, location_matches, remote_mode_from_payload, title_matches, within_days


class TheMuseSource(JobSource):
    name = "themuse"

    def search(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[JobPosting]:
        endpoint = self.settings["base_url"]
        params = self._params(location, remote)
        payload = self.get_json(endpoint, params=params)
        results = payload.get("results", []) if isinstance(payload, dict) else []
        postings: list[JobPosting] = []
        for raw in results:
            if not isinstance(raw, dict):
                continue
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
        return [self.build_url(self.settings["base_url"], self._params(location, remote))]

    def _params(self, location: str, remote: bool) -> dict[str, Any]:
        params: dict[str, Any] = {"page": 1}
        params["location"] = "Remote" if remote else location
        return params

    def _map_job(self, raw: dict[str, Any], search_term: str) -> JobPosting:
        company = raw.get("company") if isinstance(raw.get("company"), dict) else {}
        locations = raw.get("locations") if isinstance(raw.get("locations"), list) else []
        location_names = [item.get("name") for item in locations if isinstance(item, dict) and item.get("name")]
        refs = raw.get("refs") if isinstance(raw.get("refs"), dict) else {}
        location = ", ".join(location_names) if location_names else None
        remote_mode = remote_mode_from_payload(raw, location, raw.get("contents"), raw.get("name"))
        return JobPosting(
            job_id=str(first_value(raw, "id")) if first_value(raw, "id") else None,
            title=str(first_value(raw, "name", "title") or "Untitled job"),
            company=str(first_value(company, "name")) if isinstance(company, dict) and first_value(company, "name") else None,
            location=location,
            date_of_posting=first_value(raw, "publication_date", "updated_at"),
            source_website=self.name,
            url=str(first_value(refs, "landing_page") if isinstance(refs, dict) else first_value(raw, "url") or "https://www.themuse.com/jobs"),
            search_term=search_term,
            remote_mode=remote_mode,
            raw_payload=raw,
        )
