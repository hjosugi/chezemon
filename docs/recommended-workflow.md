# Recommended chezmoi workflow

This is the management model Chezemon is designed to enforce.

## Keep the states separate

- **Source** is the versioned declaration you intentionally maintain.
- **Target** is source rendered for the current machine.
- **Live home** is what applications and people are changing right now.
- **Git upstream** is distribution and history, not proof that the live machine
  is synchronized.

A clean Git working tree only answers a source-control question. Always check
rendered/live drift separately.

## Safest review order

### 1. Protect changes made on both sides

Treat an `MM`, `MD`, `DM`, or other two-column status as a merge decision.
Review one file at a time:

```bash
chezmoi diff ~/.config/example/config.json
chezmoi merge ~/.config/example/config.json
chezmoi status
```

Do not run a broad `apply` or `re-add` until these files are understood. For a
template, merge useful intent into the template; generated destination content
cannot reconstruct the source template safely.

### 2. Decide what to do with live-only changes

For each live edit, choose explicitly:

- Keep it: add the reviewed plain file with secret scanning.
- Restore it: apply the specific managed target.
- Keep only part: edit or merge the source declaration.

```bash
chezmoi add --secrets=error ~/.config/example/config.json
chezmoi apply ~/.config/example/config.json
```

Avoid an unscoped `re-add`. It does not overwrite templates, but it can still
capture more plain-file changes than intended.

### 3. Stabilize and synchronize source Git

Inspect source history after live changes have been reconciled:

```bash
chezmoi cd
git status
git diff
```

Commit related changes in small units. Pull only with a clean, reviewed source
tree; push only after local rendering and apply results are understood.

### 4. Inspect scripts separately

Scripts can install packages, use the network, change services, or request
privileges. A file diff does not fully describe those effects.

```bash
chezmoi diff --script-contents
```

Read the run condition (`run_once_`, `run_onchange_`, or always-run) and the
script body before applying.

### 5. Apply narrow targets, then verify

Start with selected paths:

```bash
chezmoi apply ~/.config/example
chezmoi status
git -C "$(chezmoi source-path)" status
```

Use a global apply only when the review queue no longer contains unexpected
divergence, deletions, scripts, or sensitive changes.

## Configuration hygiene

- Keep deterministic preferences under management; ignore caches, window
  geometry, recents, generated IDs, and runtime state.
- When an application mixes stable and transient keys, use a template,
  `modify_` script, or structured merge instead of tracking the whole file.
- Prefer the destination path in user-facing tools; treat `dot_`, `private_`,
  `executable_`, and `.tmpl` names as implementation attributes.
- Encrypt secret-bearing files or reference a secret manager. Never assume a
  private repository is adequate secret protection.
- Make machine differences explicit with small templates and data, and preview
  the rendered target on each platform.
