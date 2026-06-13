from __future__ import annotations

import os
from typing import Any

from ..models import JobPosting
from .base import JobSource, SourceAdapterError, first_value, location_matches, remote_mode_from_payload, title_matches, within_days


class AdzunaSource(JobSource):
    name = "adzuna"

    def is_configured(self) -> bool:
        return bool(self._credentials()[0] and self._credentials()[1])

    def search(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[JobPosting]:
        app_id, app_key = self._credentials()
        if not app_id or not app_key:
            raise SourceAdapterError("Adzuna requires ADZUNA_APP_ID and ADZUNA_APP_KEY.")
        endpoint = self._endpoint()
        params = self._params(title, location, remote, days, limit, app_id, app_key)
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
        return [self.build_url(self._endpoint(), self._params(title, location, remote, days, limit, "{ADZUNA_APP_ID}", "{ADZUNA_APP_KEY}"))]

    def _endpoint(self) -> str:
        country = self.settings.get("country") or "de"
        return f"{self.settings['base_url'].rstrip('/')}/jobs/{country}/search/1"

    def _params(
        self,
        title: str,
        location: str,
        remote: bool,
        days: int,
        limit: int,
        app_id: str,
        app_key: str,
    ) -> dict[str, Any]:
        what = f"{title} remote" if remote and "remote" not in title.lower() else title
        return {
            "app_id": app_id,
            "app_key": app_key,
            "what": what,
            "where": location,
            "results_per_page": min(max(limit, 1), 50),
            "max_days_old": max(days, 1),
            "sort_by": "date",
            "content-type": "application/json",
        }

    def _credentials(self) -> tuple[str | None, str | None]:
        app_id = os.environ.get(self.settings.get("app_id_env", "ADZUNA_APP_ID"))
        app_key = os.environ.get(self.settings.get("app_key_env", "ADZUNA_APP_KEY"))
        return app_id, app_key

    def _map_job(self, raw: dict[str, Any], search_term: str) -> JobPosting:
        company = raw.get("company") if isinstance(raw.get("company"), dict) else {}
        location = raw.get("location") if isinstance(raw.get("location"), dict) else {}
        location_name = first_value(location, "display_name", "area") if isinstance(location, dict) else None
        remote_mode = remote_mode_from_payload(raw, location_name, raw.get("description"), raw.get("title"))
        return JobPosting(
            job_id=str(first_value(raw, "id")) if first_value(raw, "id") else None,
            title=str(first_value(raw, "title") or "Untitled job"),
            company=str(first_value(company, "display_name")) if isinstance(company, dict) and first_value(company, "display_name") else None,
            location=str(location_name) if location_name else None,
            date_of_posting=first_value(raw, "created"),
            source_website=self.name,
            url=str(first_value(raw, "redirect_url", "adref") or "https://www.adzuna.de/"),
            search_term=search_term,
            remote_mode=remote_mode,
            raw_payload=raw,
        )
