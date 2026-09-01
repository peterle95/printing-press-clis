from __future__ import annotations

import pytest
from scrapling import Selector

from job_research_cli.sources.base import SourceAdapterError
from job_research_cli.sources import SOURCE_REGISTRY
from job_research_cli.sources.public_pages import GitHubJobsSource, PublicJobBoardSource


class FakeHttp:
    def __init__(self, robots: str = "User-agent: *\nAllow: /\n") -> None:
        self.robots = robots
        self.robots_urls: list[str] = []
        self.waits: list[str] = []

    def get_text(self, source_name: str, url: str, **kwargs) -> str:
        self.robots_urls.append(url)
        return self.robots

    def wait_for_slot(self, source_name: str, **kwargs) -> None:
        self.waits.append(source_name)

    def build_url(self, url: str, params=None) -> str:
        return url


class FakePage:
    status = 200

    def __init__(self, html: str) -> None:
        self.selector = Selector(html)

    def css(self, selector: str):
        return self.selector.css(selector)


def _source(http: FakeHttp) -> PublicJobBoardSource:
    return PublicJobBoardSource(
        {
            "base_url": "https://de.indeed.com",
            "selectors": {
                "card": ["article.job"],
                "title": ["a.title::text"],
                "company": [".company::text"],
                "location": [".location::text"],
                "date": ["time::attr(datetime)"],
                "description": [".description::text"],
                "link": ["a.title::attr(href)"],
            },
        },
        http,
    )


def test_public_page_source_checks_robots_and_normalizes_search_result(monkeypatch) -> None:
    http = FakeHttp()
    source = _source(http)
    page = FakePage(
        """
        <article class="job">
          <a class="title" href="/viewjob?jk=1">Frontend Developer</a>
          <span class="company">Acme</span>
          <span class="location">Berlin</span>
          <time datetime="2026-08-28"></time>
          <p class="description">Build accessible web apps.</p>
        </article>
        """
    )
    monkeypatch.setattr("job_research_cli.sources.public_pages.Fetcher.get", lambda *args, **kwargs: page)

    results = source.search("Frontend Developer", "Berlin", False, 7, 10)

    assert http.robots_urls == ["https://de.indeed.com/robots.txt"]
    assert http.waits == ["indeed"]
    assert len(results) == 1
    assert results[0].title == "Frontend Developer"
    assert results[0].company == "Acme"
    assert results[0].location == "Berlin"
    assert results[0].date_of_posting.isoformat() == "2026-08-28"
    assert results[0].description == "Build accessible web apps."
    assert results[0].url == "https://de.indeed.com/viewjob?jk=1"
    assert results[0].source_type == "public_page"


def test_public_page_source_fails_closed_when_robots_disallows_search(monkeypatch) -> None:
    source = _source(FakeHttp("User-agent: *\nDisallow: /jobs\n"))
    monkeypatch.setattr(
        "job_research_cli.sources.public_pages.Fetcher.get",
        lambda *args, **kwargs: (_ for _ in ()).throw(AssertionError("fetch must not run")),
    )

    with pytest.raises(SourceAdapterError, match="robots.txt disallows"):
        source.search("Frontend Developer", "Berlin", False, 7, 10)


def test_browser_retry_uses_normal_renderer_without_bypass_options(monkeypatch) -> None:
    source = _source(FakeHttp())
    page = FakePage('<article class="job"><a class="title" href="/viewjob?jk=1">Frontend Developer</a></article>')
    kwargs: dict[str, object] = {}

    def fetch(*args, **received_kwargs):
        kwargs.update(received_kwargs)
        return page

    monkeypatch.setattr("job_research_cli.sources.public_pages.DynamicFetcher.fetch", fetch)

    results = source.stealth_search("Frontend Developer", "Berlin", False, 7, 10)

    assert len(results) == 1
    assert kwargs["google_search"] is False
    assert "solve_cloudflare" not in kwargs
    assert "proxy" not in kwargs
    assert kwargs["retries"] == 1


def test_browser_retry_blocks_robots_disallowed_redirect(monkeypatch) -> None:
    source = _source(FakeHttp("User-agent: *\nDisallow: /blocked\n"))
    page = FakePage('<article class="job"><a class="title" href="/viewjob?jk=1">Frontend Developer</a></article>')

    class Route:
        request = type("Request", (), {"resource_type": "document", "url": "https://de.indeed.com/blocked"})()

        def __init__(self) -> None:
            self.aborted = False

        def abort(self) -> None:
            self.aborted = True

        def continue_(self) -> None:
            raise AssertionError("disallowed redirect must not continue")

    def fetch(*args, **kwargs):
        browser_page = type("BrowserPage", (), {"route": lambda self, pattern, handler: handler(Route())})()
        kwargs["page_setup"](browser_page)
        return page

    monkeypatch.setattr("job_research_cli.sources.public_pages.DynamicFetcher.fetch", fetch)

    with pytest.raises(SourceAdapterError, match="robots.txt disallows"):
        source.stealth_search("Frontend Developer", "Berlin", False, 7, 10)


def test_static_redirect_checks_target_robots_and_resolves_relative_links(monkeypatch) -> None:
    http = FakeHttp()
    source = _source(http)
    redirect = type("Redirect", (), {"status": 302, "headers": {"location": "https://jobs.example.test/results"}, "body": b""})()
    page = FakePage('<article class="job"><a class="title" href="/viewjob?jk=1">Frontend Developer</a></article>')
    responses = iter([redirect, page])
    monkeypatch.setattr("job_research_cli.sources.public_pages.Fetcher.get", lambda *args, **kwargs: next(responses))

    results = source.search("Frontend Developer", "Berlin", False, 7, 10)

    assert http.robots_urls == ["https://de.indeed.com/robots.txt", "https://jobs.example.test/robots.txt"]
    assert http.waits == ["indeed", "indeed"]
    assert results[0].url == "https://jobs.example.test/viewjob?jk=1"


def test_public_page_source_rejects_non_web_job_links(monkeypatch) -> None:
    source = _source(FakeHttp())
    page = FakePage('<article class="job"><a class="title" href="javascript:alert(1)">Frontend Developer</a></article>')
    monkeypatch.setattr("job_research_cli.sources.public_pages.Fetcher.get", lambda *args, **kwargs: page)

    with pytest.raises(SourceAdapterError, match="no recognizable job listings"):
        source.search("Frontend Developer", "Berlin", False, 7, 10)


def test_github_jobs_reports_retired_service_without_request() -> None:
    source = GitHubJobsSource({"base_url": "https://jobs.github.com"}, FakeHttp())

    assert source.unavailable_reason == "GitHub Jobs was retired and has no public search service."


def test_all_requested_boards_have_public_page_adapters() -> None:
    names = {"xing", "indeed", "stepstone", "glassdoor", "monster", "google_jobs", "kununu", "wellfound", "github_jobs"}

    assert names <= SOURCE_REGISTRY.keys()
    assert all(SOURCE_REGISTRY[name].source_type == "public_page" for name in names)
    assert all(SOURCE_REGISTRY[name].selectors["card"][0] != "article" for name in names - {"github_jobs"})
