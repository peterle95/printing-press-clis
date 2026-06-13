# Doctolib Printed CLI Agent Guide

This directory is a generated `doctolib-pp-cli` project. Treat systemic fixes
as upstream CLI Printing Press work and keep API-specific patches documented in
`.printing-press-patches.json`.

## Operating Rules

- Use German search terms for reasons and specialties.
- Public search and availability checks are read-only; booking is out of scope.
- Include direct profile URLs when presenting doctors.
- Keep locations and other operator preferences in `AGENTS.override.md` or
  local configuration.

## Build And Test

```bash
go test ./...
go vet ./...
go build ./...
```
