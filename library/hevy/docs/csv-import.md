# CSV import

Export workouts manually from the Hevy mobile app, then run `hevy sync FILE`.
Headers are normalized dynamically; unknown fields are retained as raw metadata.
Imports use SHA-256 file hashing, stable workout/set fingerprints, transactions,
and a 50 MiB input limit. Use `--dry-run` before writing and `--directory DIR
--latest` to select the newest CSV by filesystem modification time.

