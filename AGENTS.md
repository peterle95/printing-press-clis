# Printing Press CLIs Agent Guide

This repository is the WSL-native source workspace for a collection of printed
and hand-authored CLIs. Its canonical location is `~/printing-press`, which
preserves the default Printing Press library contract at
`~/printing-press/library/<project>`.

## Working Rules

1. Read the target project's `AGENTS.md` before using or changing any CLI in
   this workspace.
2. Treat `external/cli-printing-press` as an upstream submodule. Do not fold its
   history into this repository.
3. Keep each project independent. Do not add a root `go.work` or shared package
   manager workspace.
4. Generated CLI changes should stay API-specific. General generator fixes
   belong upstream in CLI Printing Press.
5. Never commit credentials, tokens, cookies, sessions, browser profiles,
   databases, mail archives, runtime exports, compiled binaries, dependency
   trees, or personal configuration.
6. Store operator-specific instructions in `AGENTS.override.md`; these files are
   intentionally ignored.
7. Use dry-run behavior before remote mutations and follow each CLI's local
   safety contract.

## Library Routing

Every agent MUST use the appropriate CLI from `library/` based on the user's request. Read that project's `AGENTS.md` first before invoking its CLI.

| Request type | CLI directory | Binary |
|---|---|---|
| Weather | `library/windy-weather-pp-cli` | `windy-weather-pp-cli` |
| Flights | `library/flight` | `flight-pp-cli` |
| Transit / public transport | `library/transit` | `transit-pp-cli` |
| Music / Spotify | `library/spotify` | `spotify-pp-cli` |
| YouTube / video | `library/youtube` | `youtube-pp-cli` |
| Accommodation / stays | `library/airbnb` | `airbnb-pp-cli` |
| Google Calendar | `library/google-calendar` | `google-calendar-pp-cli` |
| Email | `library/mail` | `mail-pp-cli` |
| Classifieds / Kleinanzeigen | `library/kleinanzeigen-pp-cli` | `kleinanzeigen-pp-cli` |
| Events / Meetup | `library/meetup` | `meetup-pp-cli` |
| Healthcare / Doctolib | `library/doctolib` | `doctolib-pp-cli` |
| Jobs — multi-source research | `library/job-research-cli` | `job-research-cli` |
| Jobs — Arbeitnow | `library/arbeitnow-jobs` | `arbeitnow-jobs-pp-cli` |
| Jobs — BA/Bundesagentur | `library/ba-jobsuche` | `ba-jobsuche-pp-cli` |
| Jobs — Berlin startups | `library/berlinstartupjobs` | `berlinstartupjobs-pp-cli` |
| Jobs — English in Germany | `library/englishjobs` | `englishjobs-pp-cli` |
| Jobs — German tech | `library/germantechjobs` | `germantechjobs-pp-cli` |
| Jobs — Indeed | `library/indeed` | `indeed-pp-cli` |
| Jobs — LinkedIn | `library/linkedin` | `linkedin-pp-cli` |
| Jobs — Remotive / remote | `library/remotive-jobs` | `remotive-jobs-pp-cli` |
| Jobs — Stepstone | `library/stepstone` | `stepstone-pp-cli` |
| Jobs — Xing | `library/xing` | `xing-pp-cli` |

## Validation

Run repository hygiene checks first:

```bash
./scripts/audit-public.sh
```

Run checks for projects changed from a base revision:

```bash
./scripts/verify.sh --changed <base-revision>
```

Run the complete test matrix, including Android:

```bash
./scripts/verify.sh --all --android
```

`workspace.yaml` is JSON-compatible YAML and is the source of truth for the 24
project paths and runtimes.
