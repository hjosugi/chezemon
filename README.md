# Chezemon

[日本語](README.ja.md) · English

[![CI](https://github.com/hjosugi/chezemon/actions/workflows/ci.yml/badge.svg)](https://github.com/hjosugi/chezemon/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-b8f36b.svg)](LICENSE)

Chezemon is a local-first visual control plane for
[chezmoi](https://www.chezmoi.io/). It shows the full path from upstream
history to the files currently living in your home directory, then tells you
what to review next.

Chezemon is intentionally read-only today. It helps you understand and protect
your dotfiles before any tool is allowed to change them.

## Quick start

Requirements:

- Go 1.26 or newer
- chezmoi initialized for the current user
- Git when the chezmoi source directory is a Git repository

```bash
git clone https://github.com/hjosugi/chezemon.git
cd chezemon
go run ./cmd/chezemon --open
```

Chezemon prints a loopback URL such as `http://127.0.0.1:41273`. If the browser
does not open automatically, open the printed URL yourself.

Windows, macOS, and Linux are supported. The UI is embedded in a single Go
binary; Electron, Node.js, Python, and platform-specific webview runtimes are
not required.

## What problem does it solve?

Dotfile state is spread across four layers:

```text
remote/upstream -> Git/source -> rendered target -> live home
     history       declaration    desired state     actual state
```

`git status` only reports source-repository changes. `chezmoi status` reports
drift between rendered target state and the live home directory. A clean Git
tree therefore does not mean that the machine is synchronized.

Chezemon joins those layers into one risk-ranked review queue. It:

- explains chezmoi's two-column status in plain language;
- puts files changed on both sides before ordinary pending changes;
- identifies the current phase and the single safest next move;
- shows the remaining workflow as clear, current, or queued;
- previews one file diff at a time;
- separates scripts from file changes;
- masks likely-sensitive diffs until explicitly revealed;
- shows source Git state, upstream position, and recent commits.

## Recommended workflow

Chezemon derives a six-stage path from every fresh snapshot:

1. Protect files changed on both sides.
2. Decide whether to keep live-only changes.
3. Stabilize the source Git working tree.
4. Reconcile source history with its upstream.
5. Inspect scripts and their side effects.
6. Review and apply desired file changes.

Select a stage to focus its queue. Select an entry to read the explanation,
safest next step, and diff. Refresh after resolving a stage and Chezemon will
advance the current phase automatically.

See [Recommended chezmoi workflow](docs/recommended-workflow.md) for the manual
commands and safety rules that complement the read-only UI.

## CLI snapshot

The same state model is available without starting the local server:

```bash
go run ./cmd/chezemon --snapshot
```

The JSON output includes metadata and paths, but never file or diff contents.
This interface is intended to power future VS Code, desktop, and TUI clients
without duplicating chezmoi logic.

## Safety

- Read-only by default
- Loopback-only HTTP listener
- No shell command strings
- Serialized chezmoi operations
- Built-in diff implementation
- Sensitive diff masking
- No telemetry or external file upload
- Restrictive browser content security policy

Future write operations will use a fresh plan, explicit preview and
confirmation, narrow targets, backups, secret scanning, and an operation
journal.

## Development

With Go installed:

```bash
go test ./...
go run ./cmd/chezemon --open
```

With Nix and direnv:

```bash
direnv allow
go test ./...
go build ./cmd/chezemon
```

The application has no third-party Go or browser runtime dependencies.

Every change is tested and compiled in CI on Windows, macOS, and Linux. The
codebase is also cross-built locally for amd64 and arm64 on all three operating
systems.

## Project documents

- [Research and pain inventory](docs/research.md)
- [Product and safety design](docs/product.md)
- [Recommended chezmoi workflow](docs/recommended-workflow.md)

Chezemon is an independent project and is not affiliated with the chezmoi
project.
