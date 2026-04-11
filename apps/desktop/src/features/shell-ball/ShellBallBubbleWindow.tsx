import type { ShellBallVisualState } from "./shellBall.types";
import { useShellBallHelperWindowSnapshot } from "./useShellBallCoordinator";
import { useShellBallWindowMetrics } from "./useShellBallWindowMetrics";
import { ShellBallBubbleZone } from "./components/ShellBallBubbleZone";

type ShellBallBubbleWindowProps = {
  visualState?: ShellBallVisualState;
};

export function ShellBallBubbleWindow({ visualState }: ShellBallBubbleWindowProps) {
  const snapshot = useShellBallHelperWindowSnapshot({ role: "bubble" });
  const resolvedVisualState = visualState ?? snapshot.visualState;
  const { rootRef } = useShellBallWindowMetrics({
    role: "bubble",
    visible: snapshot.visibility.bubble,
  });

  return (
    <div ref={rootRef} className="shell-ball-window shell-ball-window--bubble" aria-label="Shell-ball bubble window">
      <ShellBallBubbleZone visualState={resolvedVisualState} bubbleMessages={snapshot.bubbleMessages} />
    </div>
  );
}
