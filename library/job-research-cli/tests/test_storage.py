from job_research_cli.models import JobPosting
import json

from job_research_cli.storage import JobStatusStore, JobStore


def test_storage_upserts_by_dedupe_key(tmp_path) -> None:
    store = JobStore(tmp_path / "jobs.db")
    posting = JobPosting(
        title="Frontend Developer",
        company="Acme",
        location="Berlin",
        date_of_posting="2026-06-01",
        source_website="arbeitnow",
        url="https://example.com/jobs/1?utm_source=mail",
        search_term="Frontend Developer",
    )

    inserted, updated = store.upsert_postings([posting])
    assert (inserted, updated) == (1, 0)

    inserted, updated = store.upsert_postings([posting])
    assert (inserted, updated) == (0, 1)
    assert len(store.latest_postings()) == 1


def test_storage_round_trips_provenance(tmp_path) -> None:
    store = JobStore(tmp_path / "jobs.db")
    posting = JobPosting(
        title="Frontend Developer",
        company="Acme",
        location="Berlin",
        source_website="arbeitnow",
        url="https://example.com/jobs/1",
        search_term="Frontend Developer",
        provenance=["arbeitnow", "greenhouse"],
    )

    store.upsert_postings([posting])

    assert store.latest_postings()[0].provenance == ["arbeitnow", "greenhouse"]


def test_storage_round_trips_description(tmp_path) -> None:
    store = JobStore(tmp_path / "jobs.db")
    posting = JobPosting(
        title="Frontend Developer",
        company="Acme",
        location="Berlin",
        description="Build accessible web apps.",
        source_website="indeed",
        source_type="public_page",
        url="https://example.com/jobs/1",
        search_term="Frontend Developer",
    )

    store.upsert_postings([posting])

    assert store.latest_postings()[0].description == "Build accessible web apps."


def test_status_store_versions_writes_and_preserves_applied(tmp_path) -> None:
    path = tmp_path / "job-status.json"
    store = JobStatusStore(path)
    posting = JobPosting(
        title="Frontend Developer",
        company="Acme",
        location="Berlin",
        source_website="arbeitnow",
        url="https://example.com/jobs/1",
        search_term="Frontend Developer",
    )

    store.record_postings([posting])
    assert store.mark_applied(posting.url)
    first_applied_at = json.loads(path.read_text())["jobs"][0]["applied_at"]
    store.record_postings([posting])

    data = json.loads(path.read_text())
    assert data["version"] == 1
    assert data["jobs"][0]["status"] == "applied"
    assert data["jobs"][0]["applied_at"] == first_applied_at
    assert data["jobs"][0]["first_seen_at"]
    assert data["jobs"][0]["last_seen_at"]


def test_status_store_reads_legacy_jobs(tmp_path) -> None:
    path = tmp_path / "job-status.json"
    path.write_text(json.dumps({"jobs": [{"title": "Frontend Developer", "url": "https://example.com/jobs/1", "status": "shown"}]}))

    store = JobStatusStore(path)
    assert store.mark_applied("https://example.com/jobs/1")
    data = json.loads(path.read_text())
    assert data["version"] == 1
    assert data["jobs"][0]["status"] == "applied"


def test_sqlite_upsert_preserves_prior_provenance(tmp_path) -> None:
    store = JobStore(tmp_path / "jobs.db")
    first = JobPosting(
        title="Frontend Developer",
        company="Acme",
        location="Berlin",
        source_website="arbeitnow",
        url="https://example.com/jobs/1",
        search_term="Frontend Developer",
        provenance=["arbeitnow", "greenhouse"],
    )
    second = first.model_copy(update={"provenance": ["arbeitnow"]})

    store.upsert_postings([first])
    store.upsert_postings([second])

    assert store.latest_postings()[0].provenance == ["arbeitnow", "greenhouse"]
