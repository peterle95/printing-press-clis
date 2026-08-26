from __future__ import annotations

from email.utils import parsedate_to_datetime
from html import unescape
from html.parser import HTMLParser
import json
import re
from typing import Any
from xml.etree import ElementTree
from urllib.parse import urlencode, urljoin

from ..models import JobPosting
from .base import JobSource, SourceAdapterError, location_matches, remote_mode_from_payload, title_matches, within_days


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
            item_location = item.get("location")
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
                location_text = posting.location or item.get("description")
                if location_matches(location_text, location, posting.remote_mode) and (not remote or posting.remote_mode in {"remote", "hybrid"}):
                    results.append(posting)
                    if len(results) >= limit:
                        break
        return results

    def dry_run_urls(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[str]:
        return [self.settings.get("rss_url") or f"{self.settings['base_url'].rstrip('/')}/rss"]

    def stealth_search(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[JobPosting]:
        """Fetch one public search page after normal RSS access was rejected."""
        base_url = self.settings["base_url"].rstrip("/")
        search_url = self.settings.get("public_search_url") or f"{base_url}/jobs"
        url = f"{search_url}?{urlencode({'q': title, 'location': location})}"
        html = self.http.get_text(
            self.name,
            url,
            headers={"Accept": "text/html,application/xhtml+xml"},
            rate_limit_per_minute=self.rate_limit_per_minute,
            cooldown_seconds=max(self.cooldown_seconds, float(self.settings.get("retry_cooldown_seconds") or 0)),
        )
        if _has_access_control_signal(html):
            raise SourceAdapterError("public page reported access-control challenge")

        parser = _PublicJobParser(base_url)
        parser.feed(html)
        results: list[JobPosting] = []
        max_results = max(1, min(limit, int(self.settings.get("retry_max_results") or limit)))
        for item in parser.jobs:
            posting = JobPosting(
                job_id=item.get("job_id"),
                title=item.get("title") or "Untitled job",
                company=item.get("company"),
                location=item.get("location"),
                date_of_posting=_parse_rss_date(item.get("date")),
                source_website=self.name,
                source_type="public_page",
                url=item.get("url") or base_url,
                search_term=title,
                remote_mode=remote_mode_from_payload({}, item.get("location"), item.get("description"), item.get("title")),
                raw_payload=dict(item),
            )
            if title_matches(posting.title, title) and within_days(posting.date_of_posting, days):
                if location_matches(posting.location or item.get("description"), location, posting.remote_mode) and (not remote or posting.remote_mode in {"remote", "hybrid"}):
                    results.append(posting)
                    if len(results) >= max_results:
                        break
        return results

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


def _has_access_control_signal(html: str) -> bool:
    text = re.sub(r"<[^>]+>", " ", html).lower()
    title = re.search(r"<title[^>]*>(.*?)</title>", html, re.IGNORECASE | re.DOTALL)
    title_text = re.sub(r"<[^>]+>", " ", title.group(1)).lower() if title else ""
    return any(marker in f"{title_text} {text}" for marker in ("captcha", "access denied", "forbidden", "too many requests", "verify you are human", "log in to continue", "cloudflare", "just a moment", "security check"))


class _PublicJobParser(HTMLParser):
    def __init__(self, base_url: str) -> None:
        super().__init__()
        self.base_url = base_url
        self.jobs: list[dict[str, str]] = []
        self._json_depth = 0
        self._json_text: list[str] = []
        self._link: str | None = None
        self._link_text: list[str] = []

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        attributes = dict(attrs)
        if tag == "script" and attributes.get("type", "").lower() == "application/ld+json":
            self._json_depth = 1
            self._json_text = []
        href = attributes.get("href") or ""
        if tag == "a" and "/jobs/" in href:
            self._link = urljoin(self.base_url, href)
            self._link_text = []

    def handle_endtag(self, tag: str) -> None:
        if tag == "script" and self._json_depth:
            self._json_depth = 0
            self._add_json(self._json_text)
        if tag == "a" and self._link:
            title = _clean_text(" ".join(self._link_text))
            if title:
                self.jobs.append({"title": title, "url": self._link})
            self._link = None

    def handle_data(self, data: str) -> None:
        if self._json_depth:
            self._json_text.append(data)
        if self._link:
            self._link_text.append(data)

    def _add_json(self, chunks: list[str]) -> None:
        try:
            payload = json.loads("".join(chunks))
        except (TypeError, ValueError):
            return
        values = payload if isinstance(payload, list) else [payload]
        for value in values:
            if not isinstance(value, dict) or value.get("@type") != "JobPosting":
                continue
            employer = value.get("hiringOrganization")
            place = value.get("jobLocation")
            self.jobs.append({
                "job_id": str(value.get("identifier", {}).get("value", "")) if isinstance(value.get("identifier"), dict) else "",
                "title": str(value.get("title") or ""),
                "company": str(employer.get("name") or "") if isinstance(employer, dict) else "",
                "location": _location_from_json(place),
                "date": str(value.get("datePosted") or ""),
                "url": str(value.get("url") or ""),
                "description": re.sub(r"<[^>]+>", " ", str(value.get("description") or "")),
            })


def _location_from_json(value: object) -> str:
    if isinstance(value, list):
        return ", ".join(_location_from_json(item) for item in value)
    if isinstance(value, dict):
        address = value.get("address")
        if isinstance(address, dict):
            return ", ".join(str(address.get(key)) for key in ("addressLocality", "addressRegion", "addressCountry") if address.get(key))
        return str(value.get("name") or "")
    return str(value or "")


def _clean_text(value: str) -> str:
    return " ".join(unescape(value).split())
