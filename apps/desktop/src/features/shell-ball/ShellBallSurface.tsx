import type { PointerEvent, ReactNode, RefObject } from "react";
import type { ShellBallVoicePreview } from "./shellBall.interaction";
import type { ShellBallMotionConfig, ShellBallVisualState } from "./shellBall.types";
import { ShellBallMascot } from "./components/ShellBallMascot";

type ShellBallSurfaceProps = {
  children?: ReactNode;
  containerRef?: RefObject<HTMLDivElement>;
  dashboardTransitionPhase?: "idle" | "opening" | "hidden" | "closing";
  visualState: ShellBallVisualState;
  voicePreview: ShellBallVoicePreview;
  voiceHoldProgress?: number;
  inputFocused?: boolean;
  motionConfig: ShellBallMotionConfig;
  onDragStart: () => void;
  onPrimaryClick: () => void;
  onDoubleClick: () => void;
  onRegionEnter: () => void;
  onRegionLeave: () => void;
  onInputProxyClick?: () => void;
  onPressStart: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressMove: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressEnd: (event: PointerEvent<HTMLButtonElement>) => boolean;
  onPressCancel: (event: PointerEvent<HTMLButtonElement>) => void;
};

export function ShellBallSurface({
  children,
  containerRef,
  dashboardTransitionPhase = "idle",
  visualState,
  voicePreview,
  voiceHoldProgress = 0,
  inputFocused = false,
  motionConfig,
  onDragStart,
  onPrimaryClick,
  onDoubleClick,
  onRegionEnter,
  onRegionLeave,
  onInputProxyClick = () => {},
  onPressStart,
  onPressMove,
  onPressEnd,
  onPressCancel,
}: ShellBallSurfaceProps) {
  const showInputProxy = visualState === "hover_input" && !inputFocused;

  return (
    <div
      ref={containerRef}
      className="shell-ball-surface"
      data-dashboard-transition-phase={dashboardTransitionPhase}
    >
      <div className="shell-ball-surface__core">
        <div className="shell-ball-surface__interaction-shell">
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
                    voiceHoldProgress={voiceHoldProgress}
                    motionConfig={motionConfig}
                    onPrimaryClick={onPrimaryClick}
                    onDoubleClick={onDoubleClick}
                    onHotspotDragStart={onDragStart}
                    onPressStart={onPressStart}
                    onPressMove={onPressMove}
                    onPressEnd={onPressEnd}
                    onPressCancel={onPressCancel}
                  />
                </div>
                <button
                  aria-hidden={!showInputProxy}
                  className="shell-ball-surface__input-line-proxy"
                  data-visible={showInputProxy}
                  onClick={onInputProxyClick}
                  tabIndex={showInputProxy ? 0 : -1}
                  type="button"
                />
              </div>
          </div>
        </div>
      </div>
      {children}
    </div>
  );
}
