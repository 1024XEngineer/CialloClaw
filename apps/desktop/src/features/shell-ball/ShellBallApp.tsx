/**
 * Shell-ball app renders the merged floating mascot window together with its
 * inline bubble/input/voice affordances, drag/drop handling, and dashboard
 * transitions.
 */
import type { PointerEvent as ReactPointerEvent } from "react";
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { useEventListener } from "ahooks";
import { invoke } from "@tauri-apps/api/core";
import { getCurrentWindow, monitorFromPoint } from "@tauri-apps/api/window";
import { ShellBallSurface, shouldAcceptShellBallTextDrop } from "./ShellBallSurface";
import { ShellBallAttachmentTray } from "./components/ShellBallAttachmentTray";
import { ShellBallBubbleZone } from "./components/ShellBallBubbleZone";
import { ShellBallInputBar } from "./components/ShellBallInputBar";
import { ShellBallVoiceHints } from "./components/ShellBallVoiceHints";
import type { ShellBallSelectionSnapshot } from "./selection/selection.types";
import { useShellBallInteraction } from "./useShellBallInteraction";
import { getShellBallMotionConfig } from "./shellBall.motion";
import type { ShellBallInputBarMode, ShellBallVisualState } from "./shellBall.types";
import { useShellBallCoordinator } from "./useShellBallCoordinator";
import { type ShellBallEdgeDockSide, useShellBallWindowMetrics } from "./useShellBallWindowMetrics";
import {
  getShellBallVisibleBubbleItems,
  shellBallWindowSyncEvents,
  type ShellBallClipboardSnapshotPayload,
  type ShellBallSelectionSnapshotPayload,
} from "./shellBall.windowSync";
import type { ShellBallDashboardTransitionRequest } from "../../platform/dashboardWindowTransition";
import { shellBallDashboardTransitionEvents } from "../../platform/dashboardWindowTransition";
import {
  createShellBallLogicalPosition,
  hideShellBallWindow,
  shellBallWindowLabels,
  showShellBallWindow,
} from "../../platform/shellBallWindowController";
import {
  setShellBallAlwaysOnTop,
  setShellBallInteractiveRegions,
  setShellBallPressLock,
} from "../../platform/shellBallWindow";
import { openOrFocusDesktopWindow } from "../../platform/windowController";
import { buildDesktopOnboardingPresentation } from "@/features/onboarding/onboardingGeometry";
import {
  advanceDesktopOnboarding,
  setDesktopOnboardingLoadingState,
  setDesktopOnboardingPresentation,
  shouldAutoStartDesktopOnboarding,
  startDesktopOnboarding,
} from "@/features/onboarding/onboardingService";
import { useDesktopOnboardingActions } from "@/features/onboarding/useDesktopOnboardingActions";
import { useDesktopOnboardingLoading } from "@/features/onboarding/useDesktopOnboardingLoading";
import { useDesktopOnboardingSession } from "@/features/onboarding/useDesktopOnboardingSession";
import { shouldFocusShellBallInlineInputBeforePrimaryClick } from "./shellBallPrimaryClick";

type ShellBallAppProps = {
  isDev?: boolean;
};

type ShellBallDashboardTransitionPhase = "idle" | "opening" | "hidden" | "closing";
type ShellBallFloatingSize = "small" | "medium" | "large";

type ShellBallWindowAnchor = {
  x: number;
  y: number;
};

const SHELL_BALL_DASHBOARD_TRANSITION_DURATION_MS = 260;
const SHELL_BALL_SELECTION_PROMPT_CLEAR_DELAY_MS = 240;
const SHELL_BALL_CLIPBOARD_PROMPT_WINDOW_MS = 10_000;
const SHELL_BALL_EDGE_DOCK_REVEAL_MARGIN_PX = 24;

type ShellBallClipboardPrompt = {
  text: string;
  expiresAt: number;
};

async function pickShellBallFiles(): Promise<string[]> {
  const result = await invoke<string[]>("pick_shell_ball_files");
  return Array.isArray(result) ? result : [];
}

/**
 * Determines whether the file-drop overlay should be visible for the current
 * floating ball state.
 *
 * @param input File-drop visibility inputs from the ball window.
 * @returns Whether the overlay should be rendered.
 */
export function shouldShowShellBallFileDropOverlay(input: {
  fileDropActive: boolean;
}) {
  return input.fileDropActive;
}

/**
 * Decides when the shell-ball should arm its text drop target without fighting
 * with file drop or voice-only states.
 *
 * @param input Drag and state metadata from the shell-ball window.
 * @returns Whether text drag affordances should be active.
 */
export function shouldArmShellBallTextDropTarget(input: {
  fileDropActive: boolean;
  textDragActive: boolean;
  visualState: ShellBallVisualState;
}) {
  if (input.fileDropActive) {
    return false;
  }

  if (input.visualState === "voice_listening" || input.visualState === "voice_locked") {
    return false;
  }

  return input.textDragActive;
}

function waitForAnimationFrame() {
  return new Promise<void>((resolve) => {
    window.requestAnimationFrame(() => resolve());
  });
}

/**
 * Controls the red text-selection marker that appears above the floating ball.
 *
 * @param input Selection availability plus the current shell-ball state.
 * @returns Whether the marker should be shown.
 */
export function shouldShowShellBallSelectionIndicator(input: {
  selection: ShellBallSelectionSnapshot | null;
  visualState: ShellBallVisualState;
}) {
  return input.selection !== null && (input.visualState === "idle" || input.visualState === "hover_input");
}

/**
 * Determines whether a clipboard prompt is still eligible for click-to-submit
 * handling.
 *
 * @param prompt Current clipboard prompt state.
 * @param now Current timestamp in milliseconds.
 * @returns Whether the clipboard prompt should trigger a backend submit.
 */
export function isShellBallClipboardPromptActive(
  prompt: ShellBallClipboardPrompt | null,
  now = Date.now(),
) {
  return prompt !== null && prompt.expiresAt > now;
}

/**
 * Normalizes free-form floating-size inputs down to the supported shell-ball
 * presets. The runtime mainly passes explicit values now, but tests still
 * exercise the helper directly.
 */
export function normalizeShellBallFloatingSize(value: string | undefined): ShellBallFloatingSize {
  return value === "small" || value === "large" ? value : "medium";
}

export function resolveShellBallInlineInputMode(input: {
  shouldRenderInlineInput: boolean;
  snapshotInputBarMode: ShellBallInputBarMode;
}): ShellBallInputBarMode {
  if (!input.shouldRenderInlineInput) {
    return "hidden";
  }

  return input.snapshotInputBarMode === "hidden" ? "interactive" : input.snapshotInputBarMode;
}

export function shouldRetainShellBallEdgeDockReveal(input: {
  bounds: {
    minX: number;
    maxX: number;
    minY: number;
    maxY: number;
  };
  edgeDockSide: ShellBallEdgeDockSide | null;
  screenX: number;
  screenY: number;
}) {
  switch (input.edgeDockSide) {
    case "left":
      return input.screenX <= input.bounds.minX + SHELL_BALL_EDGE_DOCK_REVEAL_MARGIN_PX;
    case "right":
      return input.screenX >= input.bounds.maxX - SHELL_BALL_EDGE_DOCK_REVEAL_MARGIN_PX;
    case "top":
      return input.screenY <= input.bounds.minY + SHELL_BALL_EDGE_DOCK_REVEAL_MARGIN_PX;
    case "bottom":
      return input.screenY >= input.bounds.maxY - SHELL_BALL_EDGE_DOCK_REVEAL_MARGIN_PX;
    case "top_left":
      return input.screenX <= input.bounds.minX + SHELL_BALL_EDGE_DOCK_REVEAL_MARGIN_PX
        && input.screenY <= input.bounds.minY + SHELL_BALL_EDGE_DOCK_REVEAL_MARGIN_PX;
    case "top_right":
      return input.screenX >= input.bounds.maxX - SHELL_BALL_EDGE_DOCK_REVEAL_MARGIN_PX
        && input.screenY <= input.bounds.minY + SHELL_BALL_EDGE_DOCK_REVEAL_MARGIN_PX;
    case "bottom_left":
      return input.screenX <= input.bounds.minX + SHELL_BALL_EDGE_DOCK_REVEAL_MARGIN_PX
        && input.screenY >= input.bounds.maxY - SHELL_BALL_EDGE_DOCK_REVEAL_MARGIN_PX;
    case "bottom_right":
      return input.screenX >= input.bounds.maxX - SHELL_BALL_EDGE_DOCK_REVEAL_MARGIN_PX
        && input.screenY >= input.bounds.maxY - SHELL_BALL_EDGE_DOCK_REVEAL_MARGIN_PX;
    default:
      return false;
  }
}

function easeShellBallDashboardTransition(progress: number) {
  return 1 - Math.pow(1 - progress, 3);
}

async function resolveShellBallDashboardTransitionTarget(input: {
  width: number;
  height: number;
}) {
  const currentWindow = getCurrentWindow();
  const outerPosition = await currentWindow.outerPosition();
  const scaleFactor = await currentWindow.scaleFactor();
  const logicalPosition = outerPosition.toLogical(scaleFactor);
  const monitor = await monitorFromPoint(outerPosition.x, outerPosition.y);

  if (monitor === null) {
    return {
      anchor: {
        x: logicalPosition.x,
        y: logicalPosition.y,
      },
      center: {
        x: logicalPosition.x,
        y: logicalPosition.y,
      },
    };
  }

  const monitorPosition = monitor.position.toLogical(monitor.scaleFactor);
  const monitorSize = monitor.size.toLogical(monitor.scaleFactor);
  const dashboardTargetYOffset = Math.round(Math.min(42, Math.max(22, monitorSize.height * 0.032)));

  return {
    anchor: {
      x: logicalPosition.x,
      y: logicalPosition.y,
    },
    center: {
      x: Math.round(monitorPosition.x + (monitorSize.width - input.width) / 2),
      y: Math.round(monitorPosition.y + (monitorSize.height - input.height) / 2 + dashboardTargetYOffset),
    },
  };
}

async function resolveShellBallDashboardTransitionFrame(windowFrame: { width: number; height: number } | null) {
  if (windowFrame !== null) {
    return windowFrame;
  }

  const currentWindow = getCurrentWindow();
  const outerSize = await currentWindow.outerSize();
  const scaleFactor = await currentWindow.scaleFactor();
  const logicalSize = outerSize.toLogical(scaleFactor);

  return {
    width: logicalSize.width,
    height: logicalSize.height,
  };
}

async function animateShellBallDashboardWindow(input: {
  from: ShellBallWindowAnchor;
  to: ShellBallWindowAnchor;
  durationMs: number;
}) {
  const currentWindow = getCurrentWindow();
  const startTime = performance.now();

  while (true) {
    const elapsed = performance.now() - startTime;
    const progress = Math.min(elapsed / input.durationMs, 1);
    const easedProgress = easeShellBallDashboardTransition(progress);
    const nextX = Math.round(input.from.x + (input.to.x - input.from.x) * easedProgress);
    const nextY = Math.round(input.from.y + (input.to.y - input.from.y) * easedProgress);

    await currentWindow.setPosition(createShellBallLogicalPosition(nextX, nextY));

    if (progress >= 1) {
      return;
    }

    await waitForAnimationFrame();
  }
}

export function ShellBallApp({ isDev = false }: ShellBallAppProps) {
  void isDev;
  const onboardingSession = useDesktopOnboardingSession();
  const onboardingLoading = useDesktopOnboardingLoading("shell-ball");
  const {
    visualState,
    inputValue,
    pendingFiles,
    finalizedSpeechPayload,
    voicePreview,
    voiceHintMode,
    voiceHoldProgress,
    regionActive,
    inputFocused,
    shouldOpenDashboardFromDoubleClick,
    handleRegionEnter,
    handleRegionLeave,
    handlePressStart,
    handlePressMove,
    handlePressEnd,
    handlePressCancel,
    handleSubmitText,
    handleSubmitVoiceText,
    handleAttachFile,
    handleDroppedFiles: handleAppendPendingFiles,
    handleDroppedText,
    handleRemovePendingFile,
    handleInputHoverChange,
    handleInputFocusChange,
    handleInputFocusRequest,
    setInputValue,
    acknowledgeFinalizedSpeechPayload,
  } = useShellBallInteraction();
  const motionConfig = getShellBallMotionConfig(visualState);
  const [dashboardTransitionPhase, setDashboardTransitionPhase] = useState<ShellBallDashboardTransitionPhase>("idle");
  const [fileDropActive, setFileDropActive] = useState(false);
  const [inputFocusToken, setInputFocusToken] = useState(0);
  const [textDragActive, setTextDragActive] = useState(false);
  const [selectionPrompt, setSelectionPrompt] = useState<ShellBallSelectionSnapshot | null>(null);
  const [clipboardPrompt, setClipboardPrompt] = useState<ShellBallClipboardPrompt | null>(null);
  const anchorRef = useRef<ShellBallWindowAnchor | null>(null);
  const pressCaptureLockRef = useRef(false);
  const mascotRef = useRef<HTMLDivElement>(null);
  const lastReportedInteractiveRegionsRef = useRef<string>("");
  const dashboardTransitionPhaseRef = useRef<ShellBallDashboardTransitionPhase>("idle");
  const clipboardPromptClearTimeoutRef = useRef<number | null>(null);
  const selectionPromptClearTimeoutRef = useRef<number | null>(null);
  const previousVisualStateRef = useRef<ShellBallVisualState>(visualState);
  const transitionQueueRef = useRef(Promise.resolve());
  const dragDropHandlersRef = useRef<{
    handleDroppedFiles: (paths: string[]) => Promise<void> | void;
  }>({
    handleDroppedFiles: () => undefined,
  });
  const inputFocusRequestRef = useRef<() => void>(() => undefined);
  const shellBallWindowTarget = typeof window === "undefined" ? undefined : window;
  const focusInlineInputField = useCallback((syncInteraction = true) => {
    if (syncInteraction) {
      handleInputFocusRequest();
    }

    setInputFocusToken((current) => current + 1);
  }, [handleInputFocusRequest]);
  inputFocusRequestRef.current = () => focusInlineInputField(false);
  const {
    handleClipboardPrompt: handleCoordinatorClipboardPrompt,
    handleDroppedFiles: handleCoordinatorDroppedFiles,
    handleSelectedTextPrompt: handleCoordinatorSelectedTextPrompt,
    handlePrimaryAction: handleCoordinatorPrimaryAction,
    handleBubbleAction: handleCoordinatorBubbleAction,
    handleErrorSignalAccept: handleCoordinatorErrorSignalAccept,
    handleErrorSignalIgnore: handleCoordinatorErrorSignalIgnore,
    handleRecommendationAccept: handleCoordinatorRecommendationAccept,
    handleRecommendationIgnore: handleCoordinatorRecommendationIgnore,
    handleConfirmIntentBubble: handleCoordinatorConfirmIntentBubble,
    handleBubbleHoverChange: handleCoordinatorBubbleHoverChange,
    handleInputHoverChange: handleCoordinatorInputHoverChange,
    handleInputFocusChange: handleCoordinatorInputFocusChange,
    handleRegionEnter: handleCoordinatorRegionEnter,
    handleRegionLeave: handleCoordinatorRegionLeave,
    snapshot,
  } = useShellBallCoordinator({
    getBallClientRect: () => mascotRef.current?.getBoundingClientRect() ?? null,
    visualState,
    helperWindowsVisible: dashboardTransitionPhase === "idle",
    regionActive,
    inputValue,
    inputFocused,
    pendingFiles,
    finalizedSpeechPayload,
    voicePreview,
    voiceHintMode,
    setInputValue,
    onAppendPendingFiles: handleAppendPendingFiles,
    onRemovePendingFile: handleRemovePendingFile,
    onFinalizedSpeechHandled: acknowledgeFinalizedSpeechPayload,
    onRegionEnter: handleRegionEnter,
    onRegionLeave: handleRegionLeave,
    onInputHoverChange: handleInputHoverChange,
    onInputFocusChange: handleInputFocusChange,
    onSubmitText: handleSubmitText,
    onSubmitVoiceText: handleSubmitVoiceText,
    onAttachFile: handleAttachFile,
    onRequestInputFocus: () => focusInlineInputField(),
  });
  const shouldRenderInlineInput = snapshot.visibility.input || visualState === "idle";
  const inlineInputMode = resolveShellBallInlineInputMode({
    shouldRenderInlineInput,
    snapshotInputBarMode: snapshot.inputBarMode,
  });
  const visibleBubbleItems = getShellBallVisibleBubbleItems(snapshot.bubbleItems);
  const {
    beginBallWindowPointerDrag,
    edgeDockState,
    endBallWindowPointerDrag,
    freezeBallWindowPointerDrag,
    rootRef,
    setEdgeDockRevealed,
    updateBallWindowPointerDrag,
    windowFrame,
  } = useShellBallWindowMetrics({
    role: "ball",
    helperVisibility: {
      bubble: false,
      input: false,
      voice: false,
    },
  });
  const isEdgeDocked = edgeDockState.side !== null;
  const windowFrameRef = useRef(windowFrame);
  dragDropHandlersRef.current = {
    handleDroppedFiles: handleCoordinatorDroppedFiles,
  };
  windowFrameRef.current = windowFrame;

  const reportInteractiveRegions = useCallback(async () => {
    const currentWindow = getCurrentWindow();

    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }

    const scaleFactor = await currentWindow.scaleFactor();
    const regionElements = [
      mascotRef.current?.querySelector<HTMLElement>(".shell-ball-mascot__hotspot") ?? null,
      rootRef.current?.querySelector<HTMLElement>(".shell-ball-window--input textarea") ?? null,
      ...Array.from(rootRef.current?.querySelectorAll<HTMLElement>(".shell-ball-window--input .shell-ball-uiverse-action") ?? []),
      ...Array.from(rootRef.current?.querySelectorAll<HTMLElement>(".shell-ball-attachment-tray__item") ?? []),
      ...Array.from(rootRef.current?.querySelectorAll<HTMLElement>(".shell-ball-attachment-tray__remove") ?? []),
      rootRef.current?.querySelector<HTMLElement>(".shell-ball-bubble-zone__scroll") ?? null,
    ].filter((element): element is HTMLElement => element !== null);

    if (regionElements.length === 0) {
      await setShellBallInteractiveRegions([]);
      lastReportedInteractiveRegionsRef.current = "";
      return;
    }

    const regions = regionElements.map((element) => {
      const rect = element.getBoundingClientRect();

      return {
        x: Math.round(rect.left * scaleFactor),
        y: Math.round(rect.top * scaleFactor),
        width: Math.max(1, Math.round(rect.width * scaleFactor)),
        height: Math.max(1, Math.round(rect.height * scaleFactor)),
      };
    });
    const signature = JSON.stringify(regions);

    if (signature === lastReportedInteractiveRegionsRef.current) {
      return;
    }

    lastReportedInteractiveRegionsRef.current = signature;
    await setShellBallInteractiveRegions(regions);
  }, [rootRef]);

  const handleInlineAttachFile = useCallback(() => {
    void (async () => {
      try {
        const selectedPaths = await pickShellBallFiles();
        if (selectedPaths.length === 0) {
          return;
        }

        await handleCoordinatorDroppedFiles(selectedPaths);
        focusInlineInputField();
      } catch (error) {
        console.warn("shell-ball file picker failed", error);
        await handleCoordinatorPrimaryAction("attach_file");
      }
    })();
  }, [focusInlineInputField, handleCoordinatorDroppedFiles, handleCoordinatorPrimaryAction]);

  const handleInlineInputFocusChange = useCallback((focused: boolean) => {
    if (!focused) {
      // Blur should fully retire any outstanding focus request so later orb
      // interactions cannot accidentally replay a stale textarea focus token.
      setInputFocusToken(0);
    }

    handleCoordinatorInputFocusChange(focused);
  }, [handleCoordinatorInputFocusChange]);

  const handleLockedPressStart = useCallback((event: ReactPointerEvent<HTMLButtonElement>) => {
    pressCaptureLockRef.current = true;
    void setShellBallPressLock(true);
    handlePressStart(event);
  }, [handlePressStart]);

  const handleLockedPressEnd = useCallback((event: ReactPointerEvent<HTMLButtonElement>) => {
    try {
      return handlePressEnd(event);
    } finally {
      pressCaptureLockRef.current = false;
      void setShellBallPressLock(false);
    }
  }, [handlePressEnd]);

  const handleLockedPressCancel = useCallback((event: ReactPointerEvent<HTMLButtonElement>) => {
    try {
      handlePressCancel(event);
    } finally {
      pressCaptureLockRef.current = false;
      void setShellBallPressLock(false);
    }
  }, [handlePressCancel]);

  useEffect(() => {
    if (getCurrentWindow().label !== shellBallWindowLabels.ball) {
      return;
    }

    if (!shouldAutoStartDesktopOnboarding()) {
      return;
    }

    if (onboardingSession !== null) {
      return;
    }

    void setDesktopOnboardingLoadingState({
      message: "正在打开引导...",
      windowLabel: "shell-ball",
    });
    void startDesktopOnboarding("first_launch");
  }, [onboardingSession]);

  useDesktopOnboardingActions(
    "shell-ball",
    useCallback((action) => {
      if (action.type === "open_dashboard") {
        void openOrFocusDesktopWindow("dashboard");
      }

       if (action.type === "show_shell_ball") {
         void showShellBallWindow("ball");
       }
    }, []),
  );

  useEffect(() => {
    void setShellBallAlwaysOnTop(true);
  }, []);

  useEffect(() => {
    const wasVoiceActive =
      previousVisualStateRef.current === "voice_listening" || previousVisualStateRef.current === "voice_locked";
    const isVoiceActive = visualState === "voice_listening" || visualState === "voice_locked";

    // Voice gestures should operate against a stationary orb once capture starts.
    if (!wasVoiceActive && isVoiceActive) {
      void freezeBallWindowPointerDrag();
    }

    previousVisualStateRef.current = visualState;

    if (
      onboardingSession?.isOpen === true &&
      onboardingSession.step === "shell_ball_hold_voice" &&
      !wasVoiceActive &&
      isVoiceActive
    ) {
      void advanceDesktopOnboarding("shell_ball_double_click");
    }
  }, [freezeBallWindowPointerDrag, onboardingSession, visualState]);

  useEffect(() => {
    const currentWindow = getCurrentWindow();

    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }

    let disposed = false;
    let cleanup: (() => void) | null = null;

    function applyDashboardTransitionPhase(nextPhase: ShellBallDashboardTransitionPhase) {
      dashboardTransitionPhaseRef.current = nextPhase;
      setDashboardTransitionPhase(nextPhase);
    }

    async function runForwardTransition() {
      if (dashboardTransitionPhaseRef.current !== "idle") {
        return;
      }

      const transitionFrame = await resolveShellBallDashboardTransitionFrame(windowFrameRef.current);
      const transitionTarget = await resolveShellBallDashboardTransitionTarget(transitionFrame);
      anchorRef.current = transitionTarget.anchor;
      applyDashboardTransitionPhase("opening");
      await waitForAnimationFrame();
      await animateShellBallDashboardWindow({
        from: transitionTarget.anchor,
        to: transitionTarget.center,
        durationMs: SHELL_BALL_DASHBOARD_TRANSITION_DURATION_MS,
      });
      applyDashboardTransitionPhase("hidden");
      await hideShellBallWindow("ball");
    }

    async function runReverseTransition(requestId?: string) {
      if (dashboardTransitionPhaseRef.current === "idle") {
        if (requestId !== undefined) {
          await currentWindow.emitTo("dashboard", shellBallDashboardTransitionEvents.complete, {
            direction: "close",
            requestId,
          });
        }

        return;
      }

      const transitionFrame = await resolveShellBallDashboardTransitionFrame(windowFrameRef.current);
      const transitionTarget = await resolveShellBallDashboardTransitionTarget(transitionFrame);
      const center = transitionTarget.center;
      const anchor = anchorRef.current ?? transitionTarget.anchor;

      await currentWindow.setPosition(createShellBallLogicalPosition(center.x, center.y));
      applyDashboardTransitionPhase("hidden");
      await showShellBallWindow("ball");
      await waitForAnimationFrame();
      await waitForAnimationFrame();
      applyDashboardTransitionPhase("closing");
      await animateShellBallDashboardWindow({
        from: center,
        to: anchor,
        durationMs: SHELL_BALL_DASHBOARD_TRANSITION_DURATION_MS,
      });
      applyDashboardTransitionPhase("idle");

      if (requestId !== undefined) {
        await currentWindow.emitTo("dashboard", shellBallDashboardTransitionEvents.complete, {
          direction: "close",
          requestId,
        });
      }
    }

    void currentWindow
      .listen<ShellBallDashboardTransitionRequest>(shellBallDashboardTransitionEvents.request, ({ payload }) => {
        transitionQueueRef.current = transitionQueueRef.current
          .catch((): void => undefined)
          .then(async () => {
            if (disposed) {
              return;
            }

            if (payload.direction === "open") {
              await runForwardTransition();
              return;
            }

            await runReverseTransition(payload.requestId);
          });
      })
      .then((unlisten) => {
        if (disposed) {
          unlisten();
          return;
        }

        cleanup = unlisten;
      });

    return () => {
      disposed = true;
      cleanup?.();
    };
  }, []);

  useEffect(() => {
    const currentWindow = getCurrentWindow();

    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }

    let cleanup: (() => void) | null = null;
    let disposed = false;

    void currentWindow.onDragDropEvent((event) => {
      switch (event.payload.type) {
        case "enter":
        case "over":
          setFileDropActive(true);
          return;
        case "leave":
          setFileDropActive(false);
          return;
        case "drop": {
          setFileDropActive(false);
          if (event.payload.paths.length === 0) {
            return;
          }

          const droppedPaths = event.payload.paths;
          void (async () => {
            try {
              await dragDropHandlersRef.current.handleDroppedFiles(droppedPaths);
              inputFocusRequestRef.current();
            } catch (error) {
              console.warn("shell-ball file drop handling failed", error);
            }
          })();
          return;
        }
      }
    }).then((unlisten) => {
      if (disposed) {
        unlisten();
        return;
      }

      cleanup = unlisten;
    });

    return () => {
      disposed = true;
      cleanup?.();
    };
  }, []);

  useLayoutEffect(() => {
    const currentWindow = getCurrentWindow();

    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }

    void (async () => {
      await reportInteractiveRegions();
    })();
  }, [reportInteractiveRegions, shouldRenderInlineInput, snapshot.visibility.bubble, visibleBubbleItems, windowFrame]);

  useEffect(() => {
    // Reset native mascot hotspot state only when the shell-ball host actually
    // unmounts so ordinary frame updates do not churn IPC requests.
    return () => {
      void setShellBallInteractiveRegions([]);
      void setShellBallPressLock(false);
      lastReportedInteractiveRegionsRef.current = "";
    };
  }, []);

  useEffect(() => {
    if (getCurrentWindow().label !== shellBallWindowLabels.ball) {
      return;
    }

    void reportInteractiveRegions();
  }, [inputFocused, reportInteractiveRegions]);

  function handleDoubleClick() {
    if (!shouldOpenDashboardFromDoubleClick) {
      return;
    }

    if (onboardingSession?.isOpen === true && onboardingSession.step === "shell_ball_double_click") {
      void advanceDesktopOnboarding("dashboard_overview");
    }

    void openOrFocusDesktopWindow("dashboard");
  }

  useEffect(() => {
    if (
      onboardingSession?.isOpen !== true ||
      (onboardingSession.step !== "welcome" &&
        onboardingSession.step !== "shell_ball_intro" &&
        onboardingSession.step !== "shell_ball_hold_voice" &&
        onboardingSession.step !== "shell_ball_double_click")
    ) {
      return;
    }

    void (async () => {
      const presentation = await buildDesktopOnboardingPresentation({
        anchors: [],
        placement: "center",
        step: onboardingSession.step,
        windowLabel: "shell-ball",
      });

      if (presentation !== null) {
        await setDesktopOnboardingPresentation(presentation);
      }
    })();
  }, [mascotRef, onboardingSession]);

  const handleSurfaceTextDrop = useCallback((text: string) => {
    handleDroppedText(text);
    window.requestAnimationFrame(() => {
      focusInlineInputField(false);
    });
  }, [focusInlineInputField, handleDroppedText]);

  const clearTextDragState = useCallback(() => {
    setTextDragActive(false);
  }, []);

  const handleWindowTextDrag = useCallback((event: DragEvent) => {
    if (!shouldAcceptShellBallTextDrop(event.dataTransfer)) {
      clearTextDragState();
      return;
    }

    if (!shouldArmShellBallTextDropTarget({
      fileDropActive,
      textDragActive: true,
      visualState,
    })) {
      clearTextDragState();
      return;
    }

    setTextDragActive(true);
  }, [clearTextDragState, fileDropActive, visualState]);

  useEventListener("dragenter", handleWindowTextDrag, {
    target: shellBallWindowTarget,
    enable: shellBallWindowTarget !== undefined,
  });
  useEventListener("dragover", handleWindowTextDrag, {
    target: shellBallWindowTarget,
    enable: shellBallWindowTarget !== undefined,
  });
  useEventListener("dragleave", clearTextDragState, {
    target: shellBallWindowTarget,
    enable: shellBallWindowTarget !== undefined,
  });
  useEventListener("drop", clearTextDragState, {
    target: shellBallWindowTarget,
    enable: shellBallWindowTarget !== undefined,
  });

  useEffect(() => {
    if (!fileDropActive && visualState !== "voice_listening" && visualState !== "voice_locked") {
      return;
    }

    setTextDragActive(false);
  }, [fileDropActive, visualState]);

  useEffect(() => {
    if (visualState !== "idle" && visualState !== "hover_input") {
      if (selectionPromptClearTimeoutRef.current !== null) {
        window.clearTimeout(selectionPromptClearTimeoutRef.current);
        selectionPromptClearTimeoutRef.current = null;
      }
      setSelectionPrompt(null);
    }
  }, [visualState]);

  useEffect(() => {
    if (clipboardPrompt === null) {
      if (clipboardPromptClearTimeoutRef.current !== null) {
        window.clearTimeout(clipboardPromptClearTimeoutRef.current);
        clipboardPromptClearTimeoutRef.current = null;
      }
      return;
    }

    const remainingMs = clipboardPrompt.expiresAt - Date.now();
    if (remainingMs <= 0) {
      setClipboardPrompt(null);
      return;
    }

    clipboardPromptClearTimeoutRef.current = window.setTimeout(() => {
      clipboardPromptClearTimeoutRef.current = null;
      setClipboardPrompt(null);
    }, remainingMs);

    return () => {
      if (clipboardPromptClearTimeoutRef.current !== null) {
        window.clearTimeout(clipboardPromptClearTimeoutRef.current);
        clipboardPromptClearTimeoutRef.current = null;
      }
    };
  }, [clipboardPrompt]);

  useEffect(() => {
    const currentWindow = getCurrentWindow();

    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }

    let cleanup: (() => void) | null = null;
    let disposed = false;

    void currentWindow
      .listen<ShellBallSelectionSnapshotPayload>(shellBallWindowSyncEvents.selectionSnapshot, ({ payload }) => {
        if (payload.snapshot !== null) {
          if (selectionPromptClearTimeoutRef.current !== null) {
            window.clearTimeout(selectionPromptClearTimeoutRef.current);
            selectionPromptClearTimeoutRef.current = null;
          }

          setSelectionPrompt(payload.snapshot);
          return;
        }

        if (selectionPromptClearTimeoutRef.current !== null) {
          window.clearTimeout(selectionPromptClearTimeoutRef.current);
        }

        selectionPromptClearTimeoutRef.current = window.setTimeout(() => {
          selectionPromptClearTimeoutRef.current = null;
          setSelectionPrompt(null);
        }, SHELL_BALL_SELECTION_PROMPT_CLEAR_DELAY_MS);
      })
      .then((unlisten) => {
        if (disposed) {
          unlisten();
          return;
        }

        cleanup = unlisten;
      });

    return () => {
      disposed = true;
      if (selectionPromptClearTimeoutRef.current !== null) {
        window.clearTimeout(selectionPromptClearTimeoutRef.current);
        selectionPromptClearTimeoutRef.current = null;
      }
      cleanup?.();
    };
  }, []);

  useEffect(() => {
    const currentWindow = getCurrentWindow();

    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }

    let cleanup: (() => void) | null = null;
    let disposed = false;

    void currentWindow
      .listen<ShellBallClipboardSnapshotPayload>(shellBallWindowSyncEvents.clipboardSnapshot, ({ payload }) => {
        if (payload.text.trim() === "") {
          setClipboardPrompt(null);
          return;
        }

        setClipboardPrompt({
          text: payload.text,
          expiresAt: Date.now() + SHELL_BALL_CLIPBOARD_PROMPT_WINDOW_MS,
        });
      })
      .then((unlisten) => {
        if (disposed) {
          unlisten();
          return;
        }

        cleanup = unlisten;
      });

    return () => {
      disposed = true;
      cleanup?.();
    };
  }, []);

  const handleMascotPrimaryAction = useCallback(() => {
    if (selectionPrompt !== null) {
      if (selectionPromptClearTimeoutRef.current !== null) {
        window.clearTimeout(selectionPromptClearTimeoutRef.current);
        selectionPromptClearTimeoutRef.current = null;
      }

      setSelectionPrompt(null);
      void handleCoordinatorSelectedTextPrompt(selectionPrompt);
      return;
    }

    if (clipboardPrompt !== null) {
      if (isShellBallClipboardPromptActive(clipboardPrompt)) {
        setClipboardPrompt(null);
        void handleCoordinatorClipboardPrompt(clipboardPrompt.text);
        return;
      }

      setClipboardPrompt(null);
    }

    if (shouldFocusShellBallInlineInputBeforePrimaryClick({ inputValue, pendingFiles })) {
      focusInlineInputField();
      return;
    }

    void handleCoordinatorPrimaryAction("primary_click");
  }, [
    clipboardPrompt,
    focusInlineInputField,
    handleCoordinatorClipboardPrompt,
    handleCoordinatorPrimaryAction,
    handleCoordinatorSelectedTextPrompt,
    inputValue,
    pendingFiles,
    selectionPrompt,
  ]);

  const handleDockAwareRegionEnter = useCallback(() => {
    setEdgeDockRevealed(true);
    handleCoordinatorRegionEnter();
  }, [handleCoordinatorRegionEnter, setEdgeDockRevealed]);

  const handleDockAwareRegionLeave = useCallback(() => {
    setEdgeDockRevealed(false);
    handleCoordinatorRegionLeave();
  }, [handleCoordinatorRegionLeave, setEdgeDockRevealed]);

  return (
    <ShellBallSurface
      containerRef={rootRef}
      dashboardTransitionPhase={dashboardTransitionPhase}
      edgeDockRevealed={edgeDockState.revealed}
      edgeDockSide={edgeDockState.side}
      mascotRef={mascotRef}
      fileDropActive={shouldShowShellBallFileDropOverlay({
        fileDropActive,
      })}
      topContent={isEdgeDocked ? null : (
        <div className="shell-ball-surface__bubble-reserve" data-visible={snapshot.visibility.bubble && visibleBubbleItems.length > 0 ? "true" : "false"}>
          <div className="shell-ball-surface__bubble-reserve-content">
            {snapshot.visibility.bubble && visibleBubbleItems.length > 0 ? (
              <div
                className="shell-ball-window shell-ball-window--bubble"
                data-shell-ball-interactive="true"
                data-visibility-phase={snapshot.bubbleRegion.visibilityPhase}
                onPointerEnter={() => {
                  handleCoordinatorBubbleHoverChange(true);
                }}
                onPointerLeave={() => {
                  handleCoordinatorBubbleHoverChange(false);
                }}
              >
                <ShellBallBubbleZone
                  visualState={snapshot.visualState}
                  bubbleItems={visibleBubbleItems}
                  onAllowApprovalBubble={(bubbleId) => {
                    handleCoordinatorBubbleAction({ action: "allow_approval", bubbleId, source: "bubble" });
                  }}
                  onDeleteBubble={(bubbleId) => {
                    handleCoordinatorBubbleAction({ action: "delete", bubbleId, source: "bubble" });
                  }}
                  onDenyApprovalBubble={(bubbleId) => {
                    handleCoordinatorBubbleAction({ action: "deny_approval", bubbleId, source: "bubble" });
                  }}
                  onConfirmIntentBubble={handleCoordinatorConfirmIntentBubble}
                  onAcceptErrorSignalBubble={handleCoordinatorErrorSignalAccept}
                  onIgnoreErrorSignalBubble={handleCoordinatorErrorSignalIgnore}
                  onAcceptRecommendationBubble={handleCoordinatorRecommendationAccept}
                  onIgnoreRecommendationBubble={handleCoordinatorRecommendationIgnore}
                  onPinBubble={(bubbleId) => {
                    handleCoordinatorBubbleAction({ action: "pin", bubbleId, source: "bubble" });
                  }}
                />
              </div>
            ) : null}
          </div>
        </div>
      )}
      overlayContent={!isEdgeDocked && snapshot.visibility.voice ? <div className="shell-ball-voice-window"><ShellBallVoiceHints hintMode={snapshot.voiceHintMode} voicePreview={snapshot.voicePreview} /></div> : null}
      bottomContent={!isEdgeDocked && shouldRenderInlineInput ? (
        <div
          className="shell-ball-window shell-ball-window--input"
          data-shell-ball-input-window="true"
          data-shell-ball-interactive="true"
          onPointerEnter={() => {
            handleCoordinatorInputHoverChange(true);
          }}
          onPointerLeave={() => {
            handleCoordinatorInputHoverChange(false);
          }}
        >
          <ShellBallAttachmentTray paths={pendingFiles} onRemove={handleRemovePendingFile} />
          <ShellBallInputBar
            focusToken={inputFocusToken}
            mode={inlineInputMode}
            voicePreview={snapshot.voicePreview}
            value={inputValue}
            hasPendingFiles={pendingFiles.length > 0}
            onValueChange={setInputValue}
            onAttachFile={handleInlineAttachFile}
            onSubmit={() => {
              void handleCoordinatorPrimaryAction("submit");
            }}
            onFocusChange={handleInlineInputFocusChange}
          />
        </div>
      ) : null}
      textDropActive={shouldArmShellBallTextDropTarget({
        fileDropActive,
        textDragActive,
        visualState,
      })}
      visualState={visualState}
      selectionIndicatorVisible={shouldShowShellBallSelectionIndicator({
        selection: selectionPrompt,
        visualState,
      })}
      voicePreview={voicePreview}
      voiceHoldProgress={voiceHoldProgress}
      motionConfig={motionConfig}
      onDragStart={(event) => {
        beginBallWindowPointerDrag({
          x: event.screenX,
          y: event.screenY,
        });
      }}
      onDragMove={(event) => {
        updateBallWindowPointerDrag({
          x: event.screenX,
          y: event.screenY,
        });
      }}
      onDragEnd={(event) => {
        void endBallWindowPointerDrag({
          x: event.screenX,
          y: event.screenY,
        });
      }}
      onDragCancel={(event) => {
        void endBallWindowPointerDrag({
          x: event.screenX,
          y: event.screenY,
        });
      }}
      onPrimaryClick={handleMascotPrimaryAction}
      onDoubleClick={handleDoubleClick}
      onRegionEnter={handleDockAwareRegionEnter}
      onRegionLeave={handleDockAwareRegionLeave}
      onTextDrop={handleSurfaceTextDrop}
      inputFocused={inputFocused}
      onPressStart={handleLockedPressStart}
      onPressMove={handlePressMove}
      onPressEnd={handleLockedPressEnd}
      onPressCancel={handleLockedPressCancel}
    >
      {onboardingLoading ? <div className="shell-ball-onboarding-loading">{onboardingLoading.message}</div> : null}
    </ShellBallSurface>
  );
}
