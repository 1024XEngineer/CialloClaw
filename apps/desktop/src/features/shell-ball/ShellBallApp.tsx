import { ShellBallDevLayer } from "./ShellBallDevLayer";
import { shouldShowShellBallDemoSwitcher } from "./shellBall.dev";
import { ShellBallSurface } from "./ShellBallSurface";
import { useShellBallInteraction } from "./useShellBallInteraction";
import { getShellBallMotionConfig } from "./shellBall.motion";

type ShellBallAppProps = {
  isDev?: boolean;
};

export function ShellBallApp({ isDev = false }: ShellBallAppProps) {
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
  const showDemoSwitcher = shouldShowShellBallDemoSwitcher(isDev);

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
      {showDemoSwitcher ? (
        <ShellBallDevLayer value={visualState} onChange={handleForceState} />
      ) : null}
    </ShellBallSurface>
  );
}
