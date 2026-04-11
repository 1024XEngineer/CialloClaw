# Desktop Shell-Ball Three-Window Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `shell-ball` as a real Tauri 2 desktop floating-ball app with three transparent windows: ball, bubble, and input.

**Architecture:** The `shell-ball` window remains the primary interaction and coordination surface. `shell-ball-bubble` and `shell-ball-input` are predeclared transparent helper windows controlled through a frontend platform adapter that synchronizes visibility, position, and size while keeping Tauri APIs out of feature presentation code.

**Tech Stack:** Tauri 2 window API, React 18, TypeScript, Vite multi-entry build, Zustand, Tauri event/window permissions.

---

## Coordination Contract

- **Source of truth:** `shell-ball` owns `visualState`, input draft, hover retention, and focus-retention rules.
- **Transport:** use Tauri 2 frontend window events (`emitTo` / `listen`) instead of Rust-managed shared state.
- **Outbound events from ball window:**
  - `desktop-shell-ball:snapshot` with `visualState`, input mode, draft text, helper visibility flags
  - `desktop-shell-ball:geometry` with ball window outer position, scale factor, and measured content box
- **Ready handshake:** helper windows emit `desktop-shell-ball:helper-ready` on mount; the ball window immediately replays the latest snapshot and geometry to that helper before first reveal.
- **Inbound events to ball window:**
  - `desktop-shell-ball:helper-ready` for helper mount/resync
  - `desktop-shell-ball:input-hover` for helper hover enter/leave
  - `desktop-shell-ball:input-focus` for helper focus changes
  - `desktop-shell-ball:input-draft` for text updates
  - `desktop-shell-ball:primary-action` for submit / attach / click actions emitted from helper windows
- **Helper behavior:** bubble is read-only display and does not own state; input can emit interaction events but never becomes the source of truth.

## Runtime Window Rules

- `shell-ball`, `shell-ball-bubble`, and `shell-ball-input` are all `transparent`, `decorations: false`, `alwaysOnTop: true`, `resizable: false`, `shadow: false`.
- helper windows start with `visible: false`; `shell-ball-bubble` should apply runtime `setFocusable(false)` and `setIgnoreCursorEvents(true)` so it neither steals focus nor blocks clicks underneath.
- all three windows should use `skipTaskbar: true` to behave like desktop floating surfaces rather than normal app windows.
- anchor and positioning calculations must use the ball window outer position and scale factor to stay DPI-safe.
- helper windows should open/close without moving the ball window and should not create scrollbars or padded transparent rectangles.
- work-area clamping should use the current monitor bounds returned by Tauri before applying final helper positions.

## File Map

- Create: `apps/desktop/shell-ball-bubble.html`
- Create: `apps/desktop/shell-ball-input.html`
- Create: `apps/desktop/src/app/shell-ball-bubble/main.tsx`
- Create: `apps/desktop/src/app/shell-ball-input/main.tsx`
- Create: `apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx`
- Create: `apps/desktop/src/features/shell-ball/ShellBallInputWindow.tsx`
- Create: `apps/desktop/src/features/shell-ball/shellBall.windowSync.ts`
- Create: `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`
- Create: `apps/desktop/src/features/shell-ball/useShellBallWindowMetrics.ts`
- Create: `apps/desktop/src/platform/shellBallWindowController.ts`
- Create: `apps/desktop/src-tauri/capabilities/default.json`
- Modify: `apps/desktop/vite.config.ts`
- Modify: `apps/desktop/src-tauri/tauri.conf.json`
- Modify: `apps/desktop/src/app/shell-ball/main.tsx`
- Modify: `apps/desktop/src/styles/globals.css`
- Modify: `apps/desktop/src/features/shell-ball/ShellBallApp.tsx`
- Modify: `apps/desktop/src/features/shell-ball/ShellBallSurface.tsx`
- Modify: `apps/desktop/src/features/shell-ball/useShellBallInteraction.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.types.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.css`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallInputBar.tsx`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallMascot.tsx`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

### Task 1: Add Tauri Three-Window Host Entries

**Files:**
- Create: `apps/desktop/shell-ball-bubble.html`
- Create: `apps/desktop/shell-ball-input.html`
- Create: `apps/desktop/src/app/shell-ball-bubble/main.tsx`
- Create: `apps/desktop/src/app/shell-ball-input/main.tsx`
- Modify: `apps/desktop/vite.config.ts`
- Modify: `apps/desktop/src-tauri/tauri.conf.json`

- [ ] **Step 1: Write the failing contract test for expected window labels and entry points**

Add assertions in `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts` that document the three expected window labels and helper entry URLs.

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because the new labels/entries do not exist yet.

- [ ] **Step 3: Add the new HTML and React entry points**

Create `shell-ball-bubble.html`, `shell-ball-input.html`, and matching `main.tsx` files that mount the new helper window roots through `AppProviders`.

- [ ] **Step 4: Extend the Vite multi-entry build**

Update `apps/desktop/vite.config.ts` so build input includes `shell-ball`, `shell-ball-bubble`, and `shell-ball-input`.

- [ ] **Step 5: Declare the three Tauri windows**

Update `apps/desktop/src-tauri/tauri.conf.json` so:
- `shell-ball` is visible on launch
- `shell-ball-bubble` and `shell-ball-input` start hidden
- all three are transparent, undecorated, always-on-top, and `shadow: false`
- all three use `resizable: false` and `skipTaskbar: true`
- `shell-ball-bubble` keeps config minimal; focus and cursor passthrough are applied at runtime

- [ ] **Step 6: Re-run the targeted test**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for the new entry/window contract assertions.

- [ ] **Step 7: Commit the slice**

Run: `git add apps/desktop/vite.config.ts apps/desktop/src-tauri/tauri.conf.json apps/desktop/shell-ball-bubble.html apps/desktop/shell-ball-input.html apps/desktop/src/app/shell-ball-bubble/main.tsx apps/desktop/src/app/shell-ball-input/main.tsx apps/desktop/src/features/shell-ball/shellBall.contract.test.ts && git commit -m "feat(desktop-shell-ball): add three-window desktop host entries"`

### Task 2: Add Window Permissions and Platform Controller

**Files:**
- Create: `apps/desktop/src-tauri/capabilities/default.json`
- Create: `apps/desktop/src/platform/shellBallWindowController.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write failing tests for window controller constants and capabilities contract**

Add tests covering expected window labels, helper visibility semantics, and permission identifiers such as `core:window:allow-start-dragging`, `core:window:allow-set-size`, and `core:window:allow-set-position`.

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because the platform controller and capability file do not exist yet.

- [ ] **Step 3: Add the capability manifest**

Create `apps/desktop/src-tauri/capabilities/default.json` with a concrete Tauri 2 capability object including:
- `identifier`
- `windows`: `shell-ball`, `shell-ball-bubble`, `shell-ball-input`, `dashboard`, `control-panel`
- `permissions`: `core:default`, `core:window:allow-show`, `core:window:allow-hide`, `core:window:allow-set-position`, `core:window:allow-set-size`, `core:window:allow-set-size-constraints`, `core:window:allow-set-focus`, `core:window:allow-set-focusable`, `core:window:allow-set-ignore-cursor-events`, `core:window:allow-set-always-on-top`, `core:window:allow-start-dragging`

- [ ] **Step 4: Add the shell-ball platform controller**

Create `apps/desktop/src/platform/shellBallWindowController.ts` to centralize:
- window labels
- current window lookup
- helper window lookup by label
- show/hide
- set position/size
- start dragging

- [ ] **Step 5: Re-run the test**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for controller/capability contract tests.

- [ ] **Step 6: Commit the slice**

Run: `git add apps/desktop/src-tauri/capabilities/default.json apps/desktop/src/platform/shellBallWindowController.ts apps/desktop/src/features/shell-ball/shellBall.contract.test.ts && git commit -m "feat(desktop-shell-ball): add tauri window control adapter"`

### Task 3: Split the Existing Surface into Ball, Bubble, and Input Window Components

**Files:**
- Create: `apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx`
- Create: `apps/desktop/src/features/shell-ball/ShellBallInputWindow.tsx`
- Modify: `apps/desktop/src/features/shell-ball/ShellBallApp.tsx`
- Modify: `apps/desktop/src/features/shell-ball/ShellBallSurface.tsx`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallInputBar.tsx`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write failing tests for the three-window rendering split**

Add assertions that:
- `ShellBallApp` renders only ball content
- `ShellBallBubbleWindow` owns bubble rendering
- `ShellBallInputWindow` owns input rendering

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because the ball window still renders combined content.

- [ ] **Step 3: Extract bubble and input window roots**

Create `ShellBallBubbleWindow.tsx` and `ShellBallInputWindow.tsx` with minimal props so each one only renders its own content.

- [ ] **Step 4: Shrink the main shell-ball window to mascot-only**

Refactor `ShellBallApp.tsx` and `ShellBallSurface.tsx` so the production ball window only contains the mascot shell and drag frame.

- [ ] **Step 5: Update component boundaries**

Refactor `ShellBallBubbleZone.tsx` and `ShellBallInputBar.tsx` if needed so they can be mounted independently without page-shell assumptions.

- [ ] **Step 6: Re-run the test**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for the split rendering assertions.

- [ ] **Step 7: Commit the slice**

Run: `git add apps/desktop/src/features/shell-ball/ShellBallApp.tsx apps/desktop/src/features/shell-ball/ShellBallSurface.tsx apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx apps/desktop/src/features/shell-ball/ShellBallInputWindow.tsx apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx apps/desktop/src/features/shell-ball/components/ShellBallInputBar.tsx apps/desktop/src/features/shell-ball/shellBall.contract.test.ts && git commit -m "feat(desktop-shell-ball): split ball bubble and input windows"`

### Task 4: Add Cross-Window Sync and Coordinator Logic

**Files:**
- Create: `apps/desktop/src/features/shell-ball/shellBall.windowSync.ts`
- Create: `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`
- Modify: `apps/desktop/src/features/shell-ball/useShellBallInteraction.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.types.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write failing tests for cross-window visibility rules and payload mapping**

Cover:
- `idle` hides bubble/input
- `hover_input` shows bubble and input
- task/voice states keep helper windows visible according to existing UI semantics

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because no coordinator/payload mapping exists yet.

- [ ] **Step 3: Add front-end-only sync payloads**

Create `shellBall.windowSync.ts` with typed payloads and string constants for:
- `desktop-shell-ball:snapshot`
- `desktop-shell-ball:geometry`
- `desktop-shell-ball:helper-ready`
- `desktop-shell-ball:input-hover`
- `desktop-shell-ball:input-focus`
- `desktop-shell-ball:input-draft`
- `desktop-shell-ball:primary-action`

- [ ] **Step 4: Add the coordinator hook**

Create `useShellBallCoordinator.ts` so the ball window becomes the source of truth that emits state to helper windows via `emitTo` and reacts to helper-window feedback via `listen`.

- [ ] **Step 5: Wire interaction state into the coordinator**

Update `useShellBallInteraction.ts` and `shellBall.types.ts` so the current visual state, input mode, and hover retention are usable across windows without changing shared protocol entities.

- [ ] **Step 6: Re-run the test**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for helper-window visibility and payload tests.

- [ ] **Step 7: Commit the slice**

Run: `git add apps/desktop/src/features/shell-ball/shellBall.windowSync.ts apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts apps/desktop/src/features/shell-ball/useShellBallInteraction.ts apps/desktop/src/features/shell-ball/shellBall.types.ts apps/desktop/src/features/shell-ball/shellBall.contract.test.ts && git commit -m "feat(desktop-shell-ball): add helper window state sync"`

### Task 5: Implement Window Size and Anchor Synchronization

**Files:**
- Create: `apps/desktop/src/features/shell-ball/useShellBallWindowMetrics.ts`
- Modify: `apps/desktop/src/features/shell-ball/ShellBallApp.tsx`
- Modify: `apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx`
- Modify: `apps/desktop/src/features/shell-ball/ShellBallInputWindow.tsx`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write failing tests for anchor and size calculations**

Cover pure helpers that compute:
- ball safe size
- bubble anchor above the ball
- input anchor below the ball
- fixed gap and safe margin behavior

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because the measurement/anchor helpers do not exist yet.

- [ ] **Step 3: Add measurement helpers and hooks**

Create `useShellBallWindowMetrics.ts` using `ResizeObserver` and pure anchor helpers that the tests can call directly.

- [ ] **Step 4: Wire measurements to the platform controller**

Update the three window roots so they report measured content size and the controller applies `setSize` and `setPosition` with safe margins, current monitor work-area clamping, and scale-factor-aware coordinates.

- [ ] **Step 5: Re-run the test**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for the geometry helpers.

- [ ] **Step 6: Commit the slice**

Run: `git add apps/desktop/src/features/shell-ball/useShellBallWindowMetrics.ts apps/desktop/src/features/shell-ball/ShellBallApp.tsx apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx apps/desktop/src/features/shell-ball/ShellBallInputWindow.tsx apps/desktop/src/features/shell-ball/shellBall.contract.test.ts && git commit -m "feat(desktop-shell-ball): sync helper window geometry"`

### Task 6: Add Real Desktop Dragging Without Breaking Voice Gestures

**Files:**
- Modify: `apps/desktop/src/features/shell-ball/ShellBallSurface.tsx`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallMascot.tsx`
- Modify: `apps/desktop/src/platform/shellBallWindowController.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write failing tests documenting drag-zone separation**

Add assertions that the production ball window renders a drag region separate from the voice hotspot.

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because no dedicated drag-region behavior exists yet.

- [ ] **Step 3: Add a dedicated drag region**

Update `ShellBallSurface.tsx` / `ShellBallMascot.tsx` so dragging starts from a dedicated outer zone rather than the center voice hotspot.

- [ ] **Step 4: Wire native `startDragging` through the platform controller**

Update `shellBallWindowController.ts` to call the Tauri drag API from the ball window.

- [ ] **Step 5: Re-run the test**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for drag-zone assertions.

- [ ] **Step 6: Commit the slice**

Run: `git add apps/desktop/src/features/shell-ball/ShellBallSurface.tsx apps/desktop/src/features/shell-ball/components/ShellBallMascot.tsx apps/desktop/src/platform/shellBallWindowController.ts apps/desktop/src/features/shell-ball/shellBall.contract.test.ts && git commit -m "feat(desktop-shell-ball): add draggable floating ball host"`

### Task 7: Make the Three Windows Fully Transparent and Scroll-Free

**Files:**
- Modify: `apps/desktop/src/styles/globals.css`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.css`
- Modify: `apps/desktop/src/app/shell-ball/main.tsx`
- Modify: `apps/desktop/src/app/shell-ball-bubble/main.tsx`
- Modify: `apps/desktop/src/app/shell-ball-input/main.tsx`

- [ ] **Step 1: Write failing tests for window-level CSS markers if practical; otherwise add render assertions documenting root window classes**

Add minimal assertions that each shell-ball entry marks itself as a dedicated transparent window mode.

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because the entry markers and CSS mode are not present yet.

- [ ] **Step 3: Add per-window root markers**

Update each `main.tsx` entry so `html` / `body` / `#root` can be styled for shell-ball window mode.

- [ ] **Step 4: Override global page-shell behavior**

Update `globals.css` so shell-ball windows use transparent background and `overflow: hidden` instead of full-page dark app-shell styling.

- [ ] **Step 5: Rewrite shell-ball layout CSS for content-fit windows**

Update `shellBall.css` to remove page-style width/padding assumptions and make each window hug its rendered content.

- [ ] **Step 6: Re-run the test**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for entry marker/render assertions.

- [ ] **Step 7: Run typecheck and build**

Run: `pnpm --dir apps/desktop typecheck && pnpm --dir apps/desktop build`
Expected: PASS with no type errors and successful multi-entry build.

- [ ] **Step 8: Commit the slice**

Run: `git add apps/desktop/src/styles/globals.css apps/desktop/src/features/shell-ball/shellBall.css apps/desktop/src/app/shell-ball/main.tsx apps/desktop/src/app/shell-ball-bubble/main.tsx apps/desktop/src/app/shell-ball-input/main.tsx apps/desktop/src/features/shell-ball/shellBall.contract.test.ts && git commit -m "feat(desktop-shell-ball): render transparent desktop helper windows"`

### Task 8: Final Desktop Verification

**Files:**
- Verify only

- [ ] **Step 1: Run shell-ball tests**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS.

- [ ] **Step 2: Run typecheck**

Run: `pnpm --dir apps/desktop typecheck`
Expected: PASS.

- [ ] **Step 3: Run production build**

Run: `pnpm --dir apps/desktop build`
Expected: PASS.

- [ ] **Step 4: Run the desktop app manually**

Run: `pnpm --dir apps/desktop exec tauri dev`
Expected:
- only the ball window is visible at launch
- bubble appears on hover or click-driven non-idle state
- input appears below the ball on hover/input states
- all non-content areas are transparent
- no window shows scrollbars
- dragging the ball moves the floating system without breaking voice hotspot behavior

- [ ] **Step 5: Commit the verification checkpoint if code changed during manual fixes**

Run: `git status --short`
Expected: no changes; if changes were needed during manual fix-up, commit them using `feat(desktop-shell-ball): polish three-window desktop behavior`.
