import { Window, getCurrentWindow } from "@tauri-apps/api/window";
import { shellBallWindowLabels } from "./shellBallWindowController";

export const shellBallDashboardTransitionEvents = Object.freeze({
  request: "desktop-shell-ball:dashboard-transition-request",
  complete: "desktop-shell-ball:dashboard-transition-complete",
});

export type ShellBallDashboardTransitionDirection = "open" | "close";

export type ShellBallDashboardTransitionRequest = {
  direction: ShellBallDashboardTransitionDirection;
  requestId?: string;
};

export type ShellBallDashboardTransitionComplete = {
  direction: ShellBallDashboardTransitionDirection;
  requestId?: string;
};

const SHELL_BALL_DASHBOARD_CLOSE_TRANSITION_TIMEOUT_MS = 1_200;

async function getShellBallWindowHandle() {
  const currentWindow = getCurrentWindow();

  if (currentWindow.label === shellBallWindowLabels.ball) {
    return currentWindow;
  }

  return Window.getByLabel(shellBallWindowLabels.ball);
}

function createShellBallDashboardTransitionRequestId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }

  return `shell-ball-transition-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

export async function requestShellBallDashboardOpenTransition() {
  const shellBallWindow = await getShellBallWindowHandle();
  if (shellBallWindow === null) {
    return false;
  }

  const currentWindow = getCurrentWindow();
  const payload = {
    direction: "open",
  } satisfies ShellBallDashboardTransitionRequest;

  if (currentWindow.label === shellBallWindowLabels.ball) {
    await currentWindow.emit(shellBallDashboardTransitionEvents.request, payload);
  } else {
    await currentWindow.emitTo(shellBallWindowLabels.ball, shellBallDashboardTransitionEvents.request, payload);
  }

  return true;
}

export async function requestShellBallDashboardCloseTransition() {
  const shellBallWindow = await getShellBallWindowHandle();
  if (shellBallWindow === null) {
    return false;
  }

  const currentWindow = getCurrentWindow();
  const requestId = createShellBallDashboardTransitionRequestId();

  return new Promise<boolean>((resolve) => {
    let settled = false;
    let cleanup: (() => void) | null = null;
    const timeoutHandle = globalThis.setTimeout(() => {
      if (settled) {
        return;
      }

      settled = true;
      cleanup?.();
      resolve(false);
    }, SHELL_BALL_DASHBOARD_CLOSE_TRANSITION_TIMEOUT_MS);

    function finalize(result: boolean) {
      if (settled) {
        return;
      }

      settled = true;
      globalThis.clearTimeout(timeoutHandle);
      cleanup?.();
      resolve(result);
    }

    void currentWindow
      .listen<ShellBallDashboardTransitionComplete>(shellBallDashboardTransitionEvents.complete, ({ payload }) => {
        if (payload.direction !== "close" || payload.requestId !== requestId || settled) {
          return;
        }

        finalize(true);
      })
      .then((unlisten) => {
        if (settled) {
          unlisten();
          return;
        }

        cleanup = unlisten;

        const payload = {
          direction: "close",
          requestId,
        } satisfies ShellBallDashboardTransitionRequest;

        if (currentWindow.label === shellBallWindowLabels.ball) {
          void currentWindow.emit(shellBallDashboardTransitionEvents.request, payload);
          return;
        }

        void currentWindow.emitTo(shellBallWindowLabels.ball, shellBallDashboardTransitionEvents.request, payload);
      })
      .catch(() => {
        finalize(false);
      });
  });
}
