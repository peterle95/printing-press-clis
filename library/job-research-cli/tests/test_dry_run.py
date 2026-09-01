from rich.console import Console
from typer.testing import CliRunner

from job_research_cli import cli
from job_research_cli.cli import app
from job_research_cli.http_client import PoliteHttpClient


def test_dry_run_reports_capabilities_and_configuration_without_side_effects(tmp_path, monkeypatch) -> None:
    config_dir = tmp_path / "config"
    config_dir.mkdir()
    (config_dir / "sources.yaml").write_text(
        """
sources:
  arbeitnow:
    enabled: true
    type: api
    base_url: 123
  linkedin:
    enabled: true
    type: manual-only
  rss_board:
    enabled: true
    type: rss
    base_url: https://example.com/jobs.rss
  public_board:
    enabled: true
    type: permitted_public_page
    base_url: https://example.com/jobs
  adzuna:
    enabled: true
    type: api
  greenhouse:
    enabled: true
    type: api
    board_tokens: [123]
  remoteok:
    enabled: false
    type: api
""",
        encoding="utf-8",
    )
    (config_dir / "titles.yaml").write_text("titles: [Frontend Developer]\nlocations: [Berlin]\n", encoding="utf-8")

    def fail_request(*args, **kwargs):
        raise AssertionError("dry-run made an HTTP request")

    monkeypatch.setattr(PoliteHttpClient, "get_json", fail_request)
    monkeypatch.setattr(cli, "JobStore", lambda *args, **kwargs: (_ for _ in ()).throw(AssertionError("dry-run opened storage")))

    result = CliRunner().invoke(
        app,
        [
            "search",
            "--dry-run",
            "--config-dir",
            str(config_dir),
            "--title",
            "Frontend Developer",
            "--location",
            "Berlin",
        ],
    )

    assert result.exit_code == 0, result.stdout
    assert "Configuration diagnostics" in result.stdout
    assert "arbeitnow" in result.stdout
    assert "API" in result.stdout
    assert "public base_url" in result.stdout
    assert "linkedin" in result.stdout
    assert "manual-only" in result.stdout
    assert "adzuna" in result.stdout
    assert "ADZUNA_APP_ID" in result.stdout
    assert "misconfigured" in result.stdout
    assert "greenhouse" in result.stdout
    assert "board_tokens" in result.stdout
    assert "remoteok" in result.stdout
    assert "disabled" in result.stdout
    assert "rss_board" in result.stdout
    assert "RSS" in result.stdout
    assert "public_board" in result.stdout
    assert "permitted public-page" in result.stdout
    assert "unavailable" in result.stdout


def test_dry_run_reports_non_mapping_sources_configuration(tmp_path, monkeypatch) -> None:
    config_dir = tmp_path / "config"
    config_dir.mkdir()
    (config_dir / "sources.yaml").write_text("sources: null\n", encoding="utf-8")
    (config_dir / "titles.yaml").write_text("titles: [Frontend Developer]\nlocations: [Berlin]\n", encoding="utf-8")
    monkeypatch.setattr(PoliteHttpClient, "get_json", lambda *args, **kwargs: (_ for _ in ()).throw(AssertionError("dry-run made an HTTP request")))
    monkeypatch.setattr(cli, "JobStore", lambda *args, **kwargs: (_ for _ in ()).throw(AssertionError("dry-run opened storage")))

    result = CliRunner().invoke(
        app,
        [
            "search",
            "--dry-run",
            "--config-dir",
            str(config_dir),
            "--source",
            "arbeitnow",
            "--title",
            "Frontend Developer",
            "--location",
            "Berlin",
        ],
    )

    assert result.exit_code == 0, result.stdout
    assert "sources" in result.stdout
    assert "Source settings must be a mapping" in result.stdout
    assert "misconfigured" in result.stdout


def test_dry_run_reports_non_string_source_names(tmp_path) -> None:
    config_dir = tmp_path / "config"
    config_dir.mkdir()
    (config_dir / "sources.yaml").write_text(
        "sources:\n  123:\n    enabled: true\n    type: api\n    base_url: https://example.com/jobs\n",
        encoding="utf-8",
    )
    (config_dir / "titles.yaml").write_text("titles: [Frontend Developer]\nlocations: [Berlin]\n", encoding="utf-8")

    result = CliRunner().invoke(
        app,
        [
            "search",
            "--dry-run",
            "--config-dir",
            str(config_dir),
            "--title",
            "Frontend Developer",
            "--location",
            "Berlin",
        ],
    )

    assert result.exit_code == 0, result.stdout
    assert "123" in result.stdout
    assert "Source names must be strings" in result.stdout
    assert "misconfigured" in result.stdout


def test_dry_run_reports_planner_failure_without_aborting(tmp_path, monkeypatch) -> None:
    config_dir = tmp_path / "config"
    config_dir.mkdir()
    (config_dir / "titles.yaml").write_text("titles: [Frontend Developer]\nlocations: [Berlin]\n", encoding="utf-8")

    def fail_planning(*args, **kwargs):
        raise RuntimeError("planner failed")

    monkeypatch.setattr(cli, "make_source", fail_planning)
    monkeypatch.setattr(cli, "JobStore", lambda *args, **kwargs: (_ for _ in ()).throw(AssertionError("dry-run opened storage")))

    result = CliRunner().invoke(
        app,
        [
            "search",
            "--dry-run",
            "--config-dir",
            str(config_dir),
            "--source",
            "arbeitnow",
            "--title",
            "Frontend Developer",
            "--location",
            "Berlin",
        ],
    )

    assert result.exit_code == 0, result.stdout
    assert "Unable to plan: planner" in result.stdout
    assert "failed" in result.stdout


def test_dry_run_reports_malformed_base_url(tmp_path) -> None:
    config_dir = tmp_path / "config"
    config_dir.mkdir()
    (config_dir / "sources.yaml").write_text(
        "sources:\n  arbeitnow:\n    enabled: true\n    type: api\n    base_url: not-a-url\n",
        encoding="utf-8",
    )
    (config_dir / "titles.yaml").write_text("titles: [Frontend Developer]\nlocations: [Berlin]\n", encoding="utf-8")

    result = CliRunner().invoke(
        app,
        [
            "search",
            "--dry-run",
            "--config-dir",
            str(config_dir),
            "--source",
            "arbeitnow",
            "--title",
            "Frontend Developer",
            "--location",
            "Berlin",
        ],
    )

    assert result.exit_code == 0, result.stdout
    assert "arbeitnow" in result.stdout
    assert "misconfigured" in result.stdout
    assert "public base_url" in result.stdout


def test_dry_run_reports_invalid_base_url_port(tmp_path) -> None:
    config_dir = tmp_path / "config"
    config_dir.mkdir()
    (config_dir / "sources.yaml").write_text(
        "sources:\n  arbeitnow:\n    enabled: true\n    type: api\n    base_url: http://example.test:bad\n",
        encoding="utf-8",
    )
    (config_dir / "titles.yaml").write_text("titles: [Frontend Developer]\nlocations: [Berlin]\n", encoding="utf-8")

    result = CliRunner().invoke(
        app,
        [
            "search",
            "--dry-run",
            "--config-dir",
            str(config_dir),
            "--source",
            "arbeitnow",
            "--title",
            "Frontend Developer",
            "--location",
            "Berlin",
        ],
    )

    assert result.exit_code == 0, result.stdout
    assert "misconfigured" in result.stdout
    assert "public base_url" in result.stdout
    assert not cli._is_public_base_url("http://:8080")
    assert not cli._is_public_base_url("http://example.test:bad")


def test_dry_run_redacts_planned_url_credentials(tmp_path, monkeypatch) -> None:
    config_dir = tmp_path / "config"
    config_dir.mkdir()
    (config_dir / "sources.yaml").write_text(
        "sources:\n  custom:\n    enabled: true\n    type: api\n    base_url: https://example.test/jobs\n",
        encoding="utf-8",
    )
    (config_dir / "titles.yaml").write_text("titles: [Frontend Developer]\nlocations: [Berlin]\n", encoding="utf-8")

    class CustomSource:
        source_type = "api"

        def dry_run_urls(self, *args, **kwargs):
            return ["https://example.test/jobs?api_key=secret-key"]

    monkeypatch.setattr(
        cli,
        "load_runtime_config",
        lambda config_dir: (
            {"sources": {"custom": {"enabled": True, "type": "api", "base_url": "https://example.test/jobs"}}},
            {"titles": ["Frontend Developer"], "locations": ["Berlin"]},
            config_dir,
        ),
    )
    monkeypatch.setattr(cli, "SOURCE_REGISTRY", {"custom": CustomSource})
    monkeypatch.setattr(cli, "make_source", lambda *args, **kwargs: CustomSource())
    monkeypatch.setattr(cli, "console", Console(width=200))

    result = CliRunner().invoke(
        app,
        [
            "search",
            "--dry-run",
            "--config-dir",
            str(config_dir),
            "--title",
            "Frontend Developer",
            "--location",
            "Berlin",
        ],
    )

    assert result.exit_code == 0, result.stdout
    assert "secret-key" not in result.stdout
    assert "api_key=%5Bredacted%5D" in result.stdout
