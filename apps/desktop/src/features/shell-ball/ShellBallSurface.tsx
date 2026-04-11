import type { PointerEvent, ReactNode, RefObject } from "react";
import type { ShellBallVoicePreview } from "./shellBall.interaction";
import type { ShellBallMotionConfig, ShellBallVisualState } from "./shellBall.types";
import { ShellBallMascot } from "./components/ShellBallMascot";

type ShellBallSurfaceProps = {
  children?: ReactNode;
  containerRef?: RefObject<HTMLDivElement>;
  visualState: ShellBallVisualState;
  voicePreview: ShellBallVoicePreview;
  motionConfig: ShellBallMotionConfig;
  onDragStart: () => void;
  onPrimaryClick: () => void;
  onDoubleClick: () => void;
  onRegionEnter: () => void;
  onRegionLeave: () => void;
  onPressStart: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressMove: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressEnd: (event: PointerEvent<HTMLButtonElement>) => boolean;
  onPressCancel: (event: PointerEvent<HTMLButtonElement>) => void;
};

export function ShellBallSurface({
  children,
  containerRef,
  visualState,
  voicePreview,
  motionConfig,
  onDragStart,
  onPrimaryClick,
  onDoubleClick,
  onRegionEnter,
  onRegionLeave,
  onPressStart,
  onPressMove,
  onPressEnd,
  onPressCancel,
}: ShellBallSurfaceProps) {
  return (
    <div ref={containerRef} className="shell-ball-surface" aria-label="Shell-ball floating surface">
      <div className="shell-ball-surface__core">
        <div className="shell-ball-surface__interaction-shell">
          <div
            className="shell-ball-surface__host-drag-zone"
            data-shell-ball-zone="host-drag"
            data-shell-ball-drag-handle="true"
            aria-hidden="true"
            onPointerDown={(event) => {
              if (event.button !== 0) {
                return;
              }

              event.preventDefault();
              onDragStart();
            }}
          />
          <div
            className="shell-ball-surface__interaction-zone"
            data-shell-ball-zone="interaction"
            onPointerEnter={onRegionEnter}
            onPointerLeave={onRegionLeave}
          >
            <div className="shell-ball-surface__body">
              <div className="shell-ball-surface__mascot-shell">
                <ShellBallMascot
                  visualState={visualState}
                  voicePreview={voicePreview}
                  motionConfig={motionConfig}
                  onPrimaryClick={onPrimaryClick}
                  onDoubleClick={onDoubleClick}
                  onPressStart={onPressStart}
                  onPressMove={onPressMove}
                  onPressEnd={onPressEnd}
                  onPressCancel={onPressCancel}
                />
              </div>
            </div>
          </div>
        </div>
      </div>
      {children}
    </div>
  );
}
