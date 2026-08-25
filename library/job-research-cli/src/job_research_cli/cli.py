from __future__ import annotations

from dataclasses import dataclass
from enum import Enum
import logging
import os
from pathlib import Path
import re
import webbrowser

import typer
from rich.console import Console
from rich.table import Table

from .config import (
    append_env_values,
    default_db_path,
    load_runtime_config,
    load_titles_file,
    write_default_config,
)
from .dedupe import dedupe_postings
from .exporters import export_report, infer_format
from .http_client import PoliteHttpClient
from .models import JobPosting, ManualSearchLink, ProviderOutcome, SearchParameters, SearchReport, SourceError
from .result_files import load_postings_from_files
from .sources import SOURCE_REGISTRY, make_source
from .sources.manual_links import build_manual_link
from .storage import JobStore

app = typer.Typer(help="Safe job research CLI for Berlin/Germany software searches.")
console = Console()


class SearchFormat(str, Enum):
    table = "table"
    csv = "csv"
    json = "json"
    markdown = "markdown"


class ExportFormat(str, Enum):
    markdown = "markdown"
    csv = "csv"
    json = "json"
    sqlite = "sqlite"


@dataclass(frozen=True)
class SourceDiagnostic:
    name: str
    enabled: bool
    capability: str
    status: str
    guidance: str


_SOURCE_CAPABILITIES = {
    "api": "API",
    "rss": "RSS",
    "public_page": "permitted public-page",
    "permitted_public_page": "permitted public-page",
    "manual_search_link": "manual-only",
}
_SOURCE_TYPE_ORDER = {
    "api": 0,
    "rss": 1,
    "public_page": 2,
    "permitted_public_page": 2,
    "manual_search_link": 3,
}


@app.command("init")
def init_command(
    config_dir: Path | None = typer.Option(None, "--config-dir", help="Config directory. Defaults to the user config directory."),
    overwrite: bool = typer.Option(False, "--overwrite", help="Overwrite existing config files."),
    no_prompt: bool = typer.Option(False, "--no-prompt", help="Skip optional API key prompts."),
) -> None:
    """Create config files and optionally store API env values outside the repo."""
    written = write_default_config(config_dir, overwrite=overwrite)
    _, _, resolved_config_dir = load_runtime_config(config_dir)
    env_values: dict[str, str] = {}
    if not no_prompt:
        app_id = typer.prompt("Optional Adzuna app ID", default="", show_default=False)
        app_key = typer.prompt("Optional Adzuna app key", default="", show_default=False, hide_input=True)
        if app_id:
            env_values["ADZUNA_APP_ID"] = app_id
        if app_key:
            env_values["ADZUNA_APP_KEY"] = app_key
    env_path = append_env_values(resolved_config_dir, env_values)

    console.print(f"Config directory: {resolved_config_dir}")
    console.print(f"SQLite database: {default_db_path()}")
    if written:
        for path in written:
            console.print(f"Created {path}")
    else:
        console.print("Config files already exist.")
    if env_path:
        console.print(f"Stored optional API env values in {env_path}")


@app.command("search")
def search_command(
    title: str | None = typer.Option(None, "--title", "-t", help="Single job title to search."),
    titles_file: Path | None = typer.Option(None, "--titles", help="Text/YAML file containing job titles."),
    location: str | None = typer.Option(None, "--location", "-l", help="Location. Defaults to configured locations."),
    remote: bool = typer.Option(False, "--remote", help="Prefer remote/hybrid results when available."),
    days: int = typer.Option(7, "--days", min=0, help="Only include known posting dates from the last N days."),
    limit: int = typer.Option(50, "--limit", min=1, help="Maximum deduplicated structured results."),
    source: str | None = typer.Option(None, "--source", help="Comma-separated source names."),
    output_format: SearchFormat = typer.Option(SearchFormat.table, "--format", help="table, csv, json, or markdown."),
    out: Path | None = typer.Option(None, "--out", help="Write results to a file."),
    dry_run: bool = typer.Option(False, "--dry-run", help="Show source capabilities, setup status, and URLs without HTTP requests."),
    verbose: bool = typer.Option(False, "--verbose", help="Enable debug logs."),
    config_dir: Path | None = typer.Option(None, "--config-dir", help="Config directory."),
    db: Path | None = typer.Option(None, "--db", help="SQLite database path."),
) -> None:
    """Search enabled safe sources and generate manual search links."""
    _configure_logging(verbose)
    sources_config, titles_config, _ = load_runtime_config(config_dir)
    titles = _resolve_titles(title, titles_file, titles_config)
    locations = [location] if location else list(titles_config.get("locations") or ["Berlin"])
    requested_sources = _parse_source_filter(source)
    source_settings = sources_config.get("sources", {})
    selected_sources = _select_sources(source_settings, requested_sources)
    parameters = SearchParameters(titles=titles, locations=locations, remote=remote, days=days, limit=limit, sources=selected_sources)

    if dry_run:
        diagnostic_sources = _order_sources(source_settings, list(source_settings))
        _print_dry_run(source_settings, selected_sources, diagnostic_sources, parameters)
        return

    report = _run_search(source_settings, selected_sources, parameters)
    store = JobStore(db)
    if report.structured_results:
        inserted, updated = store.upsert_postings(report.structured_results)
        if verbose:
            console.print(f"Stored {inserted} new and updated {updated} existing rows in {store.db_path}")

    resolved_format = infer_format(output_format.value, out)
    if out:
        _write_report(report, resolved_format, out)
        console.print(f"Wrote {out}")
        if report.errors:
            _print_errors(report.errors)
        return
    if resolved_format == "table":
        _print_structured_table(report.structured_results)
        _print_manual_links(report.manual_search_links)
        _print_provider_outcomes(report.provider_outcomes)
        _print_errors(report.errors)
    else:
        console.print(export_report(report, resolved_format), markup=False, end="")


@app.command("open")
def open_command(
    latest: int = typer.Option(10, "--latest", min=1, help="Open the latest N stored structured job links."),
    db: Path | None = typer.Option(None, "--db", help="SQLite database path."),
) -> None:
    """Open latest stored job links in the browser."""
    store = JobStore(db)
    postings = [posting for posting in store.latest_postings(latest) if posting.url]
    if not postings:
        console.print("No stored job links found. Run `jobs search` first.")
        return
    for posting in postings:
        webbrowser.open(posting.url)
        console.print(f"Opened: {posting.title} - {posting.company or posting.source_website}")


@app.command("dedupe")
def dedupe_command(
    paths: list[Path] | None = typer.Argument(None, help="Result files to dedupe. If omitted, dedupes the SQLite store."),
    out: Path | None = typer.Option(None, "--out", help="Write deduped file output."),
    output_format: SearchFormat = typer.Option(SearchFormat.markdown, "--format", help="csv, json, or markdown for file dedupe."),
    db: Path | None = typer.Option(None, "--db", help="SQLite database path when no files are passed."),
) -> None:
    """Deduplicate stored jobs or old result files."""
    if not paths:
        store = JobStore(db)
        kept, removed = store.dedupe_database()
        console.print(f"SQLite dedupe complete: kept {kept}, removed {removed}.")
        return

    postings = load_postings_from_files(paths)
    deduped = dedupe_postings(postings)
    report = SearchReport(
        parameters=SearchParameters(titles=[], locations=[], remote=False, days=0, limit=len(deduped), sources=["imported"]),
        structured_results=deduped,
    )
    resolved_format = infer_format(output_format.value, out)
    if resolved_format == "table":
        resolved_format = "markdown"
    if out:
        _write_report(report, resolved_format, out)
        console.print(f"Deduped {len(postings)} rows to {len(deduped)} rows in {out}")
    else:
        console.print(export_report(report, resolved_format), markup=False, end="")


@app.command("export")
def export_command(
    output_format: ExportFormat = typer.Option(ExportFormat.markdown, "--format", help="markdown, csv, json, or sqlite."),
    out: Path | None = typer.Option(None, "--out", help="Output path."),
    limit: int = typer.Option(100, "--limit", min=1, help="Maximum rows for non-SQLite exports."),
    db: Path | None = typer.Option(None, "--db", help="SQLite database path."),
) -> None:
    """Export stored results to Markdown, CSV, JSON, or SQLite."""
    store = JobStore(db)
    if output_format == ExportFormat.sqlite:
        if out is None:
            raise typer.BadParameter("--out is required for SQLite export")
        store.export_sqlite(out)
        console.print(f"Exported SQLite database to {out}")
        return

    postings = store.latest_postings(limit)
    report = SearchReport(
        parameters=SearchParameters(titles=[], locations=[], remote=False, days=0, limit=limit, sources=["stored"]),
        structured_results=postings,
    )
    text = export_report(report, output_format.value)
    if out:
        out.parent.mkdir(parents=True, exist_ok=True)
        out.write_text(text, encoding="utf-8")
        console.print(f"Wrote {out}")
    else:
        console.print(text, markup=False, end="")


def _run_search(source_settings: dict[str, dict], selected_sources: list[str], parameters: SearchParameters) -> SearchReport:
    http = PoliteHttpClient()
    structured: list[JobPosting] = []
    manual_links: list[ManualSearchLink] = []
    provider_outcomes: list[ProviderOutcome] = []
    errors: list[SourceError] = []
    try:
        for source_name in selected_sources:
            settings = source_settings.get(source_name)
            if not isinstance(settings, dict):
                message = "Source settings must be a mapping."
                errors.append(SourceError(source=source_name, message=message))
                provider_outcomes.append(ProviderOutcome(source=source_name, status="unavailable", error=message))
                continue

            diagnostic = _diagnose_source(source_name, settings)
            if diagnostic.status in {"misconfigured", "unavailable"}:
                errors.append(SourceError(source=source_name, message=diagnostic.guidance))
                provider_outcomes.append(ProviderOutcome(source=source_name, status="unavailable", error=diagnostic.guidance))
                continue

            if _normalize_source_type(settings.get("type")) == "manual_search_link":
                query_count = 0
                failed_query_count = 0
                result_count = 0
                query_errors: list[str] = []
                for title in parameters.titles:
                    for location in parameters.locations:
                        query_count += 1
                        try:
                            link = build_manual_link(source_name, title, location, parameters.remote, parameters.days)
                            if link:
                                manual_links.append(link)
                                result_count += 1
                        except Exception as exc:
                            failed_query_count += 1
                            message = f"{title} / {location}: {_safe_error_message(exc)}"
                            query_errors.append(message)
                            errors.append(SourceError(source=source_name, message=message))
                provider_outcomes.append(
                    ProviderOutcome(
                        source=source_name,
                        status="failed" if failed_query_count else "manual-only",
                        query_count=query_count,
                        failed_query_count=failed_query_count,
                        result_count=result_count,
                        error="; ".join(query_errors) if query_errors else None,
                    )
                )
                continue

            try:
                adapter = make_source(source_name, settings, http)
                if not adapter.is_configured():
                    message = "Source is enabled but missing required API configuration."
                    errors.append(SourceError(source=source_name, message=message))
                    provider_outcomes.append(ProviderOutcome(source=source_name, status="unavailable", error=message))
                    continue
            except Exception as exc:
                message = _safe_error_message(exc, settings)
                errors.append(SourceError(source=source_name, message=message))
                provider_outcomes.append(ProviderOutcome(source=source_name, status="failed", error=message))
                continue

            per_query_limit = _per_query_limit(parameters)
            query_count = 0
            failed_query_count = 0
            result_count = 0
            query_errors: list[str] = []
            for title in parameters.titles:
                for location in parameters.locations:
                    query_count += 1
                    try:
                        results = adapter.search(title, location, parameters.remote, parameters.days, per_query_limit)
                        structured.extend(results)
                        result_count += len(results)
                    except Exception as exc:
                        failed_query_count += 1
                        message = f"{title} / {location}: {_safe_error_message(exc, settings)}"
                        query_errors.append(message)
                        errors.append(SourceError(source=source_name, message=message))
            provider_outcomes.append(
                ProviderOutcome(
                    source=source_name,
                    status="failed" if failed_query_count else "queried",
                    query_count=query_count,
                    failed_query_count=failed_query_count,
                    result_count=result_count,
                    error="; ".join(query_errors) if query_errors else None,
                )
            )
    finally:
        http.close()

    deduped = dedupe_postings(structured, limit=parameters.limit)
    return SearchReport(
        parameters=parameters,
        structured_results=deduped,
        manual_search_links=manual_links,
        provider_outcomes=provider_outcomes,
        errors=errors,
    )


def _print_dry_run(
    source_settings: dict[str, dict],
    selected_sources: list[str],
    diagnostic_sources: list[str],
    parameters: SearchParameters,
) -> None:
    table = Table(title="Configuration diagnostics")
    table.add_column("Source")
    table.add_column("Enabled")
    table.add_column("Capability", no_wrap=True)
    table.add_column("Status", no_wrap=True)
    diagnostics = {
        name: _diagnose_source(name, source_settings.get(name, {}))
        for name in diagnostic_sources
    }
    for diagnostic in diagnostics.values():
        table.add_row(
            diagnostic.name,
            "yes" if diagnostic.enabled else "no",
            diagnostic.capability,
            diagnostic.status,
        )
    console.print(table)
    console.print("Setup guidance:")
    for diagnostic in diagnostics.values():
        console.print(f"{diagnostic.name}: {diagnostic.guidance}", overflow="fold")

    rows: list[tuple[str, str, str, str]] = []
    http = PoliteHttpClient()
    table = Table(title="Dry run: planned source queries")
    table.add_column("Source")
    table.add_column("Type")
    table.add_column("Search")
    table.add_column("URL or action")
    try:
        for source_name in selected_sources:
            settings = source_settings.get(source_name, {})
            diagnostic = diagnostics.get(source_name) or _diagnose_source(source_name, settings)
            if diagnostic.status in {"misconfigured", "unavailable"}:
                continue
            for title in parameters.titles:
                for location in parameters.locations:
                    label = f"{title} / {location}"
                    source_type = _normalize_source_type(settings.get("type"))
                    if source_type == "manual_search_link":
                        link = build_manual_link(source_name, title, location, parameters.remote, parameters.days)
                        rows.append((source_name, "manual_search_link", label, link.url if link else "manual only"))
                        continue
                    if source_name not in SOURCE_REGISTRY:
                        continue
                    adapter = make_source(source_name, settings, http)
                    try:
                        urls = adapter.dry_run_urls(title, location, parameters.remote, parameters.days, _per_query_limit(parameters))
                    except (KeyError, TypeError, ValueError):
                        rows.append((source_name, str(settings.get("type", "api")), label, "Unable to plan from source configuration"))
                        continue
                    if urls:
                        for url in urls:
                            rows.append((source_name, "api", label, url))
                    else:
                        rows.append((source_name, "api", label, "No configured boards/slugs to query"))
    finally:
        http.close()
    if rows:
        for row in rows:
            table.add_row(*row)
        console.print(table)


def _diagnose_source(source_name: str, settings: object) -> SourceDiagnostic:
    if not isinstance(settings, dict):
        return SourceDiagnostic(
            name=source_name,
            enabled=False,
            capability="unavailable",
            status="misconfigured",
            guidance="Source settings must be a mapping.",
        )

    source_type = _normalize_source_type(settings.get("type"))
    capability = _SOURCE_CAPABILITIES.get(source_type, "unavailable")
    enabled = settings.get("enabled") is True
    issues: list[str] = []
    unavailable = False

    if "enabled" not in settings or not isinstance(settings.get("enabled"), bool):
        issues.append("Set enabled to true or false.")
    if not source_type:
        issues.append("Set source type to api, rss, permitted_public_page, or manual_search_link.")
    elif source_type not in _SOURCE_CAPABILITIES:
        unavailable = True
        issues.append(f"Unsupported source type: {source_type}.")

    if source_type in {"api", "rss", "public_page", "permitted_public_page"}:
        if not isinstance(settings.get("base_url"), str) or not settings["base_url"].strip():
            issues.append("Set a public base_url.")
        if source_name not in SOURCE_REGISTRY:
            unavailable = True
            issues.append("No adapter is implemented for this source.")
        else:
            adapter_type = getattr(SOURCE_REGISTRY[source_name], "source_type", "api")
            if adapter_type != source_type:
                unavailable = True
                issues.append(f"No {capability} adapter is implemented for this source.")
            issues.extend(_source_setup_requirements(source_name, settings))
    elif source_type == "manual_search_link" and build_manual_link(source_name, "", "", False, 0) is None:
        unavailable = True
        issues.append("No manual search-link builder is available for this source.")

    if unavailable:
        status = "unavailable"
    elif issues:
        status = "misconfigured"
    else:
        status = "ready" if enabled else "disabled"
    if not enabled and "Disabled in sources.yaml." not in issues:
        issues.append("Disabled in sources.yaml.")
    guidance = " ".join(issues) or "Ready; dry-run makes no requests."
    return SourceDiagnostic(source_name, enabled, capability, status, guidance)


def _source_setup_requirements(source_name: str, settings: dict) -> list[str]:
    requirements: list[str] = []
    if settings.get("requires_api_key"):
        missing_env = [
            str(settings.get(key))
            for key in ("app_id_env", "app_key_env")
            if settings.get(key) and not os.environ.get(str(settings[key]))
        ]
        if missing_env:
            requirements.append(f"Set {' and '.join(missing_env)} in the environment (or run jobs init).")
    api_key_env = settings.get("api_key_env")
    if api_key_env and not settings.get("default_api_key") and not os.environ.get(str(api_key_env)):
        requirements.append(f"Set {api_key_env} in the environment (or run jobs init).")
    if source_name == "greenhouse" and not _configured_values(settings.get("board_tokens")):
        requirements.append("Add at least one board_tokens value to sources.yaml.")
    if source_name == "lever" and not _configured_values(settings.get("company_slugs")):
        requirements.append("Add at least one company_slugs value to sources.yaml.")
    return requirements


def _configured_values(value: object) -> bool:
    return isinstance(value, list) and any(isinstance(item, str) and item.strip() for item in value)


def _normalize_source_type(value: object) -> str:
    source_type = str(value or "").strip().lower().replace("-", "_")
    if source_type in {"manual", "manual_only"}:
        return "manual_search_link"
    return source_type


def _resolve_titles(title: str | None, titles_file: Path | None, titles_config: dict) -> list[str]:
    titles: list[str] = []
    if title:
        titles.append(title)
    if titles_file:
        titles.extend(load_titles_file(titles_file))
    if not titles:
        titles = list(titles_config.get("titles") or [])
    deduped: list[str] = []
    seen: set[str] = set()
    for value in titles:
        clean = str(value).strip()
        key = clean.lower()
        if clean and key not in seen:
            deduped.append(clean)
            seen.add(key)
    if not deduped:
        raise typer.BadParameter("No titles configured. Pass --title or --titles.")
    return deduped


def _select_sources(source_settings: dict[str, dict], requested_sources: list[str] | None) -> list[str]:
    if requested_sources:
        unknown = [name for name in requested_sources if name not in source_settings]
        if unknown:
            raise typer.BadParameter(f"Unknown source(s): {', '.join(unknown)}")
        return _order_sources(source_settings, list(dict.fromkeys(requested_sources)))
    enabled = [name for name, settings in source_settings.items() if isinstance(settings, dict) and settings.get("enabled") is True]
    return _order_sources(source_settings, enabled)


def _order_sources(source_settings: dict[str, dict], source_names: list[str]) -> list[str]:
    registry_order = {name: index for index, name in enumerate(SOURCE_REGISTRY)}
    return sorted(
        source_names,
        key=lambda name: (
            _source_type_order(source_settings.get(name)),
            registry_order.get(name, len(registry_order)),
            name,
        ),
    )


def _source_type_order(settings: object) -> int:
    source_type = _normalize_source_type(settings.get("type")) if isinstance(settings, dict) else ""
    return _SOURCE_TYPE_ORDER.get(source_type, len(_SOURCE_TYPE_ORDER))


def _parse_source_filter(source: str | None) -> list[str] | None:
    if not source:
        return None
    return [item.strip().lower() for item in source.split(",") if item.strip()]


def _per_query_limit(parameters: SearchParameters) -> int:
    combinations = max(1, len(parameters.titles) * len(parameters.locations))
    return max(1, min(parameters.limit, (parameters.limit + combinations - 1) // combinations))


def _write_report(report: SearchReport, output_format: str, out: Path) -> None:
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(export_report(report, output_format), encoding="utf-8")


def _print_structured_table(postings: list[JobPosting]) -> None:
    table = Table(title="Structured results")
    table.add_column("#", justify="right")
    table.add_column("Title")
    table.add_column("Company")
    table.add_column("Location")
    table.add_column("Posted")
    table.add_column("Website")
    table.add_column("Link")
    for index, posting in enumerate(postings, start=1):
        table.add_row(
            str(index),
            posting.title,
            posting.company or "",
            posting.location or "",
            posting.date_of_posting.isoformat() if posting.date_of_posting else "",
            posting.source_website,
            posting.url,
        )
    if not postings:
        table.add_row("", "No structured results", "", "", "", "", "")
    console.print(table)


def _print_manual_links(links: list[ManualSearchLink]) -> None:
    if not links:
        return
    table = Table(title="Manual search links")
    table.add_column("Search term")
    table.add_column("Website")
    table.add_column("Link")
    for link in links:
        term = link.search_term if not link.location else f"{link.search_term} ({link.location})"
        table.add_row(term, link.website, link.url)
    console.print(table)


def _print_errors(errors: list[SourceError]) -> None:
    if not errors:
        return
    table = Table(title="Source errors")
    table.add_column("Source")
    table.add_column("Error")
    for error in errors:
        table.add_row(error.source, error.message)
    console.print(table)


def _print_provider_outcomes(outcomes: list[ProviderOutcome]) -> None:
    if not outcomes:
        return
    table = Table(title="Provider outcomes")
    table.add_column("Source")
    table.add_column("Status")
    table.add_column("Queries", justify="right")
    table.add_column("Failed", justify="right")
    table.add_column("Results", justify="right")
    table.add_column("Error")
    for outcome in outcomes:
        table.add_row(
            outcome.source,
            outcome.status,
            str(outcome.query_count),
            str(outcome.failed_query_count),
            str(outcome.result_count),
            outcome.error or "",
        )
    console.print(table)


def _configure_logging(verbose: bool) -> None:
    logging.basicConfig(level=logging.DEBUG if verbose else logging.WARNING, format="%(levelname)s %(name)s: %(message)s")


def _safe_error_message(exc: Exception, settings: object | None = None) -> str:
    message = str(exc) or exc.__class__.__name__
    secrets: list[str] = []
    if isinstance(settings, dict):
        for key in ("app_id_env", "app_key_env", "api_key_env"):
            env_name = settings.get(key)
            if env_name:
                value = os.environ.get(str(env_name))
                if value:
                    secrets.append(value)
    for secret in secrets:
        message = message.replace(secret, "[redacted]")
    return re.sub(
        r"(?i)((?:app[_-]?key|api[_-]?key|token|secret|password)=)[^&\s'\"]+",
        r"\1[redacted]",
        message,
    )
