# Routine synchronization

Routine inspection and diffing are based only on visible website UI. Always run
`hevy routines apply PLAN --dry-run` first. Live writes deliberately remain
disabled until selectors and post-action verification are manually validated
against the current Hevy website; this prevents accidental changes when the UI
changes. The CLI never uses private APIs, stealth tools, CAPTCHA solvers, or
fingerprint spoofing.

