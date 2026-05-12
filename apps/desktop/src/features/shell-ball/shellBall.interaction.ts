import type {
  ShellBallInputBarMode,
  ShellBallInteractionEvent,
  ShellBallTransitionResult,
  ShellBallVisualState,
  ShellBallVoiceHintMode,
} from "./shellBall.types";
import type { TaskStatus } from "@cialloclaw/protocol";

export const SHELL_BALL_HOVER_INTENT_MS = 360;
export const SHELL_BALL_LEAVE_GRACE_MS = 360;
export const SHELL_BALL_LONG_PRESS_MS = 1000;
export const SHELL_BALL_PRESS_DRIFT_TOLERANCE_PX = 12;
export const SHELL_BALL_LOCKED_CANCEL_HOLD_MS = 200;
export const SHELL_BALL_LOCK_DELTA_PX = 48;
export const SHELL_BALL_CANCEL_DELTA_PX = 48;
export const SHELL_BALL_VERTICAL_PRIORITY_RATIO = 1.25;
export const SHELL_BALL_CONFIRMING_MS = 600;
export const SHELL_BALL_PROCESSING_MS = 1200;

export type ShellBallVoicePreview = "lock" | "cancel" | null;

export type ShellBallGestureAxisIntent = "vertical" | "horizontal";

type ShellBallControllerDispatchOptions = {
  regionActive?: boolean;
  hoverRetained?: boolean;
  scheduleProcessingReturn?: boolean;
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

type ShellBallHoverTimingEvent = Extract<
  ShellBallInteractionEvent,
  "pointer_enter_hotspot" | "pointer_leave_region"
>;

type ShellBallHoverTimingResolution = {
  transition: ShellBallTransitionResult;
  cancelPending: boolean;
};

type ShellBallResolvedInteraction = {
  transition: ShellBallTransitionResult;
  cancelPending: boolean;
};

type ShellBallHoverRetentionInput = {
  regionActive: boolean;
  inputFocused: boolean;
  inputHovered?: boolean;
  hasDraft: boolean;
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
      return "interactive";
    case "hover_input":
    case "confirming_intent":
      return "interactive";
    case "voice_listening":
    case "voice_locked":
      return "hidden";
    case "processing":
    case "waiting_auth":
      return "readonly";
  }
}

export function shouldRetainShellBallHoverInput(input: ShellBallHoverRetentionInput): boolean {
  // Draft content should not keep the shell-ball trapped in an input-dominant
  // state after blur. Only an active focus/hover relationship may retain the
  // hover input state once the pointer leaves the mascot hotspot.
  return !input.regionActive && (input.inputFocused || input.inputHovered === true);
}

export function getShellBallGestureAxisIntent(input: {
  deltaX: number;
  deltaY: number;
}): ShellBallGestureAxisIntent {
  const verticalDistance = Math.abs(input.deltaY);
  const horizontalDistance = Math.abs(input.deltaX);

  if (verticalDistance >= horizontalDistance * SHELL_BALL_VERTICAL_PRIORITY_RATIO) {
    return "vertical";
  }

  return "horizontal";
}

export function shouldPreviewShellBallVoiceGesture(input: { deltaX: number; deltaY: number }): boolean {
  return getShellBallGestureAxisIntent(input) === "vertical";
}

export function getShellBallVoicePreview(input: { deltaX: number; deltaY: number }): ShellBallVoicePreview {
  const { deltaX, deltaY } = input;

  if (!shouldPreviewShellBallVoiceGesture({ deltaX, deltaY })) {
    return null;
  }

  if (deltaY <= -SHELL_BALL_LOCK_DELTA_PX) {
    return "lock";
  }

  if (deltaY >= SHELL_BALL_CANCEL_DELTA_PX) {
    return "cancel";
  }

  return null;
}

export function getShellBallVoicePreviewForHintMode(input: {
  hintMode: Exclude<ShellBallVoiceHintMode, "hidden">;
  deltaX: number;
  deltaY: number;
}): ShellBallVoicePreview {
  const preview = getShellBallVoicePreview({ deltaX: input.deltaX, deltaY: input.deltaY });

  if (input.hintMode === "lock") {
    return preview === "lock" ? "lock" : null;
  }

  return preview === "cancel" ? "cancel" : null;
}

export function resolveShellBallVoiceReleaseEvent(preview: Exclude<ShellBallVoicePreview, "lock">): Extract<ShellBallInteractionEvent, "voice_cancel" | "voice_finish"> {
  return preview === "cancel" ? "voice_cancel" : "voice_finish";
}

export function getShellBallVisualStateForTaskStatus(status: TaskStatus, fallback: ShellBallVisualState): ShellBallVisualState {
  switch (status) {
    case "confirming_intent":
      return "confirming_intent";
    case "processing":
      return "processing";
    case "waiting_auth":
      return "waiting_auth";
    case "waiting_input":
      return fallback === "voice_locked" ? "hover_input" : "hover_input";
    case "paused":
    case "blocked":
    case "failed":
    case "cancelled":
    case "ended_unfinished":
    case "completed":
      return fallback === "hover_input" || fallback === "voice_locked" ? "hover_input" : "idle";
  }
}

export function resolveShellBallHoverTiming(input: {
  current: ShellBallVisualState;
  event: ShellBallHoverTimingEvent;
  hoverRetained?: boolean;
}): ShellBallHoverTimingResolution {
  const { current, event, hoverRetained = false } = input;

  if (event === "pointer_enter_hotspot") {
    if (current === "idle") {
      return {
        transition: {
          next: "idle",
          autoAdvanceTo: "hover_input",
          autoAdvanceMs: SHELL_BALL_HOVER_INTENT_MS,
        },
        cancelPending: false,
      };
    }

    return {
      transition: { next: current },
      cancelPending: current === "hover_input",
    };
  }

  if (hoverRetained && current === "hover_input") {
    return {
      transition: { next: current },
      cancelPending: true,
    };
  }

  if (current === "hover_input") {
    return {
      transition: {
        next: "hover_input",
        autoAdvanceTo: "idle",
        autoAdvanceMs: SHELL_BALL_LEAVE_GRACE_MS,
      },
      cancelPending: false,
    };
  }

  return {
    transition: { next: current },
    cancelPending: current === "idle",
  };
}

function resolveShellBallInteraction(input: {
  current: ShellBallVisualState;
  event: ShellBallInteractionEvent;
  regionActive: boolean;
  hoverRetained?: boolean;
}): ShellBallResolvedInteraction {
  const { current, event, regionActive, hoverRetained = false } = input;

  if (event === "pointer_enter_hotspot" || event === "pointer_leave_region") {
    return resolveShellBallHoverTiming({ current, event, hoverRetained });
  }

  return {
    transition: resolveShellBallTransition({
      current,
      event,
      regionActive,
      hoverRetained,
    }),
    cancelPending: false,
  };
}

export function resolveShellBallTransition(input: {
  current: ShellBallVisualState;
  event: ShellBallInteractionEvent;
  regionActive: boolean;
  hoverRetained?: boolean;
}): ShellBallTransitionResult {
  const { current, event, regionActive } = input;

  switch (event) {
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
      return {
        next: current === "voice_listening" ? getShellBallProcessingReturnState(regionActive) : current,
      };

    case "primary_click_locked_voice_end":
      return {
        next: current === "voice_locked" ? getShellBallProcessingReturnState(regionActive) : current,
      };

    case "auto_advance":
      if (current === "confirming_intent") {
        return { next: "processing" };
      }

      if (current === "processing") {
        return { next: getShellBallProcessingReturnState(regionActive) };
      }

      return { next: current };

    case "pointer_enter_hotspot":
    case "pointer_leave_region":
      return resolveShellBallHoverTiming({ current, event, hoverRetained: input.hoverRetained }).transition;
  }
}

export function createShellBallInteractionController(deps: {
  initialState: ShellBallVisualState;
  schedule: (callback: () => void, ms: number) => unknown;
  cancel: (handle: unknown) => void;
}): ShellBallInteractionController {
  let currentState = deps.initialState;
  let regionActive = currentState !== "idle";
  let hoverRetained = false;
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
    applyState(target, { scheduleProcessingReturn: true });
  }

  function applyState(
    target: ShellBallVisualState,
    options: {
      scheduleProcessingReturn: boolean;
      cancelExisting?: boolean;
    },
  ) {
    const previousState = currentState;
    if (options.cancelExisting) {
      cancelPending();
    }

    currentState = target;

    if (options.scheduleProcessingReturn && currentState === "processing" && previousState !== "processing") {
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

    if (options?.hoverRetained !== undefined) {
      hoverRetained = options.hoverRetained;
    }

    const previousState = currentState;
    const resolved = resolveShellBallInteraction({
      current: currentState,
      event,
      regionActive,
      hoverRetained,
    });
    const result = resolved.transition;

    if (result.autoAdvanceMs !== undefined) {
      applyState(result.next, {
        cancelExisting: resolved.cancelPending || result.next !== previousState,
        scheduleProcessingReturn: false,
      });
      scheduleTransition({
        kind: "state",
        target: result.autoAdvanceTo,
        ms: result.autoAdvanceMs,
      });
      return currentState;
    }

    applyState(result.next, {
      cancelExisting: resolved.cancelPending || result.next !== previousState,
      scheduleProcessingReturn: true,
    });

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

    if (options?.hoverRetained !== undefined) {
      hoverRetained = options.hoverRetained;
    }

    applyState(state, { cancelExisting: true, scheduleProcessingReturn: options?.scheduleProcessingReturn ?? true });
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
