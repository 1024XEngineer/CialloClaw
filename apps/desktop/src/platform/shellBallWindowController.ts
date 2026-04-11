import { LogicalPosition, LogicalSize, type Position, type Size } from "@tauri-apps/api/dpi";
import { Window, getCurrentWindow } from "@tauri-apps/api/window";

export const shellBallWindowLabels = Object.freeze({
  ball: "shell-ball",
  bubble: "shell-ball-bubble",
  input: "shell-ball-input",
});

export const shellBallPinnedBubbleWindowLabelPrefix = "shell-ball-bubble-pinned-";

export const SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME = Object.freeze({
  width: 360,
  height: 140,
});

export const SHELL_BALL_PINNED_BUBBLE_WINDOW_ANCHOR_OFFSET_PX = 24;

export const shellBallWindowPermissions = Object.freeze([
  "core:window:allow-show",
  "core:window:allow-hide",
  "core:window:allow-set-position",
  "core:window:allow-set-size",
  "core:window:allow-set-size-constraints",
  "core:window:allow-set-focus",
  "core:window:allow-set-focusable",
  "core:window:allow-set-ignore-cursor-events",
  "core:window:allow-set-always-on-top",
  "core:window:allow-start-dragging",
] as const);

export type ShellBallWindowRole = keyof typeof shellBallWindowLabels;

export function getShellBallPinnedBubbleWindowLabel(bubbleId: string) {
  return `${shellBallPinnedBubbleWindowLabelPrefix}${bubbleId}`;
}

export function isShellBallPinnedBubbleWindowLabel(label: string) {
  return label.startsWith(shellBallPinnedBubbleWindowLabelPrefix);
}

export function getShellBallPinnedBubbleIdFromLabel(label: string) {
  if (!isShellBallPinnedBubbleWindowLabel(label)) {
    return null;
  }

  return label.slice(shellBallPinnedBubbleWindowLabelPrefix.length);
}

export function getShellBallPinnedBubbleWindowAnchor(input: {
  bubbleAnchor: {
    x: number;
    y: number;
  };
}) {
  return {
    x: input.bubbleAnchor.x + SHELL_BALL_PINNED_BUBBLE_WINDOW_ANCHOR_OFFSET_PX,
    y: input.bubbleAnchor.y + SHELL_BALL_PINNED_BUBBLE_WINDOW_ANCHOR_OFFSET_PX,
  };
}

export function getShellBallCurrentWindow() {
  return getCurrentWindow();
}

export async function focusShellBallCurrentWindow() {
  await getCurrentWindow().setFocus();
}

async function getShellBallWindowByLabel(label: string) {
  const currentWindow = getCurrentWindow();

  if (currentWindow.label === label) {
    return currentWindow;
  }

  return Window.getByLabel(label);
}

export async function getShellBallWindow(role: ShellBallWindowRole) {
  const label = shellBallWindowLabels[role];

  const windowHandle = await getShellBallWindowByLabel(label);

  if (windowHandle === null) {
    throw new Error(`Shell-ball window not found: ${label}`);
  }

  return windowHandle;
}

export async function showShellBallWindow(role: ShellBallWindowRole) {
  const windowHandle = await getShellBallWindow(role);
  await windowHandle.show();
}

export async function hideShellBallWindow(role: ShellBallWindowRole) {
  const windowHandle = await getShellBallWindow(role);
  await windowHandle.hide();
}

export async function setShellBallWindowPosition(role: ShellBallWindowRole, position: Position | LogicalPosition) {
  const windowHandle = await getShellBallWindow(role);
  await windowHandle.setPosition(position);
}

export async function setShellBallWindowSize(role: ShellBallWindowRole, size: Size | LogicalSize) {
  const windowHandle = await getShellBallWindow(role);
  await windowHandle.setSize(size);
}

export async function setShellBallWindowFocusable(role: ShellBallWindowRole, focusable: boolean) {
  const windowHandle = await getShellBallWindow(role);
  await windowHandle.setFocusable(focusable);
}

export async function setShellBallWindowIgnoreCursorEvents(role: ShellBallWindowRole, ignore: boolean) {
  const windowHandle = await getShellBallWindow(role);
  await windowHandle.setIgnoreCursorEvents(ignore);
}

export async function startShellBallWindowDragging() {
  await getCurrentWindow().startDragging();
}

export async function openShellBallPinnedBubbleWindow(input: {
  bubbleId: string;
  position: {
    x: number;
    y: number;
  };
  size?: {
    width: number;
    height: number;
  };
}) {
  const label = getShellBallPinnedBubbleWindowLabel(input.bubbleId);
  const size = input.size ?? SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME;
  const existingWindow = await getShellBallWindowByLabel(label);

  if (existingWindow !== null) {
    await existingWindow.setPosition(createShellBallLogicalPosition(input.position.x, input.position.y));
    await existingWindow.setSize(createShellBallLogicalSize(size.width, size.height));
    await existingWindow.show();
    return label;
  }

  const pinnedWindowOptions = {
    title: "CialloClaw Shell Ball Bubble",
    url: "shell-ball-bubble-pinned.html",
    x: input.position.x,
    y: input.position.y,
    width: size.width,
    height: size.height,
    focus: false,
    visible: true,
    transparent: true,
    decorations: false,
    alwaysOnTop: true,
    resizable: false,
    skipTaskbar: true,
    shadow: false,
  };

  new Window(label, pinnedWindowOptions);

  return label;
}

export async function closeShellBallPinnedBubbleWindow(bubbleId: string) {
  const windowHandle = await getShellBallWindowByLabel(getShellBallPinnedBubbleWindowLabel(bubbleId));

  if (windowHandle === null) {
    return;
  }

  await windowHandle.destroy();
}

export async function emitToShellBallWindowLabel<T>(label: string, event: string, payload?: T) {
  const windowHandle = await getShellBallWindowByLabel(label);

  if (windowHandle === null) {
    return;
  }

  await getCurrentWindow().emitTo(label, event, payload);
}

export function createShellBallLogicalSize(width: number, height: number) {
  return new LogicalSize(width, height);
}

export function createShellBallLogicalPosition(x: number, y: number) {
  return new LogicalPosition(x, y);
}
