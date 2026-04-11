# Shell-Ball Bubble Window Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the shell-ball bubble window into a real transparent message stream that shows left/right chat bubbles, scrollable history, edge fade, and subtle new-message motion.

**Architecture:** Keep the bubble window presentation-only and feed it with a shell-ball-local bubble message snapshot contract. The coordinator remains the source of truth for mock/live-ready bubble data, while the bubble window only renders the list, motion state, and scroll behavior without introducing protocol-layer coupling.

**Tech Stack:** React 18, TypeScript, shell-ball window sync/coordinator layer, CSS animations, existing shell-ball contract test suite.

---

## File Map

- Create: `apps/desktop/src/features/shell-ball/shellBall.bubble.ts`
- Create: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleMessage.tsx`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.windowSync.ts`
- Modify: `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`
- Modify: `apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.css`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

## Constraints

- Bubble window only shows message bubbles; no title, caption, input bar, send area, card shell, border, backdrop panel, or window-level framing.
- Bubble window background must remain transparent.
- User messages align right; agent messages align left.
- History scrolls upward; scrollbar stays hidden.
- Top and bottom edges fade messages out with a transparent mask effect.
- New messages enter with a light upward-float plus fade-in motion.
- This iteration should be live-ready but still shell-ball-local: do not add `/packages/protocol` changes.

### Task 1: Define Bubble Message Feed Contract

**Files:**
- Create: `apps/desktop/src/features/shell-ball/shellBall.bubble.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.windowSync.ts`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write the failing contract test**

Add tests asserting that the shell-ball bubble feed exposes a live-ready message model with:
- `id`
- `role` as `user` or `agent`
- `text`
- `createdAt`
- an optional freshness/motion hint for new-message animation

Also assert that the window snapshot shape includes a `bubbleMessages` array.

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because no bubble feed contract exists yet.

- [ ] **Step 3: Write minimal implementation**

Create `apps/desktop/src/features/shell-ball/shellBall.bubble.ts` with:
- `ShellBallBubbleMessageRole`
- `ShellBallBubbleMessage`
- `createShellBallBubbleMessages(...)` or equivalent shell-ball-local mock/live-ready feed helper

Update `apps/desktop/src/features/shell-ball/shellBall.windowSync.ts` so the snapshot type includes `bubbleMessages`.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for the bubble feed contract.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/features/shell-ball/shellBall.bubble.ts apps/desktop/src/features/shell-ball/shellBall.windowSync.ts apps/desktop/src/features/shell-ball/shellBall.contract.test.ts
git commit -m "feat(desktop-shell-ball): add bubble message feed contract"
```

### Task 2: Feed Bubble Messages Through the Coordinator

**Files:**
- Modify: `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`
- Modify: `apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write the failing contract test**

Add tests asserting that:
- the coordinator snapshot now carries `bubbleMessages`
- `ShellBallBubbleWindow` resolves messages from the helper-window snapshot
- the bubble window no longer depends on only `visualState` to render its body

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because the coordinator and bubble window do not move message arrays yet.

- [ ] **Step 3: Write minimal implementation**

Update `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts` to include a shell-ball-local bubble message feed in every snapshot.

Update `apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx` to read `snapshot.bubbleMessages` and pass them into the bubble zone.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for coordinator/bubble-window snapshot coverage.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx apps/desktop/src/features/shell-ball/shellBall.contract.test.ts
git commit -m "feat(desktop-shell-ball): sync bubble messages to helper window"
```

### Task 3: Replace the Placeholder Bubble Zone With a Real Message List

**Files:**
- Create: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleMessage.tsx`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write the failing render test**

Add tests asserting that:
- bubble zone renders message items instead of placeholder shells
- `agent` messages align left
- `user` messages align right
- no title/input/toolbar text is rendered

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because the bubble zone still renders placeholder ellipses.

- [ ] **Step 3: Write minimal implementation**

Create `apps/desktop/src/features/shell-ball/components/ShellBallBubbleMessage.tsx` for one message bubble.

Refactor `apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx` to:
- render a transparent scroll container
- render only message bubbles
- align left/right by `role`
- remove the two placeholder shell bars

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for real message-list rendering.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/features/shell-ball/components/ShellBallBubbleMessage.tsx apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx apps/desktop/src/features/shell-ball/shellBall.contract.test.ts
git commit -m "feat(desktop-shell-ball): render bubble message list"
```

### Task 4: Style the Bubble Window for Transparent History, Hidden Scrollbars, Edge Fade, and New-Message Motion

**Files:**
- Modify: `apps/desktop/src/features/shell-ball/shellBall.css`
- Modify: `apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx`
- Modify: `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`

- [ ] **Step 1: Write the failing style contract test**

Add tests asserting that the bubble styles provide:
- transparent container with no panel/card chrome
- hidden scrollbars
- top/bottom fade mask or equivalent edge fade treatment
- new-message motion classes/data attributes for slight upward-float + fade-in

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: FAIL because the current bubble CSS still targets placeholder shells.

- [ ] **Step 3: Write minimal implementation**

Update `apps/desktop/src/features/shell-ball/shellBall.css` so the bubble window:
- stays transparent
- has no outer box/border/panel
- hides scrollbars
- applies a top/bottom fade mask
- animates new messages with a light upward-float + fade-in

Keep the motion subtle and preserve window sizing behavior.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --dir apps/desktop test:shell-ball`
Expected: PASS for the bubble-window style contract.

- [ ] **Step 5: Commit**

```bash
git add apps/desktop/src/features/shell-ball/shellBall.css apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx apps/desktop/src/features/shell-ball/shellBall.contract.test.ts
git commit -m "feat(desktop-shell-ball): style transparent bubble history window"
```

### Task 5: Verify Bubble Window Behavior End-to-End

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
- bubble window shows only message bubbles
- bubble background remains fully transparent
- user bubbles are right-aligned
- agent bubbles are left-aligned
- older messages scroll upward
- scrollbar stays hidden
- top/bottom edges fade overflowing history
- new incoming message uses subtle upward-float + fade-in

- [ ] **Step 5: Commit only if manual verification required a follow-up fix**

Run: `git status --short`
Expected: clean working tree; if not, commit with:

```bash
git add <fixed-files>
git commit -m "feat(desktop-shell-ball): polish bubble window motion and history"
```
