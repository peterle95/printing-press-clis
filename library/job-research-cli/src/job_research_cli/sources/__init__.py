from __future__ import annotations

from typing import Any

from ..http_client import PoliteHttpClient
from .adzuna import AdzunaSource
from .arbeitnow import ArbeitnowSource
from .base import JobSource
from .bundesagentur import BundesagenturSource
from .greenhouse import GreenhouseSource
from .lever import LeverSource
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
}


def make_source(name: str, settings: dict[str, Any], http: PoliteHttpClient) -> JobSource:
    return SOURCE_REGISTRY[name](settings, http)
