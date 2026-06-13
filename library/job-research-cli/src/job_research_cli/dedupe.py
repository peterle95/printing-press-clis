from __future__ import annotations

from collections.abc import Iterable

from .models import JobPosting
from .normalizer import canonicalize_url, normalize_company, normalize_job_title, normalize_location


def posting_keys(posting: JobPosting) -> list[str]:
    keys: list[str] = []
    canonical = posting.canonical_url or canonicalize_url(posting.url)
    if canonical:
        keys.append(f"url:{canonical}")

    company = normalize_company(posting.company)
    title = posting.normalized_title or normalize_job_title(posting.title)
    location = normalize_location(posting.location)
    if company and title and location:
        keys.append(f"company-title-location:{company}|{title}|{location}")

    if posting.job_id:
        keys.append(f"source-id:{posting.source_website.lower()}|{posting.job_id}")
    return keys


def preferred_storage_key(posting: JobPosting) -> str:
    keys = posting_keys(posting)
    if keys:
        return keys[0]
    fallback = "|".join(
        [
            normalize_company(posting.company),
            normalize_job_title(posting.title),
            normalize_location(posting.location),
            posting.source_website.lower(),
        ]
    )
    return f"fallback:{fallback}"


def dedupe_postings(postings: Iterable[JobPosting], limit: int | None = None) -> list[JobPosting]:
    seen: set[str] = set()
    deduped: list[JobPosting] = []
    for posting in postings:
        keys = posting_keys(posting)
        if any(key in seen for key in keys):
            continue
        deduped.append(posting)
        seen.update(keys)
        if limit is not None and len(deduped) >= limit:
            break
    return deduped
