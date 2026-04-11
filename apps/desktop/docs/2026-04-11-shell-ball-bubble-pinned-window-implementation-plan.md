# Shell-Ball Bubble Pinned Window Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade the shell-ball bubble system so bubble items align to protocol semantics, support per-bubble pin/delete controls, detach pinned bubbles into standalone draggable windows, and return unpinned bubbles to their timestamp order in the transparent bubble region.

**Architecture:** Keep `packages/protocol` as the semantic source of truth for the bubble payload core by wrapping protocol `BubbleMessage` objects in a shell-ball-local desktop state envelope instead of inventing a second independent message model. The always-present `shell-ball-bubble` window renders only unpinned items in the bubble region, while each pinned item gets its own runtime-created detached window that survives bubble-region lifecycle rules until unpinned or deleted.

**Tech Stack:** Tauri 2 window API, React 18, TypeScript, existing shell-ball coordinator/window sync layer, shell-ball contract test suite.

---

## File Map

- Create: `apps/desktop/src/features/shell-ball/shellBallBubbleDesktop.ts`
- Create: `apps/desktop/src/features/shell-ball/ShellBallPinnedBubbleWindow.tsx`
- Create: `apps/desktop/src/app/shell-ball-bubble-pinned/main.tsx`
- Create: `apps/desktop/shell-ball-bubble-pinned.html`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.bubble.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.windowSync.ts`
- Modify: `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`
- Modify: `apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleMessage.tsx`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.css`
- Modify: `apps/desktop/src/features/shell-ball/useShellBallWindowMetrics.ts`
- Modify: `apps/desktop/src/platform/shellBallWindowController.ts`
- Modify: `apps/desktop/vite.config.ts`
- Modify: `apps/desktop/src-tauri/capabilities/default.json`
- Modify: `apps/desktop/src-tauri/gen/schemas/capabilities.json`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

## Target Model

The follow-up implementation should replace the current ad hoc bubble item shape with a protocol-wrapped desktop bubble item:

```ts
type ShellBallBubbleRole = "user" | "agent"

type ShellBallBubbleDesktopState = {
  detachedWindowId?: string
  lifecycleState: "visible" | "fading" | "hidden"
  freshnessHint?: "fresh" | "stale"
  motionHint?: "settle"
}

type ShellBallBubbleItem = {
  bubble: BubbleMessage
  role: ShellBallBubbleRole
  desktop: ShellBallBubbleDesktopState
}
```

Rules:

- `bubble` carries the protocol-aligned payload core (`bubble_id`, `task_id`, `type`, `text`, `pinned`, `hidden`, `created_at`).
- `role` remains a shell-ball UI concern until protocol adds role semantics.
- `desktop` is shell-ball-local and must not be pushed into `packages/protocol`.
- Bubble region renders items with `bubble.hidden === false && bubble.pinned === false`.
- Pinned windows render items with `bubble.pinned === true`.

## Window Interaction and Runtime Rules

- `shell-ball-bubble` stays structurally present as the bubble-region window, but its interaction mode depends on content state:
  - when there are no visible unpinned bubbles, it remains transparent and click-through
  - when visible unpinned bubbles exist, it must disable click-through so pin/delete controls are usable
- pinned windows use deterministic labels: `shell-ball-bubble-pinned-<bubble_id>`
- pinned windows receive one `ShellBallBubbleItem` snapshot payload and return actions through shell-ball-local event names such as:
  - `desktop-shell-ball:bubble-pin-toggle`
  - `desktop-shell-ball:bubble-delete`
- pinned windows spawn from a named pinned-window anchor near the bubble region before first drag
- once the user drags a pinned window, it becomes free-positioned and no longer follows shell-ball or bubble-region geometry updates
- only the main bubble region continues using the shared helper-window metrics/anchor flow
- unpin ordering is stable by `bubble.created_at`, then `bubble.bubble_id`
- bubble-region offset tuning must use named constants in the metrics layer and be verified against the ball frame so the region moves closer without overlapping the mascot window

## Coordinator Ownership Contract

- The coordinator is the single source of truth for bubble-item existence, ordering, `bubble.pinned`, and delete/unpin state transitions.
- Pinned windows only render snapshot data from the coordinator; they do not own independent bubble-item state.
- All `pin`, `unpin`, and `delete` actions round-trip through the coordinator before UI state changes are considered final.
- Detached position may live in local desktop/window state after the first user drag, but bubble-item existence and region ordering remain coordinator-owned.

## Behavioral Constraints

- The bubble region window always exists, but its background remains fully transparent.
- If there are no visible unpinned bubbles, the user sees nothing in the bubble region.
- Every bubble item has two controls:
  - top-left pin toggle
  - top-right delete
- Delete always destroys the bubble item immediately.
- Pin removes the bubble item from the bubble region and creates a standalone detached pinned window.
- Detached pinned windows:
  - are draggable
  - stay always-on-top
  - do not participate in bubble fade/hide/disperse behavior
- Unpin destroys the detached window and returns the bubble item to the bubble region sorted by `bubble.created_at`, then `bubble.bubble_id`.
- Bubble region offset must be defined by named constants and verified against the ball frame so it moves closer without overlap.
- Bubble region has no fixed item count limit; visibility is governed by space + scrolling only.

### Task 1: Refactor Bubble Data to Protocol-Wrapped Items

**Files:**
- Create: `apps/desktop/src/features/shell-ball/shellBallBubbleDesktop.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.bubble.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.windowSync.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write the failing contract test**

Add tests asserting that shell-ball bubble data now wraps protocol `BubbleMessage` instead of exposing only front-end-local fields, while still preserving shell-ball-local `role` and desktop state.

Cover:
- `ShellBallBubbleItem.bubble` matches protocol field names
- `role` stays outside protocol payload
- `desktop` carries shell-ball-local lifecycle/window state
- snapshot payload now uses `bubbleItems` instead of `bubbleMessages`

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because current bubble model still uses `bubbleMessages` and front-end-only fields.

- [ ] **Step 3: Write minimal implementation**

Create `apps/desktop/src/features/shell-ball/shellBallBubbleDesktop.ts` for shell-ball-local desktop extensions and mapping helpers.

Refactor `apps/desktop/src/features/shell-ball/shellBall.bubble.ts` so the exported type becomes protocol-wrapped `ShellBallBubbleItem`.

Update `apps/desktop/src/features/shell-ball/shellBall.windowSync.ts` to carry `bubbleItems` in the snapshot.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for the protocol-wrapped bubble item contract.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/features/shell-ball/shellBallBubbleDesktop.ts apps/desktop/src/features/shell-ball/shellBall.bubble.ts apps/desktop/src/features/shell-ball/shellBall.windowSync.ts apps/desktop/src/features/shell-ball/shellBall.contract.test.ts
git commit -m "feat(desktop-shell-ball): align bubble items to protocol payloads"
```

### Task 2: Verify Bubble Region Window Existence Strategy First

**Files:**
- Modify: `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.windowSync.ts`
- Modify: `apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx`
- Modify: `apps/desktop/src/features/shell-ball/useShellBallWindowMetrics.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write the failing contract test**

Add tests asserting that:
- the bubble helper window existence strategy is explicit and verified independently of rendering details
- the bubble helper window is no longer gated purely by `visualState`
- the snapshot/visibility contract distinguishes bubble-window existence from empty bubble-region content
- it receives a flat `bubbleItems` list
- the visible list excludes `bubble.pinned === true`
- the empty region stays structurally present but visually transparent and click-through when there are no visible unpinned bubbles
- non-empty bubble regions disable click-through so pin/delete controls are usable

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because current visibility/rendering still assumes the earlier message-only model and the existence strategy is not contract-defined.

- [ ] **Step 3: Write minimal implementation**

Update the coordinator to produce the new `bubbleItems` snapshot.

Update `shellBall.windowSync.ts` so bubble-region persistence is not driven only by `visualState`.

Update `ShellBallBubbleWindow.tsx`, `ShellBallBubbleZone.tsx`, and `useShellBallWindowMetrics.ts` so the bubble region only renders unpinned items while the bubble window existence strategy is explicit and the interaction mode follows content state.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for the chosen existence strategy contract and unpinned-only rendering.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts apps/desktop/src/features/shell-ball/shellBall.windowSync.ts apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx apps/desktop/src/features/shell-ball/useShellBallWindowMetrics.ts apps/desktop/src/features/shell-ball/shellBall.contract.test.ts
git commit -m "feat(desktop-shell-ball): define bubble window existence strategy"
```

### Task 3: Add Per-Bubble Pin and Delete Controls

**Files:**
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleMessage.tsx`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx`
- Modify: `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`
- Modify: `apps/desktop/src/features/shell-ball/useShellBallWindowMetrics.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write the failing render/behavior test**

Add tests asserting that each bubble renders:
- a pin control at top-left
- a delete control at top-right

And that the coordinator exposes handlers that:
- set `bubble.pinned = true` on pin
- remove the bubble item on delete
- switch the bubble window out of click-through mode when visible unpinned bubbles remain

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because bubble items do not yet have controls or state transitions.

- [ ] **Step 3: Write minimal implementation**

Add pin/delete controls to `ShellBallBubbleMessage.tsx`.

Thread handlers through `ShellBallBubbleZone.tsx`, implement the state transitions in the coordinator, and update the bubble window metrics path so the bubble window interaction mode follows visible content.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for pin/delete controls and local state transitions.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/features/shell-ball/components/ShellBallBubbleMessage.tsx apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts apps/desktop/src/features/shell-ball/useShellBallWindowMetrics.ts apps/desktop/src/features/shell-ball/shellBall.contract.test.ts
git commit -m "feat(desktop-shell-ball): add bubble pin and delete controls"
```

### Task 4: Add Coordinator Ownership Contract for Pinned Bubble State

**Files:**
- Modify: `apps/desktop/src/features/shell-ball/shellBall.windowSync.ts`
- Modify: `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write the failing ownership contract test**

Add tests asserting that:
- pinned windows only render snapshot payloads from the coordinator
- all `pin`, `unpin`, and `delete` actions round-trip through coordinator-owned events/state transitions
- detached position may be local desktop state, but bubble-item existence/order is coordinator-owned

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because pinned-window ownership and action round-trip rules are not explicit yet.

- [ ] **Step 3: Write minimal implementation**

Extend `shellBall.windowSync.ts` with the coordinator-owned pinned-window event contract.

Update `useShellBallCoordinator.ts` to become the explicit source of truth for pin/unpin/delete state transitions before detached window implementation begins.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for pinned-window ownership/state contract.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/features/shell-ball/shellBall.windowSync.ts apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts apps/desktop/src/features/shell-ball/shellBall.contract.test.ts
git commit -m "feat(desktop-shell-ball): define pinned bubble state ownership"
```

### Task 5: Create Detached Pinned Bubble Windows

**Files:**
- Create: `apps/desktop/src/app/shell-ball-bubble-pinned/main.tsx`
- Create: `apps/desktop/shell-ball-bubble-pinned.html`
- Create: `apps/desktop/src/features/shell-ball/ShellBallPinnedBubbleWindow.tsx`
- Modify: `apps/desktop/vite.config.ts`
- Modify: `apps/desktop/src/platform/shellBallWindowController.ts`
- Modify: `apps/desktop/src-tauri/capabilities/default.json`
- Modify: `apps/desktop/src-tauri/gen/schemas/capabilities.json`
- Modify: `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.windowSync.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write the failing window contract test**

Add tests asserting that pinning a bubble:
- assigns a deterministic pinned-window label using `bubble.bubble_id`
- creates a detached window label for the bubble item
- removes it from the bubble region view
- renders the bubble item in a dedicated pinned-bubble window
- keeps the pinned item outside normal bubble lifecycle rules
- routes unpin/delete actions back through shell-ball-local pinned-window events
- spawns the pinned window from a defined initial anchor near the bubble region
- stops following shell-ball geometry after the user drags it

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because pinned windows do not exist yet.

- [ ] **Step 3: Write minimal implementation**

Add the dedicated pinned bubble entrypoint and component.

Extend `shellBallWindowController.ts` with runtime create/open/close helpers for pinned bubble windows.

Extend `shellBall.windowSync.ts` with pinned-window event names and payloads.

Update capabilities to allow the required create/show/hide/focus/drag behavior.

Implement the pinned-window geometry contract explicitly:
- initial spawn anchor comes from a named pinned-window anchor near the bubble region
- free-dragged pinned windows stop following the shell-ball metrics flow after user drag
- only the main bubble region remains on the shared helper-window metrics flow

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for detached pinned-window behavior.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/app/shell-ball-bubble-pinned/main.tsx apps/desktop/shell-ball-bubble-pinned.html apps/desktop/src/features/shell-ball/ShellBallPinnedBubbleWindow.tsx apps/desktop/vite.config.ts apps/desktop/src/platform/shellBallWindowController.ts apps/desktop/src-tauri/capabilities/default.json apps/desktop/src-tauri/gen/schemas/capabilities.json apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts apps/desktop/src/features/shell-ball/shellBall.windowSync.ts apps/desktop/src/features/shell-ball/shellBall.contract.test.ts
git commit -m "feat(desktop-shell-ball): detach pinned bubbles into windows"
```

### Task 6: Support Unpin Return-by-Timestamp and Detached Delete Semantics

**Files:**
- Modify: `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write the failing behavior/style test**

Add tests asserting that:
- unpinning destroys the detached window and restores the bubble item into bubble-region order by `bubble.created_at`, then `bubble.bubble_id`
- deleting from a detached window destroys the bubble item entirely

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because unpin restore ordering and detached delete semantics are not implemented yet.

- [ ] **Step 3: Write minimal implementation**

Implement unpin restore ordering in the coordinator using `bubble.created_at`, then `bubble.bubble_id`.

Finish the detached delete path.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for unpin return and detached delete behavior.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts apps/desktop/src/features/shell-ball/shellBall.contract.test.ts
git commit -m "feat(desktop-shell-ball): restore pinned bubbles to region order"
```

### Task 7: Tune Bubble-Region Offset Closer to the Shell-Ball

**Files:**
- Modify: `apps/desktop/src/features/shell-ball/useShellBallWindowMetrics.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.css`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write the failing offset contract test**

Add tests asserting that:
- bubble region anchor uses named constants
- the region is closer to the shell-ball than the current baseline
- the chosen offset still does not overlap the ball frame

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because the old anchor gap constants still reflect the previous offset.

- [ ] **Step 3: Write minimal implementation**

Tune the bubble-region anchor gap in `useShellBallWindowMetrics.ts` and any supporting CSS constants so the region sits lower and closer to the shell-ball using named constants and verified no-overlap behavior.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for bubble-region offset tuning.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/features/shell-ball/useShellBallWindowMetrics.ts apps/desktop/src/features/shell-ball/shellBall.css apps/desktop/src/features/shell-ball/shellBall.contract.test.ts
git commit -m "feat(desktop-shell-ball): tune bubble region anchor"
```

### Task 8: Verify Bubble Region and Detached Window Behavior End-to-End

**Files:**
- Verify only

- [ ] **Step 1: Run shell-ball tests**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS.

- [ ] **Step 2: Run typecheck**

Run: `pnpm --dir apps/desktop typecheck`
Expected: PASS.

- [ ] **Step 3: Run build**

Run: `pnpm --dir apps/desktop build`
Expected: PASS.

- [ ] **Step 4: Run the desktop app manually**

Run: `pnpm --dir apps/desktop exec tauri dev`

Expected:
- bubble region stays visually transparent when empty
- each new bubble appears in timestamp order within the bubble region
- pin creates a detached window and removes the bubble from the bubble region
- detached windows are draggable and stay alive regardless of bubble fade/disperse behavior
- unpin returns the bubble to the region at its timestamp position
- delete removes the bubble immediately from either location
- bubble region sits closer to the shell-ball without overlapping it

- [ ] **Step 5: Commit only if manual verification required a follow-up fix**

Run: `git status --short`
Expected: clean working tree; if not, commit with:

```bash
git add <fixed-files>
git commit -m "feat(desktop-shell-ball): polish pinned bubble behavior"
```
