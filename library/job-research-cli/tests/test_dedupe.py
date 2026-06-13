from job_research_cli.dedupe import dedupe_postings, preferred_storage_key
from job_research_cli.models import JobPosting


def job(
    title: str,
    company: str = "Acme",
    location: str = "Berlin",
    url: str = "https://example.com/jobs/1",
    job_id: str | None = None,
    source: str = "example",
) -> JobPosting:
    return JobPosting(
        job_id=job_id,
        title=title,
        company=company,
        location=location,
        date_of_posting=None,
        source_website=source,
        url=url,
        search_term="frontend developer",
    )


def test_dedupe_prefers_canonical_url() -> None:
    postings = [
        job("Frontend Developer", url="https://example.com/jobs/1?utm_source=x"),
        job("Frontend Engineer", company="Other", url="https://EXAMPLE.com/jobs/1/"),
    ]

    assert len(dedupe_postings(postings)) == 1


def test_dedupe_uses_company_title_location_when_url_differs() -> None:
    postings = [
        job("Frontend Developer (m/f/d)", url="https://one.example/jobs/1"),
        job("Frontend Developer", url="https://two.example/jobs/2"),
    ]

    assert len(dedupe_postings(postings)) == 1


def test_dedupe_uses_source_specific_id_after_other_keys() -> None:
    postings = [
        job("Backend Developer", company="Acme", location="Berlin", url="about:blank", job_id="same", source="lever"),
        job("Backend Engineer", company="Other", location="Remote", url="about:blank", job_id="same", source="lever"),
    ]

    assert len(dedupe_postings(postings)) == 1


def test_preferred_storage_key_uses_canonical_url_first() -> None:
    posting = job("Frontend Developer", url="https://example.com/jobs/1?utm_campaign=x")

    assert preferred_storage_key(posting) == "url:https://example.com/jobs/1"
