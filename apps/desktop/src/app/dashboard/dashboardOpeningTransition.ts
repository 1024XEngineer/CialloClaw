export const DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS = 300_000;

type DashboardOpeningTransitionEnvironment = {
  cancelAnimationFrame: (handle: number) => void;
  clearTimeout: (handle: number) => void;
  hasFocus: () => boolean;
  getVisibilityState: () => DocumentVisibilityState;
  requestAnimationFrame: (callback: FrameRequestCallback) => number;
  setIsOpening: (value: boolean) => void;
  setTimeout: (callback: () => void, timeoutMs: number) => number;
};

/**
 * Isolates the dashboard opening-mask recovery state machine from the React
 * hook that wires it to Tauri window events. Keeping this in a small
 * controller makes the long-idle recovery path readable in the app code and
 * directly testable without mounting a desktop window.
 */
export function createDashboardOpeningTransitionController(environment: DashboardOpeningTransitionEnvironment) {
  let frame = 0;
  let timeout = 0;
  // Seed the hidden flag from mount-time visibility and focus so windows that
  // start hidden or backgrounded can still replay the opening mask when they
  // first become meaningfully visible to the user.
  let hidden = environment.getVisibilityState() === "hidden" || !environment.hasFocus();

  const clearPendingRelease = () => {
    environment.cancelAnimationFrame(frame);
    environment.clearTimeout(timeout);
    frame = 0;
    timeout = 0;
  };

  const trigger = () => {
    clearPendingRelease();
    environment.setIsOpening(true);
    frame = environment.requestAnimationFrame(() => {
      environment.setIsOpening(false);
    });
    // Hidden/background Tauri windows can miss the RAF edge and stay clipped.
    timeout = environment.setTimeout(() => {
      environment.setIsOpening(false);
    }, DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS);
  };

  const markHidden = () => {
    hidden = true;
    clearPendingRelease();
  };

  const restoreIfNeeded = () => {
    if (!hidden || environment.getVisibilityState() === "hidden") {
      return false;
    }

    hidden = false;
    trigger();
    return true;
  };

  const handleVisibilityChange = () => {
    if (environment.getVisibilityState() === "hidden") {
      markHidden();
      return false;
    }

    return restoreIfNeeded();
  };

  const handleWindowFocusChanged = (focused: boolean) => {
    if (!focused) {
      markHidden();
      return false;
    }

    return restoreIfNeeded();
  };

  const dispose = () => {
    clearPendingRelease();
  };

  return {
    dispose,
    handleVisibilityChange,
    handleWindowFocusChanged,
    restoreIfNeeded,
    trigger,
  };
}
