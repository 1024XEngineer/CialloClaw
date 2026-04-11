# Shell Ball Bubble Mock Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the shell-ball bubble helper window accept local mock input, preserve readable scroll history, keep old bubbles dissipating in place, and prevent pinned bubbles from disappearing.

**Architecture:** Extend the existing `useShellBallCoordinator.ts` local snapshot owner instead of adding a new store or protocol boundary. Bubble history, mock replies, dissipation, and pinning stay frontend-local and continue syncing to helper windows through the current snapshot event flow.

**Tech Stack:** React, TypeScript, Tauri helper windows, existing shell-ball contract tests, CSS animations.

---

## File Map

- Modify: `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`
  - add local mock bubble append flow
  - add lifecycle timers for dissipation
  - make pinning preserve main-history visibility
- Modify: `apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx`
  - pass state-aware pin and unpin actions into the history bubble zone
- Modify: `apps/desktop/src/features/shell-ball/shellBall.windowSync.ts`
  - expose history-visible bubbles without excluding pinned ones
  - define helper-window interaction pause payloads and keep bubble-region semantics aligned with visible history
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx`
  - make auto-scroll conditional on near-bottom state
  - emit helper-window interaction pause signals while user is interacting with history
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleMessage.tsx`
  - render lifecycle and pinned data attributes
  - swap pin action to unpin for pinned history bubbles
- Modify: `apps/desktop/src/features/shell-ball/shellBall.css`
  - add fading and pinned visual states
- Test: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`
  - cover all new behavior slices first

### Task 1: Commit Design And Plan Docs

**Files:**
- Create: `apps/desktop/docs/2026-04-12-shell-ball-bubble-mock-design.md`
- Create: `apps/desktop/docs/2026-04-12-shell-ball-bubble-mock-implementation-plan.md`

- [ ] **Step 1: Review doc wording for feature boundaries**

Confirm the docs state that all behavior stays frontend-local and does not change protocol or task objects.

- [ ] **Step 2: Commit docs**

Run:

```bash
git add apps/desktop/docs/2026-04-12-shell-ball-bubble-mock-design.md apps/desktop/docs/2026-04-12-shell-ball-bubble-mock-implementation-plan.md && git commit -m "docs(bubble): add mock bubble design and plan"
```

Expected: a new docs commit is created.

### Task 2: Add Mock Input-To-Bubble Flow

**Files:**
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`
- Modify: `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`

- [ ] **Step 1: Write the failing test**

Add a contract test that submits a draft through the coordinator helper-window flow and expects:

- a new user bubble
- at least one deterministic mock agent bubble
- cleared draft input

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
pnpm --dir apps/desktop test:shell-ball
```

Expected: the new test fails because no mock append flow exists yet.

- [ ] **Step 3: Write minimal implementation**

Implement a small helper inside `useShellBallCoordinator.ts` that:

- appends a user bubble from submitted draft text
- schedules deterministic mock agent bubbles
- clears the input draft

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
pnpm --dir apps/desktop test:shell-ball
```

Expected: the new test passes and existing shell-ball tests stay green.

- [ ] **Step 5: Commit**

Run:

```bash
git add apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts apps/desktop/src/features/shell-ball/shellBall.contract.test.ts && git commit -m "feat(bubble): add mock input bubble flow"
```

### Task 3: Make History Scrolling User-Aware

**Files:**
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.windowSync.ts`
- Modify: `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`

- [ ] **Step 1: Write the failing tests**

Add tests that prove:

- new bubbles auto-scroll only when the viewer is near the bottom
- lifecycle-only updates do not force snap-back while reading history

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
pnpm --dir apps/desktop test:shell-ball
```

Expected: the new tests fail because the component currently always scrolls to the bottom.

- [ ] **Step 3: Write minimal implementation**

Update `ShellBallBubbleZone.tsx` to track bottom proximity and only auto-scroll for real feed growth when the user is already near the bottom. Wire bubble-window interaction events back to the coordinator so later dissipation timers can be paused without changing the scroll contract again.

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
pnpm --dir apps/desktop test:shell-ball
```

Expected: the new scroll tests pass and existing shell-ball tests remain green.

- [ ] **Step 5: Commit**

Run:

```bash
git add apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx apps/desktop/src/features/shell-ball/shellBall.windowSync.ts apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts apps/desktop/src/features/shell-ball/shellBall.contract.test.ts && git commit -m "feat(bubble): preserve readable bubble history"
```

### Task 4: Add Dissipation Lifecycle

**Files:**
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`
- Modify: `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleMessage.tsx`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.css`

- [ ] **Step 1: Write the failing tests**

Add tests that prove:

- unpinned bubbles transition from `visible` to `fading`
- pinned bubbles do not auto-fade
- history still contains faded bubbles
- bubble-window hover or scroll pauses dissipation while the user is reading history

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
pnpm --dir apps/desktop test:shell-ball
```

Expected: the new lifecycle tests fail because no dissipation timer exists yet.

- [ ] **Step 3: Write minimal implementation**

Add frontend-local dissipation timers and render fading-state data attributes for CSS styling.

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
pnpm --dir apps/desktop test:shell-ball
```

Expected: lifecycle tests pass and no existing tests regress.

- [ ] **Step 5: Commit**

Run:

```bash
git add apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts apps/desktop/src/features/shell-ball/components/ShellBallBubbleMessage.tsx apps/desktop/src/features/shell-ball/shellBall.css apps/desktop/src/features/shell-ball/shellBall.contract.test.ts && git commit -m "feat(bubble): add bubble dissipation states"
```

### Task 5: Fix Pinning Regression

**Files:**
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`
- Modify: `apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.windowSync.ts`
- Modify: `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleMessage.tsx`

- [ ] **Step 1: Write the failing tests**

Add tests that prove:

- pinned bubbles remain visible in main history
- pinned bubbles render an `Unpin` control in history
- pinning still drives the detached pinned window flow
- `bubbleRegion` visibility and click-through semantics stay coherent after pinned bubbles remain in history

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
pnpm --dir apps/desktop test:shell-ball
```

Expected: the pinning regression tests fail against the current filter-and-disappear behavior.

- [ ] **Step 3: Write minimal implementation**

Stop excluding pinned bubbles from the history list, switch the history control to pin/unpin based on current state, and make the coordinator open the pinned window after state is definitely updated.

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
pnpm --dir apps/desktop test:shell-ball
```

Expected: pinning tests pass and detached pinned-window behavior stays intact.

- [ ] **Step 5: Commit**

Run:

```bash
git add apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx apps/desktop/src/features/shell-ball/shellBall.windowSync.ts apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts apps/desktop/src/features/shell-ball/components/ShellBallBubbleMessage.tsx apps/desktop/src/features/shell-ball/shellBall.contract.test.ts && git commit -m "feat(bubble): keep pinned bubbles visible"
```

### Task 6: Final Verification

**Files:**
- Modify: none expected unless fixes are needed

- [ ] **Step 1: Run shell-ball contract tests**

Run:

```bash
pnpm --dir apps/desktop test:shell-ball
```

Expected: PASS.

- [ ] **Step 2: Run desktop typecheck**

Run:

```bash
pnpm --dir apps/desktop typecheck
```

Expected: PASS.

- [ ] **Step 3: Resolve any final failures**

Fix only regressions uncovered by the verification commands.
