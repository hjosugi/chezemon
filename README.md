# Chezemon

Chezemon is a local-first visual control plane for
[chezmoi](https://www.chezmoi.io/). It puts chezmoi's source, rendered target,
live home state, and source Git repository in one review queue.

The first milestone is intentionally read-only. Chezemon helps you understand
what will happen before it is allowed to change anything.

## Why

`git status` only reports changes in the source repository. `chezmoi status`
reports drift between the generated target and the live home directory. A clean
Git tree therefore does not mean that the machine is synchronized.

Chezemon makes the full model visible:

```text
Git/source files  ->  rendered target  ->  live home
      history          desired state        actual state
```

It prioritizes diverged files, explains status codes in plain language, shows
per-file diffs, highlights scripts, and masks likely-sensitive diffs until the
user explicitly reveals them.

It also turns that state into a guided workflow. The dashboard identifies the
current phase, explains the single safest next move, and shows which later
stages are clear or waiting.

## Run

Requirements:

- Go 1.26 or newer
- chezmoi initialized for the current user
- Git when the chezmoi source directory is a Git repository

Chezemon supports Windows, macOS, and Linux. It is a single Go binary with an
embedded browser UI, so it does not require Electron, Node.js, Python, or a
platform-specific webview runtime.

```bash
go run ./cmd/chezemon
```

Chezemon prints a loopback URL such as `http://127.0.0.1:41273`. Open that URL
in a browser. To open it automatically:

```bash
go run ./cmd/chezemon --open
```

The server refuses non-loopback listen addresses.

To inspect the exact state model without starting the local server:

```bash
chezemon --snapshot
```

This is useful for diagnostics and future editor integrations. It includes
metadata and paths, but never includes file or diff contents.

## Develop

This repository follows the project-local Nix + direnv workflow:

```bash
direnv allow
go test ./...
go build ./cmd/chezemon
```

The application has no third-party Go or browser runtime dependencies.

## Current scope

- Read-only dashboard and risk-ranked review queue
- Parsed chezmoi status with human explanations
- Current-phase and next-action workflow guidance
- Unified local Git state and recent commits
- Per-entry diff preview
- Sensitive-path masking
- `chezmoi doctor` view
- Local-only HTTP server with a restrictive content security policy

Planned write operations will use an explicit plan/preview/confirm flow,
targeted changes, backups, secret scanning, and a serialized operation queue.

## Platform verification

Every change is tested and compiled on:

- Windows (amd64)
- macOS (arm64)
- Linux (amd64)

The implementation itself is architecture-neutral; release packaging will add
amd64 and arm64 artifacts for all supported operating systems where Go and
chezmoi support them.

## Project documents

- [Research and pain inventory](docs/research.md)
- [Product and safety design](docs/product.md)
- [Recommended chezmoi workflow](docs/recommended-workflow.md)

Chezemon is an independent project and is not affiliated with the chezmoi
project.
