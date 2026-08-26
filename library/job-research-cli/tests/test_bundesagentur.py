import pytest

from job_research_cli.sources.bundesagentur import BundesagenturSource


class FakeHttp:
    def __init__(self, payload=None, error=None):
        self.payload = payload
        self.error = error
        self.calls = []

    def get_json(self, source, url, **kwargs):
        self.calls.append((source, url, kwargs))
        if self.error:
            raise self.error
        return self.payload

    def build_url(self, url, params=None):
        return url


def source(http):
    return BundesagenturSource(
        {
            "base_url": "https://rest.arbeitsagentur.de/jobboerse/jobsuche-service",
            "default_api_key": "public-key",
        },
        http,
    )


def test_ba_search_normalizes_nested_job_and_request() -> None:
    http = FakeHttp(
        {
            "stellenangebote": [
                {
                    "referenznummer": "BA-1",
                    "stellenangebotsTitel": "Frontend Developer (m/w/d)",
                    "firma": "Acme GmbH",
                    "stellenlokationen": [{"adresse": {"ort": "Berlin", "land": "DEUTSCHLAND"}}],
                    "veroeffentlichungszeitraum": {"von": "2026-08-25"},
                    "externeURL": "https://jobs.example/ba-1",
                }
            ]
        }
    )

    results = source(http).search("Frontend Developer", "Berlin", False, 0, 10)

    assert len(results) == 1
    posting = results[0]
    assert posting.job_id == "BA-1"
    assert posting.company == "Acme GmbH"
    assert posting.location == "Berlin, DEUTSCHLAND"
    assert posting.url == "https://jobs.example/ba-1"
    assert posting.provenance == ["bundesagentur"]
    _, url, kwargs = http.calls[0]
    assert url.endswith("/pc/v6/jobs")
    assert kwargs["params"]["was"] == "Frontend Developer"
    assert kwargs["params"]["wo"] == "Berlin"
    assert kwargs["headers"] == {"X-API-Key": "public-key"}


@pytest.mark.parametrize("payload", [None, {}, {"stellenangebote": ["bad", 1]}])
def test_ba_search_handles_empty_and_malformed_payloads(payload) -> None:
    assert source(FakeHttp(payload)).search("Frontend Developer", "Berlin", False, 7, 10) == []


def test_ba_search_propagates_api_failure_for_orchestration() -> None:
    with pytest.raises(RuntimeError, match="unavailable"):
        source(FakeHttp(error=RuntimeError("unavailable"))).search("Frontend Developer", "Berlin", False, 7, 10)
