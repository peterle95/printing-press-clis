from __future__ import annotations

from html import unescape
import json
import re
from typing import Any
from urllib.parse import urljoin, urlsplit

from protego import Protego
from scrapling.fetchers import Fetcher, StealthyFetcher

from ..models import JobPosting
from .base import JobSource, SourceAdapterError, location_matches, remote_mode_from_payload, title_matches, within_days
from .manual_links import build_manual_link

_ROBOTS_USER_AGENT = "job-research-cli"
_PUBLIC_HEADERS = {
    "User-Agent": "job-research-cli/0.1 (+https://github.com/mvanhorn/cli-printing-press)",
    "Accept": "text/html,application/xhtml+xml",
    "Accept-Language": "de-DE,de;q=0.9,en;q=0.8",
}
_ACCESS_CONTROL_MARKERS = (
    "captcha",
    "access denied",
    "forbidden",
    "too many requests",
    "verify you are human",
    "log in to continue",
    "cloudflare",
    "just a moment",
    "security check",
)
_COMMON_SELECTORS = {
    "card": ["[data-testid='job-card']", "[data-testid='job-listing']", "article"],
    "title": ["[data-testid='job-title']::text", "h2 a::text", "h2::text", "a::text"],
    "company": ["[data-testid='company-name']::text", ".company::text"],
    "location": ["[data-testid='text-location']::text", ".location::text"],
    "date": ["time::attr(datetime)", ".date::text"],
    "description": ["[data-testid='job-snippet']::text", ".description::text"],
    "link": ["a[data-testid='job-title']::attr(href)", "h2 a::attr(href)", "a::attr(href)"],
}


def _board_selectors(**overrides: list[str]) -> dict[str, list[str]]:
    return {field: [*overrides.get(field, []), *defaults] for field, defaults in _COMMON_SELECTORS.items()}


class PublicJobBoardSource(JobSource):
    source_type = "public_page"
    name = "indeed"
    selectors = _COMMON_SELECTORS

    def __init__(self, settings: dict[str, Any], http) -> None:
        super().__init__(settings, http)
        self._robots: dict[str, Protego] = {}

    def search(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[JobPosting]:
        url = self._search_url(title, location, remote, days)
        page, page_url = self._fetch_page(url)
        return self._parse_page(page, page_url, title, location, remote, days, limit)

    def stealth_search(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[JobPosting]:
        """Render one publicly permitted results page after operator approval."""
        url = self._search_url(title, location, remote, days)
        page, page_url = self._fetch_page(url, browser=True)
        return self._parse_page(page, page_url, title, location, remote, days, limit)

    def dry_run_urls(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[str]:
        return [self._search_url(title, location, remote, days)]

    def _search_url(self, title: str, location: str, remote: bool, days: int) -> str:
        link = build_manual_link(self.name, title, location, remote, days)
        if link is None:
            raise SourceAdapterError("No public search URL is configured.")
        return link.url

    def _fetch_page(self, url: str, *, browser: bool = False):
        if browser:
            self.http.wait_for_slot(
                self.name,
                rate_limit_per_minute=self.rate_limit_per_minute,
                cooldown_seconds=self.cooldown_seconds,
            )
            blocked: list[SourceAdapterError] = []

            def gate_document_redirects(browser_page) -> None:
                def route_request(route) -> None:
                    route.continue_()

                browser_page.route("**/*", route_request)

            try:
                page = StealthyFetcher.fetch(
                    url,
                    solve_cloudflare=True,
                    hide_canvas=True,
                    block_webrtc=True,
                    real_chrome=False,
                    proxy=None,
                    page_setup=gate_document_redirects,
                )
            except Exception as exc:
                if blocked:
                    raise blocked[0] from exc
                raise SourceAdapterError(f"browser-rendered public-page request failed: {exc}") from exc
            if blocked:
                raise blocked[0]
            page_url = str(getattr(page, "url", url) or url)
            self._validate_page(page)
            return page, page_url

        current_url = url
        for _ in range(3):
            self.http.wait_for_slot(
                self.name,
                rate_limit_per_minute=self.rate_limit_per_minute,
                cooldown_seconds=self.cooldown_seconds,
            )
            try:
                page = Fetcher.get(
                    current_url,
                    headers=_PUBLIC_HEADERS,
                    timeout=float(self.settings.get("timeout_seconds") or 15),
                    retries=0,
                    follow_redirects=False,
                    stealthy_headers=False,
                    impersonate=None,
                )
            except Exception as exc:
                raise SourceAdapterError(f"public-page request failed: {exc}") from exc
            status = int(getattr(page, "status", 0) or 0)
            if 300 <= status < 400:
                headers = getattr(page, "headers", {})
                redirect = headers.get("location") or headers.get("Location")
                if not redirect:
                    raise SourceAdapterError(f"public page returned HTTP {status} without a redirect location")
                current_url = urljoin(current_url, redirect)
                continue
            self._validate_page(page)
            return page, current_url
        raise SourceAdapterError("public page redirected too many times")

    def _validate_page(self, page) -> None:
        status = int(getattr(page, "status", 0) or 0)
        if status >= 400:
            raise SourceAdapterError(f"public page returned HTTP {status}")
        body = getattr(page, "body", b"")
        text = body.decode("utf-8", errors="ignore") if isinstance(body, bytes) else str(body)
        if _has_access_control_signal(text):
            raise SourceAdapterError("public page reported access-control challenge")

    def _parse_page(
        self,
        page,
        page_url: str,
        title: str,
        location: str,
        remote: bool,
        days: int,
        limit: int,
    ) -> list[JobPosting]:
        records = [*_json_ld_records(page), *_card_records(page, self._configured_selectors())]
        results: list[JobPosting] = []
        seen: set[str] = set()
        recognized = False
        for record in records:
            posting = self._posting(record, page_url, title)
            if posting is None:
                continue
            recognized = True
            identity = posting.canonical_url or posting.url
            if identity in seen:
                continue
            seen.add(identity)
            if not title_matches(posting.title, title) or not within_days(posting.date_of_posting, days):
                continue
            if not location_matches(posting.location or posting.description, location, posting.remote_mode):
                continue
            if remote and posting.remote_mode not in {"remote", "hybrid"}:
                continue
            results.append(posting)
            if len(results) >= limit:
                break
        if not recognized:
            raise SourceAdapterError("public page has no recognizable job listings")
        return results

    def _configured_selectors(self) -> dict[str, list[str]]:
        configured = self.settings.get("selectors")
        if not isinstance(configured, dict):
            return self.selectors
        selectors: dict[str, list[str]] = {}
        for field, defaults in self.selectors.items():
            value = configured.get(field, defaults)
            if isinstance(value, str):
                selectors[field] = [value]
            elif isinstance(value, list):
                selectors[field] = [str(item) for item in value if isinstance(item, str) and item.strip()]
            else:
                selectors[field] = defaults
        return selectors

    def _posting(self, record: dict[str, str], page_url: str, search_term: str) -> JobPosting | None:
        raw_title = _clean(record.get("title"))
        raw_url = _clean(record.get("url"))
        if not raw_title or not raw_url:
            return None
        url = urljoin(page_url, raw_url)
        parsed_url = urlsplit(url)
        if parsed_url.scheme not in {"http", "https"} or not parsed_url.netloc:
            return None
        location = _clean(record.get("location"))
        description = _clean(record.get("description"))
        remote_mode = remote_mode_from_payload({}, raw_title, location, description)
        return JobPosting(
            job_id=_clean(record.get("job_id")) or None,
            title=raw_title,
            company=_clean(record.get("company")) or None,
            location=location or None,
            date_of_posting=_clean(record.get("date")) or None,
            description=description or None,
            source_website=self.name,
            source_type="public_page",
            url=url,
            search_term=search_term,
            remote_mode=remote_mode,
            raw_payload=record,
        )


class XingSource(PublicJobBoardSource):
    name = "xing"
    selectors = _board_selectors(
        card=["[data-testid='job-listing']"],
        title=["[data-testid='job-title']::text"],
        company=["[data-testid='company-name']::text"],
        location=["[data-testid='job-location']::text"],
        link=["a[data-testid='job-title']::attr(href)"],
    )


class IndeedSource(PublicJobBoardSource):
    name = "indeed"
    selectors = _board_selectors(
        card=["div.job_seen_beacon", "div.cardOutline"],
        title=["h2.jobTitle span::text", "h2.jobTitle a::text"],
        company=["[data-testid='company-name']::text"],
        location=["[data-testid='text-location']::text"],
        description=["div.job-snippet::text"],
        link=["h2.jobTitle a::attr(href)"],
    )


class StepstoneSource(PublicJobBoardSource):
    name = "stepstone"
    selectors = _board_selectors(
        card=["article[data-testid='job-item']", "[data-at='job-item']"],
        title=["[data-testid='job-item-title']::text", "[data-at='job-item-title']::text"],
        company=["[data-testid='job-item-company']::text", "[data-at='job-item-company']::text"],
        location=["[data-testid='job-item-location']::text", "[data-at='job-item-location']::text"],
        link=["a[data-testid='job-item-title']::attr(href)", "a[data-at='job-item-title']::attr(href)"],
    )


class GlassdoorSource(PublicJobBoardSource):
    name = "glassdoor"
    selectors = _board_selectors(
        card=["li[data-test='jobListing']"],
        title=["a[data-test='job-link']::text"],
        company=["div[data-test='employer-name']::text"],
        location=["div[data-test='emp-location']::text"],
        description=["div[data-test='job-description']::text"],
        link=["a[data-test='job-link']::attr(href)"],
    )


class MonsterSource(PublicJobBoardSource):
    name = "monster"
    selectors = _board_selectors(
        card=["article[data-testid='job-card']"],
        title=["a[data-testid='job-card-title']::text"],
        company=["[data-testid='company']::text"],
        location=["[data-testid='job-location']::text"],
        description=["[data-testid='job-description']::text"],
        link=["a[data-testid='job-card-title']::attr(href)"],
    )


class GoogleJobsSource(PublicJobBoardSource):
    name = "google_jobs"
    selectors = _board_selectors(
        card=["[data-attrid='Job']", "[role='listitem']"],
        title=["[role='heading']::text"],
        company=[".vNEEBe::text"],
        location=[".Qk80Jf::text"],
        link=["a::attr(href)"],
    )


class KununuSource(PublicJobBoardSource):
    name = "kununu"
    selectors = _board_selectors(
        card=["[data-testid='job-item']"],
        title=["[data-testid='job-title']::text"],
        company=["[data-testid='company-name']::text"],
        location=["[data-testid='job-location']::text"],
        link=["a[data-testid='job-title']::attr(href)"],
    )


class WellfoundSource(PublicJobBoardSource):
    name = "wellfound"
    selectors = _board_selectors(
        card=["[data-test='JobListing']", "[data-testid='job-card']"],
        title=["[data-test='JobListing-title']::text"],
        company=["[data-test='JobListing-company']::text"],
        location=["[data-test='JobListing-location']::text"],
        link=["a[href^='/jobs/']::attr(href)"],
    )


class GitHubJobsSource(JobSource):
    name = "github_jobs"
    source_type = "public_page"
    unavailable_reason = "GitHub Jobs was retired and has no public search service."

    def search(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[JobPosting]:
        raise SourceAdapterError(self.unavailable_reason)


def _card_records(page, selectors: dict[str, list[str]]) -> list[dict[str, str]]:
    records: list[dict[str, str]] = []
    for selector in selectors["card"]:
        cards = page.css(selector)
        if cards:
            for card in cards:
                record = {field: _first_selector_value(card, values) for field, values in selectors.items() if field != "card"}
                if record.get("title") and record.get("link"):
                    record["url"] = record.pop("link")
                    records.append(record)
            if records:
                return records
    return records


def _json_ld_records(page) -> list[dict[str, str]]:
    records: list[dict[str, str]] = []
    for script in page.css("script[type='application/ld+json']::text").getall():
        try:
            payload = json.loads(script)
        except (TypeError, ValueError):
            continue
        for item in _walk_json(payload):
            types = item.get("@type")
            type_values = types if isinstance(types, list) else [types]
            if "JobPosting" not in type_values:
                continue
            identifier = item.get("identifier")
            records.append(
                {
                    "job_id": str(identifier.get("value") or "") if isinstance(identifier, dict) else str(identifier or ""),
                    "title": str(item.get("title") or ""),
                    "company": _organization_name(item.get("hiringOrganization")),
                    "location": _json_location(item.get("jobLocation")),
                    "date": str(item.get("datePosted") or ""),
                    "description": _clean(str(item.get("description") or "")),
                    "url": str(item.get("url") or ""),
                }
            )
    return records


def _walk_json(value: object):
    if isinstance(value, list):
        for item in value:
            yield from _walk_json(item)
    elif isinstance(value, dict):
        yield value
        for item in value.values():
            if isinstance(item, (dict, list)):
                yield from _walk_json(item)


def _first_selector_value(node, selectors: list[str]) -> str:
    for selector in selectors:
        value = node.css(selector).get()
        if value is not None and _clean(str(value)):
            return _clean(str(value))
    return ""


def _organization_name(value: object) -> str:
    if isinstance(value, dict):
        return str(value.get("name") or "")
    return str(value or "")


def _json_location(value: object) -> str:
    if isinstance(value, list):
        return ", ".join(filter(None, (_json_location(item) for item in value)))
    if not isinstance(value, dict):
        return str(value or "")
    address = value.get("address")
    if isinstance(address, dict):
        return ", ".join(str(address.get(key)) for key in ("addressLocality", "addressRegion", "addressCountry") if address.get(key))
    return str(value.get("name") or "")


def _has_access_control_signal(html: str) -> bool:
    text = re.sub(r"<[^>]+>", " ", html).lower()
    return any(marker in text for marker in _ACCESS_CONTROL_MARKERS)


def _clean(value: object) -> str:
    text = re.sub(r"<[^>]+>", " ", str(value or ""))
    return " ".join(unescape(text).split())
