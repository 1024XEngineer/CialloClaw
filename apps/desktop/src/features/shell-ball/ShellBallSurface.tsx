import type { DragEvent, PointerEvent, ReactNode, RefObject } from "react";
import type { ShellBallVoicePreview } from "./shellBall.interaction";
import type { ShellBallMotionConfig, ShellBallVisualState } from "./shellBall.types";
import type { ShellBallEdgeDockSide } from "./useShellBallWindowMetrics";
import { ShellBallMascot } from "./components/ShellBallMascot";

type ShellBallSurfaceProps = {
  bottomContent?: ReactNode;
  children?: ReactNode;
  containerRef?: RefObject<HTMLDivElement>;
  dashboardTransitionPhase?: "idle" | "opening" | "hidden" | "closing";
  edgeDockRevealed?: boolean;
  edgeDockSide?: ShellBallEdgeDockSide | null;
  fileDropActive?: boolean;
  mascotRef?: RefObject<HTMLDivElement>;
  overlayContent?: ReactNode;
  textDropActive?: boolean;
  selectionIndicatorVisible?: boolean;
  topContent?: ReactNode;
  visualState: ShellBallVisualState;
  voicePreview: ShellBallVoicePreview;
  voiceHoldProgress?: number;
  inputFocused?: boolean;
  motionConfig: ShellBallMotionConfig;
  onDragStart: (event: PointerEvent<HTMLButtonElement>) => void;
  onDragMove: (event: PointerEvent<HTMLButtonElement>) => void;
  onDragEnd: (event: PointerEvent<HTMLButtonElement>) => void;
  onDragCancel: (event: PointerEvent<HTMLButtonElement>) => void;
  onPrimaryClick: () => void;
  onDoubleClick: () => void;
  onRegionEnter: () => void;
  onRegionLeave: () => void;
  onTextDrop?: (text: string) => void | Promise<void>;
  onPressStart: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressMove: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressEnd: (event: PointerEvent<HTMLButtonElement>) => boolean;
  onPressCancel: (event: PointerEvent<HTMLButtonElement>) => void;
};

type ShellBallDropDataTransfer = Pick<DataTransfer, "effectAllowed" | "files" | "getData">;

export function shouldAcceptShellBallTextDrop(dataTransfer: Pick<DataTransfer, "files"> | null): dataTransfer is ShellBallDropDataTransfer {
  return dataTransfer !== null && dataTransfer.files.length === 0;
}

export function resolveShellBallTextDropEffect(effectAllowed: DataTransfer["effectAllowed"]) {
  if (effectAllowed === "copy" || effectAllowed === "copyLink" || effectAllowed === "copyMove" || effectAllowed === "all" || effectAllowed === "uninitialized") {
    return "copy" as const;
  }

  if (effectAllowed === "move" || effectAllowed === "linkMove") {
    return "move" as const;
  }

  if (effectAllowed === "link") {
    return "link" as const;
  }

  return null;
}

export function extractShellBallDroppedText(dataTransfer: ShellBallDropDataTransfer | null) {
  if (!shouldAcceptShellBallTextDrop(dataTransfer)) {
    return "";
  }

  // The acceptability check is not a TypeScript type guard, so keep the null
  // branch explicit before reading transfer payloads.
  if (dataTransfer === null) {
    return "";
  }

  for (const type of ["text/plain", "text", "Text", "text/uri-list"]) {
    const value = dataTransfer.getData(type).replace(/\r\n/g, "\n").trim();
    if (value !== "") {
      return value;
    }
  }

  return "";
}

export function ShellBallSurface({
  bottomContent,
  children,
  containerRef,
  dashboardTransitionPhase = "idle",
  edgeDockRevealed = false,
  edgeDockSide = null,
  fileDropActive = false,
  mascotRef,
  overlayContent,
  textDropActive = false,
  selectionIndicatorVisible = false,
  topContent,
  visualState,
  voicePreview,
  voiceHoldProgress = 0,
  inputFocused: _inputFocused = false,
  motionConfig,
  onDragStart,
  onDragMove,
  onDragEnd,
  onDragCancel,
  onPrimaryClick,
  onDoubleClick,
  onRegionEnter,
  onRegionLeave,
  onTextDrop = () => {},
  onPressStart,
  onPressMove,
  onPressEnd,
  onPressCancel,
}: ShellBallSurfaceProps) {
  // Only the armed text target is allowed to consume drag events.
  function handleDragOver(event: DragEvent<HTMLElement>) {
    if (!textDropActive || !shouldAcceptShellBallTextDrop(event.dataTransfer)) {
      return;
    }

    event.preventDefault();
    const dropEffect = resolveShellBallTextDropEffect(event.dataTransfer.effectAllowed);
    if (dropEffect !== null) {
      event.dataTransfer.dropEffect = dropEffect;
    }
  }

  function handleDrop(event: DragEvent<HTMLElement>) {
    if (!textDropActive || !shouldAcceptShellBallTextDrop(event.dataTransfer)) {
      return;
    }

    event.preventDefault();
    const droppedText = extractShellBallDroppedText(event.dataTransfer);
    if (droppedText === "") {
      return;
    }

    void onTextDrop(droppedText);
  }

  return (
    <div
      ref={containerRef}
      className="shell-ball-surface"
      data-dashboard-transition-phase={dashboardTransitionPhase}
      data-file-drop-active={fileDropActive ? "true" : "false"}
      onDragEnterCapture={handleDragOver}
      onDragOverCapture={handleDragOver}
      onDropCapture={handleDrop}
    >
      <div className="shell-ball-surface__core">
        {topContent ? <div className="shell-ball-surface__slot shell-ball-surface__slot--top">{topContent}</div> : null}
        <div className="shell-ball-surface__interaction-shell">
          <section
            aria-label="Shell-ball interaction zone"
            className="shell-ball-surface__interaction-zone"
            data-shell-ball-zone="interaction"
          >
              <div className="shell-ball-surface__body">
                <div
                  aria-hidden={!fileDropActive}
                  className="shell-ball-surface__file-drop-overlay"
                  data-visible={fileDropActive ? "true" : "false"}
                >
                  <span className="shell-ball-surface__file-drop-plus shell-ball-surface__file-drop-plus--horizontal" />
                  <span className="shell-ball-surface__file-drop-plus shell-ball-surface__file-drop-plus--vertical" />
                </div>
                <textarea
                  aria-hidden={!textDropActive}
                  className="shell-ball-surface__text-drop-target"
                  data-visible={textDropActive ? "true" : "false"}
                  tabIndex={-1}
                  value=""
                  onChange={() => {}}
                />
                <div
                  ref={mascotRef}
                  className="shell-ball-surface__mascot-shell"
                >
                    <ShellBallMascot
                      edgeDockRevealed={edgeDockRevealed}
                      edgeDockSide={edgeDockSide}
                      visualState={visualState}
                      voicePreview={voicePreview}
                    selectionIndicatorVisible={selectionIndicatorVisible}
                    voiceHoldProgress={voiceHoldProgress}
                    motionConfig={motionConfig}
                    onPrimaryClick={onPrimaryClick}
                    onDoubleClick={onDoubleClick}
                    onHotspotEnter={onRegionEnter}
                    onHotspotLeave={onRegionLeave}
                    onHotspotDragStart={onDragStart}
                    onHotspotDragMove={onDragMove}
                    onHotspotDragEnd={onDragEnd}
                    onHotspotDragCancel={onDragCancel}
                    onPressStart={onPressStart}
                    onPressMove={onPressMove}
                    onPressEnd={onPressEnd}
                    onPressCancel={onPressCancel}
                  />
                </div>
                {overlayContent ? (
                  <div className="shell-ball-surface__overlay">
                    <div className="shell-ball-surface__voice-anchor">{overlayContent}</div>
                  </div>
                ) : null}
              </div>
            </section>
        </div>
        {bottomContent ? (
          <div className="shell-ball-surface__slot shell-ball-surface__slot--bottom">
            <div className="shell-ball-surface__slot-visual shell-ball-surface__slot-visual--bottom">{bottomContent}</div>
          </div>
        ) : null}
        {children ? <div className="shell-ball-surface__stack">{children}</div> : null}
      </div>
    </div>
  );
}
