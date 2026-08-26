from __future__ import annotations

from email.utils import parsedate_to_datetime
from typing import Any
from xml.etree import ElementTree

from ..models import JobPosting
from .base import JobSource, location_matches, remote_mode_from_payload, title_matches, within_days


class GermanTechJobsSource(JobSource):
    name = "germantechjobs"
    source_type = "rss"

    def __init__(self, settings: dict[str, Any], http) -> None:
        super().__init__(settings, http)
        self._items: list[dict[str, str]] | None = None

    def search(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[JobPosting]:
        results: list[JobPosting] = []
        seen: set[str] = set()
        for item in self._load_items():
            identity = item.get("guid") or item.get("link") or ""
            if identity in seen:
                continue
            seen.add(identity)
            item_title = item.get("title") or "Untitled job"
            item_location = item.get("location") or (location if location_matches(item.get("description"), location) else None)
            remote_mode = remote_mode_from_payload({}, item_location, item.get("description"), item_title)
            posting = JobPosting(
                job_id=item.get("guid"),
                title=item_title,
                company=item.get("company"),
                location=item_location,
                date_of_posting=_parse_rss_date(item.get("pubDate")),
                source_website=self.name,
                source_type="rss",
                url=item.get("link") or "https://germantechjobs.de/",
                search_term=title,
                remote_mode=remote_mode,
                raw_payload={key: value for key, value in item.items() if key != "description"},
            )
            if title_matches(posting.title, title) and within_days(posting.date_of_posting, days):
                if location_matches(posting.location or posting.title, location, posting.remote_mode) and (not remote or posting.remote_mode in {"remote", "hybrid"}):
                    results.append(posting)
                    if len(results) >= limit:
                        break
        return results

    def dry_run_urls(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[str]:
        return [self.settings.get("rss_url") or f"{self.settings['base_url'].rstrip('/')}/rss"]

    def _load_items(self) -> list[dict[str, str]]:
        if self._items is not None:
            return self._items
        url = self.settings.get("rss_url") or f"{self.settings['base_url'].rstrip('/')}/rss"
        xml = self.http.get_text(
            self.name,
            url,
            headers={"Accept": "application/rss+xml, application/xml, text/xml"},
            rate_limit_per_minute=self.rate_limit_per_minute,
            cooldown_seconds=self.cooldown_seconds,
        )
        try:
            root = ElementTree.fromstring(xml)
        except ElementTree.ParseError:
            self._items = []
            return self._items
        items: list[dict[str, str]] = []
        for element in root.iter():
            if element.tag.rsplit("}", 1)[-1] != "item":
                continue
            values = {
                child.tag.rsplit("}", 1)[-1]: (child.text or "").strip()
                for child in element
                if (child.text or "").strip()
            }
            if values.get("title") and values.get("link"):
                items.append(values)
        self._items = items
        return items


def _parse_rss_date(value: str | None):
    if not value:
        return None
    try:
        return parsedate_to_datetime(value)
    except (TypeError, ValueError, OverflowError):
        return value
