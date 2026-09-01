from __future__ import annotations

from datetime import date
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

from .normalizer import canonicalize_url, normalize_job_title, parse_date_value

SourceType = Literal["api", "rss", "public_page", "manual_search_link"]
RemoteMode = Literal["remote", "hybrid", "on-site"]
ProviderOutcomeStatus = Literal["queried", "retried", "manual-only", "unavailable", "failed"]


class JobPosting(BaseModel):
    model_config = ConfigDict(extra="ignore")

    job_id: str | None = None
    title: str
    company: str | None = None
    location: str | None = None
    date_of_posting: date | None = None
    description: str | None = None
    source_website: str
    source_type: SourceType = "api"
    url: str
    search_term: str
    remote_mode: RemoteMode | None = None
    raw_payload: dict[str, Any] = Field(default_factory=dict)
    normalized_title: str | None = None
    canonical_url: str | None = None
    provenance: list[str] = Field(default_factory=list)

    @field_validator("date_of_posting", mode="before")
    @classmethod
    def _parse_date(cls, value: object) -> date | None:
        return parse_date_value(value)

    @field_validator("job_id", "company", "location", "remote_mode", mode="before")
    @classmethod
    def _empty_to_none(cls, value: object) -> object | None:
        if value is None:
            return None
        if isinstance(value, str) and not value.strip():
            return None
        return value

    @model_validator(mode="after")
    def _derive_normalized_fields(self) -> "JobPosting":
        if self.normalized_title is None:
            self.normalized_title = normalize_job_title(self.title)
        if self.canonical_url is None:
            self.canonical_url = canonicalize_url(self.url)
        self.provenance = list(dict.fromkeys([*self.provenance, self.source_website]))
        return self

    @property
    def matched_search_term(self) -> str:
        return self.search_term


class ManualSearchLink(BaseModel):
    search_term: str
    website: str
    url: str
    location: str | None = None
    source_type: SourceType = "manual_search_link"


class SourceError(BaseModel):
    source: str
    message: str


class ProviderOutcome(BaseModel):
    source: str
    status: ProviderOutcomeStatus
    query_count: int = 0
    failed_query_count: int = 0
    result_count: int = 0
    error: str | None = None
    retry_authorized: bool = False
    retry_authorization_source: str | None = None
    retry_outcome: str | None = None
    stop_reason: str | None = None

    @property
    def provider(self) -> str:
        return self.source


class AuditRecord(BaseModel):
    event: str
    provider: str
    title: str | None = None
    location: str | None = None
    attempt: str | None = None
    authorized: bool | None = None
    authorization_source: str | None = None
    outcome: str | None = None
    result_count: int | None = None
    final_status: str | None = None
    stop_reason: str | None = None
    error: str | None = None


class SearchParameters(BaseModel):
    titles: list[str]
    locations: list[str]
    remote: bool = False
    days: int = 7
    limit: int = 50
    sources: list[str] = Field(default_factory=list)


class SearchReport(BaseModel):
    parameters: SearchParameters
    structured_results: list[JobPosting] = Field(default_factory=list)
    manual_search_links: list[ManualSearchLink] = Field(default_factory=list)
    provider_outcomes: list[ProviderOutcome] = Field(default_factory=list)
    audit_records: list[AuditRecord] = Field(default_factory=list)
    errors: list[SourceError] = Field(default_factory=list)

    @property
    def source_outcomes(self) -> list[ProviderOutcome]:
        return self.provider_outcomes
