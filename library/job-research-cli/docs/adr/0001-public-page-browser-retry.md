# Use browser rendering, not anti-bot bypass

Public job-board searches use standard Scrapling requests first. After explicit operator approval, an accessible results page may be rendered with Scrapling's normal browser fetcher, while robots.txt rules, no-login access, no CAPTCHA solving, and no proxy rotation remain mandatory. This preserves access to JavaScript-rendered public results without turning provider rejection into a bypass attempt.
