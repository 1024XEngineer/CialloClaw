import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { getCurrentWindow, monitorFromPoint, type Monitor } from "@tauri-apps/api/window";
import {
  applyShellBallCurrentWindowFrame,
  createShellBallLogicalPosition,
  createShellBallLogicalSize,
  setShellBallWindowSize,
  shellBallWindowLabels,
} from "../../platform/shellBallWindowController";
import {
  type ShellBallHelperWindowRole,
  type ShellBallHelperWindowVisibility,
  type ShellBallWindowGeometry,
} from "./shellBall.windowSync";

type AnchoredShellBallHelperWindowRole = Exclude<ShellBallHelperWindowRole, "pinned">;

export const SHELL_BALL_WINDOW_SAFE_MARGIN_PX = 12;
export const SHELL_BALL_BUBBLE_GAP_PX = 6;
export const SHELL_BALL_BUBBLE_DRAG_CLEARANCE_PX = 24;
export const SHELL_BALL_BUBBLE_REPOSITION_DURATION_MS = 180;
export const SHELL_BALL_INPUT_GAP_PX = 4;
export const SHELL_BALL_COMPACT_WINDOW_SAFE_MARGIN_PX = 50;
const SHELL_BALL_INITIAL_RIGHT_MARGIN_PX = 18;
const SHELL_BALL_INITIAL_BOTTOM_MARGIN_PX = 26;
const SHELL_BALL_EDGE_DOCK_SNAP_THRESHOLD_PX = 30;
const SHELL_BALL_EDGE_DOCK_HORIZONTAL_ANIMATION_DURATION_MS = 180;
const SHELL_BALL_EDGE_DOCK_VERTICAL_ANIMATION_DURATION_MS = 220;
const SHELL_BALL_EDGE_DOCK_HORIZONTAL_OVERSHOOT_PX = 6;
const SHELL_BALL_EDGE_DOCK_VERTICAL_OVERSHOOT_PX = 8;
const SHELL_BALL_EDGE_DOCK_OVERSHOOT_PROGRESS = 0.82;
const SHELL_BALL_EDGE_DOCK_TOP_HIDDEN_RATIO = 0.18;
const SHELL_BALL_EDGE_DOCK_BOTTOM_HIDDEN_RATIO = 0.28;

type ShellBallContentSize = {
  width: number;
  height: number;
};

type ShellBallMeasurableElement = {
  getBoundingClientRect: () => {
    width: number;
    height: number;
  };
  scrollWidth: number;
  scrollHeight: number;
};

type ShellBallWindowSize = {
  width: number;
  height: number;
};

type ShellBallAnchorOffset = {
  x: number;
  y: number;
};

type ShellBallGlobalAnchor = {
  x: number;
  y: number;
};

type ShellBallRelativeFrame = {
  x: number;
  y: number;
  width: number;
  height: number;
};

type ShellBallWindowFrame = ShellBallWindowSize & {
  x: number;
  y: number;
};

type ShellBallPointerPosition = {
  x: number;
  y: number;
};

type ShellBallWindowBounds = {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
};

type ShellBallHorizontalDockSide = "left" | "right";
type ShellBallVerticalDockSide = "top" | "bottom";
type ShellBallCardinalDockSide = ShellBallHorizontalDockSide | ShellBallVerticalDockSide;

export type ShellBallEdgeDockSide =
  | ShellBallHorizontalDockSide
  | ShellBallVerticalDockSide
  | "top_left"
  | "top_right"
  | "bottom_left"
  | "bottom_right";

export type ShellBallEdgeDockState = {
  revealed: boolean;
  side: ShellBallEdgeDockSide | null;
};

type ShellBallDockAnimationMode = "dock" | "reveal";

type ShellBallDockAnimationAxisConfig = {
  direction: -1 | 1;
  overshootPx: number;
};

type ShellBallDockAnimationConfig = {
  durationMs: number;
  x?: ShellBallDockAnimationAxisConfig;
  y?: ShellBallDockAnimationAxisConfig;
};

type UseShellBallWindowMetricsInput = {
  role: "ball" | AnchoredShellBallHelperWindowRole;
  visible?: boolean;
  clickThrough?: boolean;
  helperVisibility?: ShellBallHelperWindowVisibility;
};

type ShellBallHelperWindowInteractionMode = {
  focusable: boolean;
  ignoreCursorEvents: boolean;
};

type ShellBallBallDragSession = {
  pointerStart: ShellBallPointerPosition;
  latestPointer: ShellBallPointerPosition;
  frameStart: ShellBallWindowFrame;
  originBounds: ShellBallWindowBounds;
};

function resolveShellBallInitialGlobalAnchor(input: {
  bounds: ShellBallWindowBounds;
  mascotFrame: ShellBallRelativeFrame | null;
}) {
  const mascotFrame = input.mascotFrame;
  if (mascotFrame === null) {
    return null;
  }

  return {
    x: Math.round(input.bounds.maxX - SHELL_BALL_INITIAL_RIGHT_MARGIN_PX - mascotFrame.width),
    y: Math.round(input.bounds.maxY - SHELL_BALL_INITIAL_BOTTOM_MARGIN_PX - mascotFrame.height),
  } satisfies ShellBallGlobalAnchor;
}

export function createShellBallWindowGeometry(input: {
  position: {
    x: number;
    y: number;
  };
  size: {
    width: number;
    height: number;
  };
  bounds: {
    minX: number;
    minY: number;
    maxX: number;
    maxY: number;
  };
  scaleFactor: number;
  clampToBounds?: boolean;
}): ShellBallWindowGeometry {
  const nextFrame = {
    x: Math.round(input.position.x),
    y: Math.round(input.position.y),
    width: input.size.width,
    height: input.size.height,
  };

  return {
    ballFrame: input.clampToBounds === false ? nextFrame : clampShellBallFrameToBounds(nextFrame, input.bounds),
    bounds: input.bounds,
    scaleFactor: input.scaleFactor,
  };
}

export function createShellBallWindowFrame(
  contentSize: ShellBallContentSize,
  safeMargin = SHELL_BALL_WINDOW_SAFE_MARGIN_PX,
): ShellBallWindowSize {
  return {
    width: Math.ceil(contentSize.width + safeMargin * 2),
    height: Math.ceil(contentSize.height + safeMargin * 2),
  };
}

export function measureShellBallContentSize(element: ShellBallMeasurableElement, includeScrollBounds = true): ShellBallContentSize {
  const rect = element.getBoundingClientRect();
  const hasHTMLElement = typeof HTMLElement !== "undefined";

  if (hasHTMLElement && element instanceof HTMLElement && element.classList.contains("shell-ball-surface")) {
    // The merged ball window measures only stable anchor wrappers so visual
    // nudges inside those wrappers never feed back into the native frame.
    const measuredRegions = [
      element.querySelector<HTMLElement>(".shell-ball-surface__slot--top"),
      element.querySelector<HTMLElement>(".shell-ball-surface__mascot-shell"),
      element.querySelector<HTMLElement>(".shell-ball-surface__slot--bottom"),
      element.querySelector<HTMLElement>(".shell-ball-surface__voice-anchor"),
    ].filter((region): region is HTMLElement => region !== null);

    if (measuredRegions.length > 0) {
      const regionRects = measuredRegions
        .map((region) => region.getBoundingClientRect())
        .filter((regionRect) => regionRect.width > 0 && regionRect.height > 0);

      if (regionRects.length > 0) {
        const minLeft = Math.min(...regionRects.map((regionRect) => regionRect.left));
        const minTop = Math.min(...regionRects.map((regionRect) => regionRect.top));
        const maxRight = Math.max(...regionRects.map((regionRect) => regionRect.right));
        const maxBottom = Math.max(...regionRects.map((regionRect) => regionRect.bottom));

        return {
          width: maxRight - minLeft,
          height: maxBottom - minTop,
        };
      }

      return {
        width: Math.max(...measuredRegions.map((region) => Math.max(region.getBoundingClientRect().width, region.scrollWidth))),
        height: Math.max(...measuredRegions.map((region) => Math.max(region.getBoundingClientRect().height, region.scrollHeight))),
      };
    }
  }

  if (hasHTMLElement && element instanceof HTMLElement && element.dataset.shellBallInputWindow === "true") {
    const inputBoxes = Array.from(element.querySelectorAll<HTMLElement>(".shell-ball-uiverse-inputbox"));
    const actions = Array.from(element.querySelectorAll<HTMLElement>(".shell-ball-uiverse-actions"));

    if (inputBoxes.length > 0) {
      const contentWidth = Math.max(
        ...inputBoxes.map((inputBox) => inputBox.getBoundingClientRect().width),
        ...actions.map((actionRow) => actionRow.getBoundingClientRect().width),
        0,
      );

      return {
        width: contentWidth,
        height: includeScrollBounds ? Math.max(rect.height, element.scrollHeight) : rect.height,
      };
    }
  }

  return {
    width: includeScrollBounds ? Math.max(rect.width, element.scrollWidth) : rect.width,
    height: includeScrollBounds ? Math.max(rect.height, element.scrollHeight) : rect.height,
  };
}

export function getShellBallBubbleAnchor(input: {
  ballFrame: ShellBallWindowFrame;
  helperFrame: ShellBallWindowSize;
  gap?: number;
  clearance?: number;
}) {
  const gap = input.gap ?? SHELL_BALL_BUBBLE_GAP_PX;
  const clearance = input.clearance ?? SHELL_BALL_BUBBLE_DRAG_CLEARANCE_PX;

  return {
    x: Math.round(input.ballFrame.x + input.ballFrame.width / 2 - input.helperFrame.width / 2),
    y: Math.round(input.ballFrame.y - gap - clearance - input.helperFrame.height),
  };
}

export function getShellBallInputAnchor(input: {
  ballFrame: ShellBallWindowFrame;
  helperFrame: ShellBallWindowSize;
  gap?: number;
}) {
  const gap = input.gap ?? SHELL_BALL_INPUT_GAP_PX;

  return {
    x: Math.round(input.ballFrame.x + input.ballFrame.width / 2 - input.helperFrame.width / 2),
    y: Math.round(input.ballFrame.y + input.ballFrame.height + gap),
  };
}

export function getShellBallVoiceAnchor(input: {
  ballFrame: ShellBallWindowFrame;
  helperFrame: ShellBallWindowSize;
}) {
  return {
    x: Math.round(input.ballFrame.x + input.ballFrame.width / 2 - input.helperFrame.width / 2),
    y: Math.round(input.ballFrame.y + input.ballFrame.height / 2 - input.helperFrame.height / 2),
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

function clampShellBallAxisPosition(value: number, min: number, max: number) {
  if (max <= min) {
    return min;
  }

  return Math.min(Math.max(Math.round(value), min), max);
}

function easeOutCubic(progress: number) {
  return 1 - (1 - progress) ** 3;
}

function easeInOutCubic(progress: number) {
  if (progress < 0.5) {
    return 4 * progress ** 3;
  }

  return 1 - ((-2 * progress + 2) ** 3) / 2;
}

function interpolateShellBallFrame(startFrame: ShellBallWindowFrame, endFrame: ShellBallWindowFrame, progress: number): ShellBallWindowFrame {
  return {
    ...endFrame,
    x: Math.round(startFrame.x + (endFrame.x - startFrame.x) * progress),
    y: Math.round(startFrame.y + (endFrame.y - startFrame.y) * progress),
  };
}

/**
 * Returns how much of the mascot should stay outside the monitor while parked
 * on a given edge. Vertical docks keep the face visible instead of reusing the
 * side-dock half-hidden treatment.
 */
export function getShellBallParkedDockInsetPx(input: {
  side: ShellBallCardinalDockSide;
  mascotFrame: Pick<ShellBallRelativeFrame, "width" | "height">;
}) {
  if (input.side === "left" || input.side === "right") {
    return input.mascotFrame.width / 2;
  }

  if (input.side === "top") {
    return input.mascotFrame.height * SHELL_BALL_EDGE_DOCK_TOP_HIDDEN_RATIO;
  }

  return input.mascotFrame.height * SHELL_BALL_EDGE_DOCK_BOTTOM_HIDDEN_RATIO;
}

function resolveShellBallDockAxes(side: ShellBallEdgeDockSide | null): {
  horizontal: ShellBallHorizontalDockSide | null;
  vertical: ShellBallVerticalDockSide | null;
} {
  switch (side) {
    case "left":
      return { horizontal: "left" as const, vertical: null };
    case "right":
      return { horizontal: "right" as const, vertical: null };
    case "top":
      return { horizontal: null, vertical: "top" as const };
    case "bottom":
      return { horizontal: null, vertical: "bottom" as const };
    case "top_left":
      return { horizontal: "left" as const, vertical: "top" as const };
    case "top_right":
      return { horizontal: "right" as const, vertical: "top" as const };
    case "bottom_left":
      return { horizontal: "left" as const, vertical: "bottom" as const };
    case "bottom_right":
      return { horizontal: "right" as const, vertical: "bottom" as const };
    default:
      return { horizontal: null, vertical: null };
  }
}

function resolveShellBallDockSideFromAxes(input: {
  horizontal: ShellBallHorizontalDockSide | null;
  vertical: ShellBallVerticalDockSide | null;
}): ShellBallEdgeDockSide | null {
  if (input.horizontal === null && input.vertical === null) {
    return null;
  }

  if (input.horizontal === null) {
    return input.vertical;
  }

  if (input.vertical === null) {
    return input.horizontal;
  }

  if (input.horizontal === "left" && input.vertical === "top") {
    return "top_left";
  }

  if (input.horizontal === "right" && input.vertical === "top") {
    return "top_right";
  }

  if (input.horizontal === "left" && input.vertical === "bottom") {
    return "bottom_left";
  }

  return "bottom_right";
}

function resolveShellBallDockParkedInsets(input: {
  side: ShellBallEdgeDockSide;
  mascotFrame: Pick<ShellBallRelativeFrame, "width" | "height">;
}) {
  const axes = resolveShellBallDockAxes(input.side);

  return {
    horizontal: axes.horizontal === null
      ? 0
      : getShellBallParkedDockInsetPx({
          side: axes.horizontal,
          mascotFrame: input.mascotFrame,
        }),
    vertical: axes.vertical === null
      ? 0
      : getShellBallParkedDockInsetPx({
          side: axes.vertical,
          mascotFrame: input.mascotFrame,
        }),
  };
}

/**
 * Clamps the transparent host window back into a recoverable on-screen position
 * by using the mascot footprint instead of the full host frame. Free dragging
 * may leave bounds temporarily, but release-time settling and non-drag layout
 * paths still use this helper before the orb becomes stationary again.
 */
export function clampShellBallHostFrameToVisibleBounds(input: {
  hostFrame: ShellBallWindowFrame;
  bounds: ShellBallWindowBounds;
  mascotFrame: ShellBallRelativeFrame | null;
}): ShellBallWindowFrame {
  if (input.mascotFrame === null) {
    return clampShellBallFrameToBounds(input.hostFrame, input.bounds);
  }

  const minX = input.bounds.minX - input.mascotFrame.x;
  const maxX = input.bounds.maxX - input.mascotFrame.x - input.mascotFrame.width;
  const minY = input.bounds.minY - input.mascotFrame.y;
  const maxY = input.bounds.maxY - input.mascotFrame.y - input.mascotFrame.height;

  return {
    ...input.hostFrame,
    x: clampShellBallAxisPosition(input.hostFrame.x, minX, maxX),
    y: clampShellBallAxisPosition(input.hostFrame.y, minY, maxY),
  };
}

/**
 * Resolves the edge-specific snap animation profile. Dock animations can
 * overshoot slightly, while hover reveal animations stay on a direct easing
 * path without bounce.
 */
export function getShellBallDockAnimationConfig(input: {
  side: ShellBallEdgeDockSide;
  mode: ShellBallDockAnimationMode;
}): ShellBallDockAnimationConfig {
  const overshootX = input.mode === "dock" ? SHELL_BALL_EDGE_DOCK_HORIZONTAL_OVERSHOOT_PX : 0;
  const overshootY = input.mode === "dock" ? SHELL_BALL_EDGE_DOCK_VERTICAL_OVERSHOOT_PX : 0;

  if (input.side === "left" || input.side === "right") {
    return {
      durationMs: SHELL_BALL_EDGE_DOCK_HORIZONTAL_ANIMATION_DURATION_MS,
      x: {
        direction: input.side === "left" ? -1 : 1,
        overshootPx: overshootX,
      },
    };
  }

  if (input.side === "top" || input.side === "bottom") {
    return {
      durationMs: SHELL_BALL_EDGE_DOCK_VERTICAL_ANIMATION_DURATION_MS,
      y: {
        direction: input.side === "top" ? -1 : 1,
        overshootPx: overshootY,
      },
    };
  }

  return {
    durationMs: SHELL_BALL_EDGE_DOCK_VERTICAL_ANIMATION_DURATION_MS,
    x: {
      direction: input.side === "top_left" || input.side === "bottom_left" ? -1 : 1,
      overshootPx: overshootX,
    },
    y: {
      direction: input.side === "top_left" || input.side === "top_right" ? -1 : 1,
      overshootPx: overshootY,
    },
  };
}

function resolveShellBallDockAnimationOvershootFrame(input: {
  nextFrame: ShellBallWindowFrame;
  config: ShellBallDockAnimationConfig;
}) {
  return {
    ...input.nextFrame,
    x: input.nextFrame.x + (input.config.x?.direction ?? 0) * (input.config.x?.overshootPx ?? 0),
    y: input.nextFrame.y + (input.config.y?.direction ?? 0) * (input.config.y?.overshootPx ?? 0),
  };
}

/**
 * Determines whether the released shell-ball should settle into one of the
 * visible-edge parking states. Corner snaps win whenever the mascot lands
 * inside both threshold bands at once.
 */
export function resolveShellBallReleaseSnapTarget(input: {
  bounds: ShellBallWindowBounds;
  hostFrame: ShellBallWindowFrame;
  mascotFrame: ShellBallRelativeFrame | null;
  thresholdPx?: number;
}) {
  const mascotFrame = input.mascotFrame;

  if (mascotFrame === null) {
    return null;
  }

  const mascotLeft = input.hostFrame.x + mascotFrame.x;
  const mascotTop = input.hostFrame.y + mascotFrame.y;
  const mascotRight = mascotLeft + mascotFrame.width;
  const mascotBottom = mascotTop + mascotFrame.height;

  const thresholdPx = input.thresholdPx ?? SHELL_BALL_EDGE_DOCK_SNAP_THRESHOLD_PX;
  const leftDistance = Math.abs(mascotLeft - input.bounds.minX);
  const rightDistance = Math.abs(input.bounds.maxX - mascotRight);
  const topDistance = Math.abs(mascotTop - input.bounds.minY);
  const bottomDistance = Math.abs(input.bounds.maxY - mascotBottom);

  const horizontal = Math.min(leftDistance, rightDistance) <= thresholdPx
    ? leftDistance <= rightDistance
      ? "left"
      : "right"
    : null;
  const vertical = Math.min(topDistance, bottomDistance) <= thresholdPx
    ? topDistance <= bottomDistance
      ? "top"
      : "bottom"
    : null;

  return resolveShellBallDockSideFromAxes({
    horizontal,
    vertical,
  });
}

export function resolveShellBallDockedHostPosition(input: {
  bounds: ShellBallWindowBounds;
  currentPosition: { x: number; y: number };
  edgeDockState: ShellBallEdgeDockState;
  mascotFrame: ShellBallRelativeFrame | null;
}) {
  const mascotFrame = input.mascotFrame;

  if (mascotFrame === null || input.edgeDockState.side === null) {
    return input.currentPosition;
  }

  const axes = resolveShellBallDockAxes(input.edgeDockState.side);
  const parkedInsets = resolveShellBallDockParkedInsets({
    side: input.edgeDockState.side,
    mascotFrame,
  });
  const nextPosition = {
    ...input.currentPosition,
  };

  if (axes.horizontal === "left") {
    const targetMascotLeft = input.edgeDockState.revealed
      ? input.bounds.minX
      : input.bounds.minX - parkedInsets.horizontal;

    nextPosition.x = Math.round(targetMascotLeft - mascotFrame.x);
  } else if (axes.horizontal === "right") {
    const targetMascotRight = input.edgeDockState.revealed
      ? input.bounds.maxX
      : input.bounds.maxX + parkedInsets.horizontal;

    nextPosition.x = Math.round(targetMascotRight - mascotFrame.x - mascotFrame.width);
  }

  if (axes.vertical === "top") {
    const targetMascotTop = input.edgeDockState.revealed
      ? input.bounds.minY
      : input.bounds.minY - parkedInsets.vertical;

    nextPosition.y = Math.round(targetMascotTop - mascotFrame.y);
  } else if (axes.vertical === "bottom") {
    const targetMascotBottom = input.edgeDockState.revealed
      ? input.bounds.maxY
      : input.bounds.maxY + parkedInsets.vertical;

    nextPosition.y = Math.round(targetMascotBottom - mascotFrame.y - mascotFrame.height);
  }

  return nextPosition;
}

export function getShellBallHelperWindowInteractionMode(input: {
  role: AnchoredShellBallHelperWindowRole;
  visible: boolean;
  clickThrough: boolean;
}): ShellBallHelperWindowInteractionMode {
  if (input.role === "bubble") {
    return {
      focusable: !input.clickThrough && input.visible,
      ignoreCursorEvents: input.clickThrough || input.visible === false,
    };
  }

  if (input.role === "input") {
    return {
      focusable: input.visible && !input.clickThrough,
      ignoreCursorEvents: input.clickThrough || input.visible === false,
    };
  }

  if (input.role === "voice") {
    return {
      focusable: false,
      ignoreCursorEvents: true,
    };
  }

  return {
    focusable: true,
    ignoreCursorEvents: false,
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

export function useShellBallWindowMetrics({
  role,
  visible = true,
  clickThrough: _clickThrough = false,
  helperVisibility: _helperVisibility,
}: UseShellBallWindowMetricsInput) {
  const rootRef = useRef<HTMLDivElement>(null);
  const [windowFrame, setWindowFrame] = useState<ShellBallWindowSize | null>(null);
  const geometryRef = useRef<ShellBallWindowGeometry | null>(null);
  const ballDragSessionRef = useRef<ShellBallBallDragSession | null>(null);
  const ballDragMoveAnimationFrameRef = useRef<number | null>(null);
  const pendingBallDragFrameRef = useRef<ShellBallWindowFrame | null>(null);
  const ballDragPositionQueueRef = useRef<Promise<void> | null>(null);
  const ballGeometryEmitAnimationFrameRef = useRef<number | null>(null);
  const pendingBallGeometryRef = useRef<ShellBallWindowGeometry | null>(null);
  const ballGeometryPublishAnimationFrameRef = useRef<number | null>(null);
  const ballGeometryPublishSnapToBoundsRef = useRef(false);
  const helperWindowShouldBeVisibleRef = useRef(visible);
  const appliedWindowSizeRef = useRef<ShellBallWindowSize | null>(null);
  const measuredAnchorOffsetRef = useRef<ShellBallAnchorOffset | null>(null);
  const measuredMascotFrameRef = useRef<ShellBallRelativeFrame | null>(null);
  const appliedAnchorOffsetRef = useRef<ShellBallAnchorOffset | null>(null);
  const globalAnchorRef = useRef<ShellBallGlobalAnchor | null>(null);
  const initialBallFrameAppliedRef = useRef(false);
  const edgeDockStateRef = useRef<ShellBallEdgeDockState>({ side: null, revealed: false });
  const previousEdgeDockStateRef = useRef<ShellBallEdgeDockState>({ side: null, revealed: false });
  const suppressEdgeDockStateAnimationRef = useRef(false);
  const ballDockAnimationFrameRef = useRef<number | null>(null);
  const [edgeDockState, setEdgeDockState] = useState<ShellBallEdgeDockState>({ side: null, revealed: false });
  const [ballDragActive, setBallDragActive] = useState(false);
  const [ballDockSettling, setBallDockSettling] = useState(false);

  helperWindowShouldBeVisibleRef.current = visible;
  const cancelBallWindowDragAnimation = useCallback(() => {
    if (ballDragMoveAnimationFrameRef.current !== null) {
      window.cancelAnimationFrame(ballDragMoveAnimationFrameRef.current);
      ballDragMoveAnimationFrameRef.current = null;
    }
  }, []);

  const cancelBallDockAnimation = useCallback(() => {
    if (ballDockAnimationFrameRef.current !== null) {
      window.cancelAnimationFrame(ballDockAnimationFrameRef.current);
      ballDockAnimationFrameRef.current = null;
    }
  }, []);

  const cancelBallGeometryEmitAnimation = useCallback(() => {
    if (ballGeometryEmitAnimationFrameRef.current !== null) {
      window.cancelAnimationFrame(ballGeometryEmitAnimationFrameRef.current);
      ballGeometryEmitAnimationFrameRef.current = null;
    }
    pendingBallGeometryRef.current = null;
  }, []);

  const cancelBallGeometryPublishAnimation = useCallback(() => {
    if (ballGeometryPublishAnimationFrameRef.current !== null) {
      window.cancelAnimationFrame(ballGeometryPublishAnimationFrameRef.current);
      ballGeometryPublishAnimationFrameRef.current = null;
    }
    ballGeometryPublishSnapToBoundsRef.current = false;
  }, []);

  const commitEdgeDockState = useCallback((nextState: ShellBallEdgeDockState) => {
    edgeDockStateRef.current = nextState;
    setEdgeDockState((current) => {
      if (current.side === nextState.side && current.revealed === nextState.revealed) {
        return current;
      }

      return nextState;
    });
  }, []);

  const resolveManagedBallFrame = useCallback((input: {
    hostFrame: ShellBallWindowFrame;
    bounds: ShellBallWindowBounds;
    edgeDockState?: ShellBallEdgeDockState;
  }) => {
    const nextEdgeDockState = input.edgeDockState ?? edgeDockStateRef.current;

    if (nextEdgeDockState.side === null) {
      return clampShellBallHostFrameToVisibleBounds({
        hostFrame: input.hostFrame,
        bounds: input.bounds,
        mascotFrame: measuredMascotFrameRef.current,
      });
    }

    const dockedHostPosition = resolveShellBallDockedHostPosition({
      bounds: input.bounds,
      currentPosition: {
        x: input.hostFrame.x,
        y: input.hostFrame.y,
      },
      edgeDockState: nextEdgeDockState,
      mascotFrame: measuredMascotFrameRef.current,
    });

    return {
      ...input.hostFrame,
      x: dockedHostPosition.x,
      y: dockedHostPosition.y,
    };
  }, []);

  const readCurrentBallFrameContext = useCallback(async () => {
    if (role !== "ball" || windowFrame === null) {
      return null;
    }

    const currentWindow = getCurrentWindow();
    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return null;
    }

    const physicalPosition = await currentWindow.outerPosition();
    const physicalSize = await currentWindow.outerSize();
    const scaleFactor = await currentWindow.scaleFactor();
    const logicalPosition = physicalPosition.toLogical(scaleFactor);
    const monitor = await monitorFromPoint(
      Math.round(physicalPosition.x + physicalSize.width / 2),
      Math.round(physicalPosition.y + physicalSize.height / 2),
    );

    return {
      bounds: getShellBallBoundsFromMonitor(monitor, geometryRef.current),
      currentFrame: {
        x: logicalPosition.x,
        y: logicalPosition.y,
        width: windowFrame.width,
        height: windowFrame.height,
      },
      scaleFactor,
    };
  }, [role, windowFrame]);

  const commitBallGeometry = useCallback((input: {
    ballFrame: ShellBallWindowFrame;
    bounds: ShellBallWindowBounds;
    scaleFactor: number;
  }) => {
    const geometry = createShellBallWindowGeometry({
      position: {
        x: input.ballFrame.x,
        y: input.ballFrame.y,
      },
      size: {
        width: input.ballFrame.width,
        height: input.ballFrame.height,
      },
      bounds: input.bounds,
      scaleFactor: input.scaleFactor,
      clampToBounds: false,
    });

    geometryRef.current = geometry;
    const currentAnchorOffset = appliedAnchorOffsetRef.current;

    if (currentAnchorOffset !== null) {
      globalAnchorRef.current = {
        x: geometry.ballFrame.x + currentAnchorOffset.x,
        y: geometry.ballFrame.y + currentAnchorOffset.y,
      };
    }

    return geometry;
  }, []);

  const animateBallWindowToFrame = useCallback(async (
    currentFrame: ShellBallWindowFrame,
    nextFrame: ShellBallWindowFrame,
    input?: {
      side: ShellBallEdgeDockSide | null;
      mode: ShellBallDockAnimationMode;
    },
  ) => {
    cancelBallDockAnimation();

    const currentWindow = getCurrentWindow();
    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }

    const animationConfig = input?.side == null
      ? null
      : getShellBallDockAnimationConfig({
          side: input.side,
          mode: input.mode,
        });
    const durationMs = animationConfig?.durationMs ?? SHELL_BALL_EDGE_DOCK_HORIZONTAL_ANIMATION_DURATION_MS;
    const overshootFrame = animationConfig === null
      ? nextFrame
      : resolveShellBallDockAnimationOvershootFrame({
          nextFrame,
          config: animationConfig,
        });
    const hasOvershoot = animationConfig !== null
      && (
        (animationConfig.x?.overshootPx ?? 0) > 0
        || (animationConfig.y?.overshootPx ?? 0) > 0
      );
    const startTime = performance.now();

    await new Promise<void>((resolve) => {
      const step = (timestamp: number) => {
        const progress = Math.min(1, (timestamp - startTime) / durationMs);
        const frame = !hasOvershoot
          ? interpolateShellBallFrame(currentFrame, nextFrame, easeOutCubic(progress))
          : progress < SHELL_BALL_EDGE_DOCK_OVERSHOOT_PROGRESS
            ? interpolateShellBallFrame(
                currentFrame,
                overshootFrame,
                easeOutCubic(progress / SHELL_BALL_EDGE_DOCK_OVERSHOOT_PROGRESS),
              )
            : interpolateShellBallFrame(
                overshootFrame,
                nextFrame,
                easeInOutCubic((progress - SHELL_BALL_EDGE_DOCK_OVERSHOOT_PROGRESS) / (1 - SHELL_BALL_EDGE_DOCK_OVERSHOOT_PROGRESS)),
              );

        void currentWindow.setPosition(createShellBallLogicalPosition(frame.x, frame.y));

        if (geometryRef.current !== null) {
          geometryRef.current = {
            ...geometryRef.current,
            ballFrame: frame,
          };
        }

        if (progress >= 1) {
          ballDockAnimationFrameRef.current = null;
          resolve();
          return;
        }

        ballDockAnimationFrameRef.current = window.requestAnimationFrame(step);
      };

      ballDockAnimationFrameRef.current = window.requestAnimationFrame(step);
    });
  }, [cancelBallDockAnimation]);

  const emitBallGeometry = useCallback(async (_geometry: ShellBallWindowGeometry) => {}, []);

  const scheduleBallGeometryEmit = useCallback((geometry: ShellBallWindowGeometry) => {
    if (role !== "ball") {
      return;
    }

    pendingBallGeometryRef.current = geometry;

    if (ballGeometryEmitAnimationFrameRef.current !== null) {
      return;
    }

    // Dragging should stay coupled to raw window movement only. Cross-window
    // geometry sync is coalesced onto the next frame so the orb never waits on
    // helper-window IPC while following the pointer.
    ballGeometryEmitAnimationFrameRef.current = window.requestAnimationFrame(() => {
      ballGeometryEmitAnimationFrameRef.current = null;
      const pendingGeometry = pendingBallGeometryRef.current;
      pendingBallGeometryRef.current = null;

      if (pendingGeometry === null) {
        return;
      }

      void emitBallGeometry(pendingGeometry);
    });
  }, [emitBallGeometry, role]);

  const publishBallGeometry = useCallback(async (input?: { snapToBounds?: boolean }) => {
    const frameContext = await readCurrentBallFrameContext();

    if (frameContext === null) {
      return;
    }

    const effectiveHostFrame = input?.snapToBounds
      ? resolveManagedBallFrame({
          hostFrame: frameContext.currentFrame,
          bounds: frameContext.bounds,
        })
      : frameContext.currentFrame;

    if (
      input?.snapToBounds
      && (effectiveHostFrame.x !== frameContext.currentFrame.x || effectiveHostFrame.y !== frameContext.currentFrame.y)
    ) {
      await animateBallWindowToFrame(
        frameContext.currentFrame,
        effectiveHostFrame,
        {
          side: edgeDockStateRef.current.side,
          mode: "dock",
        },
      );
    }

    const geometry = commitBallGeometry({
      ballFrame: effectiveHostFrame,
      bounds: frameContext.bounds,
      scaleFactor: frameContext.scaleFactor,
    });

    await emitBallGeometry(geometry);
  }, [animateBallWindowToFrame, commitBallGeometry, emitBallGeometry, readCurrentBallFrameContext, resolveManagedBallFrame]);

  const scheduleBallGeometryPublish = useCallback((input?: { snapToBounds?: boolean }) => {
    if (role !== "ball") {
      return;
    }

    ballGeometryPublishSnapToBoundsRef.current = ballGeometryPublishSnapToBoundsRef.current || Boolean(input?.snapToBounds);

    if (ballDragSessionRef.current !== null && !input?.snapToBounds) {
      return;
    }

    if (ballGeometryPublishAnimationFrameRef.current !== null) {
      return;
    }

    ballGeometryPublishAnimationFrameRef.current = window.requestAnimationFrame(() => {
      ballGeometryPublishAnimationFrameRef.current = null;
      const shouldSnapToBounds = ballGeometryPublishSnapToBoundsRef.current;
      ballGeometryPublishSnapToBoundsRef.current = false;
      void publishBallGeometry(shouldSnapToBounds ? { snapToBounds: true } : undefined);
    });
  }, [publishBallGeometry, role]);

  const snapBallWindowToBounds = useCallback(async () => {
    await publishBallGeometry({ snapToBounds: true });
  }, [publishBallGeometry]);

  const queueBallWindowDragPosition = useCallback((nextFrame: ShellBallWindowFrame) => {
    if (role !== "ball") {
      return Promise.resolve();
    }

    // Dragging only cares about the latest pointer sample. Replaying every
    // historical frame turns slow window moves into a backlog that makes the
    // orb feel sticky, so keep one pending frame and overwrite stale ones.
    pendingBallDragFrameRef.current = nextFrame;

    if (ballDragPositionQueueRef.current !== null) {
      return ballDragPositionQueueRef.current;
    }

    const flushBallWindowDragPosition = async () => {
      while (pendingBallDragFrameRef.current !== null) {
        const frameToApply = pendingBallDragFrameRef.current;
        pendingBallDragFrameRef.current = null;
        const currentWindow = getCurrentWindow();

        if (currentWindow.label !== shellBallWindowLabels.ball) {
          return;
        }

        // Pointer-driven dragging keeps the orb at the raw pointer position,
        // even when that temporarily leaves the origin monitor bounds. Release
        // handling owns the later clamp-and-dock pass.
        const effectiveFrame = frameToApply;

        if (geometryRef.current !== null) {
          geometryRef.current = {
            ...geometryRef.current,
            ballFrame: effectiveFrame,
          };
        }

        await currentWindow.setPosition(createShellBallLogicalPosition(effectiveFrame.x, effectiveFrame.y));

        if (geometryRef.current !== null) {
          scheduleBallGeometryEmit(geometryRef.current);
        }
      }
    };

    ballDragPositionQueueRef.current = flushBallWindowDragPosition().finally(() => {
      ballDragPositionQueueRef.current = null;
    });

    return ballDragPositionQueueRef.current;
  }, [role, scheduleBallGeometryEmit]);

  const beginBallWindowPointerDrag = useCallback((pointerStart: ShellBallPointerPosition) => {
    if (role !== "ball" || windowFrame === null) {
      return;
    }

    cancelBallWindowDragAnimation();
    cancelBallDockAnimation();
    setBallDockSettling(false);
    const frameStart = geometryRef.current?.ballFrame;
    const originBounds = geometryRef.current?.bounds;

    if (frameStart === undefined || originBounds === undefined) {
      return;
    }

    ballDragSessionRef.current = {
      pointerStart,
      latestPointer: pointerStart,
      frameStart,
      originBounds,
    };
    setBallDragActive(true);

    if (edgeDockStateRef.current.side !== null) {
      suppressEdgeDockStateAnimationRef.current = true;
      commitEdgeDockState({ side: null, revealed: false });
    }
  }, [cancelBallDockAnimation, cancelBallWindowDragAnimation, commitEdgeDockState, role, windowFrame]);

  const updateBallWindowPointerDrag = useCallback((pointer: ShellBallPointerPosition) => {
    if (role !== "ball") {
      return;
    }

    const dragSession = ballDragSessionRef.current;
    if (dragSession === null) {
      return;
    }

    dragSession.latestPointer = pointer;

    if (ballDragMoveAnimationFrameRef.current !== null) {
      return;
    }

    ballDragMoveAnimationFrameRef.current = window.requestAnimationFrame(() => {
      ballDragMoveAnimationFrameRef.current = null;
      const activeSession = ballDragSessionRef.current;

      if (activeSession === null) {
        return;
      }

      const nextFrame = {
        ...activeSession.frameStart,
        x: Math.round(activeSession.frameStart.x + (activeSession.latestPointer.x - activeSession.pointerStart.x)),
        y: Math.round(activeSession.frameStart.y + (activeSession.latestPointer.y - activeSession.pointerStart.y)),
      };

      void queueBallWindowDragPosition(nextFrame);
    });
  }, [queueBallWindowDragPosition, role]);

  const endBallWindowPointerDrag = useCallback(async (pointer?: ShellBallPointerPosition) => {
    if (role !== "ball" || windowFrame === null) {
      return;
    }

    cancelBallWindowDragAnimation();
    const dragSession = ballDragSessionRef.current;
    ballDragSessionRef.current = null;
    setBallDragActive(false);

    if (dragSession !== null) {
      const finalPointer = pointer ?? dragSession.latestPointer;
      const finalFrame = {
        ...dragSession.frameStart,
        x: Math.round(dragSession.frameStart.x + (finalPointer.x - dragSession.pointerStart.x)),
        y: Math.round(dragSession.frameStart.y + (finalPointer.y - dragSession.pointerStart.y)),
      };

      await queueBallWindowDragPosition(finalFrame);
    }

    const frameContext = await readCurrentBallFrameContext();

    if (frameContext === null) {
      setBallDockSettling(false);
      return;
    }

    // Release-time settling stays anchored to the monitor where the drag
    // started, even if the user temporarily drags the orb beyond that screen.
    const releaseBounds = dragSession?.originBounds ?? frameContext.bounds;
    const clampedHostFrame = clampShellBallHostFrameToVisibleBounds({
      hostFrame: frameContext.currentFrame,
      bounds: releaseBounds,
      mascotFrame: measuredMascotFrameRef.current,
    });
    const nextDockSide = resolveShellBallReleaseSnapTarget({
      bounds: releaseBounds,
      hostFrame: clampedHostFrame,
      mascotFrame: measuredMascotFrameRef.current,
    });
    const nextDockState: ShellBallEdgeDockState = nextDockSide === null
      ? { side: null, revealed: false }
      : { side: nextDockSide, revealed: false };
    const targetFrame = resolveManagedBallFrame({
      hostFrame: clampedHostFrame,
      bounds: releaseBounds,
      edgeDockState: nextDockState,
    });

    suppressEdgeDockStateAnimationRef.current =
      edgeDockStateRef.current.side !== nextDockState.side
      || edgeDockStateRef.current.revealed !== nextDockState.revealed;
    commitEdgeDockState(nextDockState);

    if (targetFrame.x !== frameContext.currentFrame.x || targetFrame.y !== frameContext.currentFrame.y) {
      setBallDockSettling(nextDockSide !== null);
      await animateBallWindowToFrame(
        frameContext.currentFrame,
        targetFrame,
        nextDockSide === null
          ? undefined
          : {
              side: nextDockSide,
              mode: "dock",
            },
      );
    }

    setBallDockSettling(false);
    const geometry = commitBallGeometry({
      ballFrame: targetFrame,
      bounds: releaseBounds,
      scaleFactor: frameContext.scaleFactor,
    });
    await emitBallGeometry(geometry);
  }, [
    animateBallWindowToFrame,
    cancelBallWindowDragAnimation,
    commitBallGeometry,
    commitEdgeDockState,
    emitBallGeometry,
    queueBallWindowDragPosition,
    readCurrentBallFrameContext,
    resolveManagedBallFrame,
    role,
    windowFrame,
  ]);

  /**
   * Freezes the active pointer drag at its latest resolved position without
   * snapping to bounds so voice gestures can continue against a stable orb.
   */
  const freezeBallWindowPointerDrag = useCallback(async () => {
    if (role !== "ball") {
      return;
    }

    cancelBallWindowDragAnimation();
    const dragSession = ballDragSessionRef.current;
    ballDragSessionRef.current = null;
    setBallDragActive(false);

    if (dragSession === null) {
      return;
    }

    const finalFrame = {
      ...dragSession.frameStart,
      x: Math.round(dragSession.frameStart.x + (dragSession.latestPointer.x - dragSession.pointerStart.x)),
      y: Math.round(dragSession.frameStart.y + (dragSession.latestPointer.y - dragSession.pointerStart.y)),
    };

    await queueBallWindowDragPosition(finalFrame);
  }, [cancelBallWindowDragAnimation, queueBallWindowDragPosition, role]);

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

      if (role === "ball") {
        const rootRect = nextElement.getBoundingClientRect();
        const mascotElement = nextElement.querySelector<HTMLElement>(".shell-ball-surface__mascot-shell");

        if (mascotElement !== null) {
          const mascotRect = mascotElement.getBoundingClientRect();

          // The mascot top-left corner is the stable shell-ball anchor. Helper
          // panels can expand around it, but this corner stays pinned in screen
          // space across merged-window resizes.
          measuredAnchorOffsetRef.current = {
            x: mascotRect.left - rootRect.left,
            y: mascotRect.top - rootRect.top,
          };
          measuredMascotFrameRef.current = {
            x: mascotRect.left - rootRect.left,
            y: mascotRect.top - rootRect.top,
            width: mascotRect.width,
            height: mascotRect.height,
          };
        }
      }

      const isBallWindow = role === "ball";
      const includeScrollBounds = !isBallWindow && role !== "bubble";
      const contentSize = measureShellBallContentSize(nextElement, includeScrollBounds);
      setWindowFrame(
        createShellBallWindowFrame(
          contentSize,
          isBallWindow ? SHELL_BALL_COMPACT_WINDOW_SAFE_MARGIN_PX : SHELL_BALL_WINDOW_SAFE_MARGIN_PX,
        ),
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
  }, [role]);

  useLayoutEffect(() => {
    if (windowFrame === null) {
      return;
    }

    if (
      appliedWindowSizeRef.current?.width === windowFrame.width
      && appliedWindowSizeRef.current?.height === windowFrame.height
      && (role !== "ball"
        || measuredAnchorOffsetRef.current === null
        || (
          appliedAnchorOffsetRef.current?.x === measuredAnchorOffsetRef.current.x
          && appliedAnchorOffsetRef.current?.y === measuredAnchorOffsetRef.current.y
        ))
    ) {
      return;
    }

    const nextAnchorOffset = measuredAnchorOffsetRef.current;

    void (async () => {
      if (role === "ball") {
        const currentWindow = getCurrentWindow();

        if (currentWindow.label === shellBallWindowLabels.ball) {
          const outerPosition = await currentWindow.outerPosition();
          const scaleFactor = await currentWindow.scaleFactor();
          const logicalPosition = outerPosition.toLogical(scaleFactor);
          const monitor = await monitorFromPoint(
            Math.round(outerPosition.x + windowFrame.width * scaleFactor / 2),
            Math.round(outerPosition.y + windowFrame.height * scaleFactor / 2),
          );
          const bounds = getShellBallBoundsFromMonitor(monitor, geometryRef.current);

          if (nextAnchorOffset !== null) {
            const stableGlobalAnchor = globalAnchorRef.current
              ?? (!initialBallFrameAppliedRef.current
                ? resolveShellBallInitialGlobalAnchor({
                    bounds,
                    mascotFrame: measuredMascotFrameRef.current,
                  })
                : null)
              ?? {
                x: logicalPosition.x + nextAnchorOffset.x,
                y: logicalPosition.y + nextAnchorOffset.y,
              };

            globalAnchorRef.current = stableGlobalAnchor;

            await applyShellBallCurrentWindowFrame({
              x: stableGlobalAnchor.x - nextAnchorOffset.x,
              y: stableGlobalAnchor.y - nextAnchorOffset.y,
              width: windowFrame.width,
              height: windowFrame.height,
            });
          } else {
            const initialAnchor = !initialBallFrameAppliedRef.current
              ? resolveShellBallInitialGlobalAnchor({
                  bounds,
                  mascotFrame: measuredMascotFrameRef.current,
                })
              : null;
            await applyShellBallCurrentWindowFrame({
              x: initialAnchor?.x ?? logicalPosition.x,
              y: initialAnchor?.y ?? logicalPosition.y,
              width: windowFrame.width,
              height: windowFrame.height,
            });
          }

          initialBallFrameAppliedRef.current = true;
        } else {
          await setShellBallWindowSize(role, createShellBallLogicalSize(windowFrame.width, windowFrame.height));
        }
      } else {
        await setShellBallWindowSize(role, createShellBallLogicalSize(windowFrame.width, windowFrame.height));
      }

      appliedWindowSizeRef.current = {
        width: windowFrame.width,
        height: windowFrame.height,
      };
      appliedAnchorOffsetRef.current = nextAnchorOffset;
    })();
  }, [role, windowFrame]);

  useEffect(() => {
    if (role !== "ball" || windowFrame === null) {
      return;
    }

    scheduleBallGeometryPublish();
  }, [
    edgeDockState.revealed,
    edgeDockState.side,
    role,
    scheduleBallGeometryPublish,
    windowFrame,
  ]);

  useEffect(() => {
    if (role !== "ball" || windowFrame === null) {
      return;
    }

    const currentWindow = getCurrentWindow();
    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }

    const previousDockState = previousEdgeDockStateRef.current;
    previousEdgeDockStateRef.current = edgeDockState;

    if (
      previousDockState.side === edgeDockState.side
      && previousDockState.revealed === edgeDockState.revealed
    ) {
      return;
    }

    if (suppressEdgeDockStateAnimationRef.current) {
      suppressEdgeDockStateAnimationRef.current = false;
      return;
    }

    if (ballDragSessionRef.current !== null) {
      return;
    }

    void (async () => {
      const frameContext = await readCurrentBallFrameContext();

      if (frameContext === null) {
        return;
      }

      const targetFrame = resolveManagedBallFrame({
        hostFrame: frameContext.currentFrame,
        bounds: frameContext.bounds,
        edgeDockState,
      });

      if (targetFrame.x === frameContext.currentFrame.x && targetFrame.y === frameContext.currentFrame.y) {
        return;
      }

      await animateBallWindowToFrame(frameContext.currentFrame, targetFrame, {
        side: edgeDockState.side,
        mode: "reveal",
      });

      const geometry = commitBallGeometry({
        ballFrame: targetFrame,
        bounds: frameContext.bounds,
        scaleFactor: frameContext.scaleFactor,
      });
      scheduleBallGeometryEmit(geometry);
    })();
  }, [
    animateBallWindowToFrame,
    commitBallGeometry,
    edgeDockState,
    readCurrentBallFrameContext,
    resolveManagedBallFrame,
    role,
    scheduleBallGeometryEmit,
    windowFrame,
  ]);

  useEffect(() => {
    if (role !== "ball" || windowFrame === null) {
      return;
    }

    const currentWindow = getCurrentWindow();
    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }
    let disposed = false;
    let cleanupFns: Array<() => void> = [];

    void publishBallGeometry({ snapToBounds: true });

    void Promise.all([
      currentWindow.onMoved(() => {
        scheduleBallGeometryPublish();
      }),
      currentWindow.onResized(() => {
        scheduleBallGeometryPublish({ snapToBounds: true });
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
      cancelBallGeometryEmitAnimation();
      cancelBallGeometryPublishAnimation();
      cancelBallDockAnimation();
      for (const cleanup of cleanupFns) {
        cleanup();
      }
    };
  }, [
    cancelBallDockAnimation,
    cancelBallGeometryEmitAnimation,
    cancelBallGeometryPublishAnimation,
    publishBallGeometry,
    role,
    scheduleBallGeometryPublish,
    windowFrame,
  ]);

  useEffect(() => {
    return () => {
      cancelBallDockAnimation();
      cancelBallGeometryEmitAnimation();
      cancelBallGeometryPublishAnimation();
      cancelBallWindowDragAnimation();
      appliedWindowSizeRef.current = null;
      appliedAnchorOffsetRef.current = null;
      globalAnchorRef.current = null;
      initialBallFrameAppliedRef.current = false;
      measuredAnchorOffsetRef.current = null;
      ballDragSessionRef.current = null;
      pendingBallDragFrameRef.current = null;
      ballDragPositionQueueRef.current = null;
      suppressEdgeDockStateAnimationRef.current = false;
      edgeDockStateRef.current = { side: null, revealed: false };
    };
  }, [cancelBallDockAnimation, cancelBallGeometryEmitAnimation, cancelBallGeometryPublishAnimation, cancelBallWindowDragAnimation]);

  const setEdgeDockRevealed = useCallback((revealed: boolean) => {
    setEdgeDockState((current) => {
      if (current.side === null || current.revealed === revealed) {
        return current;
      }

      const nextState = {
        ...current,
        revealed,
      };
      edgeDockStateRef.current = nextState;
      return nextState;
    });
  }, []);

  return {
    ballDockSettling,
    ballDragActive,
    beginBallWindowPointerDrag,
    edgeDockState,
    endBallWindowPointerDrag,
    freezeBallWindowPointerDrag,
    rootRef,
    setEdgeDockRevealed,
    snapBallWindowToBounds,
    updateBallWindowPointerDrag,
    windowFrame,
  };
}
