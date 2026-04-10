import type {
  ShellBallInputBarMode,
  ShellBallInteractionEvent,
  ShellBallTransitionResult,
  ShellBallVisualState,
} from "./shellBall.types";

export const SHELL_BALL_LONG_PRESS_MS = 300;
export const SHELL_BALL_LOCK_DELTA_PX = 24;
export const SHELL_BALL_CANCEL_DELTA_PX = 24;
export const SHELL_BALL_CONFIRMING_MS = 600;
export const SHELL_BALL_WAITING_AUTH_MS = 700;
export const SHELL_BALL_PROCESSING_MS = 1200;

type ShellBallControllerDispatchOptions = {
  regionActive?: boolean;
};

type ShellBallScheduledTransition =
  | {
      kind: "state";
      target: ShellBallVisualState;
      ms: number;
    }
  | {
      kind: "processing_return";
      ms: number;
    };

type ShellBallInteractionController = {
  dispatch: (event: ShellBallInteractionEvent, options?: ShellBallControllerDispatchOptions) => ShellBallVisualState;
  forceState: (state: ShellBallVisualState, options?: ShellBallControllerDispatchOptions) => ShellBallVisualState;
  getState: () => ShellBallVisualState;
  dispose: () => void;
};

export function getShellBallProcessingReturnState(regionActive: boolean): ShellBallVisualState {
  return regionActive ? "hover_input" : "idle";
}

export function getShellBallInputBarMode(state: ShellBallVisualState): ShellBallInputBarMode {
  switch (state) {
    case "idle":
      return "hidden";
    case "hover_input":
      return "interactive";
    case "voice_listening":
    case "voice_locked":
      return "voice";
    case "confirming_intent":
    case "processing":
    case "waiting_auth":
      return "readonly";
  }
}

export function resolveShellBallTransition(input: {
  current: ShellBallVisualState;
  event: ShellBallInteractionEvent;
  regionActive: boolean;
}): ShellBallTransitionResult {
  const { current, event, regionActive } = input;

  switch (event) {
    case "pointer_enter_hotspot":
      return { next: current === "idle" ? "hover_input" : current };

    case "pointer_leave_region":
      return {
        next: current === "idle" || current === "hover_input" ? "idle" : current,
      };

    case "submit_text":
      if (current === "hover_input") {
        return {
          next: "confirming_intent",
          autoAdvanceTo: "processing",
          autoAdvanceMs: SHELL_BALL_CONFIRMING_MS,
        };
      }
      return { next: current };

    case "attach_file":
      if (current === "hover_input") {
        return {
          next: "waiting_auth",
          autoAdvanceTo: "processing",
          autoAdvanceMs: SHELL_BALL_WAITING_AUTH_MS,
        };
      }
      return { next: current };

    case "press_start":
      if (current === "idle" || current === "hover_input") {
        return { next: "voice_listening" };
      }
      return { next: current };

    case "voice_lock":
      return { next: current === "voice_listening" ? "voice_locked" : current };

    case "voice_cancel":
      return { next: current === "voice_listening" ? "idle" : current };

    case "voice_finish":
      return { next: current === "voice_listening" ? "processing" : current };

    case "primary_click_locked_voice_end":
      return { next: current === "voice_locked" ? "processing" : current };

    case "auto_advance":
      if (current === "confirming_intent" || current === "waiting_auth") {
        return { next: "processing" };
      }

      if (current === "processing") {
        return { next: getShellBallProcessingReturnState(regionActive) };
      }

      return { next: current };
  }
}

export function createShellBallInteractionController(deps: {
  initialState: ShellBallVisualState;
  schedule: (callback: () => void, ms: number) => unknown;
  cancel: (handle: unknown) => void;
}): ShellBallInteractionController {
  let currentState = deps.initialState;
  let regionActive = currentState !== "idle";
  let pendingHandle: unknown;
  let disposed = false;

  function cancelPending() {
    if (pendingHandle === undefined) {
      return;
    }

    deps.cancel(pendingHandle);
    pendingHandle = undefined;
  }

  function scheduleTransition(transition: ShellBallScheduledTransition) {
    cancelPending();
    pendingHandle = deps.schedule(() => {
      pendingHandle = undefined;
      if (disposed) {
        return;
      }

      if (transition.kind === "state") {
        applyScheduledTarget(transition.target);
        return;
      }

      applyScheduledTarget(getShellBallProcessingReturnState(regionActive));
    }, transition.ms);
  }

  function getProcessingReturnTransition(): ShellBallScheduledTransition {
    return {
      kind: "processing_return",
      ms: SHELL_BALL_PROCESSING_MS,
    };
  }

  function applyScheduledTarget(target: ShellBallVisualState) {
    const previousState = currentState;
    currentState = target;

    if (currentState === "processing" && previousState !== "processing") {
      scheduleTransition(getProcessingReturnTransition());
    }
  }

  function applyTransition(
    event: ShellBallInteractionEvent,
    options: ShellBallControllerDispatchOptions | undefined,
  ): ShellBallVisualState {
    if (disposed) {
      return currentState;
    }

    if (options?.regionActive !== undefined) {
      regionActive = options.regionActive;
    }

    const previousState = currentState;
    const result = resolveShellBallTransition({
      current: currentState,
      event,
      regionActive,
    });

    if (result.next !== previousState) {
      cancelPending();
    }

    currentState = result.next;

    if (result.autoAdvanceMs !== undefined) {
      scheduleTransition({
        kind: "state",
        target: result.autoAdvanceTo,
        ms: result.autoAdvanceMs,
      });
      return currentState;
    }

    if (currentState === "processing" && previousState !== "processing") {
      scheduleTransition(getProcessingReturnTransition());
    }

    return currentState;
  }

  function dispatch(event: ShellBallInteractionEvent, options?: ShellBallControllerDispatchOptions) {
    return applyTransition(event, options);
  }

  function forceState(state: ShellBallVisualState, options?: ShellBallControllerDispatchOptions) {
    if (disposed) {
      return currentState;
    }

    if (options?.regionActive !== undefined) {
      regionActive = options.regionActive;
    }

    cancelPending();
    currentState = state;
    return currentState;
  }

  function getState() {
    return currentState;
  }

  function dispose() {
    if (disposed) {
      return;
    }

    disposed = true;
    cancelPending();
  }

  return {
    dispatch,
    forceState,
    getState,
    dispose,
  };
}
