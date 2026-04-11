import { ShellBallSurface } from "./ShellBallSurface";
import { useShellBallInteraction } from "./useShellBallInteraction";
import { getShellBallMotionConfig } from "./shellBall.motion";
import { emitShellBallInputRequestFocus, useShellBallCoordinator } from "./useShellBallCoordinator";
import { useShellBallWindowMetrics } from "./useShellBallWindowMetrics";
import { startShellBallWindowDragging } from "../../platform/shellBallWindowController";
import { openOrFocusDesktopWindow } from "../../platform/windowController";

type ShellBallAppProps = {
  isDev?: boolean;
};

export function ShellBallApp({ isDev = false }: ShellBallAppProps) {
  const {
    visualState,
    inputValue,
    voicePreview,
    voiceHoldProgress,
    inputFocused,
    handlePrimaryClick,
    shouldOpenDashboardFromDoubleClick,
    handleRegionEnter,
    handleRegionLeave,
    handlePressStart,
    handlePressMove,
    handlePressEnd,
    handlePressCancel,
    handleSubmitText,
    handleAttachFile,
    handleInputFocusChange,
    handleInputFocusRequest,
    setInputValue,
  } = useShellBallInteraction();
  const motionConfig = getShellBallMotionConfig(visualState);
  const { rootRef } = useShellBallWindowMetrics({ role: "ball" });

  function handleDoubleClick() {
    if (!shouldOpenDashboardFromDoubleClick) {
      return;
    }

    void openOrFocusDesktopWindow("dashboard");
  }

  useShellBallCoordinator({
    visualState,
    inputValue,
    voicePreview,
    setInputValue,
    onRegionEnter: handleRegionEnter,
    onRegionLeave: handleRegionLeave,
    onInputFocusChange: handleInputFocusChange,
    onSubmitText: handleSubmitText,
    onAttachFile: handleAttachFile,
    onPrimaryClick: handlePrimaryClick,
  });

  return (
    <ShellBallSurface
      containerRef={rootRef}
      visualState={visualState}
      voicePreview={voicePreview}
      voiceHoldProgress={voiceHoldProgress}
      motionConfig={motionConfig}
      onDragStart={() => {
        void startShellBallWindowDragging();
      }}
      onPrimaryClick={handlePrimaryClick}
      onDoubleClick={handleDoubleClick}
      onRegionEnter={handleRegionEnter}
      onRegionLeave={handleRegionLeave}
      inputFocused={inputFocused}
      onInputProxyClick={() => {
        handleInputFocusRequest();
        void emitShellBallInputRequestFocus(Date.now());
      }}
      onPressStart={handlePressStart}
      onPressMove={handlePressMove}
      onPressEnd={handlePressEnd}
      onPressCancel={handlePressCancel}
    />
  );
}
