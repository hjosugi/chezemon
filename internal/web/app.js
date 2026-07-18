const state = {
  snapshot: null,
  selected: null,
  filter: "all",
  search: "",
};

const $ = (selector) => document.querySelector(selector);
const list = $("#entry-list");
const detail = $("#detail-panel");

async function api(path) {
  const response = await fetch(path, { headers: { Accept: "application/json" } });
  const payload = await response.json();
  if (!response.ok) {
    throw new Error(payload.error || `Request failed (${response.status})`);
  }
  return payload;
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function diffLineClass(line) {
  if (
    line.startsWith("diff ") ||
    line.startsWith("index ") ||
    line.startsWith("--- ") ||
    line.startsWith("+++ ") ||
    line.startsWith("new file") ||
    line.startsWith("deleted file") ||
    line.startsWith("old mode") ||
    line.startsWith("new mode") ||
    line.startsWith("rename ") ||
    line.startsWith("similarity ")
  ) {
    return "diff-meta";
  }
  if (line.startsWith("@@")) return "diff-hunk";
  if (line.startsWith("+")) return "diff-add";
  if (line.startsWith("-")) return "diff-del";
  return "";
}

function renderDiff(target, content) {
  target.innerHTML = content
    .split("\n")
    .map((line) => `<span class="diff-line ${diffLineClass(line)}">${escapeHTML(line)}</span>`)
    .join("");
}

function riskIcon(risk) {
  if (risk === "critical") return "!";
  if (risk === "high") return "▲";
  if (risk === "medium") return "●";
  return "·";
}

function statusCopy(entry) {
  const base = `${entry.code.replaceAll(" ", "·")} · ${entry.label}`;
  return entry.scriptTiming ? `${base} · runs ${entry.scriptTiming}` : base;
}

function visibleEntries() {
  if (!state.snapshot) return [];
  return state.snapshot.entries.filter((entry) => {
    const matchesFilter =
      state.filter === "all" ||
      entry.risk === state.filter ||
      entry.kind === state.filter ||
      (state.filter === "local" && entry.kind.startsWith("local-")) ||
      (state.filter === "pending" && entry.kind.startsWith("pending-")) ||
      (state.filter === "sensitive" && entry.sensitive);
    const matchesSearch = entry.displayPath.toLowerCase().includes(state.search);
    return matchesFilter && matchesSearch;
  });
}

function renderEntries() {
  const entries = visibleEntries();
  if (!entries.length) {
    list.innerHTML = `<div class="empty-state">No changes match this view.</div>`;
    return;
  }
  list.innerHTML = entries
    .map(
      (entry) => `
        <button class="entry ${entry.risk} ${state.selected?.path === entry.path ? "selected" : ""}"
                data-path="${escapeHTML(entry.path)}">
          <span class="risk-icon">${riskIcon(entry.risk)}</span>
          <span class="entry-main">
            <strong>${escapeHTML(entry.displayPath)}</strong>
            <small>${escapeHTML(statusCopy(entry))}</small>
          </span>
          ${entry.sensitive ? '<span class="sensitive-tag">masked</span>' : ""}
          <span class="entry-chevron">›</span>
        </button>`,
    )
    .join("");

  list.querySelectorAll(".entry").forEach((button) => {
    button.addEventListener("click", () => {
      const entry = state.snapshot.entries.find((item) => item.path === button.dataset.path);
      selectEntry(entry);
    });
  });
}

function selectEntry(entry) {
  state.selected = entry;
  renderEntries();
  detail.innerHTML = `
    <div class="detail-heading">
      <span class="risk-badge ${entry.risk}">${escapeHTML(entry.risk)}</span>
      ${entry.sensitive ? '<span class="sensitive-tag">sensitive path</span>' : ""}
    </div>
    <h2>${escapeHTML(entry.displayPath)}</h2>
    <p class="detail-status">${escapeHTML(statusCopy(entry))}</p>
    ${entry.sourcePath ? `<p class="detail-source">source: <code>${escapeHTML(entry.sourcePath)}</code></p>` : ""}
    ${entry.sensitiveReason ? `<p class="detail-source">masked because ${escapeHTML(entry.sensitiveReason)}</p>` : ""}
    <div class="explanation">
      <span>What this means</span>
      <p>${escapeHTML(entry.explanation)}</p>
    </div>
    <div class="recommendation">
      <span>Safest next step</span>
      <p>${escapeHTML(entry.recommendedAction)}</p>
    </div>
    <button class="button primary wide" id="diff-button">Preview diff</button>
    <div class="diff-shell hidden" id="diff-shell">
      <div class="diff-toolbar">
        <span>chezmoi diff</span>
        <button class="text-button hidden" id="reveal-button">Reveal sensitive diff</button>
      </div>
      <pre id="diff-output">Loading…</pre>
    </div>
  `;
  $("#diff-button").addEventListener("click", () => loadDiff(entry, false));
}

async function loadDiff(entry, reveal) {
  const shell = $("#diff-shell");
  const output = $("#diff-output");
  const revealButton = $("#reveal-button");
  shell.classList.remove("hidden");
  output.textContent = "Rendering target and diff…";
  try {
    const diff = await api(
      `/api/diff?path=${encodeURIComponent(entry.path)}&reveal=${reveal ? "true" : "false"}`,
    );
    if (diff.sensitive && !diff.revealed) {
      output.textContent = diff.message;
      revealButton.classList.remove("hidden");
      revealButton.onclick = () => loadDiff(entry, true);
      return;
    }
    revealButton.classList.add("hidden");
    if (diff.content) {
      renderDiff(output, diff.content);
    } else {
      output.textContent = "No textual diff is available for this entry.";
    }
  } catch (error) {
    output.textContent = `Could not render diff:\n${error.message}`;
  }
}

function renderNotices(notices) {
  const container = $("#notices");
  container.innerHTML = notices
    .map(
      (notice) => `
        <article class="notice ${escapeHTML(notice.level)}">
          <span class="notice-mark">${notice.level === "critical" ? "!" : notice.level === "warning" ? "▲" : "i"}</span>
          <div><strong>${escapeHTML(notice.title)}</strong><p>${escapeHTML(notice.message)}</p></div>
        </article>`,
    )
    .join("");
}

function setQueueFilter(filter) {
  state.filter = filter;
  document.querySelectorAll(".filter").forEach((item) => {
    item.classList.toggle("active", item.dataset.filter === filter);
  });
  renderEntries();
}

function renderWorkflow(workflow) {
  $("#workflow-phase").textContent = workflow.phaseLabel;
  $("#workflow-title").textContent =
    workflow.currentStep > 0
      ? workflow.steps[workflow.currentStep - 1].title
      : "Everything visible is synchronized";
  $("#workflow-summary").textContent = workflow.summary;
  $("#workflow-clear").textContent = workflow.clear;
  $("#workflow-total").textContent = workflow.total;
  $("#workflow-outstanding").textContent = workflow.outstanding;

  // Long per-stage prose lives in the tooltip: on a dashboard the count and
  // the state are what get scanned, not the paragraph.
  const container = $("#workflow-steps");
  container.innerHTML = workflow.steps
    .map(
      (step) => `
        <button class="step ${escapeHTML(step.state)}"
                data-filter="${escapeHTML(step.queueFilter || "")}"
                title="${escapeHTML(`${step.title} — ${step.description}`)}"
                ${step.state === "clear" ? "disabled" : ""}>
          <span class="step-marker">${step.state === "clear" ? "–" : step.number}</span>
          <span class="step-label">${escapeHTML(step.shortTitle || step.title)}</span>
          <span class="step-count">${step.count}</span>
        </button>`,
    )
    .join("");

  container.querySelectorAll(".step:not(:disabled)").forEach((button) => {
    button.addEventListener("click", () => {
      const filter = button.dataset.filter;
      if (filter === "git") {
        const details = $("#git-details");
        details.open = true;
        details.scrollIntoView({ behavior: "smooth", block: "start" });
        return;
      }
      setQueueFilter(filter || "all");
    });
  });
}

function setStage(id, value, tone) {
  const node = $(id);
  node.textContent = value;
  node.className = `stage-value${tone ? ` ${tone}` : ""}`;
}

function renderSummary(snapshot) {
  const { counts, git } = snapshot;

  setStage(
    "#upstream-state",
    !git.available
      ? "n/a"
      : !git.hasUpstream
        ? "no upstream"
        : git.ahead || git.behind
          ? `↑${git.ahead} ↓${git.behind}`
          : "aligned",
    git.available && git.hasUpstream && (git.ahead || git.behind) ? "warn" : "",
  );
  setStage(
    "#git-state",
    !git.available ? "n/a" : git.clean ? "clean" : `${git.changes.length} changed`,
    git.available && !git.clean ? "warn" : "",
  );
  setStage("#target-state", counts.pending ? `${counts.pending} pending` : "clean", counts.pending ? "warn" : "");
  setStage(
    "#home-state",
    counts.critical
      ? `${counts.critical} diverged`
      : counts.total
        ? `${counts.total} drifted`
        : "in sync",
    counts.critical ? "bad" : counts.total ? "warn" : "",
  );

  $("#timestamp").textContent =
    `${new Date(snapshot.generatedAt).toLocaleTimeString()} · ${snapshot.durationMs}ms`;
}

function renderGit(git) {
  $("#branch-pill").textContent = git.available
    ? `${git.branch || "detached"}${git.hasUpstream ? ` · ↑${git.ahead} ↓${git.behind}` : ""}`
    : "not a Git tree";
  $("#git-summary-hint").textContent = git.available
    ? git.clean
      ? "working tree clean"
      : `${git.changes.length} working-tree change${git.changes.length === 1 ? "" : "s"}`
    : git.error || "unavailable";
  const changes = $("#git-changes");
  if (!git.available) {
    changes.innerHTML = `<p class="muted">${escapeHTML(git.error || "Git is unavailable.")}</p>`;
  } else if (git.clean) {
    changes.innerHTML = `<div class="git-clean"><span>✓</span><div><strong>Working tree clean</strong><small>This says nothing about live-home drift.</small></div></div>`;
  } else {
    changes.innerHTML = git.changes
      .map(
        (change) => `<div class="git-row"><code>${escapeHTML(change.code)}</code><span>${escapeHTML(change.path)}</span></div>`,
      )
      .join("");
  }

  const commits = $("#git-commits");
  commits.innerHTML = git.commits?.length
    ? git.commits
        .map(
          (commit) => `
            <div class="commit">
              <code>${escapeHTML(commit.hash)}</code>
              <span><strong>${escapeHTML(commit.subject)}</strong><small>${escapeHTML(commit.date)}</small></span>
            </div>`,
        )
        .join("")
    : `<p class="muted">No commit history available.</p>`;
}

async function refresh(force = false) {
  const button = $("#refresh-button");
  button.disabled = true;
  button.textContent = "Refreshing…";
  try {
    const snapshot = await api(`/api/snapshot${force ? "?force=true" : ""}`);
    state.snapshot = snapshot;
    renderSummary(snapshot);
    renderNotices(snapshot.notices || []);
    renderWorkflow(snapshot.workflow);
    renderGit(snapshot.git);
    renderEntries();
    if (state.selected) {
      const updated = snapshot.entries.find((entry) => entry.path === state.selected.path);
      if (updated) selectEntry(updated);
      else {
        state.selected = null;
        detail.innerHTML = `<div class="detail-empty"><h3>Queue changed</h3><p>Select another entry to continue.</p></div>`;
      }
    }
  } catch (error) {
    list.innerHTML = `<div class="error-state"><strong>Could not read chezmoi state</strong><p>${escapeHTML(error.message)}</p></div>`;
  } finally {
    button.disabled = false;
    button.textContent = "Refresh";
  }
}

document.querySelectorAll(".filter").forEach((button) => {
  button.addEventListener("click", () => {
    setQueueFilter(button.dataset.filter);
  });
});

$("#search-input").addEventListener("input", (event) => {
  state.search = event.target.value.trim().toLowerCase();
  renderEntries();
});

$("#refresh-button").addEventListener("click", () => refresh(true));

const healthDialog = $("#health-dialog");
$("#doctor-button").addEventListener("click", async () => {
  healthDialog.showModal();
  $("#health-output").textContent = "Running chezmoi doctor…";
  try {
    const result = await api("/api/doctor");
    $("#health-output").textContent = result.output || "No output.";
  } catch (error) {
    $("#health-output").textContent = error.message;
  }
});
$("#health-close").addEventListener("click", () => healthDialog.close());
healthDialog.addEventListener("click", (event) => {
  if (event.target === healthDialog) healthDialog.close();
});

refresh();
