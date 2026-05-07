import { useEffect, useRef, useState } from "react";
import type { CSSProperties, MouseEvent, PointerEvent } from "react";
import { Mic, ShieldAlert } from "lucide-react";
import { cn } from "../../../utils/cn";
import { SHELL_BALL_PRESS_DRIFT_TOLERANCE_PX, type ShellBallVoicePreview } from "../shellBall.interaction";
import type { ShellBallMotionConfig, ShellBallVisualState } from "../shellBall.types";
import { FLOATING_PET_HAPPY_DURATION_MS, type FloatingPetMode } from "./floating-pet/petAssets";
import { FloatingPet } from "./floating-pet/FloatingPet";
import type { ShellBallEdgeDockSide } from "../useShellBallWindowMetrics";

type ShellBallMascotProps = {
  dockTarget?: ShellBallEdgeDockSide | null;
  edgeDockRevealed?: boolean;
  edgeDockSide?: ShellBallEdgeDockSide | null;
  floatingBallSize?: "small" | "medium" | "large";
  hasAlertOpportunity?: boolean;
  hasPendingAgentLoading?: boolean;
  hasPendingApproval?: boolean;
  isDragging?: boolean;
  isSettling?: boolean;
  visualState: ShellBallVisualState;
  voicePreview?: ShellBallVoicePreview;
  showVoiceHints?: boolean;
  selectionIndicatorVisible?: boolean;
  voiceHoldProgress?: number;
  motionConfig: ShellBallMotionConfig;
  onPrimaryClick?: () => void;
  onDoubleClick?: () => void;
  onHotspotEnter?: () => void;
  onHotspotLeave?: () => void;
  onHotspotDragStart?: (event: PointerEvent<HTMLButtonElement>) => void;
  onHotspotDragMove?: (event: PointerEvent<HTMLButtonElement>) => void;
  onHotspotDragEnd?: (event: PointerEvent<HTMLButtonElement>) => void;
  onHotspotDragCancel?: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressStart?: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressMove?: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressEnd?: (event: PointerEvent<HTMLButtonElement>) => boolean;
  onPressCancel?: (event: PointerEvent<HTMLButtonElement>) => void;
};

type ShellBallMascotHotspotGesture = "single_click" | "double_click";

type ShellBallMascotHotspotGestureAction = "noop" | "primary_click" | "double_click";

type ShellBallMascotPointerPhase = "pointer_down" | "pointer_up" | "pointer_cancel";

type ShellBallMascotPointerPhaseAction = "noop" | "start_press" | "finish_press" | "suppress_gestures" | "cleanup_only";

type ShellBallMascotPetState = {
  listenLocked: boolean;
  mode: FloatingPetMode;
};

type MascotCssVars = CSSProperties & Record<string, string | number | undefined>;

function canTriggerShellBallMascotHappyPulse(visualState: ShellBallVisualState) {
  return visualState === "idle" || visualState === "hover_input";
}

function canTriggerShellBallMascotSecondaryGestures(visualState: ShellBallVisualState) {
  return visualState !== "voice_listening" && visualState !== "voice_locked";
}

export function getShellBallMascotHotspotGestureAction(input: {
  visualState: ShellBallVisualState;
  gesture: ShellBallMascotHotspotGesture;
  suppressed: boolean;
  alertOpportunityAvailable?: boolean;
  selectionIndicatorVisible?: boolean;
}): ShellBallMascotHotspotGestureAction {
  if (input.suppressed) {
    return "noop";
  }

  if (input.gesture === "single_click") {
    return input.selectionIndicatorVisible || input.alertOpportunityAvailable ? "primary_click" : "noop";
  }

  if (canTriggerShellBallMascotSecondaryGestures(input.visualState)) {
    return "double_click";
  }

  return "noop";
}

export function getShellBallMascotPointerPhaseAction(input: {
  phase: ShellBallMascotPointerPhase;
  button: number;
  isPrimary: boolean;
  pressHandled: boolean;
}): ShellBallMascotPointerPhaseAction {
  if (input.phase === "pointer_cancel") {
    return input.isPrimary ? "cleanup_only" : "noop";
  }

  const isPrimaryButtonSequence = input.button === 0 && input.isPrimary;

  if (!isPrimaryButtonSequence) {
    return "noop";
  }

  if (input.phase === "pointer_down") {
    return "start_press";
  }

  return input.pressHandled ? "suppress_gestures" : "finish_press";
}

export function shouldSuppressShellBallMascotHotspotGestures(input: {
  startX: number | null;
  startY: number | null;
  pointerX: number;
  pointerY: number;
}) {
  if (input.startX === null || input.startY === null) {
    return false;
  }

  return Math.hypot(input.pointerX - input.startX, input.pointerY - input.startY) > SHELL_BALL_PRESS_DRIFT_TOLERANCE_PX;
}

export function getShellBallMascotPetState(input: {
  hasAlertOpportunity?: boolean;
  hasPendingAgentLoading?: boolean;
  hasPendingApproval?: boolean;
  happyActive: boolean;
  selectionIndicatorVisible?: boolean;
  visualState: ShellBallVisualState;
}): ShellBallMascotPetState {
  if (input.hasPendingAgentLoading || input.visualState === "processing") {
    return {
      listenLocked: false,
      mode: "think",
    };
  }

  if (input.visualState === "voice_locked") {
    return {
      listenLocked: true,
      mode: "listen",
    };
  }

  if (input.visualState === "voice_listening") {
    return {
      listenLocked: false,
      mode: "listen",
    };
  }

  if (input.visualState === "confirming_intent") {
    return {
      listenLocked: false,
      mode: "alert",
    };
  }

  if (input.hasPendingApproval || input.visualState === "waiting_auth") {
    return {
      listenLocked: false,
      mode: "safe",
    };
  }

  if (input.selectionIndicatorVisible || input.hasAlertOpportunity) {
    return {
      listenLocked: false,
      mode: "alert",
    };
  }

  if (input.happyActive) {
    return {
      listenLocked: false,
      mode: "happy",
    };
  }

  return {
    listenLocked: false,
    mode: "idle",
  };
}

function resolveShellBallDockStyle(input: {
  edgeDockSide: ShellBallEdgeDockSide | null;
  edgeDockRevealed: boolean;
}): CSSProperties | undefined {
  if (input.edgeDockSide === null) {
    return undefined;
  }

  let shiftX = 0;
  let shiftY = 0;

  switch (input.edgeDockSide) {
    case "left":
      shiftX = input.edgeDockRevealed ? 2 : 6;
      break;
    case "right":
      shiftX = input.edgeDockRevealed ? -2 : -6;
      break;
    case "top":
      shiftY = input.edgeDockRevealed ? 0 : -8;
      break;
    case "top_left":
      shiftX = input.edgeDockRevealed ? 1 : 5;
      shiftY = input.edgeDockRevealed ? 0 : -8;
      break;
    case "top_right":
      shiftX = input.edgeDockRevealed ? -1 : -5;
      shiftY = input.edgeDockRevealed ? 0 : -8;
      break;
    case "bottom_left":
      shiftX = input.edgeDockRevealed ? 1 : 4;
      shiftY = input.edgeDockRevealed ? 0 : 4;
      break;
    case "bottom_right":
      shiftX = input.edgeDockRevealed ? -1 : -4;
      shiftY = input.edgeDockRevealed ? 0 : 4;
      break;
    default:
      shiftY = input.edgeDockRevealed ? 0 : 4;
      break;
  }

  if (shiftX === 0 && shiftY === 0) {
    return undefined;
  }

  return {
    transform: `translate(${shiftX}px, ${shiftY}px)`,
  };
}

export function ShellBallMascot({
  dockTarget = null,
  edgeDockRevealed = false,
  edgeDockSide = null,
  floatingBallSize,
  hasAlertOpportunity = false,
  hasPendingAgentLoading = false,
  hasPendingApproval = false,
  isDragging = false,
  isSettling = false,
  visualState,
  voicePreview = null,
  showVoiceHints = true,
  selectionIndicatorVisible = false,
  voiceHoldProgress = 0,
  motionConfig,
  onPrimaryClick = () => {},
  onDoubleClick = () => {},
  onHotspotEnter = () => {},
  onHotspotLeave = () => {},
  onHotspotDragStart = () => {},
  onHotspotDragMove = () => {},
  onHotspotDragEnd = () => {},
  onHotspotDragCancel = () => {},
  onPressStart = () => {},
  onPressMove = () => {},
  onPressEnd = () => false,
  onPressCancel = () => {},
}: ShellBallMascotProps) {
  const activeSequenceRef = useRef(false);
  const draggingSequenceRef = useRef(false);
  const pointerStartXRef = useRef<number | null>(null);
  const pointerStartYRef = useRef<number | null>(null);
  const suppressGestureRef = useRef(false);
  const happyTimeoutRef = useRef<number | null>(null);
  const [happyActive, setHappyActive] = useState(false);

  const holdRingCircumference = 2 * Math.PI * 84;
  const holdRingDashOffset = holdRingCircumference * (1 - voiceHoldProgress);
  const showVoiceHoldRing = voiceHoldProgress > 0 && visualState !== "voice_listening" && visualState !== "voice_locked";
  const shouldRenderVoiceHints = showVoiceHints && (visualState === "voice_listening" || visualState === "voice_locked");
  const showVoiceMarker = visualState === "voice_listening" || visualState === "voice_locked";
  const shouldRouteHotspotDrag = visualState !== "voice_listening" && visualState !== "voice_locked";
  const petState = getShellBallMascotPetState({
    hasAlertOpportunity,
    hasPendingAgentLoading,
    hasPendingApproval,
    happyActive,
    selectionIndicatorVisible,
    visualState,
  });
  const showAuthMarker = motionConfig.showAuthMarker || petState.mode === "safe";
  const dockStyle = resolveShellBallDockStyle({ edgeDockRevealed, edgeDockSide });
  const orbitalStyle: MascotCssVars = { "--shell-ball-orbital-scale": 1 };
  const shadowStyle: MascotCssVars = {
    "--shell-ball-shadow-offset-y": "0px",
    "--shell-ball-shadow-scale-x": 1,
    "--shell-ball-shadow-scale-y": 1,
  };

  useEffect(() => {
    return () => {
      if (happyTimeoutRef.current !== null) {
        window.clearTimeout(happyTimeoutRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (canTriggerShellBallMascotHappyPulse(visualState)) {
      return;
    }

    if (happyTimeoutRef.current !== null) {
      window.clearTimeout(happyTimeoutRef.current);
      happyTimeoutRef.current = null;
    }

    setHappyActive(false);
  }, [visualState]);

  function triggerHappyPulse() {
    if (!canTriggerShellBallMascotHappyPulse(visualState)) {
      return;
    }

    if (happyTimeoutRef.current !== null) {
      return;
    }

    setHappyActive(true);
    happyTimeoutRef.current = window.setTimeout(() => {
      happyTimeoutRef.current = null;
      setHappyActive(false);
    }, FLOATING_PET_HAPPY_DURATION_MS);
  }

  function resetPointerSequence() {
    activeSequenceRef.current = false;
    draggingSequenceRef.current = false;
    pointerStartXRef.current = null;
    pointerStartYRef.current = null;
  }

  function handlePointerDown(event: PointerEvent<HTMLButtonElement>) {
    if (
      getShellBallMascotPointerPhaseAction({
        phase: "pointer_down",
        button: event.button,
        isPrimary: event.isPrimary,
        pressHandled: false,
      }) !== "start_press"
    ) {
      return;
    }

    // Prevent pointer drag from leaving a focus ring on the hotspot button.
    event.preventDefault();
    event.currentTarget.blur();
    suppressGestureRef.current = false;
    activeSequenceRef.current = true;
    draggingSequenceRef.current = false;
    pointerStartXRef.current = event.screenX;
    pointerStartYRef.current = event.screenY;
    event.currentTarget.setPointerCapture(event.pointerId);
    onPressStart(event);
    onHotspotDragStart(event);
  }

  function handlePointerMove(event: PointerEvent<HTMLButtonElement>) {
    if (!activeSequenceRef.current) {
      return;
    }

    if (shouldRouteHotspotDrag) {
      onHotspotDragMove(event);
    }

    if (
      !draggingSequenceRef.current &&
      shouldSuppressShellBallMascotHotspotGestures({
        startX: pointerStartXRef.current,
        startY: pointerStartYRef.current,
        pointerX: event.screenX,
        pointerY: event.screenY,
      })
    ) {
      draggingSequenceRef.current = true;
      suppressGestureRef.current = true;
    }

    onPressMove(event);
  }

  function handlePointerEnd(event: PointerEvent<HTMLButtonElement>) {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }

    if (!activeSequenceRef.current) {
      return;
    }

    const pointerAction = getShellBallMascotPointerPhaseAction({
      phase: "pointer_up",
      button: event.button,
      isPrimary: event.isPrimary,
      pressHandled: false,
    });

    if (pointerAction === "noop") {
      return;
    }

    const dragSuppressed = draggingSequenceRef.current;
    const handled = onPressEnd(event);

    if (!handled) {
      onHotspotDragEnd(event);
    }

    resetPointerSequence();
    const action = getShellBallMascotPointerPhaseAction({
      phase: "pointer_up",
      button: event.button,
      isPrimary: event.isPrimary,
      pressHandled: handled,
    });

    if (action !== "suppress_gestures" && !dragSuppressed) {
      return;
    }

    suppressGestureRef.current = true;
  }

  function handlePointerCancel(event: PointerEvent<HTMLButtonElement>) {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }

    const action = getShellBallMascotPointerPhaseAction({
      phase: "pointer_cancel",
      button: event.button,
      isPrimary: event.isPrimary,
      pressHandled: false,
    });

    if (action !== "cleanup_only") {
      return;
    }

    suppressGestureRef.current = false;
    const shouldNotifyCancel = activeSequenceRef.current;
    if (shouldNotifyCancel && shouldRouteHotspotDrag) {
      onHotspotDragCancel(event);
    }
    resetPointerSequence();
    if (shouldNotifyCancel) {
      onPressCancel(event);
    }
  }

  function handleClick(event: MouseEvent<HTMLButtonElement>) {
    const action = getShellBallMascotHotspotGestureAction({
      alertOpportunityAvailable: hasAlertOpportunity,
      visualState,
      gesture: "single_click",
      suppressed: suppressGestureRef.current,
      selectionIndicatorVisible,
    });

    if (event.detail === 1 && !suppressGestureRef.current) {
      triggerHappyPulse();
    }

    if (action === "primary_click") {
      onPrimaryClick();
    }
  }

  function handleDoubleClick(_event: MouseEvent<HTMLButtonElement>) {
    const action = getShellBallMascotHotspotGestureAction({
      visualState,
      gesture: "double_click",
      suppressed: suppressGestureRef.current,
    });

    if (action !== "double_click") {
      return;
    }

    onDoubleClick();
  }

  return (
    <div
      className={cn("shell-ball-mascot", voicePreview !== null && `shell-ball-mascot--preview-${voicePreview}`)}
      data-shell-ball-pet-listen-locked={petState.listenLocked ? "true" : "false"}
      data-shell-ball-pet-mode={petState.mode}
      data-state={visualState}
      data-dock-target={dockTarget ?? "none"}
      data-edge-dock-revealed={edgeDockRevealed ? "true" : "false"}
      data-edge-dock-side={edgeDockSide ?? "none"}
      data-floating-ball-size={floatingBallSize}
      data-shell-ball-dragging={isDragging ? "true" : "false"}
      data-shell-ball-settling={isSettling ? "true" : "false"}
      data-tone={motionConfig.accentTone}
      data-voice-hints={shouldRenderVoiceHints ? "true" : "false"}
      data-voice-preview={voicePreview ?? undefined}
    >
      <div className="shell-ball-mascot__orbital shell-ball-mascot__orbital--back" style={orbitalStyle} />
      <div className="shell-ball-mascot__shadow" style={shadowStyle} />

      {showVoiceHoldRing ? (
        <svg aria-hidden="true" className="shell-ball-mascot__hold-ring" viewBox="0 0 190 190">
          <circle cx="95" cy="95" fill="none" r="84" stroke="rgba(255,255,255,0.28)" strokeWidth="4" />
          <circle
            cx="95"
            cy="95"
            fill="none"
            r="84"
            stroke="rgba(106,145,200,0.78)"
            strokeDasharray={holdRingCircumference}
            strokeDashoffset={holdRingDashOffset}
            strokeLinecap="round"
            strokeWidth="5"
            transform="rotate(-90 95 95)"
          />
        </svg>
      ) : null}

      <div className="shell-ball-mascot__attitude" style={dockStyle}>
        <div className="shell-ball-mascot__visual">
          <div className="shell-ball-mascot__pet-shell">
            <FloatingPet className="shell-ball-mascot__pet" listenLocked={petState.listenLocked} mode={petState.mode} />
          </div>

          {showVoiceMarker ? (
            <div className={cn("shell-ball-mascot__voice-marker", visualState === "voice_locked" && "is-locked")} aria-hidden="true">
              <Mic className="shell-ball-mascot__voice-marker-icon" />
            </div>
          ) : null}

          {showAuthMarker ? (
            <div className="shell-ball-mascot__auth-marker" aria-hidden="true">
              <ShieldAlert className="shell-ball-mascot__auth-icon" />
            </div>
          ) : null}
        </div>
      </div>

      <div className="shell-ball-mascot__orbital shell-ball-mascot__orbital--front" style={orbitalStyle} />
      <button
        type="button"
        className="shell-ball-mascot__hotspot"
        aria-label="Shell-ball mascot"
        data-shell-ball-interactive="true"
        data-shell-ball-zone="voice-hotspot"
        onClick={handleClick}
        onDoubleClick={handleDoubleClick}
        onPointerEnter={onHotspotEnter}
        onPointerLeave={onHotspotLeave}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerEnd}
        onPointerCancel={handlePointerCancel}
      />
    </div>
  );
}
