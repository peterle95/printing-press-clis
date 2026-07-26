# Routine synchronization

Routine inspection and diffing are based only on visible website UI. Run
`hevy routines list` to read routines and their visible exercise/set targets.
Run `hevy routines diff PLAN` to compare them with a local plan.

Only exercise names, set counts, and visible fixed rep targets are imported.
Weights, rest timers, and other fields absent from routine detail pages remain
unset. Always run `hevy routines apply PLAN --dry-run` first. Live writes remain
disabled until selectors and post-action verification are manually validated
against the current Hevy website. The CLI never uses private APIs, stealth
tools, CAPTCHA solvers, or fingerprint spoofing.

Create routines with visible exercises using `hevy routines create NAME --yes
--exercise "Exercise Name"`. Each resistance exercise receives three sets of
ten reps by default; `Running` and `Treadmill` receive one cardio set. Weights
remain unset for operator adjustment in Hevy.
