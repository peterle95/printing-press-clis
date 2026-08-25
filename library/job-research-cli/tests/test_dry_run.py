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
