from __future__ import annotations

from enum import Enum
import logging
from pathlib import Path
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
from .http_client import HttpClientError, PoliteHttpClient
from .models import JobPosting, ManualSearchLink, SearchParameters, SearchReport, SourceError
from .result_files import load_postings_from_files
from .sources import SOURCE_REGISTRY, make_source
from .sources.base import SourceAdapterError
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
    dry_run: bool = typer.Option(False, "--dry-run", help="Show sources and URLs without making HTTP requests."),
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
    selected_sources = _select_sources(sources_config.get("sources", {}), requested_sources)
    parameters = SearchParameters(titles=titles, locations=locations, remote=remote, days=days, limit=limit, sources=selected_sources)

    if dry_run:
        _print_dry_run(sources_config.get("sources", {}), selected_sources, parameters)
        return

    report = _run_search(sources_config.get("sources", {}), selected_sources, parameters)
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
    errors: list[SourceError] = []
    try:
        for source_name in selected_sources:
            settings = source_settings[source_name]
            if settings.get("type") == "manual_search_link":
                for title in parameters.titles:
                    for location in parameters.locations:
                        link = build_manual_link(source_name, title, location, parameters.remote, parameters.days)
                        if link:
                            manual_links.append(link)
                continue

            if source_name not in SOURCE_REGISTRY:
                errors.append(SourceError(source=source_name, message="No API adapter is implemented for this source."))
                continue
            adapter = make_source(source_name, settings, http)
            if not adapter.is_configured():
                errors.append(SourceError(source=source_name, message="Source is enabled but missing required API configuration."))
                continue

            per_query_limit = _per_query_limit(parameters)
            source_failed = False
            for title in parameters.titles:
                for location in parameters.locations:
                    try:
                        structured.extend(adapter.search(title, location, parameters.remote, parameters.days, per_query_limit))
                    except (SourceAdapterError, HttpClientError) as exc:
                        errors.append(SourceError(source=source_name, message=str(exc)))
                        source_failed = True
                        break
                if source_failed:
                    break
    finally:
        http.close()

    deduped = dedupe_postings(structured, limit=parameters.limit)
    return SearchReport(parameters=parameters, structured_results=deduped, manual_search_links=manual_links, errors=errors)


def _print_dry_run(source_settings: dict[str, dict], selected_sources: list[str], parameters: SearchParameters) -> None:
    http = PoliteHttpClient()
    table = Table(title="Dry run: planned source queries")
    table.add_column("Source")
    table.add_column("Type")
    table.add_column("Search")
    table.add_column("URL or action")
    try:
        for source_name in selected_sources:
            settings = source_settings[source_name]
            for title in parameters.titles:
                for location in parameters.locations:
                    label = f"{title} / {location}"
                    if settings.get("type") == "manual_search_link":
                        link = build_manual_link(source_name, title, location, parameters.remote, parameters.days)
                        table.add_row(source_name, "manual_search_link", label, link.url if link else "manual only")
                        continue
                    if source_name not in SOURCE_REGISTRY:
                        table.add_row(source_name, str(settings.get("type", "api")), label, "No adapter implemented")
                        continue
                    adapter = make_source(source_name, settings, http)
                    urls = adapter.dry_run_urls(title, location, parameters.remote, parameters.days, _per_query_limit(parameters))
                    if urls:
                        for url in urls:
                            table.add_row(source_name, "api", label, url)
                    else:
                        table.add_row(source_name, "api", label, "No configured boards/slugs to query")
    finally:
        http.close()
    console.print(table)


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
        return requested_sources
    return [name for name, settings in source_settings.items() if settings.get("enabled", False)]


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


def _configure_logging(verbose: bool) -> None:
    logging.basicConfig(level=logging.DEBUG if verbose else logging.WARNING, format="%(levelname)s %(name)s: %(message)s")
