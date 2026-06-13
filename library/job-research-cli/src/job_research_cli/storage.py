from __future__ import annotations

from datetime import datetime, timezone
import json
from pathlib import Path
import shutil
import sqlite3
from typing import Iterable

from .config import default_db_path
from .dedupe import dedupe_postings, preferred_storage_key
from .models import JobPosting

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
  dedupe_key TEXT NOT NULL UNIQUE
);

CREATE INDEX IF NOT EXISTS idx_jobs_last_seen ON jobs(last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_source ON jobs(source_website);
"""


class JobStore:
    def __init__(self, db_path: Path | None = None) -> None:
        self.db_path = db_path or default_db_path()
        self.db_path.parent.mkdir(parents=True, exist_ok=True)
        self.init_db()

    def init_db(self) -> None:
        with self.connect() as conn:
            conn.executescript(SCHEMA)

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
                existing = conn.execute("SELECT id, first_seen_at FROM jobs WHERE dedupe_key = ?", (key,)).fetchone()
                payload = _posting_row(posting, key, now, now if existing is None else existing["first_seen_at"])
                if existing is None:
                    conn.execute(
                        """
                        INSERT INTO jobs (
                          job_id, normalized_title, raw_title, company, location, date_of_posting,
                          source_website, source_type, url, canonical_url, search_term, remote_mode,
                          first_seen_at, last_seen_at, raw_payload_json, dedupe_key
                        ) VALUES (
                          :job_id, :normalized_title, :raw_title, :company, :location, :date_of_posting,
                          :source_website, :source_type, :url, :canonical_url, :search_term, :remote_mode,
                          :first_seen_at, :last_seen_at, :raw_payload_json, :dedupe_key
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
                            raw_payload_json = :raw_payload_json
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
        "dedupe_key": key,
    }


def _row_to_posting(row: sqlite3.Row) -> JobPosting:
    try:
        raw_payload = json.loads(row["raw_payload_json"] or "{}")
    except json.JSONDecodeError:
        raw_payload = {}
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
    )


def _now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat()
