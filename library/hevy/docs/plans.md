# Plans

Plans are local YAML or JSON with `version: 1`, a name, routines, exercises,
and unit-explicit targets such as `target_weight_kg`. Validate before import:
`hevy plans validate plan.yaml`. Plan files are atomically written, use
sanitized filenames, preserve ordering, and reject invalid repetitions or
negative measurements.

