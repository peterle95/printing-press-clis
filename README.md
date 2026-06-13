# Printing Press CLIs

A WSL-native monorepo containing generated and hand-authored command-line tools
built around [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press).

The repository intentionally lives at `~/printing-press`. That keeps the
Printing Press default library path, `~/printing-press/library`, usable without
environment overrides or compatibility links.

## Layout

```text
~/printing-press/
├── library/                  # 23 independent CLI projects
├── external/cli-printing-press/
├── scripts/                  # Bootstrap, audit, verification, installers
├── manuscripts/              # Local runtime state; ignored
├── .runstate/                # Local runtime state; ignored
└── workspace.yaml            # Project and runtime inventory
```

The generator is a Git submodule because it has its own upstream history and
release lifecycle. Each CLI remains an independent Go, Node.js, or Python
project; this repository does not impose a shared language workspace.

## Bootstrap

Clone with submodules and prepare project dependencies:

```bash
git clone --recurse-submodules https://github.com/peterle95/printing-press-clis.git ~/printing-press
cd ~/printing-press
./scripts/bootstrap.sh
```

To also install CLI binaries into user-local locations:

```bash
./scripts/bootstrap.sh --install
```

## Verification

```bash
./scripts/audit-public.sh
./scripts/verify.sh --all
```

## Windows Runtime

Linux CLIs are invoked from PowerShell through wrappers installed under
`%LOCALAPPDATA%\Programs\printing-press\bin`.

From WSL:

```bash
./scripts/install-windows-runtime.sh
```

The installer does not modify the Windows user `PATH`; add the destination to
`PATH` separately if desired.

## Private State

Credentials and runtime data belong outside Git. Existing local Google and mail
state is stored below `~/.config/printing-press` and
`~/.local/share/printing-press`; ignored compatibility links may exist under
`library/` on an operator machine.

Operator-specific behavior belongs in ignored `AGENTS.override.md` files.

## License

Repository tooling and hand-authored CLIs are licensed under Apache-2.0.
Generated and imported projects retain their own `LICENSE` and `NOTICE` files.
