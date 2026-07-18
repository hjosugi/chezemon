# Chezemon product and safety design

## Product statement

Chezemon is a local-first visual companion for chezmoi. It turns ambiguous
dotfile drift into a risk-ranked review queue with enough context to choose the
right action safely.

Tagline: **See the drift. Keep what matters.**

## State model

```text
remote/upstream
      |
      v
Git working tree / source state
      |
      | render templates and attributes
      v
target state for this machine
      |
      | apply
      v
live destination under HOME
```

Reverse flows are intentionally explicit:

- Live to source for a plain file: add/re-add after review.
- Live to source for a template: guided extraction/merge, never blind re-add.
- Source to live: targeted apply after review.
- Both changed: three-way merge.

## Information architecture

Sections below marked *Planned* describe intended design that is not built yet.
Everything else describes what ships today.

### Guided workflow

The overview always answers three questions without requiring knowledge of
chezmoi's status codes:

1. What state am I in now?
2. What is the safest next action?
3. What stages remain before the machine is synchronized?

The first implementation derives a six-stage flow from the current snapshot:

1. Protect files changed on both sides.
2. Decide whether to keep live-only changes.
3. Stabilize the source Git working tree.
4. Reconcile source history with its upstream.
5. Inspect scripts and other side effects.
6. Review and apply desired file changes.

Only the first outstanding stage is marked current. Later outstanding stages
are queued, and empty stages are visibly clear. Selecting a stage focuses the
relevant review queue or Git panel.

### Overview

The four-state pipeline lives in the top bar and carries its own figures —
upstream position, source Git changes, pending target changes, and live-home
divergence. It replaced a separate metric-card grid so that no count is stated
twice on the same screen.

Alongside it: derived notices for critical divergence and pending scripts, the
per-stage counts on the workflow stepper, and source Git changes and recent
commits in a collapsed "Source history" panel. Doctor output is behind the
Health button and runs only when asked.

Chezemon computes create/modify/delete counts separately and publishes them in
the `--snapshot` JSON, but the UI shows only the aggregate.

### Review queue

Ordering is by risk of losing work, then alphabetically by displayed path
within a tier:

1. `critical` — diverged, changed on both sides
2. `high` — scripts, pending deletions, live deletions
3. `medium` — pending modifications, live modifications
4. `low` — creations, on either side

Sensitivity is deliberately **not** a sort key: it is orthogonal to how much
work a mistake would destroy. It surfaces as a `masked` badge and a filter
instead.

Each item explains:

- What changed
- Which state is newer or different
- What the suggested action would keep and discard
- Whether the diff is masked, and why — chezmoi encrypts the file, or the path
  matches the secret-name heuristic

For a pending script, the entry is named after the source script and states
when it runs, because chezmoi reports scripts against a destination path that
is never written.

### Explorer (planned)

Not built. Today the queue shows only entries chezmoi reports as drifted, with
six filters (all, critical, live, scripts, pending, sensitive) and a
case-insensitive substring match on the displayed path.

- Destination-style paths by default
- Managed, unmanaged, ignored, and exact-directory filters
- Fuzzy path/content search
- A local-only ignore list for unmanaged discovery

### Health

Built: `chezmoi doctor --no-network`, rendered verbatim in a dialog.

Planned, none of it parsed or derived yet:

- Template/render errors
- Missing editor/diff/merge helpers
- Cross-device hardlink warning
- Slow command history
- Script and external dependency inventory

## Safety invariants

1. Read-only is the default mode.
2. The HTTP server binds only to a loopback address.
3. No shell command strings: subprocesses receive an executable and argument
   vector.
4. All chezmoi mutations are serialized. (No mutations exist yet. Read-only
   commands run concurrently within a snapshot; this was measured against
   chezmoi's persistent state lock without contention.)
5. Global apply is unavailable while unresolved divergence exists unless the
   user explicitly overrides a high-friction warning.
6. Every mutation starts with a fresh status and dry-run plan.
7. A backup and operation journal are created before destination changes.
8. Add/re-add uses secret scanning in error mode.
9. Sensitive diffs are masked by default and never written to application logs.
   Masking uses chezmoi's own encrypted-file list plus a filename heuristic;
   the `private_` attribute is deliberately not a signal, because it denotes
   file mode 0600 rather than secrecy.
10. Templates are never blindly reconstructed from destination output.
11. Script execution is shown separately from file changes.
12. The UI never treats Git clean as machine synchronized.
13. The HTTP server rejects non-loopback `Host` headers, so binding to
    127.0.0.1 cannot be bypassed by DNS rebinding.
14. Diff requests are restricted to paths already present in the review queue.

## MVP

The first milestone is read-only:

- Local web UI embedded in one Go binary
- Windows, macOS, and Linux from the same codebase and artifact shape
- Status parsing and explanations
- Derived current phase, next action, and end-to-end workflow
- Risk-ranked review queue
- Git summary and recent commits
- Masked, colorized per-entry diff, truncated past 256 KiB
- Doctor output
- Short-lived snapshot cache; read-only chezmoi calls issued concurrently

## Next milestones

### M2: safe targeted actions

- Apply one selected non-script entry
- Re-add one selected plain entry with `--secrets=error`
- Explicit keep-source / keep-live decision cards
- Before/after plan and local backup
- Operation journal and undo guidance

### M3: difficult files

- Three-way diff/merge workspace
- Template origin and rendered-value inspection
- Stable/transient JSON, YAML, TOML, and INI key selection
- Suggested `modify_` or ignore rules
- Script privilege and dependency display (name, run reason and timing shipped
  in the MVP)

### M4: integrations

- VS Code extension backed by Chezemon's JSON API
- Desktop launcher and optional system tray status
- TUI client using the same core
- Multi-machine profile preview without applying

The core remains the product boundary. The browser UI, VS Code extension,
desktop launcher, and future TUI are clients of the same state and safety
model, rather than separate implementations.

## Non-goals

- Replacing chezmoi's source format or template engine
- Automatically uploading dotfiles or telemetry
- Sending file contents to an external service
- Silently committing, pushing, applying, or re-adding files
