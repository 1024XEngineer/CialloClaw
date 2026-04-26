import { invoke } from "@tauri-apps/api/core";
import { LogicalPosition, LogicalSize, Window } from "@tauri-apps/api/window";
import { desktopOnboardingEvents } from "@/features/onboarding/onboarding.events";
import { resetOnboardingInteractiveState, setOnboardingIgnoreCursorEvents } from "./onboardingWindow";

export const ONBOARDING_WINDOW_LABEL = "onboarding";
const ONBOARDING_CARD_WINDOW_WIDTH = 460;
const ONBOARDING_CARD_WINDOW_HEIGHT = 340;
const ONBOARDING_CARD_WINDOW_MARGIN = 24;

type OnboardingWindowPlacement = "center" | "top-left" | "top-right" | "bottom-left" | "bottom-right";

export type OnboardingWindowFrame = {
  height: number;
  width: number;
  x: number;
  y: number;
};

type SyncOnboardingWindowFrameOptions = {
  alwaysOnTop?: boolean;
  placement?: OnboardingWindowPlacement;
};

let onboardingWindowHandle: Window | null = null;
let onboardingWindowPromise: Promise<Window> | null = null;

async function waitForOnboardingWindowHandle(timeoutMs: number) {
  const startTime = Date.now();

  while (Date.now() - startTime < timeoutMs) {
    const windowHandle = await Window.getByLabel(ONBOARDING_WINDOW_LABEL);

    if (windowHandle !== null) {
      onboardingWindowHandle = windowHandle;
      return windowHandle;
    }

    await new Promise<void>((resolve) => {
      window.setTimeout(resolve, 50);
    });
  }

  throw new Error("Timed out waiting for onboarding window handle.");
}

async function waitForOnboardingWindowEvent(eventName: string, timeoutMs: number) {
  const onboardingWindow = await getOrCreateOnboardingWindow();
  let timeoutHandle = 0;
  let disposed = false;
  let resolveEvent: (() => void) | null = null;
  let rejectEvent: ((error: Error) => void) | null = null;

  const eventPromise = new Promise<void>((resolve, reject) => {
    resolveEvent = resolve;
    rejectEvent = reject;
  });

  const disposeWindowListener = await onboardingWindow.listen(eventName, () => {
    if (disposed) {
      return;
    }

    disposed = true;
    window.clearTimeout(timeoutHandle);
    void disposeWindowListener();
    resolveEvent?.();
  });

  timeoutHandle = window.setTimeout(() => {
    if (disposed) {
      return;
    }

    disposed = true;
    void disposeWindowListener();
    rejectEvent?.(new Error(`Timed out waiting for onboarding event: ${eventName}`));
  }, timeoutMs);

  return eventPromise;
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

    await invoke("desktop_open_or_focus_onboarding");
    const createdWindow = await waitForOnboardingWindowHandle(10_000);
    await resetOnboardingInteractiveState();
    await setOnboardingIgnoreCursorEvents(false);
    return createdWindow;
  })().finally(() => {
    onboardingWindowPromise = null;
  });

  return onboardingWindowPromise;
}

function resolveOnboardingCardWindowFrame(frame: OnboardingWindowFrame, placement: OnboardingWindowPlacement) {
  const margin = Math.min(ONBOARDING_CARD_WINDOW_MARGIN, Math.max(0, Math.floor(Math.min(frame.width, frame.height) / 8)));
  const width = Math.max(320, Math.min(ONBOARDING_CARD_WINDOW_WIDTH, frame.width - margin * 2));
  const height = Math.max(260, Math.min(ONBOARDING_CARD_WINDOW_HEIGHT, frame.height - margin * 2));

  const left = frame.x + margin;
  const right = frame.x + frame.width - width - margin;
  const top = frame.y + margin;
  const bottom = frame.y + frame.height - height - margin;

  switch (placement) {
    case "top-left":
      return { x: left, y: top, width, height } satisfies OnboardingWindowFrame;
    case "top-right":
      return { x: right, y: top, width, height } satisfies OnboardingWindowFrame;
    case "bottom-left":
      return { x: left, y: bottom, width, height } satisfies OnboardingWindowFrame;
    case "bottom-right":
      return { x: right, y: bottom, width, height } satisfies OnboardingWindowFrame;
    case "center":
    default:
      return {
        x: frame.x + (frame.width - width) / 2,
        y: frame.y + (frame.height - height) / 2,
        width,
        height,
      } satisfies OnboardingWindowFrame;
  }
}

function buildOnboardingWindowUrl(input: {
  monitorFrame: OnboardingWindowFrame;
  placement: OnboardingWindowPlacement;
  source: string;
  startedAt: string;
  step: string;
  windowLabel: string;
}) {
  const params = new URLSearchParams({
    monitor_height: String(input.monitorFrame.height),
    monitor_width: String(input.monitorFrame.width),
    monitor_x: String(input.monitorFrame.x),
    monitor_y: String(input.monitorFrame.y),
    placement: input.placement,
    source: input.source,
    started_at: input.startedAt,
    step: input.step,
    window_label: input.windowLabel,
  });

  return `onboarding.html?${params.toString()}`;
}

/**
 * Recreates the onboarding card window with enough URL state to render the
 * first guide card without waiting for cross-window event hydration.
 *
 * @param input Initial session and placement state for the new card window.
 */
export async function recreateOnboardingWindow(input: {
  monitorFrame: OnboardingWindowFrame;
  placement: OnboardingWindowPlacement;
  source: string;
  startedAt: string;
  step: string;
  windowLabel: string;
}) {
  const cardFrame = resolveOnboardingCardWindowFrame(input.monitorFrame, input.placement);
  const url = buildOnboardingWindowUrl(input);

  await invoke("desktop_recreate_onboarding", {
    height: cardFrame.height,
    url,
    width: cardFrame.width,
    x: cardFrame.x,
    y: cardFrame.y,
  });

  onboardingWindowHandle = await waitForOnboardingWindowHandle(10_000);
  await resetOnboardingInteractiveState();
  await setOnboardingIgnoreCursorEvents(false);
}

/**
 * Moves the dedicated onboarding card window near the current guide target and
 * keeps it above the active workflow window.
 *
 * @param frame The target logical monitor frame used to place the card window.
 * @param options Window ordering overrides for the current onboarding step.
 */
export async function syncOnboardingWindowFrame(
  frame: OnboardingWindowFrame,
  options: SyncOnboardingWindowFrameOptions = {},
) {
  const onboardingWindow = await getOrCreateOnboardingWindow();
  const cardFrame = resolveOnboardingCardWindowFrame(frame, options.placement ?? "center");
  await resetOnboardingInteractiveState();
  // The onboarding surface is now a normal card-sized window, so it must keep
  // receiving pointer events instead of using fullscreen click-through regions.
  await setOnboardingIgnoreCursorEvents(false);
  await onboardingWindow.setPosition(new LogicalPosition(cardFrame.x, cardFrame.y));
  await onboardingWindow.setSize(new LogicalSize(cardFrame.width, cardFrame.height));
  await onboardingWindow.setFocusable(true);
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
 * Waits until the onboarding React app has laid out its first card.
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
  // Keep the card-sized onboarding window as a normal interactive surface;
  // native promotion only handles z-order and first-frame visibility.
  await setOnboardingIgnoreCursorEvents(false);
  await onboardingWindow.setFocusable(true);
  await onboardingWindow.setAlwaysOnTop(true);
  await invoke("desktop_promote_onboarding");
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
    await setOnboardingIgnoreCursorEvents(false);
    await onboardingWindow.destroy();
  } finally {
    await resetOnboardingInteractiveState();
    onboardingWindowHandle = null;
    onboardingWindowPromise = null;
  }
}
