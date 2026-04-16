import { ShellBallVoiceHints } from "./components/ShellBallVoiceHints";
import { useShellBallHelperWindowSnapshot } from "./useShellBallCoordinator";
import { useShellBallWindowMetrics } from "./useShellBallWindowMetrics";

export function ShellBallVoiceWindow() {
  const snapshot = useShellBallHelperWindowSnapshot({ role: "voice" });
  const { rootRef } = useShellBallWindowMetrics({
    role: "voice",
    visible: snapshot.visibility.voice,
    clickThrough: true,
  });

  return (
    <div ref={rootRef} className="shell-ball-window shell-ball-window--voice">
      <div className="shell-ball-voice-window" data-state={snapshot.visualState}>
        <ShellBallVoiceHints
          hintMode={snapshot.voiceHintMode}
          voicePreview={snapshot.voicePreview}
        />
      </div>
    </div>
  );
}
