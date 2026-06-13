from job_research_cli.normalizer import canonicalize_url, infer_remote_mode, normalize_job_title, normalize_location


def test_canonicalize_url_strips_tracking_params_and_normalizes_host() -> None:
    url = "HTTPS://Example.COM/jobs/123/?utm_source=mail&ref=abc&trackingId=9&keep=yes"

    assert canonicalize_url(url) == "https://example.com/jobs/123?keep=yes"


def test_canonicalize_url_ignores_non_web_placeholders() -> None:
    assert canonicalize_url("about:blank") is None


def test_normalize_job_title_removes_gender_markers() -> None:
    assert normalize_job_title("Frontend Developer (m/f/d)") == "frontend developer"
    assert normalize_job_title("Embedded-Software-Entwickler (m/w/d)") == "embedded software entwickler"


def test_normalize_location_handles_german_country_name() -> None:
    assert normalize_location("10115 Berlin, Deutschland") == "berlin germany"


def test_infer_remote_mode_prefers_hybrid_when_present() -> None:
    assert infer_remote_mode("Hybrid role with home office") == "hybrid"
