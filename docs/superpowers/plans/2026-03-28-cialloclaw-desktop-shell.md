# CialloClaw Desktop Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wrap the existing local web prototype in the smallest runnable Electron desktop shell with a real floating ball window and a separate main panel window.

**Architecture:** Keep Electron responsibilities narrow. Use `main.js` plus small helper modules for window geometry and main-panel toggling, keep preload limited to one desktop bridge, and reuse the current `index.html` / `app.js` prototype inside the main panel. In desktop mode, hide the in-page floating ball and keep the assistant controls visible so the system-level floating ball becomes the only entry point.

**Tech Stack:** Electron, HTML, CSS, vanilla JavaScript, Node built-in test runner (`node --test`)

---

## File Structure

- `tests/` - Existing test directory; add new desktop-shell tests here
- `package.json` - Electron dependency and local run scripts
- `desktop-shell.js` - Pure helpers for floating-ball bounds, panel bounds, and desktop panel URL creation
- `main-controller.js` - Small controller that recreates, shows, hides, and preserves the main panel window
- `main.js` - Electron main process bootstrap, window creation, and IPC registration
- `preload.js` - Minimal `window.CialloDesktop.toggleMainPanel()` bridge
- `floating.html` - Floating ball renderer markup
- `floating.css` - Floating ball window styling
- `floating.js` - Floating ball click behavior
- `index.html` - Existing main panel markup, reused with desktop query mode
- `style.css` - Desktop panel styling adjustments
- `app.js` - Desktop mode detection that hides the in-page floating ball and pins the assistant controls open
- `tests/package.test.js` - Smoke test for Electron package metadata
- `tests/desktop-shell.test.js` - Pure helper tests for window geometry and panel URL rules
- `tests/main-controller.test.js` - Tests for panel recreation, hide/show toggling, and close behavior
- `tests/preload.test.js` - Tests for the preload bridge contract
- `tests/floating.test.js` - Tests for floating-ball renderer click behavior
- `tests/app.test.js` - Existing app tests plus desktop-mode coverage

## Shared Desktop Rules

- Use Electron only for window management; do not add tray, native file access, or extra desktop features
- Load the main panel with `index.html?shell=desktop`
- In desktop mode, the in-page `#floating-ball` should be hidden and the `#assistant-panel` should remain visible inside the panel window
- Keep `contextIsolation: true` and `nodeIntegration: false`
- Recreate the main panel window if the user closes it, then show it again on the next floating-ball click
- Initial main panel size target: `960 x 720`

### Task 1: Add Electron Package Metadata And Pure Desktop Helpers

**Files:**
- Create: `package.json`
- Create: `desktop-shell.js`
- Create: `tests/package.test.js`
- Create: `tests/desktop-shell.test.js`

- [ ] **Step 1: Write the failing smoke tests**

```js
// tests/package.test.js
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

test('package.json defines the Electron entry and start script', () => {
  const pkg = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8'));
  assert.equal(pkg.main, 'main.js');
  assert.equal(pkg.scripts.start, 'electron .');
  assert.ok((pkg.devDependencies && pkg.devDependencies.electron) || (pkg.dependencies && pkg.dependencies.electron));
});
```

```js
// tests/desktop-shell.test.js
const test = require('node:test');
const assert = require('node:assert/strict');
const {
  getFloatingBallBounds,
  getMainPanelBounds,
  buildDesktopPanelUrl
} = require('../desktop-shell.js');

test('getFloatingBallBounds places the floating ball near the bottom-right corner', () => {
  const bounds = getFloatingBallBounds({ x: 0, y: 0, width: 1440, height: 900 });
  assert.equal(bounds.width, 72);
  assert.equal(bounds.height, 72);
  assert.ok(bounds.x > 1200);
  assert.ok(bounds.y > 700);
});

test('getMainPanelBounds places the panel beside the floating ball and clamps on screen', () => {
  const panel = getMainPanelBounds(
    { x: 1320, y: 760, width: 72, height: 72 },
    { x: 0, y: 0, width: 1440, height: 900 },
    { width: 960, height: 720 }
  );

  assert.equal(panel.width, 960);
  assert.equal(panel.height, 720);
  assert.ok(panel.x >= 0);
  assert.ok(panel.y >= 0);
  assert.ok(panel.x + panel.width <= 1440);
  assert.ok(panel.y + panel.height <= 900);
});

test('buildDesktopPanelUrl appends desktop shell mode to index.html', () => {
  const url = buildDesktopPanelUrl('D:/Desktop/claw/mvp/index.html');
  assert.match(url, /index\.html\?shell=desktop$/);
});
```

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `node --test tests/package.test.js tests/desktop-shell.test.js`
Expected: FAIL with `ENOENT` for `package.json` and `Cannot find module '../desktop-shell.js'`

- [ ] **Step 3: Write the minimal package and helper implementation**

```json
{
  "name": "cialloclaw-desktop-shell",
  "version": "0.1.0",
  "private": true,
  "main": "main.js",
  "scripts": {
    "start": "electron .",
    "test": "node --test"
  },
  "devDependencies": {
    "electron": "^35.0.0"
  }
}
```

```js
// desktop-shell.js
const { pathToFileURL } = require('node:url');

const FLOATING_SIZE = 72;
const FLOATING_MARGIN = 24;
const PANEL_GAP = 16;
const DEFAULT_PANEL_SIZE = { width: 960, height: 720 };

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

function getFloatingBallBounds(workArea) {
  return {
    x: workArea.x + workArea.width - FLOATING_SIZE - FLOATING_MARGIN,
    y: workArea.y + workArea.height - FLOATING_SIZE - FLOATING_MARGIN,
    width: FLOATING_SIZE,
    height: FLOATING_SIZE
  };
}

function getMainPanelBounds(floatingBounds, workArea, panelSize = DEFAULT_PANEL_SIZE) {
  const desiredX = floatingBounds.x - panelSize.width - PANEL_GAP;
  const desiredY = floatingBounds.y + floatingBounds.height - panelSize.height;

  return {
    width: panelSize.width,
    height: panelSize.height,
    x: clamp(desiredX, workArea.x, workArea.x + workArea.width - panelSize.width),
    y: clamp(desiredY, workArea.y, workArea.y + workArea.height - panelSize.height)
  };
}

function buildDesktopPanelUrl(indexPath) {
  const url = pathToFileURL(indexPath);
  url.searchParams.set('shell', 'desktop');
  return url.toString();
}

module.exports = {
  FLOATING_SIZE,
  DEFAULT_PANEL_SIZE,
  getFloatingBallBounds,
  getMainPanelBounds,
  buildDesktopPanelUrl
};
```

- [ ] **Step 4: Run the tests again to verify they pass**

Run: `node --test tests/package.test.js tests/desktop-shell.test.js`
Expected: PASS for all package and helper tests

- [ ] **Step 5: Install Electron from the new package file**

Run: `npm install`
Expected: `electron` installed locally and `package-lock.json` created

- [ ] **Step 6: Commit if the workspace is a git repo**

```bash
git add package.json package-lock.json desktop-shell.js tests/package.test.js tests/desktop-shell.test.js
git commit -m "feat: add desktop shell scaffolding"
```

If the workspace is still not a git repo, mark the step complete without committing.

### Task 2: Wire The Electron Main Process And Secure Preload Bridge

**Files:**
- Create: `main-controller.js`
- Create: `main.js`
- Create: `preload.js`
- Create: `tests/main-controller.test.js`
- Create: `tests/preload.test.js`

- [ ] **Step 1: Write the failing controller and preload tests**

```js
// tests/main-controller.test.js
const test = require('node:test');
const assert = require('node:assert/strict');
const { createMainPanelController } = require('../main-controller.js');

function createWindowStub() {
  return {
    visible: false,
    destroyed: false,
    showCalls: 0,
    hideCalls: 0,
    focusCalls: 0,
    show() { this.visible = true; this.showCalls += 1; },
    hide() { this.visible = false; this.hideCalls += 1; },
    focus() { this.focusCalls += 1; },
    isVisible() { return this.visible; },
    isDestroyed() { return this.destroyed; }
  };
}

test('controller recreates and shows the main panel when toggled on', () => {
  let created = 0;
  const panel = createWindowStub();
  const controller = createMainPanelController({
    initialPanelWindow: null,
    createPanelWindow() {
      created += 1;
      return panel;
    },
    positionPanelWindow() {}
  });

  const visible = controller.togglePanel();
  assert.equal(created, 1);
  assert.equal(visible, true);
  assert.equal(panel.showCalls, 1);
  assert.equal(panel.focusCalls, 1);
});

test('controller can keep an initially hidden panel window created during startup', () => {
  const panel = createWindowStub();
  const controller = createMainPanelController({
    initialPanelWindow: panel,
    createPanelWindow() {
      throw new Error('should not recreate panel during startup test');
    },
    positionPanelWindow() {}
  });

  assert.equal(controller.getPanelWindow(), panel);
  assert.equal(panel.isVisible(), false);
});

test('controller hides the main panel when toggled off', () => {
  const panel = createWindowStub();
  const controller = createMainPanelController({
    initialPanelWindow: null,
    createPanelWindow() {
      return panel;
    },
    positionPanelWindow() {}
  });

  controller.togglePanel();
  const visible = controller.togglePanel();
  assert.equal(visible, false);
  assert.equal(panel.hideCalls, 1);
});

test('controller intercepts close and hides the panel instead', () => {
  const panel = createWindowStub();
  const controller = createMainPanelController({
    initialPanelWindow: null,
    createPanelWindow() {
      return panel;
    },
    positionPanelWindow() {}
  });
  controller.togglePanel();

  let prevented = false;
  controller.handlePanelClose({ preventDefault() { prevented = true; } });
  assert.equal(prevented, true);
  assert.equal(panel.hideCalls, 1);
});
```

```js
// tests/preload.test.js
const test = require('node:test');
const assert = require('node:assert/strict');
const { registerDesktopBridge } = require('../preload.js');

test('registerDesktopBridge exposes toggleMainPanel on window.CialloDesktop', async () => {
  let channel;
  const target = {};
  registerDesktopBridge(
    { exposeInMainWorld(name, api) { target[name] = api; } },
    { invoke(requestedChannel) { channel = requestedChannel; return Promise.resolve(true); } }
  );

  assert.equal(typeof target.CialloDesktop.toggleMainPanel, 'function');
  await target.CialloDesktop.toggleMainPanel();
  assert.equal(channel, 'desktop:toggle-main-panel');
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `node --test tests/main-controller.test.js tests/preload.test.js`
Expected: FAIL with missing module errors for `main-controller.js` and `preload.js`

- [ ] **Step 3: Write the minimal controller, preload bridge, and Electron bootstrap**

```js
// main-controller.js
function createMainPanelController(deps) {
  let panelWindow = deps.initialPanelWindow || null;

  function ensurePanelWindow() {
    if (!panelWindow || panelWindow.isDestroyed()) {
      panelWindow = deps.createPanelWindow();
    }

    return panelWindow;
  }

  function togglePanel() {
    const panel = ensurePanelWindow();

    if (panel.isVisible()) {
      panel.hide();
      return false;
    }

    deps.positionPanelWindow(panel);
    panel.show();
    panel.focus();
    return true;
  }

  function handlePanelClose(event) {
    event.preventDefault();
    if (panelWindow && !panelWindow.isDestroyed()) {
      panelWindow.hide();
    }
  }

  return {
    togglePanel,
    handlePanelClose,
    getPanelWindow() {
      return panelWindow;
    }
  };
}

module.exports = { createMainPanelController };
```

```js
// preload.js
function registerDesktopBridge(contextBridge, ipcRenderer) {
  contextBridge.exposeInMainWorld('CialloDesktop', {
    toggleMainPanel() {
      return ipcRenderer.invoke('desktop:toggle-main-panel');
    }
  });
}

if (typeof require !== 'undefined' && typeof module !== 'undefined' && require.main !== module) {
  try {
    const { contextBridge, ipcRenderer } = require('electron');
    registerDesktopBridge(contextBridge, ipcRenderer);
  } catch (_) {
    // Test environment or non-Electron runtime.
  }
}

module.exports = { registerDesktopBridge };
```

```js
// main.js
const path = require('node:path');
const { app, BrowserWindow, ipcMain, screen } = require('electron');
const { getFloatingBallBounds, getMainPanelBounds, buildDesktopPanelUrl, DEFAULT_PANEL_SIZE } = require('./desktop-shell.js');
const { createMainPanelController } = require('./main-controller.js');

let floatingWindow;
let controller;

function getWorkArea() {
  return screen.getPrimaryDisplay().workArea;
}

function createFloatingWindow() {
  const bounds = getFloatingBallBounds(getWorkArea());
  const window = new BrowserWindow({
    ...bounds,
    frame: false,
    transparent: true,
    resizable: false,
    skipTaskbar: true,
    alwaysOnTop: true,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false
    }
  });
  window.loadFile(path.join(__dirname, 'floating.html'));
  return window;
}

function createPanelWindow() {
  const panel = new BrowserWindow({
    width: DEFAULT_PANEL_SIZE.width,
    height: DEFAULT_PANEL_SIZE.height,
    show: false,
    frame: false,
    resizable: false,
    webPreferences: {
      contextIsolation: true,
      nodeIntegration: false
    }
  });

  panel.loadURL(buildDesktopPanelUrl(path.join(__dirname, 'index.html')));
  panel.on('close', (event) => controller.handlePanelClose(event));
  return panel;
}

function positionPanelWindow(panel) {
  const bounds = getMainPanelBounds(floatingWindow.getBounds(), getWorkArea(), DEFAULT_PANEL_SIZE);
  panel.setBounds(bounds);
}

app.whenReady().then(() => {
  floatingWindow = createFloatingWindow();
  const initialPanelWindow = createPanelWindow();
  controller = createMainPanelController({
    initialPanelWindow,
    createPanelWindow,
    positionPanelWindow
  });

  ipcMain.handle('desktop:toggle-main-panel', () => controller.togglePanel());
});

app.on('window-all-closed', (event) => {
  event.preventDefault();
});
```

- [ ] **Step 4: Run the tests again to verify they pass**

Run: `node --test tests/main-controller.test.js tests/preload.test.js`
Expected: PASS for all controller and preload tests

- [ ] **Step 5: Smoke-check the Electron entry file loads without package metadata regressions**

Run: `node --test tests/package.test.js tests/desktop-shell.test.js tests/main-controller.test.js tests/preload.test.js`
Expected: PASS for all shell bootstrap tests

- [ ] **Step 6: Commit if the workspace is a git repo**

```bash
git add main-controller.js main.js preload.js tests/main-controller.test.js tests/preload.test.js
git commit -m "feat: wire desktop shell process"
```

If the workspace is still not a git repo, mark the step complete without committing.

### Task 3: Build The Floating Ball Renderer And Adapt The Main Panel For Desktop Mode

**Files:**
- Create: `floating.html`
- Create: `floating.css`
- Create: `floating.js`
- Create: `tests/floating.test.js`
- Modify: `app.js`
- Modify: `style.css`
- Modify: `tests/app.test.js`

- [ ] **Step 1: Write the failing floating-renderer and desktop-mode tests**

```js
// tests/floating.test.js
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

test('floating.js calls the desktop toggle bridge when the ball is clicked', async () => {
  const source = fs.readFileSync(path.join(__dirname, '..', 'floating.js'), 'utf8');
  let toggleCalls = 0;
  const button = {
    listeners: {},
    addEventListener(type, handler) {
      this.listeners[type] = handler;
    },
    click() {
      return this.listeners.click();
    }
  };

  const context = {
    window: {
      CialloDesktop: {
        toggleMainPanel() {
          toggleCalls += 1;
          return Promise.resolve(true);
        }
      }
    },
    document: {
      getElementById(id) {
        return id === 'desktop-floating-ball' ? button : null;
      },
      readyState: 'complete',
      addEventListener() {}
    },
    console
  };

  context.globalThis = context.window;
  context.window.window = context.window;
  context.window.document = context.document;

  vm.runInNewContext(source, context);
  await button.click();
  assert.equal(toggleCalls, 1);
});
```

```js
// add to tests/app.test.js
function loadApp(options = {}) {
  // existing setup...
  const bodyClassState = new Set();
  const document = {
    body: {
      classList: {
        add(...names) {
          names.forEach((name) => bodyClassState.add(name));
        },
        contains(name) {
          return bodyClassState.has(name);
        }
      }
    },
    // existing getElementById / readyState / addEventListener...
  };

  const context = {
    window: {
      CialloLogic: logic,
      location: {
        search: options.search || ''
      }
    },
    document,
    console
  };

  // existing vm setup...
}

test('desktop shell mode hides the internal floating ball and keeps the assistant panel visible', () => {
  const { elements } = loadApp({ search: '?shell=desktop' });

  assert.match(elements['floating-ball'].className, /is-hidden/);
  assert.equal(elements['assistant-panel'].classList.contains('is-hidden'), false);
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `node --test tests/floating.test.js tests/app.test.js`
Expected: FAIL because `floating.js` does not exist and `app.js` does not yet support desktop shell mode

- [ ] **Step 3: Write the floating renderer and desktop-mode UI changes**

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
    <button id="desktop-floating-ball" type="button" aria-label="Toggle CialloClaw panel">
      CC
    </button>
    <script src="floating.js"></script>
  </body>
</html>
```

```js
// floating.js
(function () {
  function init() {
    const button = document.getElementById('desktop-floating-ball');
    if (!button) {
      return;
    }

    button.addEventListener('click', function () {
      if (window.CialloDesktop && typeof window.CialloDesktop.toggleMainPanel === 'function') {
        return window.CialloDesktop.toggleMainPanel();
      }
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
```

```css
/* floating.css */
html, body {
  margin: 0;
  width: 100%;
  height: 100%;
  background: transparent;
}

body {
  display: grid;
  place-items: center;
}

#desktop-floating-ball {
  width: 72px;
  height: 72px;
  border: 0;
  border-radius: 999px;
  background: radial-gradient(circle at 30% 30%, #ffd6bc, #d46a3d 60%, #9f3c1f 100%);
  color: #1f1a17;
  font: 700 18px/1 "Segoe UI", sans-serif;
  cursor: pointer;
}
```

```js
// app.js additions/adjustments
const isDesktopShell = typeof window.location !== 'undefined' && /[?&]shell=desktop\b/.test(window.location.search || '');

const state = {
  isPanelOpen: isDesktopShell,
  // existing state...
};

function renderPanel() {
  if (isDesktopShell) {
    refs.assistantPanel.classList.remove('is-hidden');
    refs.floatingBall.className = 'floating-ball is-hidden';
    refs.floatingBall.setAttribute('aria-expanded', 'true');
    return;
  }

  refs.assistantPanel.classList.toggle('is-hidden', !state.isPanelOpen);
  refs.floatingBall.setAttribute('aria-expanded', String(state.isPanelOpen));
  refs.floatingBall.textContent = state.isPanelOpen ? 'Hide assistant' : 'Open assistant';
}
```

```css
/* style.css additions */
.floating-ball.is-hidden {
  display: none;
}

.assistant-panel {
  position: fixed;
}

body.desktop-shell .assistant-panel {
  position: static;
  display: block;
  margin-top: 16px;
}

body.desktop-shell .app-shell {
  max-width: none;
  min-height: 100vh;
  padding-bottom: 24px;
}
```

Also update `app.js` init to add `document.body.classList.add('desktop-shell')` when desktop mode is active.

- [ ] **Step 4: Run the updated tests to verify they pass**

Run: `node --test tests/floating.test.js tests/app.test.js`
Expected: PASS for floating renderer and desktop-mode app behavior

- [ ] **Step 5: Run the full Node test suite before final verification**

Run: `node --test`
Expected: PASS for all existing and new tests

- [ ] **Step 6: Commit if the workspace is a git repo**

```bash
git add floating.html floating.css floating.js app.js style.css tests/floating.test.js tests/app.test.js
git commit -m "feat: add desktop floating ball UI"
```

If the workspace is still not a git repo, mark the step complete without committing.

### Task 4: Launch The Desktop Shell And Verify The Real Window Flow

**Files:**
- Modify if needed: `main.js`
- Modify if needed: `floating.html`
- Modify if needed: `floating.css`
- Modify if needed: `app.js`
- Modify if needed: `style.css`

- [ ] **Step 1: Run the full automated suite**

Run: `node --test`
Expected: PASS for all tests

- [ ] **Step 2: Fix any failing tests or shell regressions with the smallest possible change**

```js
// Example fixes only if needed:
// - adjust BrowserWindow options
// - correct panel URL query handling
// - correct desktop-mode body class or hidden floating-ball state
```

- [ ] **Step 3: Launch the Electron app for desktop-shell verification**

Run: `npm start`

Manually verify:
- a separate floating ball window appears
- clicking it opens the main panel window
- clicking it again hides the main panel window
- the main panel still runs inspection -> draft confirmation -> draft -> log -> undo
- hiding or closing the panel does not remove the floating ball

- [ ] **Step 4: If launch behavior is wrong, make the smallest fix and re-run `node --test`**

Run: `node --test`
Expected: PASS after any fix

- [ ] **Step 5: Commit if the workspace is a git repo**

```bash
git add package.json package-lock.json desktop-shell.js main-controller.js main.js preload.js floating.html floating.css floating.js app.js style.css tests/package.test.js tests/desktop-shell.test.js tests/main-controller.test.js tests/preload.test.js tests/floating.test.js tests/app.test.js
git commit -m "feat: add desktop shell prototype"
```

If the workspace is still not a git repo, mark the step complete without committing.
