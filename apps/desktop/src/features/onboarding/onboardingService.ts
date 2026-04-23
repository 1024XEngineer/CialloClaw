import { Window, getCurrentWindow } from "@tauri-apps/api/window";
import { hideOnboardingWindow, syncOnboardingWindowFrame } from "@/platform/onboardingWindowController";
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
  type: "open_control_panel" | "open_dashboard";
};

const DESKTOP_ONBOARDING_STATUS_KEY = "cialloclaw.desktop.onboarding.status";
const DESKTOP_ONBOARDING_SESSION_KEY = "cialloclaw.desktop.onboarding.session";
const DESKTOP_ONBOARDING_PRESENTATION_KEY = "cialloclaw.desktop.onboarding.presentation";

const DESKTOP_ONBOARDING_WINDOW_LABELS = [
  shellBallWindowLabels.ball,
  "dashboard",
  "control-panel",
  "onboarding",
] as const;

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

export function shouldAutoStartDesktopOnboarding() {
  const status = loadDesktopOnboardingStatus();
  return !status.completed && !status.skipped;
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
    await hideOnboardingWindow();
  } else {
    saveStoredValue(DESKTOP_ONBOARDING_PRESENTATION_KEY, presentation);
    await syncOnboardingWindowFrame(presentation.monitorFrame);
  }

  await broadcastPresentation(presentation);
}

export async function startDesktopOnboarding(source: DesktopOnboardingSource) {
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

  await setDesktopOnboardingSession(session);
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
