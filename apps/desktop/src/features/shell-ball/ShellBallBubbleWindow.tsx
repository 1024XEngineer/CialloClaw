import type { ShellBallVisualState } from "./shellBall.types";
import { getShellBallVisibleBubbleItems } from "./shellBall.windowSync";
import { focusShellBallCurrentWindow } from "../../platform/shellBallWindowController";
import { emitShellBallBubbleAction, emitShellBallBubbleInteraction, useShellBallHelperWindowSnapshot } from "./useShellBallCoordinator";
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
    visible: true,
    clickThrough: snapshot.bubbleRegion.clickThrough,
  });

  return (
    <div ref={rootRef} className="shell-ball-window shell-ball-window--bubble" aria-label="Shell-ball bubble window">
      <ShellBallBubbleZone
        visualState={resolvedVisualState}
        bubbleItems={visibleBubbleItems}
        onDeleteBubble={(bubbleId) => {
          void emitShellBallBubbleAction("delete", bubbleId);
        }}
        onInteractionActiveChange={(active) => {
          if (active) {
            void focusShellBallCurrentWindow();
          }

          void emitShellBallBubbleInteraction(active);
        }}
        onPinBubble={(bubbleId, action) => {
          void emitShellBallBubbleAction(action, bubbleId);
        }}
      />
    </div>
  );
}
