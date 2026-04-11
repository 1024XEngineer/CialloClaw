import { LogicalPosition, LogicalSize, type Position, type Size } from "@tauri-apps/api/dpi";
import { Window, getCurrentWindow } from "@tauri-apps/api/window";

export const shellBallWindowLabels = Object.freeze({
  ball: "shell-ball",
  bubble: "shell-ball-bubble",
  input: "shell-ball-input",
});

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

export function getShellBallCurrentWindow() {
  return getCurrentWindow();
}

export async function getShellBallWindow(role: ShellBallWindowRole) {
  const currentWindow = getCurrentWindow();
  const label = shellBallWindowLabels[role];

  if (currentWindow.label === label) {
    return currentWindow;
  }

  const windowHandle = await Window.getByLabel(label);

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

export function createShellBallLogicalSize(width: number, height: number) {
  return new LogicalSize(width, height);
}

export function createShellBallLogicalPosition(x: number, y: number) {
  return new LogicalPosition(x, y);
}
