import { useRef } from "react";
import type { CSSProperties, MouseEvent, PointerEvent } from "react";
import { AudioLines, ShieldAlert } from "lucide-react";
import { cn } from "../../../utils/cn";
import type { ShellBallVoicePreview } from "../shellBall.interaction";
import type { ShellBallMotionConfig, ShellBallVisualState } from "../shellBall.types";

type ShellBallMascotProps = {
  visualState: ShellBallVisualState;
  voicePreview?: ShellBallVoicePreview;
  motionConfig: ShellBallMotionConfig;
  onPrimaryClick?: () => void;
  onDoubleClick?: () => void;
  onPressStart?: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressMove?: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressEnd?: (event: PointerEvent<HTMLButtonElement>) => boolean;
};

type MotionStyle = CSSProperties & Record<string, string>;

type ShellBallMascotHotspotGesture = "single_click" | "double_click";

type ShellBallMascotHotspotGestureAction = "noop" | "primary_click" | "double_click";

export function getShellBallMascotHotspotGestureAction(input: {
  visualState: ShellBallVisualState;
  gesture: ShellBallMascotHotspotGesture;
  suppressed: boolean;
}): ShellBallMascotHotspotGestureAction {
  if (input.suppressed) {
    return "noop";
  }

  if (input.gesture === "single_click") {
    return input.visualState === "voice_locked" ? "primary_click" : "noop";
  }

  if (input.visualState === "idle" || input.visualState === "hover_input") {
    return "double_click";
  }

  return "noop";
}

export function ShellBallMascot({
  visualState,
  voicePreview = null,
  motionConfig,
  onPrimaryClick = () => {},
  onDoubleClick = () => {},
  onPressStart = () => {},
  onPressMove = () => {},
  onPressEnd = () => false,
}: ShellBallMascotProps) {
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

  function handlePointerDown(event: PointerEvent<HTMLButtonElement>) {
    suppressGestureRef.current = false;
    event.currentTarget.setPointerCapture(event.pointerId);
    onPressStart(event);
  }

  function handlePointerMove(event: PointerEvent<HTMLButtonElement>) {
    onPressMove(event);
  }

  function handlePointerEnd(event: PointerEvent<HTMLButtonElement>) {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }

    const handled = onPressEnd(event);
    if (!handled) {
      return;
    }

    suppressGestureRef.current = true;
  }

  function handleClick(event: MouseEvent<HTMLButtonElement>) {
    const action = getShellBallMascotHotspotGestureAction({
      visualState,
      gesture: "single_click",
      suppressed: suppressGestureRef.current,
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
      data-voice-preview={voicePreview ?? undefined}
    >
      <div className="shell-ball-mascot__orbital shell-ball-mascot__orbital--back" />
      <div className="shell-ball-mascot__shadow" />

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

          {motionConfig.showAuthMarker ? (
            <div className="shell-ball-mascot__auth-marker" aria-hidden="true">
              <ShieldAlert className="shell-ball-mascot__auth-icon" />
            </div>
          ) : null}
        </div>
      </div>

      <div className="shell-ball-mascot__orbital shell-ball-mascot__orbital--front" />
      <div className={cn("shell-ball-mascot__perch", motionConfig.ringMode !== "hidden" && "shell-ball-mascot__perch--active")} />
      <button
        type="button"
        className="shell-ball-mascot__hotspot"
        aria-label="Shell-ball mascot"
        data-shell-ball-zone="voice-hotspot"
        onClick={handleClick}
        onDoubleClick={handleDoubleClick}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerEnd}
        onPointerCancel={handlePointerEnd}
      />
    </div>
  );
}
