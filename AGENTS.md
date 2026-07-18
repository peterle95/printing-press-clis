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
| Jobs — Arbeitnow | `library/arbeitnow-jobs` |
| Jobs — BA/Bundesagentur | `library/ba-jobsuche` |
| Jobs — Berlin startups | `library/berlinstartupjobs` |
| Jobs — English in Germany | `library/englishjobs` |
| Jobs — German tech | `library/germantechjobs` |
| Jobs — Indeed | `library/indeed` |
| Jobs — LinkedIn | `library/linkedin` |
| Jobs — Remotive / remote | `library/remotive-jobs` |
| Jobs — Stepstone | `library/stepstone` |
| Jobs — Xing | `library/xing` |

## Validation

```bash
./scripts/audit-public.sh
./scripts/verify.sh --changed <base-revision>
./scripts/verify.sh --all --android
```

`workspace.yaml` is the source of truth for project paths and runtimes.
