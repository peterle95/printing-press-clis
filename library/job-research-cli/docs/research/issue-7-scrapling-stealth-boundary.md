# Issue 7: Scrapling Stealth Boundary

**Decision:** Add no stealth bypass path to this CLI. If Scrapling is introduced later, permit public, unauthenticated, permitted pages only; use the least capable fetcher that works; treat a block, challenge, login wall, or unclear permission as a stop condition. LinkedIn remains permanently `manual_search_link`-only.

## Fetcher tiers

| Tier | Scrapling class | Safe use in this repo | Escalation rule |
| --- | --- | --- | --- |
| 1 | `Fetcher` / `FetcherSession` | Official/public APIs, RSS, static public HTML. Browser impersonation may be used only for compatibility, not to defeat a block. | First choice. |
| 2 | `DynamicFetcher` / `DynamicSession` | Public pages whose content requires JavaScript; bounded extraction with Playwright page actions and selectors. | Use only when Tier 1 cannot obtain required public content. |
| 3 | `StealthyFetcher` / `StealthySession` | **Not enabled for production job-board collection.** May be used only in a separately approved, public test fixture to assess ordinary browser compatibility. | Never use to pass a challenge or continue after access is denied. |

Scrapling's own comparison describes `Fetcher` as HTTP-only, `DynamicFetcher` as browser-based for dynamic pages and small-to-medium protections, and `StealthyFetcher` as having advanced anti-bot capabilities. That last capability is outside this project's access boundary, even though class exists.

## Allowed options

These options support normal public-page retrieval or operational safety:

- `headless=True` and normal bundled Chromium or installed Chrome.
- `load_dom`, `wait_selector`, `wait_selector_state`, `timeout`, and bounded `wait` for deterministic page loading.
- `disable_resources=True` only after verifying target still renders required content. It can prevent pages finishing.
- `blocked_domains` and `block_ads` to reduce unnecessary third-party requests.
- `capture_xhr` only for public, non-authenticated data needed to render requested page content. Do not use it to discover or replay private endpoints.
- `locale="de-DE"` and `timezone_id="Europe/Berlin"` when matching intended operator context. These are presentation/context settings, not evasion controls.
- `real_chrome=True` only for browser compatibility. It does not authorize access and is not a promise of undetectability.
- A single persistent session for ordinary cookie continuity created by the public site. Do not persist or export credentials, cookies, local storage, or auth state.

`useragent` should normally be left unset. If a target requires an explicit value for compatibility, use one stable, truthful browser user-agent and do not rotate it to avoid controls. `google_search` and `extra_headers` must not be used to fabricate provenance; disable the default Google referer where it would be misleading.

## Explicitly excluded

The following are prohibited, even if Scrapling or Playwright supports them:

- `StealthyFetcher.solve_cloudflare`, CAPTCHA solving, Turnstile/interstitial solving, challenge clicking, or any other challenge bypass.
- Login automation, account creation, authenticated scraping, logged-in LinkedIn access, cookies or storage state supplied by an operator, and `user_data_dir` containing authenticated state.
- Proxy use or rotation (`proxy`, `proxy_rotator`), residential/mobile proxies, IP rotation, DNS-over-HTTPS used with proxies, or CDP/managed-browser connections intended to change origin or evade a ban.
- Fingerprint spoofing, canvas-noise/hide-canvas, WebRTC masking, headless-detection patches, custom init scripts, browser flags, or arbitrary `additional_args` whose purpose is concealment or access-control evasion. In particular, do not enable `hide_canvas`, `block_webrtc`, or `allow_webgl` as stealth measures.
- Replaying private XHR/API calls, extracting bearer tokens, modifying request headers to defeat controls, bypassing robots/terms restrictions, or scraping protected pages.
- Retrying a `401`, `403`, `407`, `429`, challenge page, or explicit denial through another fetcher, identity, proxy, browser, or URL variant.
- LinkedIn scraping of any kind. LinkedIn provider output is direct `manual_search_link` URLs only; human opens links manually.

## Rate and stop policy

Scrapling's spider documentation describes per-domain throttling, download delays, `Retry-After` handling, adaptive backoff, and blocked-request retries. This CLI should use stricter, deterministic defaults:

- One in-flight request per domain (`max_pages=1`; no concurrent `gather` for same domain).
- Minimum 1 second between requests to same domain, with a larger configured delay when site terms, `robots.txt`, `Crawl-delay`, or `Retry-After` requires it.
- Respect `robots.txt` and public API/ATS terms where applicable. Cache stable results and avoid refetching identical URLs during one run.
- On `429` or `Retry-After`, wait at least server-specified duration, then retry once at most. If still limited, stop source for run and report it.
- On `401`, `403`, `407`, challenge/interstitial, login wall, or explicit denial: stop source immediately. No escalation or retry through another identity.
- Network failures may receive one bounded retry with increasing delay. Retries must not apply to access-control responses.
- Keep request timeout bounded (Scrapling default is 30 seconds; challenge-related longer timeouts are irrelevant because challenges are excluded).

These numbers are operational safety defaults, not claims about any site's permitted rate. A source-specific official limit always wins.

## Implementable boundary

Any future Scrapling integration must enforce:

1. Source metadata declares `public`, `permitted`, and fetcher tier before network access.
2. LinkedIn is rejected unless source mode is `manual_search_link`.
3. Configuration is an allowlist. Reject excluded options rather than silently accepting them.
4. Response classifier treats access-control and rate-limit signals as terminal source outcomes.
5. Tests mock responses for `200`, `429`/`Retry-After`, `403`, login walls, and challenge pages. Tests make no external network calls.
6. Dry-run runs before enabling any live source behavior, matching repository policy.

No application code is changed by this research. The boundary is deliberately narrower than Scrapling's feature set: browser automation can render public JavaScript pages; stealth must not become a mechanism for bypassing a site's access controls.

## Primary sources

- Scrapling fetcher comparison: <https://scrapling.readthedocs.io/en/latest/fetching/choosing.html>
- Scrapling `DynamicFetcher` options and sessions: <https://scrapling.readthedocs.io/en/latest/fetching/dynamic.html>
- Scrapling `StealthyFetcher` behavior and options: <https://scrapling.readthedocs.io/en/latest/fetching/stealthy.html>
- Scrapling spider throttling, `Retry-After`, robots, and blocked-request handling: <https://github.com/D4Vinci/Scrapling/blob/main/README.md> and <https://scrapling.readthedocs.io/en/latest/spiders/proxy-blocking.html>
- Scrapling source and version history: <https://github.com/D4Vinci/Scrapling>
- Playwright browser contexts and auth-state sensitivity: <https://playwright.dev/python/docs/auth>
- Playwright browser installation and branded Chrome behavior: <https://playwright.dev/python/docs/browsers>
- Playwright network monitoring/interception API: <https://playwright.dev/python/docs/network>
