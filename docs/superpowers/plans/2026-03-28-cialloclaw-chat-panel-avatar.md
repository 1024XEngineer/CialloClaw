# CialloClaw Chat Panel And Avatar Floating Ball Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an avatar-based floating ball and a persistent right-side chat panel to the existing desktop-shell prototype without breaking the current inspection, draft, log, and undo flow.

**Architecture:** Keep the new floating avatar isolated in the existing Electron floating renderer files and keep the right-side chat panel inside the main panel renderer. Extend `app.js` with a small in-memory chat state and deterministic local response rules, while updating markup and CSS so the left workflow and right chat area coexist cleanly in one panel.

**Tech Stack:** Electron, HTML, CSS, vanilla JavaScript, Node built-in test runner (`node --test`)

---

## File Structure

- `assets/` - New asset directory for the local avatar image file
- `assets/floating-avatar.png` - The provided character image used by the floating renderer
- `desktop-shell.js` - May need a small floating-size constant update if the avatar art needs a larger floating window
- `floating.html` - Floating renderer markup; switch from letter button to avatar image button
- `floating.css` - Avatar-specific floating window styling
- `floating.js` - Existing click-to-toggle behavior, updated only as needed for the avatar markup
- `main.js` - Should continue loading `floating.html`; only adjust if the avatar markup requires a different floating window size
- `index.html` - Add the persistent right-side chat structure next to the current workflow
- `style.css` - Add two-column layout and right-side chat styling while preserving current desktop-shell behavior
- `app.js` - Add chat state, chat rendering, send handling, quick-action handling, and deterministic mock response rules
- `tests/floating.test.js` - Extend floating renderer tests for avatar markup and asset reference
- `tests/index.test.js` - Extend shell smoke tests for the new chat panel anchors
- `tests/app.test.js` - Extend the existing app harness and add chat behavior coverage

## Shared Rules

- Use the provided image as a local asset at `assets/floating-avatar.png`
- Keep the floating avatar click behavior identical to the current floating button behavior
- Keep the left-side workflow intact: inspection, summary, draft generation, execution log, and undo continue to work
- In desktop mode, the in-page floating button remains hidden and the assistant controls stay visible
- Keep `assistant-panel` as the left-column workflow control card; do not remove it
- Keep the existing workflow buttons (`run-inspection`, `generate-draft`, `undo-last-action`, `view-log`) inside `assistant-panel`
- Keep `assistant-message` as a separate status message inside `assistant-panel`; the new chat area does not replace it
- The chat panel stays local-only and deterministic; no model/API integration
- Quick actions append both a user-style prompt and an assistant response

### Task 1: Replace The Floating Letter Button With The Avatar Asset

**Files:**
- Create: `assets/floating-avatar.png`
- Modify: `desktop-shell.js`
- Modify: `floating.html`
- Modify: `floating.css`
- Modify: `tests/desktop-shell.test.js`
- Modify: `tests/floating.test.js`

- [ ] **Step 1: Write the failing floating-avatar tests**

```js
// add to tests/floating.test.js
test('floating markup uses the avatar asset inside the floating button', () => {
  const markup = fs.readFileSync(path.join(__dirname, '..', 'floating.html'), 'utf8');
  const assetPath = path.join(__dirname, '..', 'assets', 'floating-avatar.png');

  assert.match(markup, /id="desktop-floating-ball"/);
  assert.match(markup, /<img[^>]+src="assets\/floating-avatar\.png"/);
  assert.equal(fs.existsSync(assetPath), true);
});
```

- [ ] **Step 2: Run the floating tests to verify they fail**

Run: `node --test tests/floating.test.js`
Expected: FAIL because `assets/floating-avatar.png` does not exist and `floating.html` still renders a letter button

- [ ] **Step 3: Write the minimal avatar implementation**

Create `assets/floating-avatar.png` from the user-provided image. If the image is not already available as a local file in the workspace, stop and ask the user for the local file path before continuing with implementation.

```html
<!-- floating.html -->
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>CialloClaw Floating Ball</title>
    <link rel="stylesheet" href="floating.css">
  </head>
  <body>
    <button id="desktop-floating-ball" type="button" aria-label="Toggle assistant panel">
      <img src="assets/floating-avatar.png" alt="CialloClaw assistant avatar">
    </button>
    <script src="floating.js"></script>
  </body>
</html>
```

```css
/* floating.css additions/replacements */
#desktop-floating-ball {
  width: 92px;
  height: 92px;
  padding: 0;
  border: 0;
  border-radius: 999px;
  background: transparent;
  cursor: pointer;
  box-shadow: 0 18px 36px rgba(77, 31, 18, 0.26);
}

#desktop-floating-ball img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 999px;
}
```

If the larger avatar needs a bigger floating window, update `desktop-shell.js` so the runtime sizing constants match the new avatar dimensions.

If `FLOATING_SIZE` changes, update `tests/desktop-shell.test.js` in the same task so the helper assertions match the new intended avatar window size.

- [ ] **Step 4: Run the floating tests again to verify they pass**

Run: `node --test tests/floating.test.js`
Expected: PASS for the existing toggle tests and the new avatar markup test

- [ ] **Step 5: Commit if the workspace is a git repo**

```bash
git add assets/floating-avatar.png desktop-shell.js floating.html floating.css tests/desktop-shell.test.js tests/floating.test.js
git commit -m "feat: add avatar floating ball"
```

If the workspace is still not a git repo, mark the step complete without committing.

### Task 2: Add The Right-Side Chat Panel Markup And Layout

**Files:**
- Modify: `index.html`
- Modify: `style.css`
- Modify: `tests/index.test.js`

- [ ] **Step 1: Write the failing shell smoke test for the chat panel**

```js
// add to tests/index.test.js
assert.match(markup, /id="chat-panel"/);
assert.match(markup, /id="chat-messages"/);
assert.match(markup, /id="chat-input"/);
assert.match(markup, /id="chat-send"/);
assert.match(markup, /id="chat-action-summarize"/);
assert.match(markup, /id="chat-action-explain"/);
assert.match(markup, /id="chat-action-extract-todos"/);
```

- [ ] **Step 2: Run the shell test to verify it fails**

Run: `node --test tests/index.test.js`
Expected: FAIL because the chat panel anchors do not exist yet

- [ ] **Step 3: Add the minimal two-column layout and chat markup**

```html
<!-- index.html structure update -->
<main class="app-shell">
  <section class="hero panel-card">...</section>

  <section class="workspace-shell" aria-label="Prototype workspace">
    <div class="workspace-main">
      <aside id="assistant-panel" class="assistant-panel panel-card" aria-live="polite">
        <p id="assistant-message">Ready to inspect tasks and prepare a visible draft.</p>
        <div class="assistant-actions">
          <button id="run-inspection" type="button">Run Inspection</button>
          <button id="generate-draft" type="button">Generate Draft</button>
          <button id="undo-last-action" type="button" disabled>Undo Last Action</button>
          <button id="view-log" type="button">View Log</button>
        </div>
      </aside>

      <section class="workspace-grid" aria-label="Operational workflow">
        <!-- existing task / summary / log cards remain here -->
      </section>
    </div>

    <aside id="chat-panel" class="chat-panel panel-card" aria-label="Assistant chat">
      <div class="chat-panel__header">
        <p class="section-kicker">Assistant</p>
        <h2>Conversation</h2>
      </div>

      <div id="chat-messages" class="chat-messages" aria-live="polite"></div>

      <div class="chat-quick-actions">
        <button id="chat-action-summarize" type="button">Summarize</button>
        <button id="chat-action-explain" type="button">Explain</button>
        <button id="chat-action-extract-todos" type="button">Extract Todos</button>
      </div>

      <div class="chat-compose">
        <input id="chat-input" type="text" placeholder="Ask for a summary or next steps">
        <button id="chat-send" type="button">Send</button>
      </div>
    </aside>
  </section>
</main>
```

```css
/* style.css additions */
.workspace-shell {
  display: grid;
  grid-template-columns: minmax(0, 1.55fr) minmax(300px, 0.95fr);
  gap: 18px;
  align-items: start;
}

.workspace-main {
  display: grid;
  gap: 18px;
  min-width: 0;
}

.workspace-main .assistant-panel {
  position: static;
  width: auto;
}

body.desktop-shell .assistant-panel {
  position: static;
  right: auto;
  bottom: auto;
  width: auto;
  margin-top: 0;
}

.chat-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-height: 100%;
}

.chat-messages {
  min-height: 260px;
  max-height: 420px;
  overflow: auto;
}

.chat-quick-actions,
.chat-compose {
  display: grid;
  gap: 10px;
}

.chat-quick-actions {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.chat-compose {
  grid-template-columns: minmax(0, 1fr) auto;
}

@media (max-width: 980px) {
  .workspace-shell {
    grid-template-columns: 1fr;
  }
}
```

- [ ] **Step 4: Run the shell test again to verify it passes**

Run: `node --test tests/index.test.js`
Expected: PASS with the new chat anchors present

- [ ] **Step 5: Commit if the workspace is a git repo**

```bash
git add index.html style.css tests/index.test.js
git commit -m "feat: add chat panel layout"
```

If the workspace is still not a git repo, mark the step complete without committing.

### Task 3: Add Local Chat State, Message Rendering, And Quick Actions

**Files:**
- Modify: `app.js`
- Modify: `tests/app.test.js`

- [ ] **Step 1: Extend the app test harness and write the failing chat tests**

```js
// update ids in loadApp()
const ids = [
  'floating-ball',
  'assistant-panel',
  'assistant-message',
  'task-list',
  'inspection-summary',
  'draft-output',
  'execution-log',
  'run-inspection',
  'generate-draft',
  'undo-last-action',
  'view-log',
  'chat-panel',
  'chat-messages',
  'chat-input',
  'chat-send',
  'chat-action-summarize',
  'chat-action-explain',
  'chat-action-extract-todos'
];

// add value support in createElement()
value: '',
```

```js
test('chat panel starts with a welcome assistant message', () => {
  const { elements } = loadApp();
  assert.match(elements['chat-messages'].innerHTML, /I can help summarize/i);
});

test('sending a typed message appends a user message and an assistant reply', () => {
  const { elements } = loadApp();

  elements['chat-input'].value = 'Please summarize this';
  elements['chat-send'].click();

  assert.match(elements['chat-messages'].innerHTML, /Please summarize this/);
  assert.match(elements['chat-messages'].innerHTML, /summary|follow-up/i);
  assert.equal(elements['chat-input'].value, '');
});

test('quick actions append a user-style prompt and an assistant reply', () => {
  const { elements } = loadApp();

  elements['chat-action-extract-todos'].click();

  assert.match(elements['chat-messages'].innerHTML, /Extract Todos/);
  assert.match(elements['chat-messages'].innerHTML, /Finish weekly status update|Review candidate notes/);
});
```

- [ ] **Step 2: Run the app tests to verify they fail**

Run: `node --test tests/app.test.js`
Expected: FAIL because the chat ids, welcome message, and chat behavior do not exist yet

- [ ] **Step 3: Add the minimal chat state and render behavior**

```js
// app.js additions
const WELCOME_MESSAGE = 'I can help summarize the current task state, explain the workflow, or extract next steps.';

const state = {
  // existing state...
  chatMessages: [
    { role: 'assistant', text: WELCOME_MESSAGE }
  ]
};

function createAssistantReply(prompt) {
  const normalized = prompt.trim().toLowerCase();

  if (normalized.includes('summarize')) {
    return state.inspectionSummary || 'Current status is quiet. Run inspection and I will summarize the active tasks.';
  }

  if (normalized.includes('todo') || normalized.includes('task')) {
    return state.tasks
      .filter((task) => task.status === 'pending')
      .map((task) => `- ${task.title}`)
      .join('\n');
  }

  if (normalized.includes('explain')) {
    return 'The panel inspects tasks, asks for confirmation before drafting, records execution logs, and supports undo.';
  }

  return 'I can summarize the current state, explain the workflow, or extract next steps from the task list.';
}

function appendChatExchange(userText) {
  const trimmed = userText.trim();
  if (!trimmed) {
    return;
  }

  state.chatMessages.push({ role: 'user', text: trimmed });
  state.chatMessages.push({ role: 'assistant', text: createAssistantReply(trimmed) });
}

function renderChatMessages() {
  refs.chatMessages.innerHTML = state.chatMessages.map((message) => {
    return `<article class="chat-message chat-message--${escapeHtml(message.role)}"><p>${nl2br(message.text)}</p></article>`;
  }).join('');
}

function sendChatInput() {
  appendChatExchange(refs.chatInput.value);
  refs.chatInput.value = '';
  renderChatMessages();
}
```

Also:
- add DOM refs for the new chat elements
- call `renderChatMessages()` from `render()`
- wire button handlers for `chat-send`, `chat-action-summarize`, `chat-action-explain`, and `chat-action-extract-todos`
- make each quick action call `appendChatExchange()` with a fixed prompt string such as `Summarize the current state`
- preserve the existing `assistant-message` updates for the workflow buttons; chat replies render only in `chat-messages`

- [ ] **Step 4: Run the app tests again to verify they pass**

Run: `node --test tests/app.test.js`
Expected: PASS for the existing workflow tests and the new chat tests

- [ ] **Step 5: Run the full test suite**

Run: `node --test`
Expected: PASS for all project tests

- [ ] **Step 6: Commit if the workspace is a git repo**

```bash
git add app.js tests/app.test.js
git commit -m "feat: add assistant chat panel"
```

If the workspace is still not a git repo, mark the step complete without committing.

### Task 4: Verify The Updated Electron Shell End To End

**Files:**
- Modify if needed: `floating.html`
- Modify if needed: `floating.css`
- Modify if needed: `index.html`
- Modify if needed: `style.css`
- Modify if needed: `app.js`
- Modify tests only if needed to cover a real regression fix

- [ ] **Step 1: Run the full automated suite**

Run: `node --test`
Expected: PASS for all tests

- [ ] **Step 2: Launch the Electron app and verify the updated shell**

Run: `npm start`

Verify:
- the floating window shows the avatar image instead of the letter button
- clicking the avatar still opens and hides the main panel
- the main panel shows the new right-side chat area
- the chat area starts with the welcome assistant message
- sending a typed message appends both user and assistant messages
- clicking a quick action appends a user-style prompt and assistant response
- the left-side workflow still runs inspection -> draft confirmation -> draft -> log -> undo

- [ ] **Step 3: If a regression appears, add or adjust a failing test first, then make the smallest fix**

Run after any fix: `node --test`
Expected: PASS again after the regression fix

- [ ] **Step 4: Commit if the workspace is a git repo**

```bash
git add assets/floating-avatar.png floating.html floating.css floating.js index.html style.css app.js tests/floating.test.js tests/index.test.js tests/app.test.js
git commit -m "feat: add avatar shell and chat panel"
```

If the workspace is still not a git repo, mark the step complete without committing.
