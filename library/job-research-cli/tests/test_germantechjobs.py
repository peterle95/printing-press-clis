from job_research_cli.sources.germantechjobs import GermanTechJobsSource


class FakeHttp:
    def __init__(self, text: str) -> None:
        self.text = text
        self.calls = []

    def get_text(self, source, url, **kwargs):
        self.calls.append((source, url, kwargs))
        return self.text


def test_rss_search_normalizes_filters_and_caches_feed() -> None:
    http = FakeHttp(
        """<?xml version="1.0"?><rss><channel>
        <item><guid>gtj-1</guid><title>Frontend Developer</title><link>https://jobs.example/1</link>
        <pubDate>Tue, 25 Aug 2026 10:00:00 GMT</pubDate><location>Berlin</location><company>Acme</company></item>
        <item><title>Malformed</title></item>
        <item><guid>gtj-1</guid><title>Frontend Developer</title><link>https://jobs.example/1</link></item>
        <item><guid>gtj-2</guid><title>Backend Developer</title><link>https://jobs.example/2</link><location>Munich</location></item>
        </channel></rss>"""
    )
    source = GermanTechJobsSource({"base_url": "https://germantechjobs.de"}, http)

    results = source.search("Frontend Developer", "Berlin", False, 0, 10)
    results += source.search("Frontend Developer", "Berlin", False, 0, 10)

    assert len(results) == 2
    assert results[0].job_id == "gtj-1"
    assert results[0].source_type == "rss"
    assert results[0].company == "Acme"
    assert len(http.calls) == 1


def test_rss_malformed_xml_is_empty() -> None:
    source = GermanTechJobsSource({"base_url": "https://germantechjobs.de"}, FakeHttp("not xml"))

    assert source.search("Frontend Developer", "Berlin", False, 7, 10) == []


def test_public_page_retry_parses_jobposting_json_ld() -> None:
    http = FakeHttp(
        '''<html><script type="application/ld+json">{
        "@type":"JobPosting","identifier":{"value":"job-1"},
        "title":"Frontend Developer","hiringOrganization":{"name":"Acme"},
        "jobLocation":{"address":{"addressLocality":"Berlin","addressCountry":"DE"}},
        "datePosted":"2026-08-25","url":"https://germantechjobs.de/jobs/acme-frontend"
        }</script></html>'''
    )

    results = GermanTechJobsSource({"base_url": "https://germantechjobs.de"}, http).stealth_search(
        "Frontend Developer", "Berlin", False, 7, 10
    )

    assert len(results) == 1
    assert results[0].job_id == "job-1"
    assert results[0].source_type == "public_page"
    assert results[0].company == "Acme"
    assert http.calls[0][1] == "https://germantechjobs.de/jobs?q=Frontend+Developer&location=Berlin"


def test_public_page_retry_stops_on_access_control_signal() -> None:
    source = GermanTechJobsSource({"base_url": "https://germantechjobs.de"}, FakeHttp("<html>captcha challenge</html>"))

    try:
        source.stealth_search("Frontend Developer", "Berlin", False, 7, 10)
    except RuntimeError as exc:
        assert "access-control" in str(exc)
    else:
        raise AssertionError("access-control page was parsed")
