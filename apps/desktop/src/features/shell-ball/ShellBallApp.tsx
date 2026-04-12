import { useEffect, useRef, useState } from "react";
import { getCurrentWindow, monitorFromPoint } from "@tauri-apps/api/window";
import { ShellBallDevLayer } from "./ShellBallDevLayer";
import { shouldShowShellBallDemoSwitcher } from "./shellBall.dev";
import { ShellBallSurface } from "./ShellBallSurface";
import { useShellBallInteraction } from "./useShellBallInteraction";
import { getShellBallMotionConfig } from "./shellBall.motion";
import { emitShellBallInputRequestFocus, useShellBallCoordinator } from "./useShellBallCoordinator";
import { useShellBallWindowMetrics } from "./useShellBallWindowMetrics";
import type { ShellBallDashboardTransitionRequest } from "../../platform/dashboardWindowTransition";
import { shellBallDashboardTransitionEvents } from "../../platform/dashboardWindowTransition";
import {
  createShellBallLogicalPosition,
  hideShellBallWindow,
  shellBallWindowLabels,
  showShellBallWindow,
  startShellBallWindowDragging,
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

function waitForAnimationFrame() {
  return new Promise<void>((resolve) => {
    window.requestAnimationFrame(() => resolve());
  });
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
  const {
    visualState,
    inputValue,
    finalizedSpeechPayload,
    voicePreview,
    voiceHoldProgress,
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
    handleAttachFile,
    handleInputFocusChange,
    handleInputFocusRequest,
    setInputValue,
    acknowledgeFinalizedSpeechPayload,
    handleForceState,
  } = useShellBallInteraction();
  const motionConfig = getShellBallMotionConfig(visualState);
  const showDemoSwitcher = shouldShowShellBallDemoSwitcher(isDev);
  const { rootRef, windowFrame } = useShellBallWindowMetrics({ role: "ball" });
  const [dashboardTransitionPhase, setDashboardTransitionPhase] = useState<ShellBallDashboardTransitionPhase>("idle");
  const anchorRef = useRef<ShellBallWindowAnchor | null>(null);
  const dashboardTransitionPhaseRef = useRef<ShellBallDashboardTransitionPhase>("idle");
  const transitionQueueRef = useRef(Promise.resolve());

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
          .catch(() => undefined)
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

  function handleDoubleClick() {
    if (!shouldOpenDashboardFromDoubleClick) {
      return;
    }

    void openOrFocusDesktopWindow("dashboard");
  }

  useShellBallCoordinator({
    visualState,
    helperWindowsVisible: dashboardTransitionPhase === "idle",
    inputValue,
    finalizedSpeechPayload,
    voicePreview,
    setInputValue,
    onFinalizedSpeechHandled: acknowledgeFinalizedSpeechPayload,
    onRegionEnter: handleRegionEnter,
    onRegionLeave: handleRegionLeave,
    onInputFocusChange: handleInputFocusChange,
    onSubmitText: handleSubmitText,
    onAttachFile: handleAttachFile,
    onPrimaryClick: handlePrimaryClick,
  });

  return (
    <ShellBallSurface
      containerRef={rootRef}
      dashboardTransitionPhase={dashboardTransitionPhase}
      visualState={visualState}
      voicePreview={voicePreview}
      voiceHoldProgress={voiceHoldProgress}
      motionConfig={motionConfig}
      onDragStart={() => {
        void startShellBallWindowDragging();
      }}
      onPrimaryClick={handlePrimaryClick}
      onDoubleClick={handleDoubleClick}
      onRegionEnter={handleRegionEnter}
      onRegionLeave={handleRegionLeave}
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
