import { ShellBallDemoSwitcher } from "./components/ShellBallDemoSwitcher";
import { ShellBallSurface } from "./ShellBallSurface";
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
    <ShellBallSurface
      visualState={visualState}
      voicePreview={voicePreview}
      inputBarMode={inputBarMode}
      inputValue={inputValue}
      motionConfig={motionConfig}
      onPrimaryClick={handlePrimaryClick}
      onRegionEnter={handleRegionEnter}
      onRegionLeave={handleRegionLeave}
      onInputValueChange={setInputValue}
      onAttachFile={handleAttachFile}
      onSubmitText={handleSubmitText}
      onPressStart={handlePressStart}
      onPressMove={handlePressMove}
      onPressEnd={handlePressEnd}
      onInputFocusChange={handleInputFocusChange}
    >
      <aside className="shell-ball-surface__switcher-shell" aria-label="Shell-ball demo controls">
        <ShellBallDemoSwitcher value={visualState} onChange={handleForceState} />
      </aside>
    </ShellBallSurface>
  );
}
