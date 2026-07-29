# Printing Press CLIs

Canonical location: `~/printing-press/library/<project>`.

**Before using or changing any CLI, read its `AGENTS.md` first.**

| Use case | CLI directory |
|---|---|
| Weather | `library/windy-weather-pp-cli` |
| Flights | `library/flight` |
| Transit / public transport | `library/transit` |
| Music / Spotify | `library/spotify` |
| YouTube / video | `library/youtube` |
| Accommodation / stays | `library/airbnb` |
| Google Calendar | `library/google-calendar` |
| Trello / calendar scheduling | `library/trello-calendar` |
| Email | `library/mail` |
| Investing / Trade Republic | `library/trade-republic-pp` |
| Classifieds / Kleinanzeigen | `library/kleinanzeigen-pp-cli` |
| Events / Meetup | `library/meetup` |
| Healthcare / Doctolib | `library/doctolib` |
| Jobs — multi-source | `library/job-research-cli` |
| Jobs — Arbeitnow | `library/job-research-cli/providers/arbeitnow-jobs` |
| Jobs — BA/Bundesagentur | `library/job-research-cli/providers/ba-jobsuche` |
| Jobs — Berlin startups | `library/job-research-cli/providers/berlinstartupjobs` |
| Jobs — English in Germany | `library/job-research-cli/providers/englishjobs` |
| Jobs — German tech | `library/job-research-cli/providers/germantechjobs` |
| Jobs — Indeed | `library/job-research-cli/providers/indeed` |
| Jobs — LinkedIn | `library/job-research-cli/providers/linkedin` |
| Jobs — Remotive / remote | `library/job-research-cli/providers/remotive-jobs` |
| Jobs — Stepstone | `library/job-research-cli/providers/stepstone` |
| Jobs — Xing | `library/job-research-cli/providers/xing` |

## Validation

```bash
./scripts/audit-public.sh
./scripts/verify.sh --changed <base-revision>
./scripts/verify.sh --all --android
```

`workspace.yaml` is the source of truth for project paths and runtimes.
