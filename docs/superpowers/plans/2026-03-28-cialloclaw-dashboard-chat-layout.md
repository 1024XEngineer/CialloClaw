# CialloClaw Dashboard And Stitched Chat Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rework the main panel into a stitched `left dashboard + right chat` workspace inspired by `仪表盘.png` and `聊天界面.png`, while preserving the current prototype behavior.

**Architecture:** Keep the existing Electron shell and behavior model intact. Restructure `index.html` into explicit dashboard and chat zones, do the visual heavy lifting in `style.css`, and make only small `app.js` rendering updates so the right side feels like a true chat interface instead of a generic card list. Use the two PNG files as visual references only, not as dead background UI.

**Tech Stack:** Electron, HTML, CSS, vanilla JavaScript, Node built-in test runner (`node --test`)

---

## File Structure

- `仪表盘.png` - Visual reference for the left dashboard zone; do not embed as a dead background
- `聊天界面.png` - Visual reference for the right chat zone; do not embed as a dead background
- `index.html` - Main panel structure; define explicit dashboard shell and stitched right chat shell
- `style.css` - Dashboard visual language, stitched dual-pane layout, and chat-interface styling
- `app.js` - Minor rendering updates for the chat shell and any layout-sensitive text/content tweaks
- `tests/index.test.js` - Structural smoke tests for the new stitched layout contract
- `tests/app.test.js` - Renderer behavior tests for updated chat markup without changing the workflow semantics

## Shared Rules

- Keep one main Electron panel window; do not open a separate chat window
- Left and right areas must feel stitched side-by-side, not like a cramped main pane plus a narrow sidebar
- Preserve existing DOM ids where practical so the workflow logic remains stable
- Keep `assistant-panel` as the left-side workflow control card
- Keep `assistant-message` as the workflow status area, not the chat area
- Keep the left-side workflow intact:
  - inspection
  - summary
  - draft generation
  - execution log
  - undo
- Keep the right-side chat behavior intact:
  - welcome message
  - typed send
  - quick actions
- Use `仪表盘.png` and `聊天界面.png` as style references only; do not simply place them as full-screen images

### Task 1: Rebuild The Main Panel Structure As Dashboard + Stitched Chat Shell

**Files:**
- Modify: `index.html`
- Modify: `tests/index.test.js`

- [ ] **Step 1: Write the failing structure tests**

```js
// add to tests/index.test.js
assert.match(markup, /class="dashboard-shell"/);
assert.match(markup, /class="dashboard-main"/);
assert.match(markup, /class="chat-shell"/);
assert.match(markup, /id="assistant-panel"/);
assert.match(markup, /id="chat-panel"/);
assert.match(
  markup,
  /<section class="dashboard-shell"[^>]*>[\s\S]*<section class="dashboard-main"[\s\S]*<aside id="chat-panel" class="chat-shell panel-card"/
);
```

- [ ] **Step 2: Run the structure test to verify it fails**

Run: `node --test tests/index.test.js`
Expected: FAIL because the current layout still uses the older `workspace-shell` / `workspace-main` contract

- [ ] **Step 3: Write the minimal structural rewrite**

```html
<!-- index.html structure sketch -->
<main class="app-shell">
  <section class="dashboard-shell" aria-label="Prototype workspace">
    <section class="dashboard-main">
      <header class="dashboard-hero panel-card">
        <!-- keep product title / top summary framing here -->
      </header>

      <aside id="assistant-panel" class="assistant-panel panel-card is-hidden" aria-live="polite">
        <p id="assistant-message">Ready to inspect tasks and prepare a visible draft.</p>
        <div class="assistant-actions">
          <button id="run-inspection" type="button">Run Inspection</button>
          <button id="generate-draft" type="button">Generate Draft</button>
          <button id="undo-last-action" type="button" disabled>Undo Last Action</button>
          <button id="view-log" type="button">View Log</button>
        </div>
      </aside>

      <section class="dashboard-grid" aria-label="Operational workflow">
        <!-- existing task / summary / execution-log regions stay on the left -->
      </section>
    </section>

    <aside id="chat-panel" class="chat-shell panel-card" aria-label="Assistant chat">
      <div class="chat-shell__header">
        <p class="section-kicker">Assistant</p>
        <h2>Conversation</h2>
      </div>
      <div id="chat-messages" class="chat-messages" aria-live="polite"></div>
      <div class="chat-quick-actions">...</div>
      <div class="chat-compose">...</div>
    </aside>
  </section>
</main>
```

Notes:
- keep `task-list`, `inspection-summary`, `draft-output`, and `execution-log` ids intact
- keep `chat-messages`, `chat-input`, `chat-send`, and quick-action ids intact
- keep `assistant-panel` in the left zone only

- [ ] **Step 4: Run the structure test again to verify it passes**

Run: `node --test tests/index.test.js`
Expected: PASS with the new stitched shell structure

- [ ] **Step 5: Commit if the workspace is a git repo**

```bash
git add index.html tests/index.test.js
git commit -m "feat: restructure main panel shell"
```

If the workspace is still not a git repo, mark the step complete without committing.

### Task 2: Restyle The Main Panel To Match Dashboard + Dedicated Chat References

**Files:**
- Modify: `style.css`
- Modify: `tests/index.test.js`

- [ ] **Step 1: Add a failing style smoke test for the stitched layout contract**

```js
// add to tests/index.test.js
const styles = fs.readFileSync(path.join(__dirname, '..', 'style.css'), 'utf8');

assert.match(styles, /\.dashboard-shell\s*\{[\s\S]*grid-template-columns:\s*minmax\(0,\s*1\.[0-9]+fr\)\s+minmax\(340px,\s*0\.[0-9]+fr\)/);
assert.match(styles, /\.chat-shell\s*\{[\s\S]*display:\s*grid/);
assert.match(styles, /@media\s*\(max-width:\s*(\d+)px\)\s*\{[\s\S]*\.dashboard-shell\s*\{/);
```

- [ ] **Step 2: Run the structure/style test to verify it fails**

Run: `node --test tests/index.test.js`
Expected: FAIL because the new dashboard-shell and chat-shell styles do not exist yet

- [ ] **Step 3: Write the minimal dashboard/chat visual rewrite**

```css
/* style.css direction sketch */
.dashboard-shell {
  display: grid;
  grid-template-columns: minmax(0, 1.45fr) minmax(360px, 0.95fr);
  gap: 0;
  min-height: calc(100vh - 48px);
  border-radius: 28px;
  overflow: hidden;
}

.dashboard-main {
  display: grid;
  gap: 18px;
  padding: 24px;
  background: linear-gradient(180deg, #f7f1e7 0%, #efe6d8 100%);
}

.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: 18px;
}

.chat-shell {
  display: grid;
  grid-template-rows: auto 1fr auto auto;
  gap: 14px;
  padding: 24px;
  border-left: 1px solid rgba(66, 47, 35, 0.08);
  background: linear-gradient(180deg, #fdfcf9 0%, #f2ece4 100%);
}

.chat-messages {
  display: grid;
  align-content: start;
  gap: 12px;
  overflow: auto;
}

@media (max-width: 900px) {
  .dashboard-shell {
    grid-template-columns: 1fr;
  }
}
```

Implementation goals:
- left side should visually feel like an operations dashboard inspired by `仪表盘.png`
- right side should feel like a dedicated messaging pane inspired by `聊天界面.png`
- the seam between panes should feel intentional, not cramped
- do not rely on the current narrow-aside look

- [ ] **Step 4: Run the structure/style test again to verify it passes**

Run: `node --test tests/index.test.js`
Expected: PASS with the new layout contract and breakpoint

- [ ] **Step 5: Commit if the workspace is a git repo**

```bash
git add style.css tests/index.test.js
git commit -m "feat: restyle stitched dashboard layout"
```

If the workspace is still not a git repo, mark the step complete without committing.

### Task 3: Update Chat Rendering So The Right Side Feels Like A Real Chat Interface

**Files:**
- Modify: `app.js`
- Modify: `tests/app.test.js`
- Modify: `style.css`

- [ ] **Step 1: Write the failing chat-rendering tests**

```js
// add to tests/app.test.js
test('chat messages render as user and assistant conversation bubbles', () => {
  const { elements } = loadApp();

  assert.match(elements['chat-messages'].innerHTML, /chat-message--assistant/);

  elements['chat-input'].value = 'Please summarize the current work';
  elements['chat-send'].click();

  assert.match(elements['chat-messages'].innerHTML, /chat-message--user/);
  assert.match(elements['chat-messages'].innerHTML, /chat-message__bubble/);
});

test('chat shell keeps workflow status separate from chat rendering', () => {
  const { elements } = loadApp();

  elements['chat-action-explain'].click();

  assert.match(elements['chat-messages'].innerHTML, /chat-message--assistant/);
  assert.equal(elements['assistant-message'].textContent, 'Ready to inspect tasks and prepare a visible draft.');
});
```

- [ ] **Step 2: Run the app tests to verify they fail**

Run: `node --test tests/app.test.js`
Expected: FAIL because the current chat markup does not yet render the richer conversation shell classes

- [ ] **Step 3: Write the minimal chat-rendering update**

```js
// app.js renderChat() sketch
function renderChat() {
  refs.chatMessages.innerHTML = state.chatMessages.map(function (message) {
    return [
      '<article class="chat-message chat-message--' + escapeHtml(message.role) + '">',
      '<div class="chat-message__meta">' + escapeHtml(message.role) + '</div>',
      '<div class="chat-message__bubble">' + nl2br(message.text) + '</div>',
      '</article>'
    ].join('');
  }).join('');
}
```

```css
/* style.css chat refinements */
.chat-message {
  display: grid;
  gap: 6px;
}

.chat-message--user {
  justify-items: end;
}

.chat-message__bubble {
  max-width: 88%;
  padding: 12px 14px;
  border-radius: 18px;
}

.chat-message--assistant .chat-message__bubble {
  background: #ffffff;
}

.chat-message--user .chat-message__bubble {
  background: #eadcc9;
}
```

Keep the behavior deterministic and local. Do not change the semantics of send/quick actions, only their presentation and shell feel.

- [ ] **Step 4: Run the app tests again to verify they pass**

Run: `node --test tests/app.test.js`
Expected: PASS for existing workflow tests and the new chat-rendering tests

- [ ] **Step 5: Run the full suite**

Run: `node --test`
Expected: PASS for all tests

- [ ] **Step 6: Commit if the workspace is a git repo**

```bash
git add app.js style.css tests/app.test.js
git commit -m "feat: refine stitched chat interface"
```

If the workspace is still not a git repo, mark the step complete without committing.

### Task 4: Verify The Reworked Main Panel In Electron

**Files:**
- Modify if needed: `index.html`
- Modify if needed: `style.css`
- Modify if needed: `app.js`
- Modify tests only if needed for a real regression fix

- [ ] **Step 1: Run the full automated suite**

Run: `node --test`
Expected: PASS for all tests

- [ ] **Step 2: Launch the Electron app and verify the reworked main panel**

Run: `npm start`

Verify:
- the main panel visually reads as a dashboard-inspired left area plus a dedicated right chat pane
- the right pane sits stitched to the left pane, not squeezed into the old layout
- the right chat area still shows the welcome message, typed messages, and quick actions
- the left workflow still runs inspection -> summary -> draft generation -> log -> undo
- the floating avatar still opens and hides the main panel

- [ ] **Step 3: If a regression appears, add or adjust a failing test first, then make the smallest fix**

Run after any fix: `node --test`
Expected: PASS again after the regression fix

- [ ] **Step 4: Commit if the workspace is a git repo**

```bash
git add index.html style.css app.js tests/index.test.js tests/app.test.js
git commit -m "feat: redesign dashboard and chat workspace"
```

If the workspace is still not a git repo, mark the step complete without committing.
