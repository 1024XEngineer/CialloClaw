import { useRef } from "react";
import type { CSSProperties, MouseEvent, PointerEvent } from "react";
import { AudioLines, Mic, ShieldAlert } from "lucide-react";
import { cn } from "../../../utils/cn";
import { SHELL_BALL_PRESS_DRIFT_TOLERANCE_PX, type ShellBallVoicePreview } from "../shellBall.interaction";
import type { ShellBallMotionConfig, ShellBallVisualState } from "../shellBall.types";

type ShellBallMascotProps = {
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
  onHotspotDragGestureChange?: (active: boolean) => void;
  onPressStart?: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressMove?: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressEnd?: (event: PointerEvent<HTMLButtonElement>) => boolean;
  onPressCancel?: (event: PointerEvent<HTMLButtonElement>) => void;
};

type MotionStyle = CSSProperties & Record<string, string>;

type ShellBallMascotHotspotGesture = "single_click" | "double_click";

type ShellBallMascotHotspotGestureAction = "noop" | "primary_click" | "double_click";

type ShellBallMascotPointerPhase = "pointer_down" | "pointer_up" | "pointer_cancel";

type ShellBallMascotPointerPhaseAction = "noop" | "start_press" | "finish_press" | "suppress_gestures" | "cleanup_only";

export function getShellBallMascotHotspotGestureAction(input: {
  visualState: ShellBallVisualState;
  gesture: ShellBallMascotHotspotGesture;
  suppressed: boolean;
  selectionIndicatorVisible?: boolean;
}): ShellBallMascotHotspotGestureAction {
  if (input.suppressed) {
    return "noop";
  }

  if (input.gesture === "single_click") {
    return input.selectionIndicatorVisible ? "primary_click" : "noop";
  }

  if (input.visualState === "idle" || input.visualState === "hover_input") {
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

export function ShellBallMascot({
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
  onHotspotDragGestureChange = () => {},
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

  const floatStyle: MotionStyle = {
    "--shell-ball-float-distance": `${motionConfig.floatOffsetY}px`,
    "--shell-ball-float-duration": `${motionConfig.floatDurationMs}ms`,
  };
  const bodyShellStyle: MotionStyle = {
    "--shell-ball-breathe-scale": String(motionConfig.breatheScale),
    "--shell-ball-breathe-duration": `${motionConfig.breatheDurationMs}ms`,
  };
  const attitudeStyle: CSSProperties = {
    transform: `rotate(${motionConfig.bodyTiltDeg}deg) scale(${motionConfig.bodyScale})`,
  };
  const wingStyle: MotionStyle = {
    "--shell-ball-wing-lift": `${motionConfig.wingLiftDeg}deg`,
    "--shell-ball-wing-duration": `${motionConfig.wingDurationMs}ms`,
    "--shell-ball-wing-spread": `${motionConfig.wingSpreadPx}px`,
  };
  const tailStyle: MotionStyle = {
    "--shell-ball-tail-swing": `${motionConfig.tailSwingDeg}deg`,
    "--shell-ball-tail-duration": `${motionConfig.tailDurationMs}ms`,
  };
  const eyeStyle: CSSProperties = {
    animationDelay: `${motionConfig.blinkDelayMs}ms`,
  };
  const crestStyle: CSSProperties = {
    transform: `translateY(${-motionConfig.crestLiftPx}px)`,
  };
  const holdRingCircumference = 2 * Math.PI * 84;
  const holdRingDashOffset = holdRingCircumference * (1 - voiceHoldProgress);
  const showVoiceHoldRing = voiceHoldProgress > 0 && visualState !== "voice_listening" && visualState !== "voice_locked";
  const shouldRenderVoiceHints = showVoiceHints && (visualState === "voice_listening" || visualState === "voice_locked");
  const showVoiceMarker = visualState === "voice_listening" || visualState === "voice_locked";
  const showSelectionMarker = selectionIndicatorVisible && !showVoiceMarker;
  const shouldRouteHotspotDrag = visualState !== "voice_listening" && visualState !== "voice_locked";

  function resetPointerSequence() {
    activeSequenceRef.current = false;
    draggingSequenceRef.current = false;
    pointerStartXRef.current = null;
    pointerStartYRef.current = null;
    onHotspotDragGestureChange(false);
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
      onHotspotDragGestureChange(true);
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
      visualState,
      gesture: "single_click",
      suppressed: suppressGestureRef.current,
      selectionIndicatorVisible,
    });

    if (action !== "primary_click") {
      return;
    }

    onPrimaryClick();
  }

  function handleDoubleClick(event: MouseEvent<HTMLButtonElement>) {
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
      data-state={visualState}
      data-tone={motionConfig.accentTone}
      data-voice-hints={shouldRenderVoiceHints ? "true" : "false"}
      data-voice-preview={voicePreview ?? undefined}
    >
      <div className="shell-ball-mascot__orbital shell-ball-mascot__orbital--back" />
      <div className="shell-ball-mascot__shadow" />

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

      {motionConfig.ringMode === "hidden" ? null : (
        <div className="shell-ball-mascot__rings" data-ring={motionConfig.ringMode}>
          <span className="shell-ball-mascot__ring shell-ball-mascot__ring--outer" />
          <span className="shell-ball-mascot__ring shell-ball-mascot__ring--inner" />
          <span className="shell-ball-mascot__ring-core">
            <AudioLines className="shell-ball-mascot__ring-icon" />
          </span>
        </div>
      )}

      <div className="shell-ball-mascot__float" style={floatStyle}>
        <div className="shell-ball-mascot__attitude" style={attitudeStyle}>
          <div className="shell-ball-mascot__tail-shell" style={tailStyle}>
            <div className="shell-ball-mascot__tail" />
          </div>

          <div className="shell-ball-mascot__wing-shell shell-ball-mascot__wing-shell--left" style={wingStyle}>
            <div className="shell-ball-mascot__wing" data-mode={motionConfig.wingMode} data-side="left" />
          </div>
          <div className="shell-ball-mascot__wing-shell shell-ball-mascot__wing-shell--right" style={wingStyle}>
            <div className="shell-ball-mascot__wing" data-mode={motionConfig.wingMode} data-side="right" />
          </div>

          <div className="shell-ball-mascot__body-shell" style={bodyShellStyle}>
            <div className="shell-ball-mascot__crest" style={crestStyle}>
              <span className="shell-ball-mascot__crest-feather shell-ball-mascot__crest-feather--left" />
              <span className="shell-ball-mascot__crest-feather shell-ball-mascot__crest-feather--center" />
              <span className="shell-ball-mascot__crest-feather shell-ball-mascot__crest-feather--right" />
            </div>

            <div className="shell-ball-mascot__body">
              <div className="shell-ball-mascot__belly" />
              <div className="shell-ball-mascot__cheek shell-ball-mascot__cheek--left" />
              <div className="shell-ball-mascot__cheek shell-ball-mascot__cheek--right" />

              <div className="shell-ball-mascot__face">
                <div className="shell-ball-mascot__eyes" data-eye={motionConfig.eyeMode} style={eyeStyle}>
                  <span className="shell-ball-mascot__eye" />
                  <span className="shell-ball-mascot__eye" />
                </div>
                <div className="shell-ball-mascot__beak" />
              </div>
            </div>
          </div>

          {showSelectionMarker ? (
            <div className="shell-ball-mascot__selection-marker" aria-hidden="true">
              <span className="shell-ball-mascot__selection-marker-glyph">!</span>
            </div>
          ) : null}

          {showVoiceMarker ? (
            <div className={cn("shell-ball-mascot__voice-marker", visualState === "voice_locked" && "is-locked")} aria-hidden="true">
              <Mic className="shell-ball-mascot__voice-marker-icon" />
            </div>
          ) : null}

          {motionConfig.showAuthMarker ? (
            <div className="shell-ball-mascot__auth-marker" aria-hidden="true">
              <ShieldAlert className="shell-ball-mascot__auth-icon" />
            </div>
          ) : null}
        </div>
      </div>

      <div className="shell-ball-mascot__orbital shell-ball-mascot__orbital--front" />
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
