import { useMemo, useState } from "react";
import {
  getShellBallCurrentWindow,
  getShellBallPinnedBubbleIdFromLabel,
  startShellBallWindowDragging,
} from "../../platform/shellBallWindowController";
import {
  emitShellBallBubbleAction,
  emitShellBallPinnedWindowDetached,
  useShellBallPinnedBubbleSnapshot,
} from "./useShellBallCoordinator";
import { ShellBallMarkdown } from "./components/ShellBallMarkdown";

/**
 * Renders the detached pinned bubble window while preserving the original
 * bubble content contract from the shared shell-ball snapshot.
 */
export function ShellBallPinnedBubbleWindow() {
  const windowLabel = getShellBallCurrentWindow().label;
  const bubbleId = getShellBallPinnedBubbleIdFromLabel(windowLabel);
  const snapshot = useShellBallPinnedBubbleSnapshot();
  const [followsShellBallGeometry, setFollowsShellBallGeometry] = useState(true);
  const pinnedItem = useMemo(
    () => snapshot.bubbleItems.find((item) => item.bubble.bubble_id === bubbleId && item.bubble.pinned),
    [bubbleId, snapshot.bubbleItems],
  );

  if (bubbleId === null || pinnedItem === undefined) {
    return <div className="shell-ball-window shell-ball-window--bubble" aria-label="Shell-ball pinned bubble window" />;
  }

  const pinnedBubbleId = bubbleId;

  function handleDetachDrag() {
    if (followsShellBallGeometry) {
      setFollowsShellBallGeometry(false);
      void emitShellBallPinnedWindowDetached(pinnedBubbleId);
    }

    void startShellBallWindowDragging();
  }

  return (
    <div className="shell-ball-window shell-ball-window--bubble" aria-label="Shell-ball pinned bubble window">
      <div className="shell-ball-bubble-message shell-ball-bubble-message--pinned" data-bubble-id={pinnedBubbleId}>
        <div className="shell-ball-bubble-message__chrome">
          <div className="shell-ball-bubble-message__badges" aria-hidden="true">
            <span className="shell-ball-bubble-message__badge shell-ball-bubble-message__badge--role">Ciallo</span>
            <span className="shell-ball-bubble-message__badge shell-ball-bubble-message__badge--tone">Pinned</span>
          </div>
          <div className="shell-ball-bubble-message__controls">
            <button
              type="button"
              className="shell-ball-bubble-message__control shell-ball-bubble-message__pin-control"
              data-bubble-action="unpin"
              data-bubble-id={pinnedBubbleId}
              aria-label="Unpin bubble"
              onClick={() => {
                void emitShellBallBubbleAction("unpin", pinnedBubbleId, "pinned_window");
              }}
            >
              Unpin
            </button>
            <button
              type="button"
              className="shell-ball-bubble-message__control shell-ball-bubble-message__delete-control"
              data-bubble-action="delete"
              data-bubble-id={pinnedBubbleId}
              aria-label="Delete bubble"
              onClick={() => {
                void emitShellBallBubbleAction("delete", pinnedBubbleId, "pinned_window");
              }}
            >
              Hide
            </button>
            <button
              type="button"
              className="shell-ball-bubble-message__control shell-ball-bubble-message__drag-handle"
              aria-label="Drag pinned bubble"
              onPointerDown={handleDetachDrag}
            >
              Drag
            </button>
          </div>
        </div>
        <div className="shell-ball-bubble-message__body">
          <ShellBallMarkdown text={pinnedItem.bubble.text} />
        </div>
      </div>
    </div>
  );
}
