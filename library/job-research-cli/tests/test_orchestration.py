from job_research_cli import cli
from job_research_cli.http_client import _redact_url
from job_research_cli.models import JobPosting, SearchParameters


class FakeSource:
    source_type = "api"

    def __init__(self, name: str, calls: list[tuple[str, str]], failing_title: str | None = None) -> None:
        self.name = name
        self.calls = calls
        self.failing_title = failing_title

    def is_configured(self) -> bool:
        return True

    def search(self, title: str, location: str, remote: bool, days: int, limit: int) -> list[JobPosting]:
        self.calls.append((self.name, f"{title}/{location}"))
        if title == self.failing_title:
            raise RuntimeError("query failed")
        return [
            JobPosting(
                title=f"{self.name} {title}",
                company=self.name,
                location=location,
                source_website=self.name,
                url=f"https://example.com/{self.name}/{title}/{location}",
                search_term=title,
            )
        ]


def test_sources_are_ordered_and_failures_do_not_stop_queries_or_providers(monkeypatch) -> None:
    calls: list[tuple[str, str]] = []
    sources = {
        "provider_b": {"enabled": False, "type": "api", "base_url": "https://b.example"},
        "linkedin": {"enabled": True, "type": "manual_search_link"},
        "unknown": {"enabled": True, "type": "api", "base_url": "https://unknown.example"},
        "provider_a": {"enabled": True, "type": "api", "base_url": "https://a.example"},
    }
    adapters = {
        "provider_a": FakeSource("provider_a", calls, failing_title="Broken"),
        "provider_b": FakeSource("provider_b", calls),
    }

    class ProviderA(FakeSource):
        pass

    class ProviderB(FakeSource):
        pass

    monkeypatch.setattr(cli, "SOURCE_REGISTRY", {"provider_a": ProviderA, "provider_b": ProviderB})
    monkeypatch.setattr(cli, "make_source", lambda name, settings, http: adapters[name])
    monkeypatch.setattr(cli, "PoliteHttpClient", lambda: type("Http", (), {"close": lambda self: None})())

    selected = cli._select_sources(sources, ["linkedin", "provider_b", "provider_a", "unknown"])
    assert selected == ["provider_a", "provider_b", "unknown", "linkedin"]
    assert cli._select_sources(sources, list(reversed(selected))) == selected

    report = cli._run_search(
        sources,
        selected,
        SearchParameters(titles=["Broken", "Good"], locations=["Berlin", "Munich"], limit=20),
    )

    assert [name for name, _ in calls] == ["provider_a"] * 4 + ["provider_b"] * 4
    assert len(report.structured_results) == 6
    assert [outcome.source for outcome in report.provider_outcomes] == selected
    outcomes = {outcome.source: outcome for outcome in report.provider_outcomes}
    assert outcomes["provider_a"].status == "failed"
    assert outcomes["provider_a"].query_count == 4
    assert outcomes["provider_a"].failed_query_count == 2
    assert outcomes["provider_a"].result_count == 2
    assert outcomes["provider_b"].status == "queried"
    assert outcomes["provider_b"].result_count == 4
    assert outcomes["unknown"].status == "unavailable"
    assert outcomes["linkedin"].status == "manual-only"
    assert outcomes["linkedin"].result_count == 4
    assert len(report.manual_search_links) == 4
    assert len(report.errors) == 3


def test_provider_errors_redact_configured_credentials(monkeypatch) -> None:
    monkeypatch.setenv("ADZUNA_APP_KEY", "secret-key")

    message = cli._safe_error_message(
        RuntimeError("request failed for https://example.test?app_id=secret-id&app_key=secret-key"),
        {"app_key_env": "ADZUNA_APP_KEY"},
    )

    assert "secret-id" not in message
    assert "secret-key" not in message
    assert "[redacted]" in message


def test_http_debug_urls_redact_sensitive_query_values() -> None:
    safe_url = _redact_url("https://example.test/jobs?what=frontend&app_id=secret-id&app_key=secret-key&token=abc#token=fragment-secret")

    assert "what=frontend" in safe_url
    assert "secret-id" not in safe_url
    assert "secret-key" not in safe_url
    assert "fragment-secret" not in safe_url
    assert "app_id=%5Bredacted%5D" in safe_url
    assert "app_key=%5Bredacted%5D" in safe_url
    assert "token=%5Bredacted%5D" in safe_url
