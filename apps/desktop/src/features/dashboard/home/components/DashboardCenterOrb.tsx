import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties, PointerEvent as ReactPointerEvent } from "react";
import { AudioLines } from "lucide-react";
import { getShellBallMotionConfig } from "@/features/shell-ball/shellBall.motion";
import { ShellBallMascot } from "@/features/shell-ball/components/ShellBallMascot";
import type { ShellBallVisualState } from "@/features/shell-ball/shellBall.types";
import { cn } from "@/utils/cn";

const LONG_PRESS_DURATION = 650;

type DashboardCenterOrbProps = {
  activeColor: string | null;
  onDragOffset?: (x: number, y: number) => void;
  onLongPress?: () => void;
  visualState: ShellBallVisualState;
};

type DragState = {
  pointerId: number;
  startX: number;
  startY: number;
};

function clamp(value: number, minimum: number, maximum: number) {
  return Math.min(maximum, Math.max(minimum, value));
}

export function DashboardCenterOrb({ activeColor, onDragOffset, onLongPress, visualState }: DashboardCenterOrbProps) {
  const [offset, setOffset] = useState({ x: 0, y: 0 });
  const [dragState, setDragState] = useState<DragState | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [isReturning, setIsReturning] = useState(false);
  const [progress, setProgress] = useState(0);
  const [longPressActive, setLongPressActive] = useState(false);
  const dragOffsetRef = useRef({ x: 0, y: 0 });
  const velocityRef = useRef({ x: 0, y: 0 });
  const returnFrameRef = useRef(0);
  const longPressFrameRef = useRef(0);
  const longPressStartRef = useRef(0);
  const longPressTimeoutRef = useRef<number | null>(null);
  const motionConfig = useMemo(() => getShellBallMotionConfig(visualState), [visualState]);

  useEffect(() => {
    onDragOffset?.(offset.x, offset.y);
  }, [offset, onDragOffset]);

  useEffect(() => {
    return () => {
      window.cancelAnimationFrame(returnFrameRef.current);
      window.cancelAnimationFrame(longPressFrameRef.current);
      if (longPressTimeoutRef.current) {
        window.clearTimeout(longPressTimeoutRef.current);
      }
    };
  }, []);

  const cancelLongPress = useCallback(() => {
    if (longPressTimeoutRef.current) {
      window.clearTimeout(longPressTimeoutRef.current);
      longPressTimeoutRef.current = null;
    }
    window.cancelAnimationFrame(longPressFrameRef.current);
    setLongPressActive(false);
    setProgress(0);
  }, []);

  const startReturn = useCallback(() => {
    window.cancelAnimationFrame(returnFrameRef.current);
    setIsReturning(true);

    const spring = () => {
      const stiffness = 0.16;
      const damping = 0.72;

      velocityRef.current.x += -dragOffsetRef.current.x * stiffness;
      velocityRef.current.y += -dragOffsetRef.current.y * stiffness;
      velocityRef.current.x *= damping;
      velocityRef.current.y *= damping;
      dragOffsetRef.current.x += velocityRef.current.x;
      dragOffsetRef.current.y += velocityRef.current.y;

      const next = { x: dragOffsetRef.current.x, y: dragOffsetRef.current.y };
      setOffset(next);

      if (Math.hypot(next.x, next.y) > 0.5 || Math.abs(velocityRef.current.x) > 0.12 || Math.abs(velocityRef.current.y) > 0.12) {
        returnFrameRef.current = window.requestAnimationFrame(spring);
        return;
      }

      dragOffsetRef.current = { x: 0, y: 0 };
      velocityRef.current = { x: 0, y: 0 };
      setOffset({ x: 0, y: 0 });
      setIsReturning(false);
    };

    returnFrameRef.current = window.requestAnimationFrame(spring);
  }, []);

  const beginLongPress = useCallback(() => {
    longPressStartRef.current = Date.now();
    setLongPressActive(true);
    setProgress(0);

    const tick = () => {
      const elapsed = Date.now() - longPressStartRef.current;
      const nextProgress = Math.min(elapsed / LONG_PRESS_DURATION, 1);
      setProgress(nextProgress);
      if (nextProgress < 1) {
        longPressFrameRef.current = window.requestAnimationFrame(tick);
      }
    };

    longPressFrameRef.current = window.requestAnimationFrame(tick);
    longPressTimeoutRef.current = window.setTimeout(() => {
      cancelLongPress();
      onLongPress?.();
    }, LONG_PRESS_DURATION);
  }, [cancelLongPress, onLongPress]);

  function handlePointerDown(event: ReactPointerEvent<HTMLButtonElement>) {
    event.preventDefault();
    window.cancelAnimationFrame(returnFrameRef.current);
    event.currentTarget.setPointerCapture(event.pointerId);
    setDragState({ pointerId: event.pointerId, startX: event.clientX - dragOffsetRef.current.x, startY: event.clientY - dragOffsetRef.current.y });
    setIsDragging(true);
    setIsReturning(false);
    velocityRef.current = { x: 0, y: 0 };
    beginLongPress();
  }

  function handlePointerMove(event: ReactPointerEvent<HTMLButtonElement>) {
    if (!dragState || dragState.pointerId !== event.pointerId) {
      return;
    }

    const rawX = event.clientX - dragState.startX;
    const rawY = event.clientY - dragState.startY;
    const distance = Math.hypot(rawX, rawY);
    const maxRadius = 100;
    const ratio = distance === 0 ? 0 : Math.min(distance, maxRadius) / distance;
    const next = {
      x: clamp(rawX * ratio, -maxRadius, maxRadius),
      y: clamp(rawY * ratio, -maxRadius, maxRadius),
    };

    if (Math.hypot(next.x, next.y) > 8) {
      cancelLongPress();
    }

    velocityRef.current = { x: next.x - dragOffsetRef.current.x, y: next.y - dragOffsetRef.current.y };
    dragOffsetRef.current = next;
    setOffset(next);
  }

  function handlePointerEnd(event: ReactPointerEvent<HTMLButtonElement>) {
    if (!dragState || dragState.pointerId !== event.pointerId) {
      return;
    }

    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }

    cancelLongPress();
    setDragState(null);
    setIsDragging(false);
    startReturn();
  }

  const progressRadius = 96;
  const circumference = 2 * Math.PI * progressRadius;
  const dashOffset = circumference * (1 - progress);
  const accent = activeColor ?? "#9FB7D8";
  const wrapperStyle = {
    transform: `translate(calc(-50% + ${offset.x}px), calc(-50% + ${offset.y}px))`,
  } as CSSProperties;

  return (
    <div className="dashboard-orbit-center" style={wrapperStyle}>
      <div className="dashboard-orbit-center__field dashboard-orbit-center__field--outer" style={{ boxShadow: `0 0 0 1px ${accent}22 inset` }} />
      <div className="dashboard-orbit-center__field dashboard-orbit-center__field--middle" style={{ boxShadow: `0 0 0 1px ${accent}30 inset` }} />
      <div className="dashboard-orbit-center__field dashboard-orbit-center__field--inner" style={{ boxShadow: `0 0 0 1px ${accent}24 inset` }} />

      {longPressActive ? (
        <svg aria-hidden="true" className="dashboard-orbit-center__progress-ring" viewBox="0 0 212 212">
          <circle cx="106" cy="106" fill="none" r={progressRadius} stroke="rgba(215,198,178,0.44)" strokeWidth="3" />
          <circle
            cx="106"
            cy="106"
            fill="none"
            r={progressRadius}
            stroke={accent}
            strokeDasharray={circumference}
            strokeDashoffset={dashOffset}
            strokeLinecap="round"
            strokeWidth="4"
            style={{ filter: `drop-shadow(0 0 6px ${accent})`, transition: "stroke-dashoffset 0.04s linear" }}
            transform="rotate(-90 106 106)"
          />
        </svg>
      ) : null}

      <button
        className={cn("dashboard-orbit-center__core", isDragging && "is-dragging", isReturning && "is-returning")}
        onLostPointerCapture={handlePointerEnd}
        onPointerCancel={handlePointerEnd}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerEnd}
        type="button"
      >
        <div className="dashboard-orbit-center__shell">
          <ShellBallMascot
            floatingBallSize="large"
            motionConfig={motionConfig}
            onPressEnd={() => false}
            onPressMove={() => {}}
            onPressStart={() => {}}
            onPrimaryClick={() => {}}
            showVoiceHints={false}
            visualState={visualState}
            voicePreview={null}
          />
        </div>
      </button>

      <div className="dashboard-orbit-center__caption">
        <div className="dashboard-orbit-center__status-line">
          <span className="dashboard-orbit-center__status-dot" style={{ background: accent, boxShadow: `0 0 0 4px ${accent}22` }} />
          <span className="dashboard-orbit-center__status-text">小助手核心</span>
        </div>
        <div className="dashboard-orbit-center__hint-line">
          <AudioLines className="h-3.5 w-3.5" />
          长按唤起语音场
        </div>
      </div>
    </div>
  );
}
