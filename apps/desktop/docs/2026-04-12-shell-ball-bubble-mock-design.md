# Shell Ball Bubble Mock Design

## Goal

Make the shell-ball bubble area feel interactive instead of static by letting the input helper window create local mock conversation bubbles, preserving scrollable history, and introducing a visual dissipation mechanism that fades old bubbles without deleting them.

## Context And Constraints

- This work stays inside the desktop frontend shell-ball helper windows.
- It does not add or change any formal protocol object, JSON-RPC method, task field, or delivery object.
- The single local source of truth remains `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`.
- Bubble, input, and pinned windows must keep synchronizing through the existing helper-window snapshot flow.
- The shell-ball frontend can use local mock data, but it must not masquerade as formal delivery output.

## Current Problems

1. The bubble area is still display-only because the input helper window updates draft text but does not append mock conversation items.
2. The bubble list always snaps to the bottom whenever `bubbleItems` changes, which breaks the "scroll up to read history" experience.
3. Bubble lifecycle types already include `visible`, `fading`, and `hidden`, but the runtime only behaves as `visible`.
4. Pinning currently removes the bubble from the scrollable list immediately because `getShellBallVisibleBubbleItems()` filters out `pinned === true`, while the pinned window can fail to open in the same tick. That produces the user-visible bug where a pinned bubble appears to vanish.

## Desired Experience

### Input To Mock Conversation

- When the input helper window submits a non-empty draft, the shell-ball coordinator appends a local user bubble immediately.
- The coordinator then schedules one or two mock agent bubbles so the user sees a lightweight request -> thinking -> response flow.
- The existing shell-ball visual state machine still drives the shell-ball mascot states, but the bubble stream becomes an actual local interaction log.

### Scrollable History

- The bubble window stays bottom-aligned by default.
- If the user is already near the bottom when a new bubble arrives, the list auto-scrolls to the newest item.
- If the user has scrolled upward to inspect history, new bubble arrivals do not force the list back to the bottom.
- Cosmetic lifecycle updates, such as an old bubble changing from `visible` to `fading`, must not trigger unwanted scroll jumps.

### Dissipation

- Unpinned bubbles start as `visible`.
- After a short freshness window they transition to `fading`.
- Fading changes only presentation: opacity, scale, blur, and emphasis reduce progressively, but the bubble remains in history.
- Hovering or actively scrolling the bubble area pauses automatic fading so the user can read without the UI changing underneath them.
- That pause signal is produced by the bubble helper window and sent back to the coordinator through the existing helper-window event channel, rather than by adding a new store.
- Pinned bubbles do not auto-fade.

### Pinning

- Pinning must no longer make a bubble disappear from the UI.
- A pinned bubble remains visible in the main history list with pinned styling and an `Unpin` action.
- The detached pinned helper window still opens and shows the same bubble as a fixed companion view.
- If the pinned helper window opens late, the history list still preserves the bubble, so the user never experiences a gap.

## Architecture

### Local State Ownership

Keep `useShellBallCoordinator.ts` as the only mutable owner of shell-ball bubble history. It already synchronizes helper-window snapshots to the bubble and input windows, so extending that state is the smallest change that preserves the current three-window architecture.

The coordinator gains small, frontend-only bubble lifecycle metadata:

- bubble creation timestamp and stable ordering remain on the existing `bubble.created_at`
- local lifecycle timing determines when a bubble changes from `visible` to `fading`
- pinned bubbles are exempt from fading

This is intentionally UI-local metadata and does not touch the project's formal task-centric model.

### Mock Bubble Production

Submitting from the input helper window should call a coordinator append flow instead of only advancing shell-ball animation state. The flow is:

1. validate non-empty draft
2. append a user bubble
3. clear the input draft
4. preserve the current shell-ball state transition behavior
5. schedule mock agent follow-up bubbles

The follow-up bubbles can be deterministic and lightweight, for example a status bubble followed by a result bubble. The text can come from a small local helper so tests stay stable.

### Bubble Visibility Model

The bubble window should stop treating `pinned` as synonymous with "not visible in history". Instead:

- `hidden` means absent from the main bubble history
- `pinned` means the bubble also renders in the pinned helper window
- `fading` is a desktop lifecycle style, not a removal event

That keeps the main history honest and fixes the current pin-disappear bug.

Because the current `bubbleRegion` visibility model derives from the same visible-history list, this change also intentionally updates the bubble window interaction semantics: pinned bubbles that remain in history still count as visible history content for click-through decisions.

## Files To Touch

- `apps/desktop/src/features/shell-ball/useShellBallCoordinator.ts`
  - own mock append flow, lifecycle timers, and pin/unpin behavior
- `apps/desktop/src/features/shell-ball/ShellBallBubbleWindow.tsx`
  - pass state-aware pin and unpin actions into the main history list
- `apps/desktop/src/features/shell-ball/shellBall.windowSync.ts`
  - stop filtering pinned bubbles out of the main history list
- `apps/desktop/src/features/shell-ball/components/ShellBallBubbleZone.tsx`
  - add bottom-proximity aware auto-scroll and scroll/hover pause hooks
- `apps/desktop/src/features/shell-ball/components/ShellBallBubbleMessage.tsx`
  - support pinned-state controls and lifecycle data attributes
- `apps/desktop/src/features/shell-ball/shellBallBubbleDesktop.ts`
  - extend local desktop bubble metadata helpers if needed
- `apps/desktop/src/features/shell-ball/shellBall.css`
  - add fading, pinned, and scroll-state styling
- `apps/desktop/src/features/shell-ball/shellBall.contract.test.ts`
  - drive all behavior changes through tests first

## Testing Strategy

- Add contract tests for submitting input into mock bubble history.
- Add contract tests for bottom-only auto-scroll behavior.
- Add contract tests for dissipation transitions and pause behavior.
- Add a regression test proving pinned bubbles remain visible in main history while also participating in pinned-window behavior.
- Run `pnpm --dir apps/desktop test:shell-ball` after each feature slice.

## Commit Strategy

- Commit docs with `docs(bubble): ...`.
- Commit each completed feature slice with `feat(bubble): ...`.
- Keep slices small enough that each commit corresponds to one visible behavior change.
- These commit checkpoints are intentionally part of the workflow because the user explicitly requested per-slice commits for this feature.
