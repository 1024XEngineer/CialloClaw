import { LogicalPosition, LogicalSize, Window } from "@tauri-apps/api/window";
import { desktopOnboardingEvents } from "@/features/onboarding/onboarding.events";
import { resetOnboardingInteractiveState, setOnboardingIgnoreCursorEvents } from "./onboardingWindow";

export const ONBOARDING_WINDOW_LABEL = "onboarding";

export type OnboardingWindowFrame = {
  height: number;
  width: number;
  x: number;
  y: number;
};

type SyncOnboardingWindowFrameOptions = {
  alwaysOnTop?: boolean;
};

let onboardingWindowHandle: Window | null = null;
let onboardingWindowPromise: Promise<Window> | null = null;

async function waitForOnboardingWindowEvent(eventName: string, timeoutMs: number) {
  const onboardingWindow = await getOrCreateOnboardingWindow();

  return new Promise<void>((resolve, reject) => {
    let timeoutHandle = 0;
    let disposed = false;
    let disposeWindowListener: (() => void) | null = null;

    void onboardingWindow.listen(eventName, () => {
      if (disposed) {
        return;
      }

      disposed = true;
      window.clearTimeout(timeoutHandle);
      disposeWindowListener?.();
      resolve();
    }).then((unlisten) => {
      if (disposed) {
        unlisten();
        return;
      }

      disposeWindowListener = unlisten;
    });

    timeoutHandle = window.setTimeout(() => {
      if (disposed) {
        return;
      }

      disposed = true;
      disposeWindowListener?.();
      reject(new Error(`Timed out waiting for onboarding event: ${eventName}`));
    }, timeoutMs);
  });
}

async function getOrCreateOnboardingWindow() {
  if (onboardingWindowHandle !== null) {
    return onboardingWindowHandle;
  }

  if (onboardingWindowPromise !== null) {
    return onboardingWindowPromise;
  }

  onboardingWindowPromise = (async () => {
    const existingWindow = await Window.getByLabel(ONBOARDING_WINDOW_LABEL);

    if (existingWindow !== null) {
      onboardingWindowHandle = existingWindow;
      return existingWindow;
    }

    const onboardingWindowOptions = {
      title: "CialloClaw Onboarding",
      url: "onboarding.html",
      decorations: false,
      transparent: true,
      alwaysOnTop: true,
      resizable: false,
      skipTaskbar: true,
      shadow: false,
      visible: false,
      focus: false,
      width: 1280,
      height: 720,
    } as const;

    const createdWindow = new Window(ONBOARDING_WINDOW_LABEL, onboardingWindowOptions);
    onboardingWindowHandle = createdWindow;
    await resetOnboardingInteractiveState();
    await setOnboardingIgnoreCursorEvents(true);
    return createdWindow;
  })().finally(() => {
    onboardingWindowPromise = null;
  });

  return onboardingWindowPromise;
}

/**
 * Ensures the onboarding overlay window matches the current monitor frame and
 * stays visible above the active workflow window.
 *
 * @param frame The target logical monitor frame.
 * @param options Window ordering overrides for the current onboarding step.
 */
export async function syncOnboardingWindowFrame(
  frame: OnboardingWindowFrame,
  options: SyncOnboardingWindowFrameOptions = {},
) {
  const onboardingWindow = await getOrCreateOnboardingWindow();
  await resetOnboardingInteractiveState();
  await setOnboardingIgnoreCursorEvents(true);
  await onboardingWindow.setPosition(new LogicalPosition(frame.x, frame.y));
  await onboardingWindow.setSize(new LogicalSize(frame.width, frame.height));
  await onboardingWindow.setFocusable(false);
  await onboardingWindow.setAlwaysOnTop(options.alwaysOnTop ?? true);
}

/**
 * Waits until the onboarding React app is mounted and ready to receive session
 * and presentation payloads.
 */
export function waitForOnboardingWindowReady(timeoutMs: number) {
  return waitForOnboardingWindowEvent(desktopOnboardingEvents.ready, timeoutMs);
}

/**
 * Waits until the onboarding React app has laid out its first card and
 * registered native hit-test regions.
 */
export function waitForOnboardingCardReady(timeoutMs: number) {
  return waitForOnboardingWindowEvent(desktopOnboardingEvents.cardReady, timeoutMs);
}

/**
 * Shows the onboarding window after the frontend reports that its first card is
 * laid out and the native hit-test state is ready.
 */
export async function showOnboardingWindow() {
  const onboardingWindow = await getOrCreateOnboardingWindow();
  await setOnboardingIgnoreCursorEvents(false);
  await onboardingWindow.setFocusable(true);
  await onboardingWindow.show();
}

/**
 * Destroys the onboarding overlay window when the guide is idle.
 */
export async function destroyOnboardingWindow() {
  const onboardingWindow = onboardingWindowHandle ?? await Window.getByLabel(ONBOARDING_WINDOW_LABEL);

  if (onboardingWindow === null) {
    await resetOnboardingInteractiveState();
    onboardingWindowHandle = null;
    return;
  }

  try {
    await resetOnboardingInteractiveState();
    await setOnboardingIgnoreCursorEvents(true);
    await onboardingWindow.destroy();
  } finally {
    await resetOnboardingInteractiveState();
    onboardingWindowHandle = null;
    onboardingWindowPromise = null;
  }
}
