# Printing Press CLIs Agent Guide

This repository is the WSL-native source workspace for a collection of printed
and hand-authored CLIs. Its canonical location is `~/printing-press`, which
preserves the default Printing Press library contract at
`~/printing-press/library/<project>`.

## Working Rules

1. Read the target project's `AGENTS.md` before changing it.
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
