import { ShellBallBubbleZone } from "./components/ShellBallBubbleZone";
import { ShellBallDemoSwitcher } from "./components/ShellBallDemoSwitcher";
import { ShellBallInputBar } from "./components/ShellBallInputBar";
import { ShellBallMascot } from "./components/ShellBallMascot";
import { useShellBallInteraction } from "./useShellBallInteraction";
import { getShellBallMotionConfig } from "./shellBall.motion";

export function ShellBallApp() {
  const {
    visualState,
    inputValue,
    setInputValue,
    voicePreview,
    inputBarMode,
    handlePrimaryClick,
    handleRegionEnter,
    handleRegionLeave,
    handleSubmitText,
    handleAttachFile,
    handlePressStart,
    handlePressMove,
    handlePressEnd,
    handleInputFocusChange,
    handleForceState,
  } = useShellBallInteraction();
  const motionConfig = getShellBallMotionConfig(visualState);

  return (
    <div className="shell-ball-surface" aria-label="Shell-ball floating surface">
      <div className="shell-ball-surface__core">
        <div className="shell-ball-surface__stack">
          <ShellBallBubbleZone visualState={visualState} />
          <div className="shell-ball-surface__region" onPointerEnter={handleRegionEnter} onPointerLeave={handleRegionLeave}>
            <div className="shell-ball-surface__body">
              <div className="shell-ball-surface__mascot-shell">
                <ShellBallMascot
                  visualState={visualState}
                  voicePreview={voicePreview}
                  motionConfig={motionConfig}
                  onPrimaryClick={handlePrimaryClick}
                  onPressStart={(event) => handlePressStart(event.clientX, event.clientY)}
                  onPressMove={(event) => handlePressMove(event.clientX, event.clientY)}
                  onPressEnd={handlePressEnd}
                />
              </div>
              <ShellBallInputBar
                mode={inputBarMode}
                voicePreview={voicePreview}
                value={inputValue}
                onValueChange={setInputValue}
                onAttachFile={handleAttachFile}
                onSubmit={handleSubmitText}
                onFocusChange={handleInputFocusChange}
              />
            </div>
          </div>
        </div>
      </div>

      <aside className="shell-ball-surface__switcher-shell" aria-label="Shell-ball demo controls">
        <ShellBallDemoSwitcher value={visualState} onChange={handleForceState} />
      </aside>
    </div>
  );
}
