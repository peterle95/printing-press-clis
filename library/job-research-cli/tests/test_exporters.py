from job_research_cli.exporters import to_csv, to_json, to_markdown
from job_research_cli.models import JobPosting, ManualSearchLink, ProviderOutcome, SearchParameters, SearchReport


def test_markdown_export_has_structured_and_manual_sections() -> None:
    report = SearchReport(
        parameters=SearchParameters(
            titles=["Frontend Developer"],
            locations=["Berlin"],
            remote=False,
            days=7,
            limit=10,
            sources=["arbeitnow", "linkedin"],
        ),
        structured_results=[
            JobPosting(
                title="Frontend Developer",
                company="Acme",
                location="Berlin",
                date_of_posting="2026-06-01",
                source_website="arbeitnow",
                url="https://example.com/jobs/1",
                search_term="Frontend Developer",
                remote_mode="hybrid",
            )
        ],
        manual_search_links=[
            ManualSearchLink(
                search_term="Frontend Developer",
                website="linkedin",
                location="Berlin",
                url="https://www.linkedin.com/jobs/search/?keywords=Frontend+Developer&location=Berlin",
            )
        ],
        provider_outcomes=[ProviderOutcome(source="arbeitnow", status="queried", query_count=1, result_count=1)],
    )

    markdown = to_markdown(report)

    assert "# Job Research Results" in markdown
    assert "## Structured results" in markdown
    assert "## Manual search links" in markdown
    assert "## Provider outcomes" in markdown
    assert "queried" in markdown
    assert "Frontend Developer" in markdown
    assert "linkedin" in markdown
    json_text = to_json(report)
    assert '"provider_outcomes"' in json_text
    assert '"source": "arbeitnow"' in json_text
    assert '"status": "queried"' in json_text
    assert '"query_count": 1' in json_text
    assert '"result_count": 1' in json_text


def test_csv_export_includes_manual_rows_marked_as_manual_search_link() -> None:
    csv_text = to_csv(
        [],
        [
            ManualSearchLink(
                search_term="React Developer",
                website="indeed",
                location="Berlin",
                url="https://de.indeed.com/jobs?q=React+Developer&l=Berlin",
            )
        ],
    )

    assert "manual_search_link" in csv_text
    assert "React Developer" in csv_text


def test_csv_export_neutralizes_formula_cells() -> None:
    posting = JobPosting(
        title="=HYPERLINK(\"https://attacker.example\")",
        description="@SUM(A1:A2)",
        source_website="indeed",
        source_type="public_page",
        url="https://example.com/jobs/1",
        search_term="Frontend Developer",
    )

    csv_text = to_csv([posting])

    assert "'=HYPERLINK" in csv_text
    assert "'@SUM(A1:A2)" in csv_text


def test_csv_export_neutralizes_whitespace_prefixed_formula_cells() -> None:
    posting = JobPosting(
        title="\t=1+1",
        source_website="indeed",
        source_type="public_page",
        url="https://example.com/jobs/1",
        search_term="Frontend Developer",
    )

    csv_text = to_csv([posting])

    assert "'\t=1+1" in csv_text
