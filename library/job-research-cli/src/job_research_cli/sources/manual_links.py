from __future__ import annotations

from collections.abc import Callable

from ..models import ManualSearchLink
from ..normalizer import quote_query


def build_manual_link(source_name: str, title: str, location: str, remote: bool, days: int) -> ManualSearchLink | None:
    builders: dict[str, Callable[[str, str, bool, int], str]] = {
        "linkedin": _linkedin,
        "xing": _xing,
        "indeed": _indeed,
        "stepstone": _stepstone,
        "glassdoor": _glassdoor,
        "monster": _monster,
        "google_jobs": _google_jobs,
        "kununu": _kununu,
        "wellfound": _wellfound,
        "github_jobs": _github_jobs,
    }
    builder = builders.get(source_name)
    if builder is None:
        return None
    return ManualSearchLink(search_term=title, website=source_name, location=location, url=builder(title, location, remote, days))


def _query(title: str, location: str, remote: bool) -> str:
    query = title.strip()
    if remote and "remote" not in query.lower():
        query = f"{query} remote"
    return query


def _linkedin(title: str, location: str, remote: bool, days: int) -> str:
    params = [f"keywords={quote_query(_query(title, location, remote))}", f"location={quote_query(location)}"]
    if days > 0:
        params.append(f"f_TPR=r{days * 86400}")
    if remote:
        params.append("f_WT=2")
    return "https://www.linkedin.com/jobs/search/?" + "&".join(params)


def _xing(title: str, location: str, remote: bool, days: int) -> str:
    return f"https://www.xing.com/jobs/search?keywords={quote_query(_query(title, location, remote))}&location={quote_query(location)}"


def _indeed(title: str, location: str, remote: bool, days: int) -> str:
    params = [f"q={quote_query(_query(title, location, remote))}", f"l={quote_query(location)}"]
    if days > 0:
        params.append(f"fromage={days}")
    return "https://de.indeed.com/jobs?" + "&".join(params)


def _stepstone(title: str, location: str, remote: bool, days: int) -> str:
    return f"https://www.stepstone.de/jobs/{quote_query(_query(title, location, remote))}?where={quote_query(location)}"


def _glassdoor(title: str, location: str, remote: bool, days: int) -> str:
    return f"https://www.glassdoor.com/Job/jobs.htm?sc.keyword={quote_query(_query(title, location, remote))}&locKeyword={quote_query(location)}"


def _monster(title: str, location: str, remote: bool, days: int) -> str:
    return f"https://www.monster.de/jobs/suche?q={quote_query(_query(title, location, remote))}&where={quote_query(location)}"


def _google_jobs(title: str, location: str, remote: bool, days: int) -> str:
    freshness = f" posted last {days} days" if days > 0 else ""
    return f"https://www.google.com/search?q={quote_query(_query(title, location, remote) + ' jobs ' + location + freshness)}"


def _kununu(title: str, location: str, remote: bool, days: int) -> str:
    return f"https://www.kununu.com/de/search/jobs?query={quote_query(_query(title, location, remote))}&location={quote_query(location)}"


def _wellfound(title: str, location: str, remote: bool, days: int) -> str:
    return f"https://wellfound.com/jobs?query={quote_query(_query(title, location, remote))}&location={quote_query(location)}"


def _github_jobs(title: str, location: str, remote: bool, days: int) -> str:
    query = f"{_query(title, location, remote)} jobs {location} GitHub Jobs alternative"
    return f"https://www.google.com/search?q={quote_query(query)}"
