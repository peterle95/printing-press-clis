from __future__ import annotations

from dataclasses import dataclass
import logging
import time
from typing import Any

import httpx

LOGGER = logging.getLogger(__name__)


class HttpClientError(RuntimeError):
    pass


@dataclass
class RateState:
    last_call: float = 0.0


class PoliteHttpClient:
    def __init__(
        self,
        *,
        timeout_seconds: float = 15,
        retries: int = 2,
        user_agent: str = "job-research-cli/0.1 (+https://github.com/mvanhorn/cli-printing-press)",
    ) -> None:
        self.retries = retries
        self._states: dict[str, RateState] = {}
        self._client = httpx.Client(
            timeout=httpx.Timeout(timeout_seconds),
            headers={"User-Agent": user_agent, "Accept": "application/json"},
            follow_redirects=True,
        )

    def close(self) -> None:
        self._client.close()

    def build_url(self, url: str, params: dict[str, Any] | None = None) -> str:
        request = self._client.build_request("GET", url, params=params)
        return str(request.url)

    def get_json(
        self,
        source_name: str,
        url: str,
        *,
        params: dict[str, Any] | None = None,
        headers: dict[str, str] | None = None,
        rate_limit_per_minute: int = 20,
        cooldown_seconds: float = 0,
    ) -> Any:
        self._wait_for_slot(source_name, rate_limit_per_minute, cooldown_seconds)
        backoff = 1.0
        last_error: Exception | None = None
        for attempt in range(self.retries + 1):
            try:
                LOGGER.debug("GET %s", self.build_url(url, params))
                response = self._client.get(url, params=params, headers=headers)
                if response.status_code in {429, 500, 502, 503, 504}:
                    if attempt < self.retries:
                        retry_after = _retry_after_seconds(response.headers.get("Retry-After"))
                        sleep_seconds = retry_after if retry_after is not None else backoff
                        LOGGER.debug(
                            "%s returned %s; retrying in %.1fs",
                            source_name,
                            response.status_code,
                            sleep_seconds,
                        )
                        time.sleep(sleep_seconds)
                        backoff *= 2
                        continue
                response.raise_for_status()
                if not response.content:
                    return None
                return response.json()
            except (httpx.TimeoutException, httpx.NetworkError, httpx.HTTPStatusError, ValueError) as exc:
                last_error = exc
                if attempt < self.retries and _is_retryable(exc):
                    time.sleep(backoff)
                    backoff *= 2
                    continue
                break
        raise HttpClientError(f"{source_name}: request failed: {last_error}") from last_error

    def _wait_for_slot(self, source_name: str, rate_limit_per_minute: int, cooldown_seconds: float) -> None:
        state = self._states.setdefault(source_name, RateState())
        min_interval = 60.0 / max(rate_limit_per_minute, 1)
        wait_seconds = max(min_interval, cooldown_seconds) - (time.monotonic() - state.last_call)
        if wait_seconds > 0:
            LOGGER.debug("Waiting %.1fs before next %s request", wait_seconds, source_name)
            time.sleep(wait_seconds)
        state.last_call = time.monotonic()


def _retry_after_seconds(value: str | None) -> float | None:
    if not value:
        return None
    try:
        return max(float(value), 0.0)
    except ValueError:
        return None


def _is_retryable(exc: Exception) -> bool:
    if isinstance(exc, (httpx.TimeoutException, httpx.NetworkError)):
        return True
    if isinstance(exc, httpx.HTTPStatusError):
        return exc.response.status_code in {429, 500, 502, 503, 504}
    return False
