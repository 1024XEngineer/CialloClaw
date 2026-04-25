import { Window, getCurrentWindow, monitorFromPoint } from "@tauri-apps/api/window";
import {
  destroyOnboardingWindow,
  showOnboardingWindow,
  syncOnboardingWindowFrame,
  waitForOnboardingCardReady,
  waitForOnboardingWindowReady,
} from "@/platform/onboardingWindowController";
import { shellBallWindowLabels } from "@/platform/shellBallWindowController";
import { loadStoredValue, removeStoredValue, saveStoredValue } from "@/platform/storage";
import { desktopOnboardingEvents, desktopOnboardingLocalEvents } from "./onboarding.events";

export type DesktopOnboardingStep =
  | "welcome"
  | "shell_ball_intro"
  | "shell_ball_hold_voice"
  | "shell_ball_double_click"
  | "dashboard_overview"
  | "tray_hint"
  | "control_panel_api_key"
  | "done";

export type DesktopOnboardingSource = "first_launch" | "manual";

export type DesktopOnboardingStatus = {
  first_seen_at: string | null;
  completed: boolean;
  completed_at: string | null;
  skipped: boolean;
  skipped_at: string | null;
};

export type DesktopOnboardingSession = {
  isOpen: boolean;
  source: DesktopOnboardingSource;
  step: DesktopOnboardingStep;
  started_at: string;
};

export type DesktopOnboardingPlacement = "center" | "top-left" | "top-right" | "bottom-left" | "bottom-right";

export type DesktopOnboardingPresentationRect = {
  height: number;
  width: number;
  x: number;
  y: number;
};

export type DesktopOnboardingPresentation = {
  highlights: DesktopOnboardingPresentationRect[];
  monitorFrame: DesktopOnboardingPresentationRect;
  placement: DesktopOnboardingPlacement;
  step: DesktopOnboardingStep;
  windowLabel: "shell-ball" | "dashboard" | "control-panel";
};

export type DesktopOnboardingActionRequest = {
  targetWindow: "shell-ball" | "dashboard" | "control-panel";
  type: "open_control_panel" | "open_dashboard" | "show_shell_ball" | "close_dashboard" | "close_control_panel";
};

export type DesktopOnboardingLoadingState = {
  message: string;
  windowLabel: "shell-ball" | "dashboard" | "control-panel";
};

const DESKTOP_ONBOARDING_STATUS_KEY = "cialloclaw.desktop.onboarding.status";
const DESKTOP_ONBOARDING_SESSION_KEY = "cialloclaw.desktop.onboarding.session";
const DESKTOP_ONBOARDING_PRESENTATION_KEY = "cialloclaw.desktop.onboarding.presentation";
const DESKTOP_ONBOARDING_READY_TIMEOUT_MS = 6_000;

const DESKTOP_ONBOARDING_WINDOW_LABELS = [
  shellBallWindowLabels.ball,
  "dashboard",
  "control-panel",
  "onboarding",
] as const;

let desktopOnboardingLoadingState: DesktopOnboardingLoadingState | null = null;

async function buildDefaultWelcomePresentation(windowLabel: DesktopOnboardingPresentation["windowLabel"]) {
  const currentWindow = getCurrentWindow();
  const outerPosition = await currentWindow.outerPosition();
  const outerSize = await currentWindow.outerSize();
  const monitor = await monitorFromPoint(outerPosition.x, outerPosition.y);

  const monitorPosition = monitor?.position.toLogical(monitor.scaleFactor) ?? outerPosition.toLogical(await currentWindow.scaleFactor());
  const monitorSize = monitor?.size.toLogical(monitor.scaleFactor) ?? outerSize.toLogical(await currentWindow.scaleFactor());

  return {
    highlights: [] as DesktopOnboardingPresentationRect[],
    monitorFrame: {
      x: monitorPosition.x,
      y: monitorPosition.y,
      width: monitorSize.width,
      height: monitorSize.height,
    },
    placement: "center",
    step: "welcome",
    windowLabel,
  } satisfies DesktopOnboardingPresentation;
}

function createDefaultDesktopOnboardingStatus(): DesktopOnboardingStatus {
  return {
    first_seen_at: null,
    completed: false,
    completed_at: null,
    skipped: false,
    skipped_at: null,
  };
}

function dispatchLocalSessionChanged(session: DesktopOnboardingSession | null) {
  window.dispatchEvent(
    new CustomEvent<DesktopOnboardingSession | null>(desktopOnboardingLocalEvents.sessionChanged, {
      detail: session,
    }),
  );
}

function dispatchLocalPresentationChanged(presentation: DesktopOnboardingPresentation | null) {
  window.dispatchEvent(
    new CustomEvent<DesktopOnboardingPresentation | null>(desktopOnboardingLocalEvents.presentationChanged, {
      detail: presentation,
    }),
  );
}

function dispatchLocalActionRequested(action: DesktopOnboardingActionRequest) {
  window.dispatchEvent(
    new CustomEvent<DesktopOnboardingActionRequest>(desktopOnboardingLocalEvents.actionRequested, {
      detail: action,
    }),
  );
}

function dispatchLocalLoadingChanged(loadingState: DesktopOnboardingLoadingState | null) {
  window.dispatchEvent(
    new CustomEvent<DesktopOnboardingLoadingState | null>(desktopOnboardingLocalEvents.loadingChanged, {
      detail: loadingState,
    }),
  );
}

async function broadcastSession(session: DesktopOnboardingSession | null) {
  dispatchLocalSessionChanged(session);

  const currentWindowLabel = getCurrentWindow().label;
  await Promise.all(
    DESKTOP_ONBOARDING_WINDOW_LABELS.map(async (label) => {
      if (label === currentWindowLabel) {
        return;
      }

      try {
        const targetWindow = await Window.getByLabel(label);
        if (targetWindow === null) {
          return;
        }

        await targetWindow.emit(desktopOnboardingEvents.sessionChanged, session);
      } catch (error) {
        console.warn("desktop onboarding session sync failed", error);
      }
    }),
  );
}

async function broadcastPresentation(presentation: DesktopOnboardingPresentation | null) {
  dispatchLocalPresentationChanged(presentation);

  const currentWindowLabel = getCurrentWindow().label;
  await Promise.all(
    DESKTOP_ONBOARDING_WINDOW_LABELS.map(async (label) => {
      if (label === currentWindowLabel) {
        return;
      }

      try {
        const targetWindow = await Window.getByLabel(label);
        if (targetWindow === null) {
          return;
        }

        await targetWindow.emit(desktopOnboardingEvents.presentationChanged, presentation);
      } catch (error) {
        console.warn("desktop onboarding presentation sync failed", error);
      }
    }),
  );
}

async function broadcastLoading(loadingState: DesktopOnboardingLoadingState | null) {
  dispatchLocalLoadingChanged(loadingState);

  const currentWindowLabel = getCurrentWindow().label;
  await Promise.all(
    DESKTOP_ONBOARDING_WINDOW_LABELS.map(async (label) => {
      if (label === currentWindowLabel) {
        return;
      }

      try {
        const targetWindow = await Window.getByLabel(label);
        if (targetWindow === null) {
          return;
        }

        await targetWindow.emit(desktopOnboardingEvents.loadingChanged, loadingState);
      } catch (error) {
        console.warn("desktop onboarding loading sync failed", error);
      }
    }),
  );
}

export async function requestDesktopOnboardingAction(action: DesktopOnboardingActionRequest) {
  dispatchLocalActionRequested(action);

  const currentWindowLabel = getCurrentWindow().label;
  await Promise.all(
    DESKTOP_ONBOARDING_WINDOW_LABELS.map(async (label) => {
      if (label === currentWindowLabel) {
        return;
      }

      try {
        const targetWindow = await Window.getByLabel(label);
        if (targetWindow === null) {
          return;
        }

        await targetWindow.emit(desktopOnboardingEvents.actionRequested, action);
      } catch (error) {
        console.warn("desktop onboarding action sync failed", error);
      }
    }),
  );
}

export function loadDesktopOnboardingStatus(): DesktopOnboardingStatus {
  return {
    ...createDefaultDesktopOnboardingStatus(),
    ...(loadStoredValue<DesktopOnboardingStatus>(DESKTOP_ONBOARDING_STATUS_KEY) ?? {}),
  };
}

export function saveDesktopOnboardingStatus(status: DesktopOnboardingStatus) {
  saveStoredValue(DESKTOP_ONBOARDING_STATUS_KEY, status);
}

export function loadDesktopOnboardingSession() {
  return loadStoredValue<DesktopOnboardingSession>(DESKTOP_ONBOARDING_SESSION_KEY);
}

export function loadDesktopOnboardingPresentation() {
  return loadStoredValue<DesktopOnboardingPresentation>(DESKTOP_ONBOARDING_PRESENTATION_KEY);
}

export function loadDesktopOnboardingLoadingState() {
  return desktopOnboardingLoadingState;
}

export function shouldAutoStartDesktopOnboarding() {
  const status = loadDesktopOnboardingStatus();
  return !status.completed && !status.skipped;
}

export async function setDesktopOnboardingLoadingState(loadingState: DesktopOnboardingLoadingState | null) {
  desktopOnboardingLoadingState = loadingState;
  await broadcastLoading(loadingState);
}

export async function setDesktopOnboardingSession(session: DesktopOnboardingSession | null) {
  if (session === null) {
    removeStoredValue(DESKTOP_ONBOARDING_SESSION_KEY);
    await setDesktopOnboardingPresentation(null);
  } else {
    saveStoredValue(DESKTOP_ONBOARDING_SESSION_KEY, session);
  }

  await broadcastSession(session);
}

export async function setDesktopOnboardingPresentation(presentation: DesktopOnboardingPresentation | null) {
  if (presentation === null) {
    removeStoredValue(DESKTOP_ONBOARDING_PRESENTATION_KEY);
    await destroyOnboardingWindow();
  } else {
    saveStoredValue(DESKTOP_ONBOARDING_PRESENTATION_KEY, presentation);
    await syncOnboardingWindowFrame(presentation.monitorFrame, {
      alwaysOnTop: true,
    });
  }

  await broadcastPresentation(presentation);
}

export async function startDesktopOnboarding(
  source: DesktopOnboardingSource,
  preferredWindowLabel?: DesktopOnboardingPresentation["windowLabel"],
) {
  const now = new Date().toISOString();
  const status = loadDesktopOnboardingStatus();

  saveDesktopOnboardingStatus({
    ...status,
    first_seen_at: status.first_seen_at ?? now,
  });

  const session: DesktopOnboardingSession = {
    isOpen: true,
    source,
    step: "welcome",
    started_at: now,
  };

  const currentWindowLabel = getCurrentWindow().label;
  const windowLabel: DesktopOnboardingPresentation["windowLabel"] =
    preferredWindowLabel ??
    (currentWindowLabel === "dashboard" || currentWindowLabel === "control-panel" ? currentWindowLabel : "shell-ball");
  const welcomePresentation = await buildDefaultWelcomePresentation(windowLabel);

  await setDesktopOnboardingLoadingState({
    message: "正在打开引导...",
    windowLabel,
  });

  try {
    if (welcomePresentation !== null) {
      const readyPromise = waitForOnboardingWindowReady(DESKTOP_ONBOARDING_READY_TIMEOUT_MS);
      await syncOnboardingWindowFrame(welcomePresentation.monitorFrame, {
        alwaysOnTop: true,
      });
      await readyPromise;

      const cardReadyPromise = waitForOnboardingCardReady(DESKTOP_ONBOARDING_READY_TIMEOUT_MS);
      await setDesktopOnboardingSession(session);
      await setDesktopOnboardingPresentation(welcomePresentation);
      while (true) {
        const result = await Promise.race([
          cardReadyPromise.then(() => "ready" as const),
          new Promise<"retry">((resolve) => {
            window.setTimeout(() => resolve("retry"), 250);
          }),
        ]);

        if (result === "ready") {
          break;
        }

        await broadcastSession(session);
        await broadcastPresentation(welcomePresentation);
      }
      await showOnboardingWindow();
    } else {
      await setDesktopOnboardingSession(session);
    }
  } catch (error) {
    console.warn("desktop onboarding launch failed", error);
    removeStoredValue(DESKTOP_ONBOARDING_SESSION_KEY);
    removeStoredValue(DESKTOP_ONBOARDING_PRESENTATION_KEY);
    await destroyOnboardingWindow();
    await broadcastSession(null);
    await broadcastPresentation(null);
    return null;
  } finally {
    await setDesktopOnboardingLoadingState(null);
  }

  return session;
}

export async function advanceDesktopOnboarding(step: DesktopOnboardingStep) {
  const currentSession = loadDesktopOnboardingSession();
  if (currentSession === null) {
    return null;
  }

  const nextSession: DesktopOnboardingSession = {
    ...currentSession,
    step,
  };

  await setDesktopOnboardingSession(nextSession);
  return nextSession;
}

export async function completeDesktopOnboarding() {
  const now = new Date().toISOString();
  const currentStatus = loadDesktopOnboardingStatus();

  saveDesktopOnboardingStatus({
    ...currentStatus,
    first_seen_at: currentStatus.first_seen_at ?? now,
    completed: true,
    completed_at: now,
  });

  await setDesktopOnboardingSession(null);
}

export async function skipDesktopOnboarding() {
  const now = new Date().toISOString();
  const currentStatus = loadDesktopOnboardingStatus();

  saveDesktopOnboardingStatus({
    ...currentStatus,
    first_seen_at: currentStatus.first_seen_at ?? now,
    skipped: true,
    skipped_at: now,
  });

  await setDesktopOnboardingSession(null);
}
