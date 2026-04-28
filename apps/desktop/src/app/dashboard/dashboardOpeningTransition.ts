export const DASHBOARD_OPENING_RECOVERY_TIMEOUT_MS = 720;

type DashboardOpeningTransitionEnvironment = {
  cancelAnimationFrame: (handle: number) => void;
  clearTimeout: (handle: number) => void;
  getVisibilityState: () => DocumentVisibilityState;
  requestAnimationFrame: (callback: FrameRequestCallback) => number;
  setIsOpening: (value: boolean) => void;
  setTimeout: (callback: () => void, timeoutMs: number) => number;
};

/**
 * Coordinates the dashboard opening mask so hidden or unfocused desktop
 * windows can replay their reveal transition once they become visible again.
 */
export function createDashboardOpeningTransitionController(environment: DashboardOpeningTransitionEnvironment) {
  let frame = 0;
  let timeout = 0;
  let hidden = false;

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
