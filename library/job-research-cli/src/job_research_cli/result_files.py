from __future__ import annotations

import csv
import json
from pathlib import Path
from typing import Any

from .models import JobPosting


def load_postings_from_files(paths: list[Path]) -> list[JobPosting]:
    postings: list[JobPosting] = []
    for path in paths:
        postings.extend(load_postings_from_file(path))
    return postings


def load_postings_from_file(path: Path) -> list[JobPosting]:
    suffix = path.suffix.lower()
    if suffix == ".json":
        return _load_json(path)
    if suffix == ".csv":
        return _load_csv(path)
    if suffix in {".md", ".markdown"}:
        return _load_markdown(path)
    raise ValueError(f"Unsupported result file: {path}")


def _load_json(path: Path) -> list[JobPosting]:
    data = json.loads(path.read_text(encoding="utf-8"))
    if isinstance(data, dict):
        rows = data.get("structured_results", [])
    else:
        rows = data
    return [_posting_from_mapping(row) for row in rows if isinstance(row, dict)]


def _load_csv(path: Path) -> list[JobPosting]:
    with path.open("r", encoding="utf-8", newline="") as handle:
        rows = list(csv.DictReader(handle))
    postings: list[JobPosting] = []
    for row in rows:
        source_type = _pick(row, "source_type", "Source type")
        if source_type == "manual_search_link":
            continue
        postings.append(_posting_from_mapping(row))
    return postings


def _load_markdown(path: Path) -> list[JobPosting]:
    lines = path.read_text(encoding="utf-8").splitlines()
    in_structured = False
    headers: list[str] = []
    postings: list[JobPosting] = []
    for line in lines:
        stripped = line.strip()
        if stripped.lower() == "## structured results":
            in_structured = True
            continue
        if in_structured and stripped.startswith("## "):
            break
        if not in_structured or not stripped.startswith("|"):
            continue
        cells = [cell.strip() for cell in stripped.strip("|").split("|")]
        if not headers:
            headers = [_normalize_header(cell) for cell in cells]
            continue
        if all(set(cell) <= {"-", ":"} for cell in cells):
            continue
        if not any(cells):
            continue
        row = dict(zip(headers, cells, strict=False))
        if row.get("title"):
            postings.append(_posting_from_mapping(row))
    return postings


def _posting_from_mapping(row: dict[str, Any]) -> JobPosting:
    title = _pick(row, "title", "raw_title", "Title") or "Untitled job"
    url = _extract_markdown_link(_pick(row, "link", "url", "Link") or "")
    return JobPosting(
        job_id=_pick(row, "job_id", "id"),
        title=title,
        company=_pick(row, "company", "Company"),
        location=_pick(row, "location", "Location"),
        date_of_posting=_pick(row, "date_of_posting", "posted", "Posted"),
        source_website=_pick(row, "source_website", "website", "Website") or "imported",
        source_type="api",
        url=url or "about:blank",
        search_term=_pick(row, "matched_search_term", "search_term", "Matched search term") or "",
        remote_mode=_pick(row, "remote_mode", "remote", "Remote") or None,
        raw_payload=dict(row),
    )


def _pick(row: dict[str, Any], *keys: str) -> Any:
    for key in keys:
        if key in row and row[key] not in (None, ""):
            return row[key]
    lower_map = {str(key).lower(): value for key, value in row.items()}
    for key in keys:
        value = lower_map.get(key.lower())
        if value not in (None, ""):
            return value
    return None


def _extract_markdown_link(value: str) -> str:
    text = value.strip()
    if "](" in text and text.endswith(")"):
        return text.rsplit("](", 1)[1][:-1]
    return text


def _normalize_header(value: str) -> str:
    return value.lower().strip().replace(" ", "_")
