from __future__ import annotations

from datetime import date, datetime, timezone
import html
import re
import unicodedata
from urllib.parse import parse_qsl, quote_plus, urlencode, urlsplit, urlunsplit

TRACKING_PARAMS = {
    "campaign",
    "fbclid",
    "gclid",
    "gh_src",
    "mc_cid",
    "mc_eid",
    "ref",
    "ref_src",
    "source",
    "src",
    "trackingid",
    "trk",
}

GENDER_MARKERS_RE = re.compile(
    r"\b(?:m\s*/\s*f\s*/\s*d|f\s*/\s*m\s*/\s*d|m\s*/\s*w\s*/\s*d|w\s*/\s*m\s*/\s*d|d\s*/\s*f\s*/\s*m|d\s*/\s*m\s*/\s*w|all genders|gn)\b",
    re.IGNORECASE,
)


def canonicalize_url(url: str | None) -> str | None:
    if not url:
        return None
    raw = url.strip()
    if not raw:
        return None

    parts = urlsplit(raw)
    if parts.scheme not in {"http", "https"} or not parts.netloc:
        return None

    scheme = parts.scheme.lower()
    host = (parts.hostname or "").lower()
    port = parts.port
    if port and not ((scheme == "http" and port == 80) or (scheme == "https" and port == 443)):
        netloc = f"{host}:{port}"
    else:
        netloc = host

    path = parts.path or ""
    if path != "/":
        path = path.rstrip("/")

    kept_params: list[tuple[str, str]] = []
    for key, value in parse_qsl(parts.query, keep_blank_values=True):
        key_lower = key.lower()
        if key_lower.startswith("utm_") or key_lower in TRACKING_PARAMS:
            continue
        kept_params.append((key, value))
    kept_params.sort(key=lambda item: (item[0].lower(), item[1]))
    query = urlencode(kept_params, doseq=True)
    return urlunsplit((scheme, netloc, path, query, ""))


def normalize_text(value: str | None) -> str:
    if value is None:
        return ""
    text = html.unescape(str(value))
    text = unicodedata.normalize("NFKD", text)
    text = "".join(ch for ch in text if not unicodedata.combining(ch))
    text = text.replace("&", " and ").lower()
    text = re.sub(r"[\u2010-\u2015]", "-", text)
    text = re.sub(r"[^a-z0-9+#.-]+", " ", text)
    return re.sub(r"\s+", " ", text).strip()


def normalize_job_title(value: str | None) -> str:
    text = html.unescape(value or "")
    text = GENDER_MARKERS_RE.sub(" ", text)
    text = re.sub(r"\((?:remote|hybrid|onsite|on-site|berlin|germany)\)", " ", text, flags=re.IGNORECASE)
    text = re.sub(r"[-_/]+", " ", text)
    return normalize_text(text)


def normalize_location(value: str | None) -> str:
    text = normalize_text(value)
    text = re.sub(r"\b\d{4,6}\b", " ", text)
    text = text.replace("deutschland", "germany")
    return re.sub(r"\s+", " ", text).strip(" ,")


def normalize_company(value: str | None) -> str:
    text = normalize_text(value)
    text = re.sub(r"\b(gmbh|ug|ag|se|inc|ltd|llc|corp|corporation)\b\.?", " ", text)
    return re.sub(r"\s+", " ", text).strip()


def infer_remote_mode(*values: object) -> str | None:
    text = " ".join(normalize_text(str(value)) for value in values if value)
    if not text:
        return None
    if re.search(r"\bhybrid\b|\bhome office\b.*\boffice\b|\bteilremote\b", text):
        return "hybrid"
    if re.search(r"\bremote\b|\bwork from home\b|\bhome office\b|\bremotely\b|\bremote germany\b|\bremote europe\b", text):
        return "remote"
    if re.search(r"\bon[- ]?site\b|\bin office\b|\bvor ort\b|\bprasenz\b", text):
        return "on-site"
    return None


def parse_date_value(value: object) -> date | None:
    if value is None or value == "":
        return None
    if isinstance(value, datetime):
        return value.astimezone(timezone.utc).date() if value.tzinfo else value.date()
    if isinstance(value, date):
        return value
    if isinstance(value, (int, float)):
        timestamp = float(value)
        if timestamp > 10_000_000_000:
            timestamp = timestamp / 1000
        try:
            return datetime.fromtimestamp(timestamp, tz=timezone.utc).date()
        except (OverflowError, OSError, ValueError):
            return None

    text = str(value).strip()
    if not text:
        return None
    if text.isdigit():
        return parse_date_value(int(text))

    candidates = [
        text,
        text.replace("Z", "+00:00"),
        text.split("T", 1)[0],
        text.split(" ", 1)[0],
    ]
    for candidate in candidates:
        try:
            parsed = datetime.fromisoformat(candidate)
            return parsed.date()
        except ValueError:
            pass
        try:
            return datetime.strptime(candidate, "%Y-%m-%d").date()
        except ValueError:
            pass
        try:
            return datetime.strptime(candidate, "%d.%m.%Y").date()
        except ValueError:
            pass
    return None


def quote_query(value: str) -> str:
    return quote_plus(value.strip())
