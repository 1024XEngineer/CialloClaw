import { invoke } from "@tauri-apps/api/core";
import { getCurrentWindow, LogicalPosition, LogicalSize } from "@tauri-apps/api/window";
import type { ShellBallSelectionSnapshot } from "@/features/shell-ball/selection/selection.types";

export type ShellBallMousePosition = {
  client_x: number;
  client_y: number;
};

export type ShellBallInteractiveRect = {
  height: number;
  width: number;
  x: number;
  y: number;
};

export type ShellBallWindowBounds = {
  height: number;
  width: number;
};

export function isTauriWindowEnvironment() {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;
}

function getShellBallWindow() {
  if (!isTauriWindowEnvironment()) {
    return null;
  }

  return getCurrentWindow();
}

export async function setShellBallAlwaysOnTop(alwaysOnTop: boolean) {
  const currentWindow = getShellBallWindow();
  if (!currentWindow) {
    return;
  }

  await currentWindow.setAlwaysOnTop(alwaysOnTop);
}

export async function setShellBallShadow(enabled: boolean) {
  const currentWindow = getShellBallWindow();
  if (!currentWindow) {
    return;
  }

  await currentWindow.setShadow(enabled);
}

/**
 * Toggles shell-ball window click-through at the native host level.
 *
 * On Windows we optionally forward native mouse move messages while the window
 * is ignoring cursor events so the React layer can restore interactivity as
 * soon as the pointer re-enters a true hotspot.
 */
export async function setShellBallIgnoreCursorEvents(ignore: boolean, forward = true) {
  const currentWindow = getShellBallWindow();
  if (!currentWindow) {
    return;
  }

  try {
    await invoke("shell_ball_set_ignore_cursor_events", {
      forward,
      ignore,
    });
  } catch {
    await currentWindow.setIgnoreCursorEvents(ignore);
  }
}

/**
 * Reads the current native cursor position once so shell-ball can sync its
 * initial click-through state before the first forwarded mousemove arrives.
 */
export async function getShellBallMousePosition() {
  if (!isTauriWindowEnvironment()) {
    return null;
  }

  return invoke<ShellBallMousePosition | null>("shell_ball_get_mouse_position");
}

/**
 * Reports the current shell-ball interactive hotspot rectangles to the native
 * host using physical client coordinates relative to the shell-ball window.
 */
export async function setShellBallInteractiveRegions(regions: ShellBallInteractiveRect[]) {
  if (!isTauriWindowEnvironment()) {
    return;
  }

  await invoke("shell_ball_set_interactive_regions", {
    regions,
  });
}

/**
 * Locks native shell-ball hit-testing to stay interactive while a mascot press
 * sequence is active.
 */
export async function setShellBallPressLock(locked: boolean) {
  if (!isTauriWindowEnvironment()) {
    return;
  }

  await invoke("shell_ball_set_press_lock", {
    locked,
  });
}

/**
 * Reads the current native selection snapshot captured by the host platform
 * adapter.
 *
 * @returns The latest native selection snapshot, or `null` when no selection is
 *          available.
 */
export async function readShellBallSelectionSnapshot() {
  if (!isTauriWindowEnvironment()) {
    return null;
  }

  const { invoke } = await import("@tauri-apps/api/core");
  return invoke<ShellBallSelectionSnapshot | null>("shell_ball_read_selection_snapshot");
}

export async function startShellBallDragging() {
  const currentWindow = getShellBallWindow();
  if (!currentWindow) {
    return;
  }

  await currentWindow.startDragging();
}

export async function syncShellBallWindowBounds(nextBounds: ShellBallWindowBounds, previousBounds: ShellBallWindowBounds | null) {
  const currentWindow = getShellBallWindow();
  if (!currentWindow) {
    return nextBounds;
  }

  const width = Math.max(160, Math.ceil(nextBounds.width));
  const height = Math.max(180, Math.ceil(nextBounds.height));

  if (previousBounds && previousBounds.width !== width) {
    const scaleFactor = await currentWindow.scaleFactor();
    const currentPosition = await currentWindow.outerPosition();
    const logicalX = currentPosition.x / scaleFactor;
    const logicalY = currentPosition.y / scaleFactor;
    const deltaX = (width - previousBounds.width) / 2;
    await currentWindow.setPosition(new LogicalPosition(logicalX - deltaX, logicalY));
  }

  await currentWindow.setSize(new LogicalSize(width, height));
  return { width, height };
}
