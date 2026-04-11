import { ShellBallDevLayer } from "./ShellBallDevLayer";
import { shouldShowShellBallDemoSwitcher } from "./shellBall.dev";
import { ShellBallSurface } from "./ShellBallSurface";
import { useShellBallInteraction } from "./useShellBallInteraction";
import { getShellBallMotionConfig } from "./shellBall.motion";
import { useShellBallCoordinator } from "./useShellBallCoordinator";
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
    setInputValue,
    handleForceState,
  } = useShellBallInteraction();
  const motionConfig = getShellBallMotionConfig(visualState);
  const showDemoSwitcher = shouldShowShellBallDemoSwitcher(isDev);
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
      motionConfig={motionConfig}
      onDragStart={() => {
        void startShellBallWindowDragging();
      }}
      onPrimaryClick={handlePrimaryClick}
      onDoubleClick={handleDoubleClick}
      onRegionEnter={handleRegionEnter}
      onRegionLeave={handleRegionLeave}
      onPressStart={handlePressStart}
      onPressMove={handlePressMove}
      onPressEnd={handlePressEnd}
      onPressCancel={handlePressCancel}
    >
      {showDemoSwitcher ? (
        <ShellBallDevLayer value={visualState} onChange={handleForceState} />
      ) : null}
    </ShellBallSurface>
  );
}