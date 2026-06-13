from job_research_cli.models import JobPosting
from job_research_cli.storage import JobStore


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
