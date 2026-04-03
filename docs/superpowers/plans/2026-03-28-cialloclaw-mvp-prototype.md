# CialloClaw MVP Prototype Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a single-page local web prototype that demonstrates the CialloClaw loop of inspection, confirmation, draft generation, logging, and one-step undo.

**Architecture:** Use a no-build static app: `index.html` for structure, `style.css` for the visual shell, `logic.js` for pure reusable helpers, and `app.js` for browser state/rendering. Keep business rules in `logic.js` so they can be tested with Node's built-in test runner, while browser interactions stay thin and manual-verification-friendly.

**Tech Stack:** HTML, CSS, vanilla JavaScript, Node built-in test runner (`node --test`)

---

## File Structure

- `tests/` - Create this directory before adding the automated test files below
- `index.html` - Static shell with floating ball, assistant panel, task list, result area, and log area
- `style.css` - Page layout, floating ball styling, panel styling, task cards, result cards, and log list styling
- `logic.js` - Pure helpers exposed to both Node tests and the browser (`inspectTasks`, `buildDraft`, `createLogEntry`)
- `app.js` - Demo task data, in-memory UI state, DOM event listeners, render functions, and confirmation/undo flow
- `tests/logic.test.js` - Automated tests for inspection, draft generation, and log entry helpers
- `tests/index.test.js` - Smoke tests that verify required markup anchors exist in `index.html`

## Shared Demo Rules

- Use this inspection rule everywhere: a task is relevant when `status === "pending"` and either `dueAt` is today or `priority === "high"`
- Put confirmation in the assistant panel by reusing the main draft button: first click changes it to `Confirm Generate`, second click performs generation
- Use deterministic demo task names:
  - `Finish weekly status update`
  - `Review candidate notes`
  - `Archive closed ticket notes`
- Ensure at least one built-in task always matches inspection by generating one `dueAt` value from today's date inside `app.js`
- Keep undo narrow: remove only the generated draft block, not the inspection summary or log history

### Task 1: Implement Pure Inspection And Draft Logic

**Files:**
- Create: `logic.js`
- Create: `tests/logic.test.js`

- [ ] **Step 1: Write the failing test**

```js
const test = require('node:test');
const assert = require('node:assert/strict');
const { inspectTasks, buildDraft, createLogEntry } = require('../logic.js');

const today = '2026-03-28';
const tasks = [
  {
    title: 'Finish weekly status update',
    dueAt: today,
    status: 'pending',
    priority: 'high',
    suggestedAction: 'Generate a progress update draft'
  },
  {
    title: 'Review candidate notes',
    dueAt: '2026-03-29',
    status: 'pending',
    priority: 'high',
    suggestedAction: 'Summarize follow-up items'
  },
  {
    title: 'Archive closed ticket notes',
    dueAt: '2026-03-27',
    status: 'done',
    priority: 'low',
    suggestedAction: 'Move notes to archive'
  }
];

test('inspectTasks returns only pending tasks due today or high priority', () => {
  const result = inspectTasks(tasks, today);
  assert.equal(result.relevantTasks.length, 2);
  assert.deepEqual(
    result.relevantTasks.map((task) => task.title),
    ['Finish weekly status update', 'Review candidate notes']
  );
  assert.match(result.summary, /2 tasks need follow-up/i);
});

test('buildDraft produces a readable draft from the summary and tasks', () => {
  const inspection = inspectTasks(tasks, today);
  const draft = buildDraft(inspection.summary, inspection.relevantTasks);
  assert.match(draft, /Finish weekly status update/);
  assert.match(draft, /Generate a progress update draft/);
});

test('createLogEntry normalizes action metadata', () => {
  const entry = createLogEntry('inspection', 'success', 'Inspection completed', '2026-03-28T09:30:00.000Z');
  assert.equal(entry.action, 'inspection');
  assert.equal(entry.status, 'success');
  assert.match(entry.timestamp, /2026-03-28/);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/logic.test.js`
Expected: FAIL with `Cannot find module '../logic.js'` or missing export errors

- [ ] **Step 3: Write minimal implementation**

```js
(function (root, factory) {
  const api = factory();
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = api;
  }
  root.CialloLogic = api;
})(typeof globalThis !== 'undefined' ? globalThis : window, function () {
  function inspectTasks(tasks, today) {
    const relevantTasks = tasks.filter((task) => {
      return task.status === 'pending' && (task.dueAt === today || task.priority === 'high');
    });

    const summary = relevantTasks.length
      ? `${relevantTasks.length} tasks need follow-up today.`
      : 'No tasks need follow-up right now.';

    return { relevantTasks, summary };
  }

  function buildDraft(summary, relevantTasks) {
    if (!relevantTasks.length) {
      return `${summary}\n\nNo action is needed right now.`;
    }

    const firstTask = relevantTasks[0];
    return [
      'Draft Ready',
      '',
      summary,
      '',
      `Top task: ${firstTask.title}`,
      `Suggested next step: ${firstTask.suggestedAction}`
    ].join('\n');
  }

  function createLogEntry(action, status, detail, timestamp) {
    return {
      id: `${action}-${Date.now()}`,
      action,
      status,
      detail,
      timestamp: timestamp || new Date().toISOString()
    };
  }

  return {
    inspectTasks,
    buildDraft,
    createLogEntry
  };
});
```

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/logic.test.js`
Expected: PASS for all three tests

- [ ] **Step 5: Commit if repository is initialized**

```bash
git add logic.js tests/logic.test.js
git commit -m "feat: add prototype inspection logic"
```

If the workspace is still not a git repository, mark the step complete without committing.

### Task 2: Build The Static Page Shell And Smoke-Test The Anchors

**Files:**
- Create: `index.html`
- Create: `style.css`
- Create: `tests/index.test.js`

- [ ] **Step 1: Write the failing smoke test**

```js
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

test('index.html exposes the required prototype regions', () => {
  const markup = fs.readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');
  assert.match(markup, /id="floating-ball"/);
  assert.match(markup, /id="assistant-panel"/);
  assert.match(markup, /id="task-list"/);
  assert.match(markup, /id="inspection-summary"/);
  assert.match(markup, /id="draft-output"/);
  assert.match(markup, /id="execution-log"/);
  assert.match(markup, /src="logic\.js"/);
  assert.match(markup, /src="app\.js"/);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test tests/index.test.js`
Expected: FAIL with `ENOENT` for `index.html` or missing anchor assertions

- [ ] **Step 3: Write the minimal shell and styling**

```html
<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>CialloClaw MVP Prototype</title>
    <link rel="stylesheet" href="style.css">
  </head>
  <body>
    <main class="app-shell">
      <section class="hero">
        <p class="eyebrow">CialloClaw</p>
        <h1>Safe, visible task assistance</h1>
      </section>

      <section class="workspace-grid">
        <article class="panel-card">
          <h2>Task Inspection</h2>
          <div id="task-list"></div>
        </article>

        <article class="panel-card">
          <h2>Results</h2>
          <div id="inspection-summary"></div>
          <div id="draft-output"></div>
        </article>

        <article class="panel-card">
          <h2>Execution Log</h2>
          <div id="execution-log"></div>
        </article>
      </section>
    </main>

    <button id="floating-ball" type="button">2 tasks</button>

    <aside id="assistant-panel" class="assistant-panel is-hidden">
      <p id="assistant-message">Ready to inspect tasks.</p>
      <button id="run-inspection" type="button">Run Inspection</button>
      <button id="generate-draft" type="button">Generate Draft</button>
      <button id="undo-last-action" type="button" disabled>Undo Last Action</button>
      <button id="view-log" type="button">View Log</button>
    </aside>

    <script src="logic.js"></script>
    <script src="app.js"></script>
  </body>
</html>
```

```css
:root {
  --bg: #f4efe6;
  --panel: rgba(255, 250, 242, 0.9);
  --ink: #1f1a17;
  --accent: #d46a3d;
  --accent-strong: #9f3c1f;
  --line: rgba(31, 26, 23, 0.12);
}

body {
  margin: 0;
  min-height: 100vh;
  font-family: "Segoe UI", sans-serif;
  background: radial-gradient(circle at top, #fff5e9, var(--bg));
  color: var(--ink);
}

.workspace-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
}

.assistant-panel.is-hidden {
  display: none;
}

#floating-ball {
  position: fixed;
  right: 24px;
  bottom: 24px;
  border-radius: 999px;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test tests/index.test.js`
Expected: PASS for the shell smoke test

- [ ] **Step 5: Commit if repository is initialized**

```bash
git add index.html style.css tests/index.test.js
git commit -m "feat: add prototype shell"
```

If the workspace is still not a git repository, mark the step complete without committing.

### Task 3: Wire Browser State, Confirmation Flow, And Undo

**Files:**
- Create: `app.js`
- Modify: `index.html`
- Modify: `style.css`

- [ ] **Step 1: Add demo task data, DOM references, and initial state**

```js
const floatingBall = document.getElementById('floating-ball');
const assistantPanel = document.getElementById('assistant-panel');
const assistantMessage = document.getElementById('assistant-message');
const runInspectionButton = document.getElementById('run-inspection');
const generateDraftButton = document.getElementById('generate-draft');
const undoButton = document.getElementById('undo-last-action');
const viewLogButton = document.getElementById('view-log');
const taskList = document.getElementById('task-list');
const inspectionSummary = document.getElementById('inspection-summary');
const draftOutput = document.getElementById('draft-output');
const logSection = document.getElementById('execution-log');

const state = {
  isPanelOpen: false,
  inspectionHasRun: false,
  inspectionSummary: '',
  generatedDraft: '',
  pendingConfirmation: false,
  highlightedTaskTitles: [],
  logs: []
};

function isoDateFromToday(offsetDays) {
  const value = new Date();
  value.setDate(value.getDate() + offsetDays);
  return value.toISOString().slice(0, 10);
}

const tasks = [
  {
    title: 'Finish weekly status update',
    dueAt: isoDateFromToday(0),
    status: 'pending',
    priority: 'high',
    suggestedAction: 'Generate a progress update draft'
  },
  {
    title: 'Review candidate notes',
    dueAt: isoDateFromToday(1),
    status: 'pending',
    priority: 'high',
    suggestedAction: 'Summarize follow-up items'
  },
  {
    title: 'Archive closed ticket notes',
    dueAt: isoDateFromToday(-1),
    status: 'done',
    priority: 'low',
    suggestedAction: 'Move notes to archive'
  }
];
```

- [ ] **Step 2: Wire the floating ball, inspection action, and all render helpers**

```js
function renderTasks() {
  taskList.innerHTML = tasks.map((task) => `
    <article class="task-card ${task.status} ${state.highlightedTaskTitles.includes(task.title) ? 'is-relevant' : ''}">
      <h3>${task.title}</h3>
      <p>${task.dueAt} · ${task.priority} · ${task.status}</p>
      <p>${task.suggestedAction}</p>
    </article>
  `).join('');
}

function renderSummary() {
  inspectionSummary.innerHTML = state.inspectionSummary
    ? `<div class="result-card"><h3>Inspection Summary</h3><p>${state.inspectionSummary}</p></div>`
    : '<p class="placeholder">Run inspection to see task guidance.</p>';
}

function renderDraft() {
  draftOutput.innerHTML = state.generatedDraft
    ? `<div class="result-card draft-card"><h3>Generated Draft</h3><pre>${state.generatedDraft}</pre></div>`
    : '<p class="placeholder">No draft generated yet.</p>';
}

function renderLog() {
  logSection.innerHTML = state.logs.length
    ? state.logs.map((entry) => `
        <div class="log-entry ${entry.status}">
          <strong>${entry.action}</strong>
          <span>${entry.timestamp}</span>
          <p>${entry.detail}</p>
        </div>
      `).join('')
    : '<p class="placeholder">No actions recorded yet.</p>';
}

function render() {
  renderTasks();
  renderSummary();
  renderDraft();
  renderLog();
}

function togglePanel() {
  state.isPanelOpen = !state.isPanelOpen;
  assistantPanel.classList.toggle('is-hidden', !state.isPanelOpen);
}

function handleInspection() {
  const today = new Date().toISOString().slice(0, 10);
  const inspection = window.CialloLogic.inspectTasks(tasks, today);
  state.inspectionHasRun = true;
  state.inspectionSummary = inspection.summary;
  state.highlightedTaskTitles = inspection.relevantTasks.map((task) => task.title);
  state.pendingConfirmation = false;
  state.logs.unshift(
    window.CialloLogic.createLogEntry(
      'inspection',
      inspection.relevantTasks.length ? 'success' : 'info',
      inspection.summary
    )
  );
  assistantMessage.textContent = 'Inspection finished. You can generate a draft now.';
  render();
}
```

- [ ] **Step 3: Wire confirmation, draft generation, log focus, and undo**

```js
function handleDraftGeneration() {
  if (!state.inspectionHasRun) {
    assistantMessage.textContent = 'Please run inspection first.';
    state.logs.unshift(window.CialloLogic.createLogEntry('draft', 'blocked', 'Draft blocked before inspection'));
    renderLog();
    return;
  }

  if (!state.pendingConfirmation) {
    state.pendingConfirmation = true;
    assistantMessage.textContent = 'Confirm draft generation to continue.';
    generateDraftButton.textContent = 'Confirm Generate';
    return;
  }

  const today = new Date().toISOString().slice(0, 10);
  const inspection = window.CialloLogic.inspectTasks(tasks, today);
  state.generatedDraft = window.CialloLogic.buildDraft(inspection.summary, inspection.relevantTasks);
  state.pendingConfirmation = false;
  generateDraftButton.textContent = 'Generate Draft';
  undoButton.disabled = false;
  state.logs.unshift(window.CialloLogic.createLogEntry('draft', 'success', 'Draft generated'));
  assistantMessage.textContent = 'Draft ready. Review the result panel.';
  render();
}

function handleUndo() {
  if (!state.generatedDraft) {
    assistantMessage.textContent = 'Nothing to undo yet.';
    state.logs.unshift(window.CialloLogic.createLogEntry('undo', 'blocked', 'Undo blocked with no draft'));
    renderLog();
    return;
  }

  state.generatedDraft = '';
  undoButton.disabled = true;
  state.logs.unshift(window.CialloLogic.createLogEntry('undo', 'success', 'Removed last generated draft'));
  assistantMessage.textContent = 'Last draft removed.';
  render();
}

function handleViewLog() {
  state.logs.unshift(window.CialloLogic.createLogEntry('view-log', 'success', 'Log section focused'));
  renderLog();
  logSection.scrollIntoView({ behavior: 'smooth', block: 'start' });
}
```

- [ ] **Step 4: Add lightweight action error handling in the assistant panel**

```js
function withActionGuard(actionName, action) {
  return function guardedAction() {
    try {
      action();
    } catch (error) {
      console.error(error);
      assistantMessage.textContent = 'Something went wrong, but your current history is still visible.';
      state.logs.unshift(
        window.CialloLogic.createLogEntry(actionName, 'error', 'Unexpected error handled in UI')
      );
      renderLog();
    }
  };
}

floatingBall.addEventListener('click', withActionGuard('panel', togglePanel));

runInspectionButton.addEventListener('click', withActionGuard('inspection', handleInspection));

generateDraftButton.addEventListener('click', withActionGuard('draft', handleDraftGeneration));

undoButton.addEventListener('click', withActionGuard('undo', handleUndo));

viewLogButton.addEventListener('click', withActionGuard('view-log', handleViewLog));

render();
```

- [ ] **Step 5: Add the supporting highlight and result styles**

```css
.task-card.is-relevant {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px rgba(212, 106, 61, 0.15);
}

.result-card,
.log-entry,
.task-card,
.assistant-panel {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 18px;
}

.draft-card pre {
  white-space: pre-wrap;
  margin: 0;
}
```

- [ ] **Step 6: Do manual verification in the browser**

Open: `index.html`

Verify:
- Floating ball opens and closes the assistant panel
- `Run Inspection` fills the summary area, highlights relevant tasks, and appends a log entry
- First `Generate Draft` click asks for confirmation
- Second click generates a separate draft block below the summary and enables undo
- `Undo Last Action` removes only the draft block and appends a log entry
- `View Log` scrolls to the execution log section and appends a log entry
- A forced error (for example, temporarily changing `renderDraft()` to `throw new Error('forced')`) still leaves the current summary/log visible and shows the fallback assistant message
- Revert the temporary forced-error change before moving to final verification

- [ ] **Step 7: Commit if repository is initialized**

```bash
git add app.js index.html style.css
git commit -m "feat: wire prototype interactions"
```

If the workspace is still not a git repository, mark the step complete without committing.

### Task 4: Run Final Verification And Tighten Copy

**Files:**
- Modify: `app.js`
- Modify: `style.css`
- Modify: `index.html`

- [ ] **Step 1: Run the full automated test set**

Run: `node --test tests/logic.test.js tests/index.test.js`
Expected: PASS for all tests

- [ ] **Step 2: Fix any failing tests with the smallest possible change**

```js
// Example adjustments only if needed:
// - update selector ids to match the smoke test
// - normalize summary wording to match the logic test
// - keep button labels stable after confirmation/undo
```

- [ ] **Step 3: Do one last manual browser pass for the acceptance checklist**

Verify:
- The page opens locally without a build step
- The interface reads clearly on desktop and mobile widths
- A blocked draft action writes a log entry instead of silently failing
- An unexpected action error keeps existing summary/log content visible
- The summary remains visible after undo
- The log is readable in reverse chronological order

- [ ] **Step 4: Commit if repository is initialized**

```bash
git add index.html style.css app.js logic.js tests/logic.test.js tests/index.test.js
git commit -m "feat: finish CialloClaw MVP prototype"
```

If the workspace is still not a git repository, mark the step complete without committing.
