/**
 * Shell-ball app renders the floating mascot window and coordinates helper
 * windows, drag/drop affordances, and dashboard transitions around it.
 */
import { useCallback, useEffect, useRef, useState } from "react";
import { useEventListener } from "ahooks";
import { getCurrentWindow, monitorFromPoint } from "@tauri-apps/api/window";
import { ShellBallSurface, shouldAcceptShellBallTextDrop } from "./ShellBallSurface";
import type { ShellBallSelectionSnapshot } from "./selection/selection.types";
import { useShellBallInteraction } from "./useShellBallInteraction";
import { getShellBallMotionConfig } from "./shellBall.motion";
import type { ShellBallVisualState } from "./shellBall.types";
import { emitShellBallInputRequestFocus, useShellBallCoordinator } from "./useShellBallCoordinator";
import { useShellBallWindowMetrics } from "./useShellBallWindowMetrics";
import {
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
import { openOrFocusDesktopWindow } from "../../platform/windowController";

type ShellBallAppProps = {
  isDev?: boolean;
};

type ShellBallDashboardTransitionPhase = "idle" | "opening" | "hidden" | "closing";

type ShellBallWindowAnchor = {
  x: number;
  y: number;
};

const SHELL_BALL_DASHBOARD_TRANSITION_DURATION_MS = 260;
const SHELL_BALL_SELECTION_PROMPT_CLEAR_DELAY_MS = 240;
const SHELL_BALL_CLIPBOARD_PROMPT_WINDOW_MS = 10_000;

type ShellBallClipboardPrompt = {
  text: string;
  expiresAt: number;
};

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
    handlePrimaryClick,
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
    ensureConversationSessionId,
    handleForceState,
  } = useShellBallInteraction();
  const motionConfig = getShellBallMotionConfig(visualState);
  const [dashboardTransitionPhase, setDashboardTransitionPhase] = useState<ShellBallDashboardTransitionPhase>("idle");
  const [fileDropActive, setFileDropActive] = useState(false);
  const [textDragActive, setTextDragActive] = useState(false);
  const [selectionPrompt, setSelectionPrompt] = useState<ShellBallSelectionSnapshot | null>(null);
  const [clipboardPrompt, setClipboardPrompt] = useState<ShellBallClipboardPrompt | null>(null);
  const anchorRef = useRef<ShellBallWindowAnchor | null>(null);
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
  const shellBallWindowTarget = typeof window === "undefined" ? undefined : window;
  const {
    handleClipboardPrompt: handleCoordinatorClipboardPrompt,
    handleDroppedFiles: handleCoordinatorDroppedFiles,
    handleSelectedTextPrompt: handleCoordinatorSelectedTextPrompt,
    handleRegionEnter: handleCoordinatorRegionEnter,
    handleRegionLeave: handleCoordinatorRegionLeave,
    snapshot,
  } = useShellBallCoordinator({
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
    ensureConversationSessionId: ensureConversationSessionId,
    onAttachFile: handleAttachFile,
    onPrimaryClick: handlePrimaryClick,
  });
  const {
    beginBallWindowPointerDrag,
    endBallWindowPointerDrag,
    freezeBallWindowPointerDrag,
    rootRef,
    updateBallWindowPointerDrag,
    windowFrame,
  } = useShellBallWindowMetrics({
    role: "ball",
    helperVisibility: snapshot.visibility,
  });

  dragDropHandlersRef.current = {
    handleDroppedFiles: handleCoordinatorDroppedFiles,
  };

  useEffect(() => {
    const wasVoiceActive =
      previousVisualStateRef.current === "voice_listening" || previousVisualStateRef.current === "voice_locked";
    const isVoiceActive = visualState === "voice_listening" || visualState === "voice_locked";

    // Voice gestures should operate against a stationary orb once capture starts.
    if (!wasVoiceActive && isVoiceActive) {
      void freezeBallWindowPointerDrag();
    }

    previousVisualStateRef.current = visualState;
  }, [freezeBallWindowPointerDrag, visualState]);

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

      const transitionFrame = await resolveShellBallDashboardTransitionFrame(windowFrame);
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

      const transitionFrame = await resolveShellBallDashboardTransitionFrame(windowFrame);
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
  }, [windowFrame]);

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
        case "drop":
          setFileDropActive(false);
          if (event.payload.paths.length === 0) {
            return;
          }

          void dragDropHandlersRef.current.handleDroppedFiles(event.payload.paths);
          return;
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

  function handleDoubleClick() {
    if (!shouldOpenDashboardFromDoubleClick) {
      return;
    }

    void openOrFocusDesktopWindow("dashboard");
  }

  const handleSurfaceTextDrop = useCallback((text: string) => {
    handleDroppedText(text);
    window.requestAnimationFrame(() => {
      void emitShellBallInputRequestFocus(Date.now());
    });
  }, [handleDroppedText]);

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
      handleInputFocusRequest();
      handleCoordinatorSelectedTextPrompt(selectionPrompt.text);
      void emitShellBallInputRequestFocus(Date.now());
      return;
    }

    if (clipboardPrompt !== null) {
      if (!isShellBallClipboardPromptActive(clipboardPrompt)) {
        setClipboardPrompt(null);
        return;
      }

      setClipboardPrompt(null);
      void handleCoordinatorClipboardPrompt(clipboardPrompt.text);
      return;
    }

    handlePrimaryClick();
  }, [clipboardPrompt, handleCoordinatorClipboardPrompt, handleCoordinatorSelectedTextPrompt, handleInputFocusRequest, handlePrimaryClick, selectionPrompt]);

  return (
    <ShellBallSurface
      containerRef={rootRef}
      dashboardTransitionPhase={dashboardTransitionPhase}
      fileDropActive={shouldShowShellBallFileDropOverlay({
        fileDropActive,
      })}
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
      onRegionEnter={handleCoordinatorRegionEnter}
      onRegionLeave={handleCoordinatorRegionLeave}
      onTextDrop={handleSurfaceTextDrop}
      inputFocused={inputFocused}
      onInputProxyClick={() => {
        handleInputFocusRequest();
        void emitShellBallInputRequestFocus(Date.now());
      }}
      onPressStart={handlePressStart}
      onPressMove={handlePressMove}
      onPressEnd={handlePressEnd}
      onPressCancel={handlePressCancel}
    />
  );
}
