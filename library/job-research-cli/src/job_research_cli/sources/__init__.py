from __future__ import annotations

from typing import Any

from ..http_client import PoliteHttpClient
from .adzuna import AdzunaSource
from .arbeitnow import ArbeitnowSource
from .base import JobSource
from .bundesagentur import BundesagenturSource
from .greenhouse import GreenhouseSource
from .germantechjobs import GermanTechJobsSource
from .lever import LeverSource
from .public_pages import (
    GitHubJobsSource,
    GlassdoorSource,
    GoogleJobsSource,
    IndeedSource,
    KununuSource,
    MonsterSource,
    StepstoneSource,
    WellfoundSource,
    XingSource,
)
from .remoteok import RemoteOKSource
from .themuse import TheMuseSource

SOURCE_REGISTRY: dict[str, type[JobSource]] = {
    "bundesagentur": BundesagenturSource,
    "arbeitnow": ArbeitnowSource,
    "adzuna": AdzunaSource,
    "themuse": TheMuseSource,
    "remoteok": RemoteOKSource,
    "greenhouse": GreenhouseSource,
    "lever": LeverSource,
    "germantechjobs": GermanTechJobsSource,
    "xing": XingSource,
    "indeed": IndeedSource,
    "stepstone": StepstoneSource,
    "glassdoor": GlassdoorSource,
    "monster": MonsterSource,
    "google_jobs": GoogleJobsSource,
    "kununu": KununuSource,
    "wellfound": WellfoundSource,
    "github_jobs": GitHubJobsSource,
}


def make_source(name: str, settings: dict[str, Any], http: PoliteHttpClient) -> JobSource:
    return SOURCE_REGISTRY[name](settings, http)
