import { useCallback, useEffect, useMemo, useRef } from "react";
import type { PointerEvent as ReactPointerEvent } from "react";
import type { ShellBallInputBarMode, ShellBallVisualState } from "./shellBall.types";
import {
  getShellBallMousePosition,
  isTauriWindowEnvironment,
  setShellBallAlwaysOnTop,
  setShellBallIgnoreCursorEvents,
  setShellBallShadow,
  startShellBallDragging,
  syncShellBallWindowBounds,
  type ShellBallMousePosition,
  type ShellBallWindowBounds,
} from "@/platform/shellBallWindow";

type UseShellBallWindowOptions = {
  inputBarMode: ShellBallInputBarMode;
  visualState: ShellBallVisualState;
};

export function useShellBallWindow({ inputBarMode, visualState }: UseShellBallWindowOptions) {
  const surfaceRef = useRef<HTMLDivElement | null>(null);
  const contentRef = useRef<HTMLDivElement | null>(null);
  const interactionRef = useRef<HTMLDivElement | null>(null);
  const dragZoneRef = useRef<HTMLDivElement | null>(null);
  const lastBoundsRef = useRef<ShellBallWindowBounds | null>(null);
  const pendingBoundsRef = useRef<ShellBallWindowBounds | null>(null);
  const applyingRef = useRef(false);
  const ignoredRef = useRef(false);
  const pollingTimerRef = useRef<number | null>(null);
  const resizeFrameRef = useRef<number | null>(null);
  const passThroughEligible = useMemo(() => visualState === "idle" && inputBarMode === "hidden", [inputBarMode, visualState]);

  const clearPollingTimer = useCallback(() => {
    if (pollingTimerRef.current) {
      window.clearInterval(pollingTimerRef.current);
      pollingTimerRef.current = null;
    }
  }, []);

  const setIgnoreState = useCallback(async (ignore: boolean) => {
    if (ignoredRef.current === ignore) {
      return;
    }

    ignoredRef.current = ignore;
    await setShellBallIgnoreCursorEvents(ignore, true);
  }, []);

  const applyBounds = useCallback(async (bounds: ShellBallWindowBounds) => {
    pendingBoundsRef.current = bounds;
    if (applyingRef.current) {
      return;
    }

    applyingRef.current = true;
    while (pendingBoundsRef.current) {
      const next = pendingBoundsRef.current;
      pendingBoundsRef.current = null;
      lastBoundsRef.current = await syncShellBallWindowBounds(next, lastBoundsRef.current);
    }
    applyingRef.current = false;
  }, []);

  const scheduleMeasure = useCallback(() => {
    if (resizeFrameRef.current) {
      window.cancelAnimationFrame(resizeFrameRef.current);
    }

    resizeFrameRef.current = window.requestAnimationFrame(() => {
      const element = contentRef.current ?? surfaceRef.current;
      if (!element) {
        return;
      }

      const rect = element.getBoundingClientRect();
      if (rect.width <= 0 || rect.height <= 0) {
        return;
      }

      void applyBounds({ width: rect.width, height: rect.height });
    });
  }, [applyBounds]);

  const containsMouse = useCallback((element: HTMLElement | null, position: ShellBallMousePosition | null) => {
    if (!element || !position) {
      return false;
    }

    const rect = element.getBoundingClientRect();
    const scale = window.devicePixelRatio || 1;
    const mouseX = position.client_x / scale;
    const mouseY = position.client_y / scale;
    const left = window.screenX + rect.left;
    const right = window.screenX + rect.right;
    const top = window.screenY + rect.top;
    const bottom = window.screenY + rect.bottom;

    return mouseX >= left && mouseX <= right && mouseY >= top && mouseY <= bottom;
  }, []);

  useEffect(() => {
    if (!isTauriWindowEnvironment()) {
      return;
    }

    void setShellBallAlwaysOnTop(true);
    void setShellBallShadow(false);
    void setIgnoreState(false);
  }, [setIgnoreState]);

  useEffect(() => {
    const element = contentRef.current ?? surfaceRef.current;
    if (!element) {
      return;
    }

    const observer = new ResizeObserver(() => {
      scheduleMeasure();
    });

    observer.observe(element);
    scheduleMeasure();

    return () => {
      observer.disconnect();
      if (resizeFrameRef.current) {
        window.cancelAnimationFrame(resizeFrameRef.current);
      }
    };
  }, [scheduleMeasure]);

  useEffect(() => {
    clearPollingTimer();

    if (!passThroughEligible) {
      void setIgnoreState(false);
      return;
    }

    const tick = async () => {
      const mousePosition = await getShellBallMousePosition();
      const insideEntity =
        containsMouse(interactionRef.current, mousePosition) ||
        containsMouse(dragZoneRef.current, mousePosition);

      await setIgnoreState(!insideEntity);
    };

    void tick();
    pollingTimerRef.current = window.setInterval(() => {
      void tick();
    }, 80);

    return clearPollingTimer;
  }, [clearPollingTimer, containsMouse, passThroughEligible, setIgnoreState]);

  const handleInteractionEnter = useCallback(() => {
    void setIgnoreState(false);
  }, [setIgnoreState]);

  const handleInteractionLeave = useCallback(() => {
    if (!passThroughEligible) {
      return;
    }

    void setIgnoreState(true);
  }, [passThroughEligible, setIgnoreState]);

  const handleHostDragStart = useCallback((event: ReactPointerEvent<HTMLDivElement>) => {
    event.preventDefault();
    void setIgnoreState(false);
    void startShellBallDragging();
  }, [setIgnoreState]);

  useEffect(() => {
    return () => {
      clearPollingTimer();
      void setIgnoreState(false);
    };
  }, [clearPollingTimer, setIgnoreState]);

  return {
    contentRef,
    dragZoneRef,
    handleHostDragStart,
    handleInteractionEnter,
    handleInteractionLeave,
    interactionRef,
    surfaceRef,
  };
}
