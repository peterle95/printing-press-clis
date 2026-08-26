from __future__ import annotations

from datetime import datetime, timezone
import json
from pathlib import Path
import shutil
import sqlite3
from typing import Any, Iterable

from .config import default_db_path
from .dedupe import dedupe_postings, preferred_storage_key
from .models import JobPosting
from .normalizer import canonicalize_url, normalize_company, normalize_job_title, normalize_location

SCHEMA = """
CREATE TABLE IF NOT EXISTS jobs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  job_id TEXT,
  normalized_title TEXT NOT NULL,
  raw_title TEXT NOT NULL,
  company TEXT,
  location TEXT,
  date_of_posting TEXT,
  source_website TEXT NOT NULL,
  source_type TEXT NOT NULL,
  url TEXT NOT NULL,
  canonical_url TEXT,
  search_term TEXT NOT NULL,
  remote_mode TEXT,
  first_seen_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  raw_payload_json TEXT NOT NULL,
  provenance_json TEXT NOT NULL DEFAULT '[]',
  dedupe_key TEXT NOT NULL UNIQUE
);

CREATE INDEX IF NOT EXISTS idx_jobs_last_seen ON jobs(last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_source ON jobs(source_website);
"""

STATUS_VERSION = 1
DEFAULT_STATUS_PATH = Path("job-status.json")


class JobStore:
    def __init__(self, db_path: Path | None = None) -> None:
        self.db_path = db_path or default_db_path()
        self.db_path.parent.mkdir(parents=True, exist_ok=True)
        self.init_db()

    def init_db(self) -> None:
        with self.connect() as conn:
            conn.executescript(SCHEMA)
            columns = {row["name"] for row in conn.execute("PRAGMA table_info(jobs)")}
            if "provenance_json" not in columns:
                conn.execute("ALTER TABLE jobs ADD COLUMN provenance_json TEXT NOT NULL DEFAULT '[]'")

    def connect(self) -> sqlite3.Connection:
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        return conn

    def upsert_postings(self, postings: Iterable[JobPosting]) -> tuple[int, int]:
        inserted = 0
        updated = 0
        now = _now()
        with self.connect() as conn:
            for posting in postings:
                key = preferred_storage_key(posting)
                existing = conn.execute(
                    "SELECT id, first_seen_at, provenance_json FROM jobs WHERE dedupe_key = ?", (key,)
                ).fetchone()
                payload = _posting_row(posting, key, now, now if existing is None else existing["first_seen_at"])
                if existing is not None:
                    payload["provenance_json"] = json.dumps(
                        list(
                            dict.fromkeys(
                                [*_parse_provenance(existing["provenance_json"]), *posting.provenance]
                            )
                        ),
                        ensure_ascii=False,
                    )
                if existing is None:
                    conn.execute(
                        """
                        INSERT INTO jobs (
                          job_id, normalized_title, raw_title, company, location, date_of_posting,
                          source_website, source_type, url, canonical_url, search_term, remote_mode,
                           first_seen_at, last_seen_at, raw_payload_json, provenance_json, dedupe_key
                        ) VALUES (
                          :job_id, :normalized_title, :raw_title, :company, :location, :date_of_posting,
                          :source_website, :source_type, :url, :canonical_url, :search_term, :remote_mode,
                           :first_seen_at, :last_seen_at, :raw_payload_json, :provenance_json, :dedupe_key
                        )
                        """,
                        payload,
                    )
                    inserted += 1
                else:
                    payload["row_id"] = existing["id"]
                    conn.execute(
                        """
                        UPDATE jobs
                        SET job_id = :job_id,
                            normalized_title = :normalized_title,
                            raw_title = :raw_title,
                            company = :company,
                            location = :location,
                            date_of_posting = :date_of_posting,
                            source_website = :source_website,
                            source_type = :source_type,
                            url = :url,
                            canonical_url = :canonical_url,
                            search_term = :search_term,
                            remote_mode = :remote_mode,
                            last_seen_at = :last_seen_at,
                             raw_payload_json = :raw_payload_json,
                             provenance_json = :provenance_json
                        WHERE id = :row_id
                        """,
                        payload,
                    )
                    updated += 1
        return inserted, updated

    def latest_postings(self, limit: int = 50) -> list[JobPosting]:
        with self.connect() as conn:
            rows = conn.execute(
                "SELECT * FROM jobs ORDER BY last_seen_at DESC, id DESC LIMIT ?",
                (limit,),
            ).fetchall()
        return [_row_to_posting(row) for row in rows]

    def all_postings(self) -> list[JobPosting]:
        with self.connect() as conn:
            rows = conn.execute("SELECT * FROM jobs ORDER BY first_seen_at ASC, id ASC").fetchall()
        return [_row_to_posting(row) for row in rows]

    def dedupe_database(self) -> tuple[int, int]:
        postings = self.all_postings()
        deduped = dedupe_postings(postings)
        removed = len(postings) - len(deduped)
        if removed <= 0:
            return len(postings), 0
        with self.connect() as conn:
            conn.execute("DELETE FROM jobs")
        self.upsert_postings(deduped)
        return len(deduped), removed

    def export_sqlite(self, out: Path) -> None:
        out.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(self.db_path, out)


class JobStatusStore:
    def __init__(self, path: Path | None = None) -> None:
        self.path = path or DEFAULT_STATUS_PATH

    def record_postings(self, postings: Iterable[JobPosting]) -> None:
        data = self._load()
        jobs = {str(job.get("identity")): job for job in data["jobs"] if job.get("identity")}
        now = _now()
        for posting in postings:
            identity = preferred_storage_key(posting)
            existing = jobs.get(identity)
            if existing is None:
                existing = {
                    "identity": identity,
                    "title": posting.title,
                    "company": posting.company,
                    "location": posting.location,
                    "source": posting.source_website,
                    "url": posting.url,
                    "status": "shown",
                    "first_seen_at": now,
                }
                jobs[identity] = existing
            else:
                existing.update(
                    title=posting.title,
                    company=posting.company,
                    location=posting.location,
                    source=posting.source_website,
                    url=posting.url,
                )
                existing.setdefault("first_seen_at", now)
            existing["provenance"] = list(dict.fromkeys([*(existing.get("provenance") or []), *posting.provenance]))
            existing["last_seen_at"] = now
            existing.setdefault("status", "shown")
        data["jobs"] = list(jobs.values())
        self._write(data)

    def mark_applied(self, identity_or_url: str) -> bool:
        data = self._load()
        for job in data["jobs"]:
            if job.get("identity") == identity_or_url or job.get("url") == identity_or_url:
                job["status"] = "applied"
                job.setdefault("applied_at", _now())
                self._write(data)
                return True
        return False

    def _load(self) -> dict[str, Any]:
        if not self.path.exists():
            return {"version": STATUS_VERSION, "jobs": []}
        raw = json.loads(self.path.read_text(encoding="utf-8"))
        if not isinstance(raw, dict) or not isinstance(raw.get("jobs"), list):
            raise ValueError(f"Invalid job status file: {self.path}")
        jobs = []
        for job in raw["jobs"]:
            if not isinstance(job, dict):
                continue
            job = dict(job)
            job.setdefault("identity", _status_identity(job))
            jobs.append(job)
        return {"version": STATUS_VERSION, "jobs": jobs}

    def _write(self, data: dict[str, Any]) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        temporary = self.path.with_name(f".{self.path.name}.tmp")
        temporary.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        temporary.replace(self.path)


def _posting_row(posting: JobPosting, key: str, last_seen: str, first_seen: str) -> dict[str, object]:
    return {
        "job_id": posting.job_id,
        "normalized_title": posting.normalized_title or "",
        "raw_title": posting.title,
        "company": posting.company,
        "location": posting.location,
        "date_of_posting": posting.date_of_posting.isoformat() if posting.date_of_posting else None,
        "source_website": posting.source_website,
        "source_type": posting.source_type,
        "url": posting.url,
        "canonical_url": posting.canonical_url,
        "search_term": posting.search_term,
        "remote_mode": posting.remote_mode,
        "first_seen_at": first_seen,
        "last_seen_at": last_seen,
        "raw_payload_json": json.dumps(posting.raw_payload, ensure_ascii=False),
        "provenance_json": json.dumps(posting.provenance, ensure_ascii=False),
        "dedupe_key": key,
    }


def _row_to_posting(row: sqlite3.Row) -> JobPosting:
    try:
        raw_payload = json.loads(row["raw_payload_json"] or "{}")
    except json.JSONDecodeError:
        raw_payload = {}
    try:
        provenance = json.loads(row["provenance_json"] or "[]")
    except (json.JSONDecodeError, IndexError):
        provenance = []
    return JobPosting(
        job_id=row["job_id"],
        title=row["raw_title"],
        company=row["company"],
        location=row["location"],
        date_of_posting=row["date_of_posting"],
        source_website=row["source_website"],
        source_type=row["source_type"],
        url=row["url"],
        search_term=row["search_term"],
        remote_mode=row["remote_mode"],
        raw_payload=raw_payload,
        normalized_title=row["normalized_title"],
        canonical_url=row["canonical_url"],
        provenance=provenance if isinstance(provenance, list) else [],
    )


def _now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat()


def _status_identity(job: dict[str, Any]) -> str:
    try:
        canonical = canonicalize_url(job.get("url"))
    except ValueError:
        canonical = None
    if canonical:
        return f"url:{canonical}"
    source = str(job.get("source") or job.get("source_website") or "").lower()
    if job.get("job_id"):
        return f"source-id:{source}|{job['job_id']}"
    fallback = "|".join(
        [
            normalize_company(job.get("company")),
            normalize_job_title(job.get("title")),
            normalize_location(job.get("location")),
            source,
        ]
    )
    return f"fallback:{fallback}"


def _parse_provenance(value: object) -> list[str]:
    try:
        parsed = json.loads(value or "[]") if isinstance(value, str) else value
    except json.JSONDecodeError:
        return []
    return [str(item) for item in parsed if str(item).strip()] if isinstance(parsed, list) else []
