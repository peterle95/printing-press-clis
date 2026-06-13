from __future__ import annotations

import csv
from datetime import datetime
import io
import json
from pathlib import Path
from typing import Iterable

from .models import JobPosting, ManualSearchLink, SearchParameters, SearchReport


def export_report(report: SearchReport, output_format: str) -> str:
    output_format = output_format.lower()
    if output_format in {"markdown", "md"}:
        return to_markdown(report)
    if output_format == "csv":
        return to_csv(report.structured_results, report.manual_search_links)
    if output_format == "json":
        return to_json(report)
    raise ValueError(f"Unsupported export format: {output_format}")


def infer_format(requested_format: str, out: Path | None = None) -> str:
    if requested_format != "table":
        return requested_format
    if out is None:
        return "table"
    suffix = out.suffix.lower()
    if suffix in {".md", ".markdown"}:
        return "markdown"
    if suffix == ".csv":
        return "csv"
    if suffix == ".json":
        return "json"
    return "markdown"


def to_markdown(report: SearchReport, generated_at: datetime | None = None) -> str:
    generated_at = generated_at or datetime.now().replace(microsecond=0)
    params = report.parameters
    lines = [
        "# Job Research Results",
        "",
        f"Generated: {generated_at:%Y-%m-%d %H:%M}",
        "",
        "Search parameters:",
        f"- titles: {', '.join(params.titles)}",
        f"- location: {', '.join(params.locations)}",
        f"- days: {params.days}",
        f"- remote: {str(params.remote).lower()}",
        "",
        "## Structured results",
        "",
        "| Title | Company | Location | Posted | Website | Remote | Matched search term | Link |",
        "|---|---|---|---|---|---|---|---|",
    ]
    for posting in report.structured_results:
        lines.append(
            "| "
            + " | ".join(
                [
                    _md(posting.title),
                    _md(posting.company),
                    _md(posting.location),
                    _md(posting.date_of_posting.isoformat() if posting.date_of_posting else ""),
                    _md(posting.source_website),
                    _md(posting.remote_mode),
                    _md(posting.search_term),
                    _md_link(posting.url),
                ]
            )
            + " |"
        )
    if not report.structured_results:
        lines.append("|  |  |  |  |  |  |  |  |")

    lines.extend(
        [
            "",
            "## Manual search links",
            "",
            "| Search term | Website | Link |",
            "|---|---|---|",
        ]
    )
    for link in report.manual_search_links:
        term = link.search_term if not link.location else f"{link.search_term} ({link.location})"
        lines.append(f"| {_md(term)} | {_md(link.website)} | {_md_link(link.url)} |")
    if not report.manual_search_links:
        lines.append("|  |  |  |")

    if report.errors:
        lines.extend(["", "## Source errors", "", "| Source | Error |", "|---|---|"])
        for error in report.errors:
            lines.append(f"| {_md(error.source)} | {_md(error.message)} |")
    return "\n".join(lines) + "\n"


def to_csv(postings: Iterable[JobPosting], manual_links: Iterable[ManualSearchLink] = ()) -> str:
    output = io.StringIO()
    fieldnames = [
        "title",
        "company",
        "location",
        "date_of_posting",
        "source_website",
        "source_type",
        "link",
        "matched_search_term",
        "remote_mode",
    ]
    writer = csv.DictWriter(output, fieldnames=fieldnames)
    writer.writeheader()
    for posting in postings:
        writer.writerow(
            {
                "title": posting.title,
                "company": posting.company or "",
                "location": posting.location or "",
                "date_of_posting": posting.date_of_posting.isoformat() if posting.date_of_posting else "",
                "source_website": posting.source_website,
                "source_type": posting.source_type,
                "link": posting.url,
                "matched_search_term": posting.search_term,
                "remote_mode": posting.remote_mode or "",
            }
        )
    for link in manual_links:
        writer.writerow(
            {
                "title": "",
                "company": "",
                "location": link.location or "",
                "date_of_posting": "",
                "source_website": link.website,
                "source_type": link.source_type,
                "link": link.url,
                "matched_search_term": link.search_term,
                "remote_mode": "",
            }
        )
    return output.getvalue()


def to_json(report: SearchReport) -> str:
    return json.dumps(report.model_dump(mode="json"), indent=2, ensure_ascii=False) + "\n"


def _md(value: object) -> str:
    text = "" if value is None else str(value)
    return text.replace("|", "\\|").replace("\n", " ").strip()


def _md_link(url: str) -> str:
    safe_url = _md(url)
    return f"[open]({safe_url})"
