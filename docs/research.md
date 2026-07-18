# Chezemon research: user pain inventory

Date: 2026-07-18

This document records the problems Chezemon should solve before features are
selected. It combines chezmoi documentation, upstream issues, current
frontends, community reports, and a real local repository with substantial
source/target drift.

## Working name

`Chezemon` is the working product name. A preliminary GitHub and general web
search found no obvious exact-name dotfile product as of the research date.
That is product discovery, not legal clearance; package registries, domains,
and relevant trademark databases should be checked again before a public
launch.

## Research summary

The core problem is not the absence of one more file browser. Chezmoi has three
filesystem states plus a separate Git state:

1. Source state: the versioned declaration, with encoded file attributes.
2. Target state: the desired state rendered for the current machine.
3. Destination state: the files currently present in the user's home.
4. Git state: source history, staging, commits, upstream and remote state.

Most existing tools expose only part of that model. This creates false
confidence: a clean source Git tree does not mean the home directory is
synchronized.

## Pain matrix

### P0: destructive ambiguity

- The two `chezmoi status` columns are compact but hard to interpret without
  knowing the three-state model.
- A file changed in both source/target and destination requires a three-way
  decision. Blind `apply` or `re-add` can discard wanted changes.
- `exact_` directory behavior can unexpectedly make sibling paths managed and
  scheduled for deletion.
- Scripts can run during apply, including scripts that request privileges or
  perform network and package operations.
- Secrets or company-specific values can be added to source and later pushed.

Evidence:

- [Status command and its two-column semantics](https://www.chezmoi.io/reference/commands/status/)
- [Merge is a three-state operation](https://www.chezmoi.io/user-guide/tools/merge/)
- [Exact recursive add can manage more than expected](https://github.com/twpayne/chezmoi/issues/4223)
- [Current request for safer overwrite behavior](https://github.com/twpayne/chezmoi/issues/5110)
- [Official warning about auto-pushing secrets](https://www.chezmoi.io/user-guide/daily-operations/)

### P0: externally modified application settings

GUI applications frequently store stable preferences together with window
geometry, recent files, caches, generated identifiers, and machine-specific
state. The result is permanent noisy drift. Users need to preserve selected
live changes without copying transient or sensitive values.

Evidence:

- [GUI applications mix settings and irrelevant state](https://github.com/twpayne/chezmoi/issues/2083)
- [Official guidance for externally modified files](https://www.chezmoi.io/user-guide/manage-different-types-of-file/)
- [`modify_` can still create overwrite anxiety](https://github.com/twpayne/chezmoi/issues/5110)

### P1: no ergonomic review/apply loop

Reviewing a large apply requires repeated diff and confirmation actions. Users
want a persistent diff followed by a simple per-file keep/apply/skip decision.
Upstream declined to build a built-in file UI, leaving room for a dedicated
frontend.

Evidence:

- [Progressive one-by-one apply request](https://github.com/twpayne/chezmoi/issues/5034)
- [Managed-file UI request closed as not planned](https://github.com/twpayne/chezmoi/issues/4099)

### P1: reverse synchronization and templates

Plain files can be re-added, but templates cannot be reconstructed reliably
from generated destination files. Users still want help extracting the useful
change, locating the source template, and merging it intentionally.

Evidence:

- [Generated file to source template feature request](https://github.com/twpayne/chezmoi/issues/4272)
- [Official note that `re-add` does not work with templates](https://www.chezmoi.io/user-guide/frequently-asked-questions/usage/)

### P1: source representation is optimized for chezmoi, not humans

Names such as `dot_`, `private_`, `executable_`, and `.tmpl` encode behavior but
obscure the destination path. Users report that directly browsing the source
tree is inconvenient.

Evidence:

- [Community complaint about encoded source names](https://www.reddit.com/r/linux/comments/1e7u2bm/how_do_you_guys_manage_your_years_of_experience/)
- [Recent KISS discussion about encoded names and reverse apply](https://www.reddit.com/r/git/comments/1tl7tjq/kiss_to_manage_dotfiles/)

### P1: unmanaged discovery is too noisy

`chezmoi unmanaged` includes caches, runtime state, virtual environments,
downloaded plugins, and application data. Users need a discovery inbox with
durable local filtering, not a raw home-directory dump.

Evidence:

- [Request for filtering unmanaged output](https://github.com/twpayne/chezmoi/issues/4813)

### P1: source Git and home drift are separate workflows

Git clients visualize source changes but cannot see rendered/live drift.
Chezmoi tools see drift but often provide limited Git review. The user should
not have to mentally join two dashboards.

### P2: performance and re-entrancy

Full status, diff, unmanaged scans, templates, externals, and secret managers
can be slow. Concurrent or nested chezmoi invocations may contend on the
persistent-state lock. A GUI must cache, cancel, serialize, and show progress.

Evidence:

- [Persistent-state lock contention from nested commands](https://github.com/twpayne/chezmoi/issues/4609)
- [Reported startup regression](https://github.com/twpayne/chezmoi/issues/4353)

### P2: editor and filesystem edge cases

`chezmoi edit` uses temporary paths and may require an editor wait flag.
Hardlinks can fail when the source directory and temporary directory are on
different filesystems. The frontend should detect and explain these conditions.

Evidence:

- [VS Code requires `--wait` with `chezmoi edit`](https://www.chezmoi.io/user-guide/frequently-asked-questions/troubleshooting/)

## Existing frontend review

### chezit

Strengths:

- Strong unified TUI for drift and Git state
- Status, files, info, commands, staging, commits, and push
- Explicit local-drift categories and preview panel

Gaps for Chezemon's target:

- Terminal-only
- Limited room for persistent three-way explanation, policy, and guided review
- Very new project; custom diff support has already been requested

Source: [daptify14/chezit](https://github.com/daptify14/chezit)

### cheznav

Strengths:

- Excellent dual-pane mental model: live home and managed tree
- Context actions and side-by-side diff
- Current AUR/PyPI/Homebrew distribution

Gaps:

- Does not make Git history the primary fourth state
- A diff reliability issue has been reported

Source: [djetelina/cheznav](https://github.com/djetelina/cheznav)

### chezmoi-mousse

Strengths:

- Rich Textual interface
- Read-only/dry-run default

Gaps:

- Terminal interface rather than a desktop/browser GUI
- Packaging and parsing robustness issues have been reported

Source: [matmaer/chezmoi-mousse](https://github.com/matmaer/chezmoi-mousse)

### chezmoi-ui

This is a web UI for managing application lists in an Install.Doctor-compatible
format. It is not a source/target/destination review UI.

Source: [johan-weitner/chezmoi-ui](https://github.com/johan-weitner/chezmoi-ui)

## Product conclusions

Chezemon should:

1. Be a frontend and safety layer over chezmoi, not a new dotfile engine.
2. Present source, rendered target, live destination, and Git together.
3. Rank review items by potential data loss, not alphabetically.
4. Show a persistent per-file diff and eventually a three-way merge view.
5. Distinguish stable configuration, transient state, generated data, and
   likely secrets.
6. Default to read-only and require plan/preview/confirm for mutations.
7. Serialize chezmoi operations and cache expensive reads.
8. Provide one local core that can serve a browser GUI, VS Code integration,
   and future TUI clients.
