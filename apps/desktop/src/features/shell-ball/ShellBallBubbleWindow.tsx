import type { ShellBallVisualState } from "./shellBall.types";
import { getShellBallVisibleBubbleItems } from "./shellBall.windowSync";
import {
  emitShellBallBubbleAction,
  emitShellBallBubbleHover,
  useShellBallHelperWindowSnapshot,
} from "./useShellBallCoordinator";
import { useShellBallWindowMetrics } from "./useShellBallWindowMetrics";
import { ShellBallBubbleZone } from "./components/ShellBallBubbleZone";

type ShellBallBubbleWindowProps = {
  visualState?: ShellBallVisualState;
};

export function ShellBallBubbleWindow({ visualState }: ShellBallBubbleWindowProps) {
  const snapshot = useShellBallHelperWindowSnapshot({ role: "bubble" });
  const resolvedVisualState = visualState ?? snapshot.visualState;
  const visibleBubbleItems = getShellBallVisibleBubbleItems(snapshot.bubbleItems);
  const { rootRef } = useShellBallWindowMetrics({
    role: "bubble",
    visible: snapshot.visibility.bubble,
    clickThrough: snapshot.bubbleRegion.clickThrough,
  });

  return (
    <div
      ref={rootRef}
      className="shell-ball-window shell-ball-window--bubble"
      data-visibility-phase={snapshot.bubbleRegion.visibilityPhase}
      onPointerEnter={() => {
        void emitShellBallBubbleHover(true);
      }}
      onPointerLeave={() => {
        void emitShellBallBubbleHover(false);
      }}
    >
      <ShellBallBubbleZone
        visualState={resolvedVisualState}
        bubbleItems={visibleBubbleItems}
        onDeleteBubble={(bubbleId) => {
          void emitShellBallBubbleAction("delete", bubbleId);
        }}
        onPinBubble={(bubbleId) => {
          void emitShellBallBubbleAction("pin", bubbleId);
        }}
      />
    </div>
  );
}
