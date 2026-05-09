import { Trash2 } from "lucide-react";
import type { ShellBallBubbleItem } from "../shellBall.bubble";
import { ShellBallMarkdown } from "./ShellBallMarkdown";

type ShellBallBubbleMessageProps = {
  item: ShellBallBubbleItem;
  onDelete?: (bubbleId: string) => void;
  onPin?: (bubbleId: string) => void;
  onAllowApproval?: (bubbleId: string) => void;
  onDenyApproval?: (bubbleId: string) => void;
};

/**
 * Renders one near-field bubble while keeping approval actions and desktop-only
 * affordances local to the shell-ball presentation layer.
 */
export function ShellBallBubbleMessage({
  item,
  onDelete,
  onPin: _onPin,
  onAllowApproval: _onAllowApproval,
  onDenyApproval: _onDenyApproval,
}: ShellBallBubbleMessageProps) {
  const bubbleId = item.bubble.bubble_id;
  const bubbleText = item.bubble.text;
  const showMarkdown = item.role === "agent" && item.bubble.type !== "intent_confirm";
  const showLoadingState = item.desktop.presentationHint === "loading";
  const inlineApproval = item.role === "agent" ? item.desktop.inlineApproval : undefined;
  const shouldShowBubbleControls = inlineApproval === undefined;
  const loadingText = bubbleText.trim() === "" ? "正在思考..." : bubbleText;

  return (
    <div
      className={`shell-ball-bubble-zone__message-row shell-ball-bubble-zone__message-row--${item.role}`}
      data-role={item.role}
    >
      <div className={`shell-ball-bubble-message shell-ball-bubble-message--${item.role}`} data-message-id={bubbleId}>
        {shouldShowBubbleControls && onDelete ? (
          <div className="shell-ball-bubble-message__hover-controls" data-role={item.role}>
            <button
              type="button"
              className="shell-ball-bubble-message__hover-control shell-ball-bubble-message__delete-control"
              data-bubble-action="delete"
              data-bubble-id={bubbleId}
              aria-label="删除"
              title="删除"
              onClick={() => {
                onDelete(bubbleId);
              }}
            >
              <Trash2 className="shell-ball-bubble-message__hover-control-icon" aria-hidden="true" />
            </button>
          </div>
        ) : null}
        <div className="shell-ball-bubble-message__body">
          {showLoadingState ? (
            <p className="shell-ball-bubble-message__text" aria-live="polite" aria-label={loadingText}>
              {loadingText}
            </p>
          ) : showMarkdown ? (
            <ShellBallMarkdown text={bubbleText} />
          ) : (
            <p className="shell-ball-bubble-message__text">{bubbleText}</p>
          )}
        </div>
      </div>
    </div>
  );
}
