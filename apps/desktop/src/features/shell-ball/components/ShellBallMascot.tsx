import type { CSSProperties } from "react";
import { AudioLines, ShieldAlert } from "lucide-react";
import { cn } from "../../../utils/cn";
import type { ShellBallMotionConfig, ShellBallVisualState } from "../shellBall.types";

type ShellBallMascotProps = {
  visualState: ShellBallVisualState;
  motionConfig: ShellBallMotionConfig;
};

type MotionStyle = CSSProperties & Record<string, string>;

export function ShellBallMascot({ visualState, motionConfig }: ShellBallMascotProps) {
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

  return (
    <div className="shell-ball-mascot" data-state={visualState} data-tone={motionConfig.accentTone}>
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
    </div>
  );
}
