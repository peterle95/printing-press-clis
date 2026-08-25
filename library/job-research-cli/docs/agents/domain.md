# Domain Docs

## Before exploring

- Read root `CONTEXT.md`, when present.
- Read relevant ADRs under `docs/adr/`.
- Use terminology defined in `CONTEXT.md`.
- Surface conflicts with existing ADRs instead of silently overriding them.

Missing domain files are normal; create them lazily when domain terms or decisions are resolved.

## Layout

This is a single-context repo:

```text
/
├── CONTEXT.md
├── docs/adr/
└── src/
```
