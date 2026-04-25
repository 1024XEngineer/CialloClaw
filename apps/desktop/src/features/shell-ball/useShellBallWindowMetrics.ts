import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { getCurrentWindow, monitorFromPoint, type Monitor } from "@tauri-apps/api/window";
import {
  applyShellBallCurrentWindowFrame,
  createShellBallLogicalPosition,
  createShellBallLogicalSize,
  hideShellBallWindow,
  setShellBallWindowFocusable,
  setShellBallWindowIgnoreCursorEvents,
  setShellBallWindowPosition,
  setShellBallWindowSize,
  shellBallWindowLabels,
  showShellBallWindow,
} from "../../platform/shellBallWindowController";
import {
  shellBallWindowSyncEvents,
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
const SHELL_BALL_EDGE_DOCK_RELEASE_DISTANCE_PX = 28;
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

type ShellBallBubblePlacement = "above" | "left" | "right" | "below";

export type ShellBallEdgeDockSide = "left" | "right" | "top" | "bottom";

export type ShellBallEdgeDockState = {
  revealed: boolean;
  side: ShellBallEdgeDockSide | null;
};

type ShellBallDockAnimationMode = "dock" | "reveal";

type ShellBallDockAnimationConfig = {
  axis: "x" | "y";
  direction: -1 | 1;
  durationMs: number;
  overshootPx: number;
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

type ShellBallResolvedHelperFrame = ShellBallWindowFrame & {
  placement?: ShellBallBubblePlacement;
};

type ShellBallBallDragSession = {
  pointerStart: ShellBallPointerPosition;
  latestPointer: ShellBallPointerPosition;
  frameStart: ShellBallWindowFrame;
};

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

  if (element instanceof HTMLElement && element.classList.contains("shell-ball-surface")) {
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

  if (element instanceof HTMLElement && element.dataset.shellBallInputWindow === "true") {
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
  side: ShellBallEdgeDockSide;
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

/**
 * Resolves the edge-specific snap animation profile. Dock animations can
 * overshoot slightly, while hover reveal animations stay on a direct easing
 * path without bounce.
 */
export function getShellBallDockAnimationConfig(input: {
  side: ShellBallEdgeDockSide;
  mode: ShellBallDockAnimationMode;
}): ShellBallDockAnimationConfig {
  if (input.side === "left") {
    return {
      axis: "x",
      direction: -1,
      durationMs: SHELL_BALL_EDGE_DOCK_HORIZONTAL_ANIMATION_DURATION_MS,
      overshootPx: input.mode === "dock" ? SHELL_BALL_EDGE_DOCK_HORIZONTAL_OVERSHOOT_PX : 0,
    };
  }

  if (input.side === "right") {
    return {
      axis: "x",
      direction: 1,
      durationMs: SHELL_BALL_EDGE_DOCK_HORIZONTAL_ANIMATION_DURATION_MS,
      overshootPx: input.mode === "dock" ? SHELL_BALL_EDGE_DOCK_HORIZONTAL_OVERSHOOT_PX : 0,
    };
  }

  if (input.side === "top") {
    return {
      axis: "y",
      direction: -1,
      durationMs: SHELL_BALL_EDGE_DOCK_VERTICAL_ANIMATION_DURATION_MS,
      overshootPx: input.mode === "dock" ? SHELL_BALL_EDGE_DOCK_VERTICAL_OVERSHOOT_PX : 0,
    };
  }

  return {
    axis: "y",
    direction: 1,
    durationMs: SHELL_BALL_EDGE_DOCK_VERTICAL_ANIMATION_DURATION_MS,
    overshootPx: input.mode === "dock" ? SHELL_BALL_EDGE_DOCK_VERTICAL_OVERSHOOT_PX : 0,
  };
}

function resolveShellBallDockAnimationOvershootFrame(input: {
  nextFrame: ShellBallWindowFrame;
  config: ShellBallDockAnimationConfig;
}) {
  if (input.config.overshootPx <= 0) {
    return input.nextFrame;
  }

  return input.config.axis === "x"
    ? {
        ...input.nextFrame,
        x: input.nextFrame.x + input.config.direction * input.config.overshootPx,
      }
    : {
        ...input.nextFrame,
        y: input.nextFrame.y + input.config.direction * input.config.overshootPx,
      };
}

function resolveShellBallEdgeDockSide(input: {
  bounds: ShellBallWindowBounds;
  current: ShellBallEdgeDockState;
  hostFrame: ShellBallWindowFrame;
  mascotFrame: ShellBallRelativeFrame | null;
}) {
  const mascotFrame = input.mascotFrame;

  if (mascotFrame === null) {
    return null;
  }

  const mascotLeft = input.hostFrame.x + mascotFrame.x;
  const mascotTop = input.hostFrame.y + mascotFrame.y;
  const mascotRight = mascotLeft + mascotFrame.width;
  const mascotBottom = mascotTop + mascotFrame.height;

  if (input.current.side === "left") {
    return mascotLeft >= input.bounds.minX + SHELL_BALL_EDGE_DOCK_RELEASE_DISTANCE_PX ? null : "left";
  }

  if (input.current.side === "right") {
    return mascotRight <= input.bounds.maxX - SHELL_BALL_EDGE_DOCK_RELEASE_DISTANCE_PX ? null : "right";
  }

  if (input.current.side === "top") {
    return mascotTop >= input.bounds.minY + SHELL_BALL_EDGE_DOCK_RELEASE_DISTANCE_PX ? null : "top";
  }

  if (input.current.side === "bottom") {
    return mascotBottom <= input.bounds.maxY - SHELL_BALL_EDGE_DOCK_RELEASE_DISTANCE_PX ? null : "bottom";
  }

  if (mascotLeft < input.bounds.minX) {
    return "left";
  }

  if (mascotRight > input.bounds.maxX) {
    return "right";
  }

  if (mascotTop < input.bounds.minY) {
    return "top";
  }

  if (mascotBottom > input.bounds.maxY) {
    return "bottom";
  }

  return null;
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

  if (input.edgeDockState.side === "left") {
    const parkedInsetPx = getShellBallParkedDockInsetPx({
      side: "left",
      mascotFrame,
    });
    const targetMascotLeft = input.edgeDockState.revealed
      ? input.bounds.minX
      : input.bounds.minX - parkedInsetPx;

    return {
      x: Math.round(targetMascotLeft - mascotFrame.x),
      y: input.currentPosition.y,
    };
  }

  if (input.edgeDockState.side === "right") {
    const parkedInsetPx = getShellBallParkedDockInsetPx({
      side: "right",
      mascotFrame,
    });
    const targetMascotRight = input.edgeDockState.revealed
      ? input.bounds.maxX
      : input.bounds.maxX + parkedInsetPx;

    return {
      x: Math.round(targetMascotRight - mascotFrame.x - mascotFrame.width),
      y: input.currentPosition.y,
    };
  }

  if (input.edgeDockState.side === "top") {
    const parkedInsetPx = getShellBallParkedDockInsetPx({
      side: "top",
      mascotFrame,
    });
    const targetMascotTop = input.edgeDockState.revealed
      ? input.bounds.minY
      : input.bounds.minY - parkedInsetPx;

    return {
      x: input.currentPosition.x,
      y: Math.round(targetMascotTop - mascotFrame.y),
    };
  }

  const parkedInsetPx = getShellBallParkedDockInsetPx({
    side: "bottom",
    mascotFrame,
  });
  const targetMascotBottom = input.edgeDockState.revealed
    ? input.bounds.maxY
    : input.bounds.maxY + parkedInsetPx;

  return {
    x: input.currentPosition.x,
    y: Math.round(targetMascotBottom - mascotFrame.y - mascotFrame.height),
  };
}

function shouldApplyShellBallDockedPosition(input: {
  dragging: boolean;
  edgeDockState: ShellBallEdgeDockState;
}) {
  if (input.edgeDockState.side === null) {
    return false;
  }

  if (input.dragging && !input.edgeDockState.revealed) {
    return false;
  }

  return true;
}

function getShellBallBubbleFrame(input: {
  ballFrame: ShellBallWindowFrame;
  helperFrame: ShellBallWindowSize;
  bounds: ShellBallWindowBounds;
  gap?: number;
}): ShellBallResolvedHelperFrame {
  const gap = input.gap ?? SHELL_BALL_BUBBLE_GAP_PX;
  const maxX = Math.max(input.bounds.minX, input.bounds.maxX - input.helperFrame.width);
  const maxY = Math.max(input.bounds.minY, input.bounds.maxY - input.helperFrame.height);
  const centeredX = input.ballFrame.x + input.ballFrame.width / 2 - input.helperFrame.width / 2;
  const centeredY = input.ballFrame.y + input.ballFrame.height / 2 - input.helperFrame.height / 2;
  const spaceAbove = input.ballFrame.y - input.bounds.minY;
  const spaceBelow = input.bounds.maxY - (input.ballFrame.y + input.ballFrame.height);
  const spaceLeft = input.ballFrame.x - input.bounds.minX;
  const spaceRight = input.bounds.maxX - (input.ballFrame.x + input.ballFrame.width);
  const canPlaceAbove = spaceAbove >= input.helperFrame.height + gap;
  const canPlaceBelow = spaceBelow >= input.helperFrame.height + gap;
  const canPlaceLeft = spaceLeft >= input.helperFrame.width + gap;
  const canPlaceRight = spaceRight >= input.helperFrame.width + gap;

  if (canPlaceAbove) {
    return {
      x: clampShellBallAxisPosition(centeredX, input.bounds.minX, maxX),
      y: Math.round(input.ballFrame.y - gap - input.helperFrame.height),
      width: input.helperFrame.width,
      height: input.helperFrame.height,
      placement: "above",
    };
  }

  if (canPlaceLeft) {
    return {
      x: Math.round(input.ballFrame.x - gap - input.helperFrame.width),
      y: clampShellBallAxisPosition(centeredY, input.bounds.minY, maxY),
      width: input.helperFrame.width,
      height: input.helperFrame.height,
      placement: "left",
    };
  }

  if (canPlaceRight) {
    return {
      x: Math.round(input.ballFrame.x + input.ballFrame.width + gap),
      y: clampShellBallAxisPosition(centeredY, input.bounds.minY, maxY),
      width: input.helperFrame.width,
      height: input.helperFrame.height,
      placement: "right",
    };
  }

  if (canPlaceBelow) {
    return {
      x: clampShellBallAxisPosition(centeredX, input.bounds.minX, maxX),
      y: Math.round(input.ballFrame.y + input.ballFrame.height + gap),
      width: input.helperFrame.width,
      height: input.helperFrame.height,
      placement: "below",
    };
  }

  const preferAbove = spaceAbove >= spaceLeft && spaceAbove >= spaceRight && spaceAbove >= spaceBelow;
  const preferLeft = !preferAbove && spaceLeft >= spaceRight && spaceLeft >= spaceBelow;
  const preferRight = !preferAbove && !preferLeft && spaceRight >= spaceBelow;

  return {
    x: preferAbove || !preferLeft && !preferRight
      ? clampShellBallAxisPosition(centeredX, input.bounds.minX, maxX)
      : preferLeft
        ? input.bounds.minX
        : maxX,
    y: preferAbove
      ? input.bounds.minY
      : preferLeft || preferRight
        ? clampShellBallAxisPosition(centeredY, input.bounds.minY, maxY)
        : maxY,
    width: input.helperFrame.width,
    height: input.helperFrame.height,
    placement: preferAbove
      ? "above"
      : preferLeft
        ? "left"
        : preferRight
          ? "right"
          : "below",
  };
}

function resolveShellBallHelperFrame(input: {
  role: AnchoredShellBallHelperWindowRole;
  ballFrame: ShellBallWindowFrame;
  helperFrame: ShellBallWindowSize;
  bounds: ShellBallWindowBounds;
}): ShellBallResolvedHelperFrame {
  if (input.role === "bubble") {
    return getShellBallBubbleFrame({
      ballFrame: input.ballFrame,
      helperFrame: input.helperFrame,
      bounds: input.bounds,
    });
  }

  const anchor =
    input.role === "input"
      ? getShellBallInputAnchor({
          ballFrame: input.ballFrame,
          helperFrame: input.helperFrame,
        })
      : getShellBallVoiceAnchor({
          ballFrame: input.ballFrame,
          helperFrame: input.helperFrame,
        });

  return clampShellBallFrameToBounds(
    {
      x: anchor.x,
      y: anchor.y,
      width: input.helperFrame.width,
      height: input.helperFrame.height,
    },
    input.bounds,
  );
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
  const helperWindowVisibleRef = useRef(false);
  const helperWindowShouldBeVisibleRef = useRef(visible);
  const helperWindowFrameRef = useRef<ShellBallResolvedHelperFrame | null>(null);
  const appliedWindowSizeRef = useRef<ShellBallWindowSize | null>(null);
  const measuredAnchorOffsetRef = useRef<ShellBallAnchorOffset | null>(null);
  const measuredMascotFrameRef = useRef<ShellBallRelativeFrame | null>(null);
  const appliedAnchorOffsetRef = useRef<ShellBallAnchorOffset | null>(null);
  const globalAnchorRef = useRef<ShellBallGlobalAnchor | null>(null);
  const edgeDockStateRef = useRef<ShellBallEdgeDockState>({ side: null, revealed: false });
  const previousEdgeDockStateRef = useRef<ShellBallEdgeDockState>({ side: null, revealed: false });
  const ballDockAnimationFrameRef = useRef<number | null>(null);
  const helperWindowMoveAnimationFrameRef = useRef<number | null>(null);
  const helperWindowMoveAnimationResolveRef = useRef<(() => void) | null>(null);
  const helperWindowMoveAnimationTokenRef = useRef(0);
  const [edgeDockState, setEdgeDockState] = useState<ShellBallEdgeDockState>({ side: null, revealed: false });

  helperWindowShouldBeVisibleRef.current = visible;
  function cancelHelperWindowMoveAnimation() {
    helperWindowMoveAnimationTokenRef.current += 1;
    if (helperWindowMoveAnimationFrameRef.current !== null) {
      window.cancelAnimationFrame(helperWindowMoveAnimationFrameRef.current);
      helperWindowMoveAnimationFrameRef.current = null;
    }
    const resolveAnimation = helperWindowMoveAnimationResolveRef.current;
    helperWindowMoveAnimationResolveRef.current = null;
    resolveAnimation?.();
  }

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

  async function snapHelperWindowToFrame(nextFrame: ShellBallResolvedHelperFrame) {
    cancelHelperWindowMoveAnimation();
    await setShellBallWindowPosition(role, createShellBallLogicalPosition(nextFrame.x, nextFrame.y));
    helperWindowFrameRef.current = nextFrame;
  }

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

    const animationConfig = input?.side === null || input?.side === undefined
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
    const hasOvershoot = animationConfig !== null && animationConfig.overshootPx > 0;
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

  const resolveEdgeDockedFrame = useCallback((input: {
    hostFrame: ShellBallWindowFrame;
    bounds: ShellBallWindowBounds;
    dragging: boolean;
  }) => {
    const nextDockSide = resolveShellBallEdgeDockSide({
      bounds: input.bounds,
      current: edgeDockStateRef.current,
      hostFrame: input.hostFrame,
      mascotFrame: measuredMascotFrameRef.current,
    });
    const nextEdgeDockState: ShellBallEdgeDockState = nextDockSide === null
      ? { side: null, revealed: false }
      : {
          side: nextDockSide,
          revealed: edgeDockStateRef.current.side === nextDockSide ? edgeDockStateRef.current.revealed : false,
        };

    if (
      nextEdgeDockState.side !== edgeDockStateRef.current.side
      || nextEdgeDockState.revealed !== edgeDockStateRef.current.revealed
    ) {
      edgeDockStateRef.current = nextEdgeDockState;
      setEdgeDockState(nextEdgeDockState);
    }

    if (!shouldApplyShellBallDockedPosition({
      dragging: input.dragging,
      edgeDockState: nextEdgeDockState,
    })) {
      return input.hostFrame;
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

  const publishBallGeometry = useCallback(async (input?: { snapToBounds?: boolean }) => {
    if (role !== "ball" || windowFrame === null) {
      return;
    }

    const currentWindow = getCurrentWindow();

    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }

    const physicalPosition = await currentWindow.outerPosition();
    const physicalSize = await currentWindow.outerSize();
    const scaleFactor = await currentWindow.scaleFactor();
    const monitor = await monitorFromPoint(
      Math.round(physicalPosition.x + physicalSize.width / 2),
      Math.round(physicalPosition.y + physicalSize.height / 2),
    );
    const logicalPosition = physicalPosition.toLogical(scaleFactor);
    const bounds = getShellBallBoundsFromMonitor(monitor, geometryRef.current);
    const hostFrame = {
      x: logicalPosition.x,
      y: logicalPosition.y,
      width: windowFrame.width,
      height: windowFrame.height,
    };
    const effectiveHostFrame = resolveEdgeDockedFrame({
      hostFrame,
      bounds,
      dragging: false,
    });
    const geometry = createShellBallWindowGeometry({
      position: {
        x: effectiveHostFrame.x,
        y: effectiveHostFrame.y,
      },
      size: {
        width: effectiveHostFrame.width,
        height: effectiveHostFrame.height,
      },
      bounds,
      scaleFactor,
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

    if (input?.snapToBounds && (geometry.ballFrame.x !== logicalPosition.x || geometry.ballFrame.y !== logicalPosition.y)) {
      await animateBallWindowToFrame(
        {
          x: logicalPosition.x,
          y: logicalPosition.y,
          width: windowFrame.width,
          height: windowFrame.height,
        },
        geometry.ballFrame,
        {
          side: edgeDockStateRef.current.side,
          mode: "dock",
        },
      );
    }

    await emitBallGeometry(geometry);
  }, [animateBallWindowToFrame, emitBallGeometry, resolveEdgeDockedFrame, role, windowFrame]);

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

        const bounds = geometryRef.current?.bounds;
        const effectiveFrame = bounds === undefined
          ? frameToApply
          : resolveEdgeDockedFrame({
              hostFrame: frameToApply,
              bounds,
              dragging: ballDragSessionRef.current !== null,
            });

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
  }, [resolveEdgeDockedFrame, role, scheduleBallGeometryEmit]);

  const beginBallWindowPointerDrag = useCallback((pointerStart: ShellBallPointerPosition) => {
    if (role !== "ball" || windowFrame === null) {
      return;
    }

    cancelBallWindowDragAnimation();
    const frameStart = geometryRef.current?.ballFrame;

    if (frameStart === undefined) {
      return;
    }

    ballDragSessionRef.current = {
      pointerStart,
      latestPointer: pointerStart,
      frameStart,
    };
  }, [cancelBallWindowDragAnimation, role, windowFrame]);

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
    if (role !== "ball") {
      return;
    }

    cancelBallWindowDragAnimation();
    const dragSession = ballDragSessionRef.current;
    ballDragSessionRef.current = null;

    if (dragSession !== null) {
      const finalPointer = pointer ?? dragSession.latestPointer;
      const finalFrame = {
        ...dragSession.frameStart,
        x: Math.round(dragSession.frameStart.x + (finalPointer.x - dragSession.pointerStart.x)),
        y: Math.round(dragSession.frameStart.y + (finalPointer.y - dragSession.pointerStart.y)),
      };

      await queueBallWindowDragPosition(finalFrame);
    }

    await snapBallWindowToBounds();
  }, [cancelBallWindowDragAnimation, queueBallWindowDragPosition, role, snapBallWindowToBounds]);

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

  async function animateBubbleWindowToFrame(nextFrame: ShellBallResolvedHelperFrame) {
    const previousFrame = helperWindowFrameRef.current;
    if (role !== "bubble" || previousFrame === null || previousFrame.placement === nextFrame.placement) {
      await snapHelperWindowToFrame(nextFrame);
      return;
    }

    cancelHelperWindowMoveAnimation();
    const animationToken = helperWindowMoveAnimationTokenRef.current;
    const startX = previousFrame.x;
    const startY = previousFrame.y;
    const deltaX = nextFrame.x - startX;
    const deltaY = nextFrame.y - startY;
    const startTime = performance.now();

    await new Promise<void>((resolve) => {
      helperWindowMoveAnimationResolveRef.current = resolve;

      const step = (timestamp: number) => {
        if (helperWindowMoveAnimationTokenRef.current !== animationToken) {
          helperWindowMoveAnimationFrameRef.current = null;
          if (helperWindowMoveAnimationResolveRef.current === resolve) {
            helperWindowMoveAnimationResolveRef.current = null;
          }
          resolve();
          return;
        }

        const progress = Math.min(1, (timestamp - startTime) / SHELL_BALL_BUBBLE_REPOSITION_DURATION_MS);
        const easedProgress = 1 - (1 - progress) ** 3;
        const nextX = Math.round(startX + deltaX * easedProgress);
        const nextY = Math.round(startY + deltaY * easedProgress);

        // Track the in-flight frame so later geometry updates continue from the
        // current visual position instead of restarting from the old edge.
        helperWindowFrameRef.current = {
          ...nextFrame,
          x: nextX,
          y: nextY,
        };
        void setShellBallWindowPosition(role, createShellBallLogicalPosition(nextX, nextY));

        if (progress >= 1) {
          helperWindowMoveAnimationFrameRef.current = null;
          if (helperWindowMoveAnimationResolveRef.current === resolve) {
            helperWindowMoveAnimationResolveRef.current = null;
          }
          resolve();
          return;
        }

        helperWindowMoveAnimationFrameRef.current = window.requestAnimationFrame(step);
      };

      helperWindowMoveAnimationFrameRef.current = window.requestAnimationFrame(step);
    });

    if (helperWindowMoveAnimationTokenRef.current !== animationToken) {
      return;
    }

    await setShellBallWindowPosition(role, createShellBallLogicalPosition(nextFrame.x, nextFrame.y));
    helperWindowFrameRef.current = nextFrame;
  }

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

          if (nextAnchorOffset !== null) {
            const stableGlobalAnchor = globalAnchorRef.current ?? {
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
            await applyShellBallCurrentWindowFrame({
              x: logicalPosition.x,
              y: logicalPosition.y,
              width: windowFrame.width,
              height: windowFrame.height,
            });
          }
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

    if (ballDragSessionRef.current !== null) {
      return;
    }

    void (async () => {
      const outerPosition = await currentWindow.outerPosition();
      const scaleFactor = await currentWindow.scaleFactor();
      const logicalPosition = outerPosition.toLogical(scaleFactor);
      const monitor = await monitorFromPoint(
        Math.round(outerPosition.x),
        Math.round(outerPosition.y),
      );
      const currentFrame = {
        x: logicalPosition.x,
        y: logicalPosition.y,
        width: windowFrame.width,
        height: windowFrame.height,
      };
      const targetFrame = resolveEdgeDockedFrame({
        hostFrame: currentFrame,
        bounds: getShellBallBoundsFromMonitor(monitor, geometryRef.current),
        dragging: false,
      });

      if (targetFrame.x === currentFrame.x && targetFrame.y === currentFrame.y) {
        return;
      }

      await animateBallWindowToFrame(currentFrame, targetFrame, {
        side: edgeDockState.side,
        mode: "reveal",
      });

      if (geometryRef.current !== null) {
        scheduleBallGeometryEmit(geometryRef.current);
      }
    })();
  }, [animateBallWindowToFrame, edgeDockState, resolveEdgeDockedFrame, role, scheduleBallGeometryEmit, windowFrame]);

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
        scheduleBallGeometryPublish();
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
      measuredAnchorOffsetRef.current = null;
      ballDragSessionRef.current = null;
      pendingBallDragFrameRef.current = null;
      ballDragPositionQueueRef.current = null;
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
