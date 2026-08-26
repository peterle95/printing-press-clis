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
    assert any(
        record.event == "query_attempt"
        and record.provider == "provider_a"
        and record.attempt == "normal"
        and record.outcome == "failed"
        and record.error == "Broken / Berlin: query failed"
        for record in report.audit_records
    )
    final = next(record for record in report.audit_records if record.event == "final" and record.provider == "provider_a")
    assert final.final_status == "failed"
    assert final.result_count == 2


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


def test_rejected_provider_retries_once_when_explicitly_authorized(monkeypatch) -> None:
    calls: list[tuple[str, str]] = []

    class RejectedSource(FakeSource):
        def search(self, title, location, remote, days, limit):
            calls.append((self.name, f"normal:{title}/{location}"))
            raise RuntimeError("403 forbidden")

        def stealth_search(self, title, location, remote, days, limit):
            calls.append((self.name, f"retry:{title}/{location}"))
            return [
                JobPosting(
                    title=title,
                    company="Acme",
                    location=location,
                    source_website=self.name,
                    url="https://example.com/retried/1",
                    search_term=title,
                )
            ]

    class HealthySource(FakeSource):
        pass

    adapters = {
        "rejected": RejectedSource("rejected", calls),
        "healthy": HealthySource("healthy", calls),
    }
    settings = {
        name: {"enabled": True, "type": "api", "base_url": f"https://{name}.example"}
        for name in adapters
    }
    monkeypatch.setattr(cli, "SOURCE_REGISTRY", {"rejected": RejectedSource, "healthy": HealthySource})
    monkeypatch.setattr(cli, "make_source", lambda name, settings, http: adapters[name])
    monkeypatch.setattr(cli, "PoliteHttpClient", lambda: type("Http", (), {"close": lambda self: None})())

    report = cli._run_search(
        settings,
        ["healthy", "rejected"],
        SearchParameters(titles=["Frontend Developer"], locations=["Berlin"], limit=10),
        allow_stealth_retry=True,
    )

    assert calls == [
        ("healthy", "Frontend Developer/Berlin"),
        ("rejected", "normal:Frontend Developer/Berlin"),
        ("rejected", "retry:Frontend Developer/Berlin"),
    ]
    outcomes = {item.source: item for item in report.provider_outcomes}
    assert outcomes["rejected"].status == "retried"
    assert outcomes["rejected"].retry_authorization_source == "--allow-stealth-retry"
    assert outcomes["rejected"].retry_outcome == "succeeded"
    authorization = next(record for record in report.audit_records if record.event == "retry_authorization")
    assert authorization.authorized is True
    assert authorization.authorization_source == "--allow-stealth-retry"
    assert any(record.event == "query_attempt" and record.attempt == "retry" for record in report.audit_records)
    final = next(record for record in report.audit_records if record.event == "final" and record.provider == "rejected")
    assert final.final_status == "retried"
    assert outcomes["healthy"].status == "queried"


def test_rejected_provider_stops_remaining_queries_without_authorization(monkeypatch) -> None:
    calls: list[str] = []

    class RejectedSource(FakeSource):
        def search(self, title, location, remote, days, limit):
            calls.append(f"{title}/{location}")
            raise RuntimeError("captcha challenge")

    adapter = RejectedSource("rejected", [])
    monkeypatch.setattr(cli, "SOURCE_REGISTRY", {"rejected": RejectedSource})
    monkeypatch.setattr(cli, "make_source", lambda name, settings, http: adapter)
    monkeypatch.setattr(cli, "PoliteHttpClient", lambda: type("Http", (), {"close": lambda self: None})())

    report = cli._run_search(
        {"rejected": {"enabled": True, "type": "api", "base_url": "https://rejected.example"}},
        ["rejected"],
        SearchParameters(titles=["One", "Two"], locations=["Berlin", "Munich"], limit=10),
    )

    assert calls == ["One/Berlin"]
    outcome = report.provider_outcomes[0]
    assert outcome.retry_authorization_source == "non-interactive"
    assert outcome.retry_outcome == "declined"
    assert outcome.stop_reason == "access-control"
    assert any(
        record.event == "retry_authorization"
        and record.authorization_source == "non-interactive"
        and record.authorized is False
        for record in report.audit_records
    )
    assert any(record.event == "retry_outcome" and record.outcome == "declined" for record in report.audit_records)
