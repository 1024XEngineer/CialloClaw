import type { ShellBallSelectionSnapshot } from "./selection.types";

/**
 * Compares two selection snapshots while ignoring transient timestamp changes.
 *
 * @param left Previous selection snapshot.
 * @param right Latest selection snapshot.
 * @returns Whether the snapshots represent the same logical selection.
 */
export function areShellBallSelectionSnapshotsEqual(
  left: ShellBallSelectionSnapshot | null,
  right: ShellBallSelectionSnapshot | null,
) {
  if (left === right) {
    return true;
  }

  if (left === null || right === null) {
    return false;
  }

  return (
    left.text === right.text
    && left.source === right.source
    && left.page_context.title === right.page_context.title
    && left.page_context.url === right.page_context.url
    && left.page_context.app_name === right.page_context.app_name
    && left.page_context.window_title === right.page_context.window_title
    && left.page_context.visible_text === right.page_context.visible_text
    && left.page_context.hover_target === right.page_context.hover_target
  );
}

/**
 * Reserves a stable frontend seam for shell-ball selection sensing while the
 * Windows host adapter owns the actual event emission.
 *
 * @returns `null`; host-side listeners now publish selection snapshots.
 */
export function ShellBallSelectionProvider(): null {
  return null;
}
