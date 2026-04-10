import { useEffect, useRef, useState } from "react";
import { getCurrentWindow, monitorFromPoint, type Monitor } from "@tauri-apps/api/window";
import {
  createShellBallLogicalPosition,
  createShellBallLogicalSize,
  hideShellBallWindow,
  setShellBallWindowPosition,
  setShellBallWindowSize,
  shellBallWindowLabels,
  showShellBallWindow,
} from "../../platform/shellBallWindowController";
import { shellBallWindowSyncEvents, type ShellBallHelperWindowRole, type ShellBallWindowGeometry } from "./shellBall.windowSync";

export const SHELL_BALL_WINDOW_SAFE_MARGIN_PX = 12;
export const SHELL_BALL_WINDOW_GAP_PX = 12;

type ShellBallContentSize = {
  width: number;
  height: number;
};

type ShellBallWindowSize = {
  width: number;
  height: number;
};

type ShellBallWindowFrame = ShellBallWindowSize & {
  x: number;
  y: number;
};

type ShellBallWindowBounds = {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
};

type UseShellBallWindowMetricsInput = {
  role: "ball" | ShellBallHelperWindowRole;
  visible?: boolean;
};

export function createShellBallWindowFrame(
  contentSize: ShellBallContentSize,
  safeMargin = SHELL_BALL_WINDOW_SAFE_MARGIN_PX,
): ShellBallWindowSize {
  return {
    width: Math.ceil(contentSize.width + safeMargin * 2),
    height: Math.ceil(contentSize.height + safeMargin * 2),
  };
}

export function getShellBallBubbleAnchor(input: {
  ballFrame: ShellBallWindowFrame;
  helperFrame: ShellBallWindowSize;
  gap?: number;
}) {
  const gap = input.gap ?? SHELL_BALL_WINDOW_GAP_PX;

  return {
    x: Math.round(input.ballFrame.x + input.ballFrame.width / 2 - input.helperFrame.width / 2),
    y: Math.round(input.ballFrame.y - gap - input.helperFrame.height),
  };
}

export function getShellBallInputAnchor(input: {
  ballFrame: ShellBallWindowFrame;
  helperFrame: ShellBallWindowSize;
  gap?: number;
}) {
  const gap = input.gap ?? SHELL_BALL_WINDOW_GAP_PX;

  return {
    x: Math.round(input.ballFrame.x + input.ballFrame.width / 2 - input.helperFrame.width / 2),
    y: Math.round(input.ballFrame.y + input.ballFrame.height + gap),
  };
}

export function clampShellBallFrameToBounds(
  frame: ShellBallWindowFrame,
  bounds: ShellBallWindowBounds,
): ShellBallWindowFrame {
  const maxX = Math.max(bounds.minX, bounds.maxX - frame.width);
  const maxY = Math.max(bounds.minY, bounds.maxY - frame.height);

  return {
    ...frame,
    x: Math.min(Math.max(frame.x, bounds.minX), maxX),
    y: Math.min(Math.max(frame.y, bounds.minY), maxY),
  };
}

function getShellBallBoundsFromMonitor(monitor: Monitor | null, geometry: ShellBallWindowGeometry | null): ShellBallWindowBounds {
  if (monitor === null) {
    return geometry?.bounds ?? {
      minX: -10000,
      minY: -10000,
      maxX: 10000,
      maxY: 10000,
    };
  }

  const logicalPosition = monitor.workArea.position.toLogical(monitor.scaleFactor);
  const logicalSize = monitor.workArea.size.toLogical(monitor.scaleFactor);

  return {
    minX: logicalPosition.x,
    minY: logicalPosition.y,
    maxX: logicalPosition.x + logicalSize.width,
    maxY: logicalPosition.y + logicalSize.height,
  };
}

export function useShellBallWindowMetrics({ role, visible = true }: UseShellBallWindowMetricsInput) {
  const rootRef = useRef<HTMLDivElement>(null);
  const [windowFrame, setWindowFrame] = useState<ShellBallWindowSize | null>(null);
  const geometryRef = useRef<ShellBallWindowGeometry | null>(null);

  useEffect(() => {
    const element = rootRef.current;
    if (element === null) {
      return;
    }

    function updateFrame() {
      const nextElement = rootRef.current;
      if (nextElement === null) {
        return;
      }

      const rect = nextElement.getBoundingClientRect();
      setWindowFrame(
        createShellBallWindowFrame({
          width: rect.width,
          height: rect.height,
        }),
      );
    }

    updateFrame();

    if (typeof ResizeObserver === "undefined") {
      return;
    }

    const observer = new ResizeObserver(() => {
      updateFrame();
    });

    observer.observe(element);

    return () => {
      observer.disconnect();
    };
  }, []);

  useEffect(() => {
    if (windowFrame === null) {
      return;
    }

    void setShellBallWindowSize(role, createShellBallLogicalSize(windowFrame.width, windowFrame.height));
  }, [role, windowFrame]);

  useEffect(() => {
    if (role !== "ball" || windowFrame === null) {
      return;
    }

    const currentWindow = getCurrentWindow();
    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }
    const ballFrame = windowFrame;

    let disposed = false;
    let cleanupFns: Array<() => void> = [];

    async function publishGeometry() {
      const physicalPosition = await currentWindow.outerPosition();
      const scaleFactor = await currentWindow.scaleFactor();
      const monitor = await monitorFromPoint(physicalPosition.x, physicalPosition.y);
      const logicalPosition = physicalPosition.toLogical(scaleFactor);
      const geometry: ShellBallWindowGeometry = {
        ballFrame: {
          x: logicalPosition.x,
          y: logicalPosition.y,
          width: ballFrame.width,
          height: ballFrame.height,
        },
        bounds: getShellBallBoundsFromMonitor(monitor, geometryRef.current),
        scaleFactor,
      };

      geometryRef.current = geometry;

      await Promise.all([
        currentWindow.emitTo(shellBallWindowLabels.bubble, shellBallWindowSyncEvents.geometry, geometry),
        currentWindow.emitTo(shellBallWindowLabels.input, shellBallWindowSyncEvents.geometry, geometry),
      ]);
    }

    void publishGeometry();

    void Promise.all([
      currentWindow.onMoved(() => {
        void publishGeometry();
      }),
      currentWindow.onResized(() => {
        void publishGeometry();
      }),
      currentWindow.listen(shellBallWindowSyncEvents.helperReady, () => {
        void publishGeometry();
      }),
    ]).then((unlisteners) => {
      if (disposed) {
        for (const unlisten of unlisteners) {
          unlisten();
        }
        return;
      }

      cleanupFns = unlisteners;
    });

    return () => {
      disposed = true;
      for (const cleanup of cleanupFns) {
        cleanup();
      }
    };
  }, [role, windowFrame]);

  useEffect(() => {
    if (role === "ball" || windowFrame === null) {
      return;
    }

    const currentWindow = getCurrentWindow();
    if (currentWindow.label !== shellBallWindowLabels[role]) {
      return;
    }
    const helperFrame = windowFrame;

    if (role === "bubble") {
      void currentWindow.setFocusable(false);
    }

    let cleanup: (() => void) | null = null;
    let disposed = false;

    async function applyGeometry(geometry: ShellBallWindowGeometry) {
      geometryRef.current = geometry;

      const anchor =
        role === "bubble"
          ? getShellBallBubbleAnchor({
              ballFrame: geometry.ballFrame,
              helperFrame,
            })
          : getShellBallInputAnchor({
              ballFrame: geometry.ballFrame,
              helperFrame,
            });

      const nextFrame = clampShellBallFrameToBounds(
        {
          x: anchor.x,
          y: anchor.y,
          width: helperFrame.width,
          height: helperFrame.height,
        },
        geometry.bounds,
      );

      await setShellBallWindowPosition(role, createShellBallLogicalPosition(nextFrame.x, nextFrame.y));

      if (visible) {
        await showShellBallWindow(role);
        return;
      }

      await hideShellBallWindow(role);
    }

    void currentWindow
      .listen<ShellBallWindowGeometry>(shellBallWindowSyncEvents.geometry, ({ payload }) => {
        void applyGeometry(payload);
      })
      .then((unlisten) => {
        if (disposed) {
          unlisten();
          return;
        }

        cleanup = unlisten;

        if (geometryRef.current !== null) {
          void applyGeometry(geometryRef.current);
        }

        void currentWindow.emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.helperReady, { role });
      });

    return () => {
      disposed = true;
      cleanup?.();
    };
  }, [role, visible, windowFrame]);

  useEffect(() => {
    if (role === "ball") {
      return;
    }

    if (!visible) {
      void hideShellBallWindow(role);
      return;
    }

    if (geometryRef.current === null || windowFrame === null) {
      return;
    }
    const helperFrame = windowFrame;

    const anchor =
      role === "bubble"
        ? getShellBallBubbleAnchor({
            ballFrame: geometryRef.current.ballFrame,
            helperFrame,
          })
        : getShellBallInputAnchor({
            ballFrame: geometryRef.current.ballFrame,
            helperFrame,
          });

    const nextFrame = clampShellBallFrameToBounds(
        {
          x: anchor.x,
          y: anchor.y,
          width: helperFrame.width,
          height: helperFrame.height,
        },
      geometryRef.current.bounds,
    );

    void (async () => {
      await setShellBallWindowPosition(role, createShellBallLogicalPosition(nextFrame.x, nextFrame.y));
      await showShellBallWindow(role);
    })();
  }, [role, visible, windowFrame]);

  return { rootRef, windowFrame };
}
