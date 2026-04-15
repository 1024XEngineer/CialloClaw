import type { DragEvent, PointerEvent, ReactNode, RefObject } from "react";
import type { ShellBallVoicePreview } from "./shellBall.interaction";
import type { ShellBallMotionConfig, ShellBallVisualState } from "./shellBall.types";
import { ShellBallMascot } from "./components/ShellBallMascot";

type ShellBallSurfaceProps = {
  children?: ReactNode;
  containerRef?: RefObject<HTMLDivElement>;
  dashboardTransitionPhase?: "idle" | "opening" | "hidden" | "closing";
  fileDropActive?: boolean;
  textDropActive?: boolean;
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
  onTextDrop?: (text: string) => void | Promise<void>;
  onInputProxyClick?: () => void;
  onPressStart: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressMove: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressEnd: (event: PointerEvent<HTMLButtonElement>) => boolean;
  onPressCancel: (event: PointerEvent<HTMLButtonElement>) => void;
};

type ShellBallDropDataTransfer = Pick<DataTransfer, "effectAllowed" | "files" | "getData">;

export function shouldAcceptShellBallTextDrop(dataTransfer: Pick<DataTransfer, "files"> | null) {
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

  for (const type of ["text/plain", "text", "Text", "text/uri-list"]) {
    const value = dataTransfer.getData(type).replace(/\r\n/g, "\n").trim();
    if (value !== "") {
      return value;
    }
  }

  return "";
}

export function ShellBallSurface({
  children,
  containerRef,
  dashboardTransitionPhase = "idle",
  fileDropActive = false,
  textDropActive = false,
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
  onTextDrop = () => {},
  onInputProxyClick = () => {},
  onPressStart,
  onPressMove,
  onPressEnd,
  onPressCancel,
}: ShellBallSurfaceProps) {
  const showInputProxy = visualState === "hover_input" && !inputFocused;

  function handleDragOver(event: DragEvent<HTMLElement>) {
    if (!shouldAcceptShellBallTextDrop(event.dataTransfer)) {
      return;
    }

    event.preventDefault();
    const dropEffect = resolveShellBallTextDropEffect(event.dataTransfer.effectAllowed);
    if (dropEffect !== null) {
      event.dataTransfer.dropEffect = dropEffect;
    }
  }

  function handleDrop(event: DragEvent<HTMLElement>) {
    if (!shouldAcceptShellBallTextDrop(event.dataTransfer)) {
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
        <div className="shell-ball-surface__interaction-shell">
          <section
            aria-label="Shell-ball interaction zone"
            className="shell-ball-surface__interaction-zone"
            data-shell-ball-zone="interaction"
            onPointerEnter={onRegionEnter}
            onPointerLeave={onRegionLeave}
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
                  onDragEnter={handleDragOver}
                  onDragOver={handleDragOver}
                  onDrop={handleDrop}
                  tabIndex={-1}
                  value=""
                  onChange={() => {}}
                />
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
          </section>
        </div>
      </div>
      {children}
    </div>
  );
}
