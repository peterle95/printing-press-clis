from __future__ import annotations

from datetime import date
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

from .normalizer import canonicalize_url, normalize_job_title, parse_date_value

SourceType = Literal["api", "manual_search_link"]
RemoteMode = Literal["remote", "hybrid", "on-site"]


class JobPosting(BaseModel):
    model_config = ConfigDict(extra="ignore")

    job_id: str | None = None
    title: str
    company: str | None = None
    location: str | None = None
    date_of_posting: date | None = None
    source_website: str
    source_type: SourceType = "api"
    url: str
    search_term: str
    remote_mode: RemoteMode | None = None
    raw_payload: dict[str, Any] = Field(default_factory=dict)
    normalized_title: str | None = None
    canonical_url: str | None = None

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
    errors: list[SourceError] = Field(default_factory=list)
