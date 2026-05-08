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
  onPin,
  onAllowApproval,
  onDenyApproval,
}: ShellBallBubbleMessageProps) {
  const bubbleId = item.bubble.bubble_id;
  const bubbleText = item.bubble.text;
  const showMarkdown = item.role === "agent" && item.bubble.type !== "intent_confirm";
  const showLoadingState = item.desktop.presentationHint === "loading";
  const inlineApproval = item.role === "agent" ? item.desktop.inlineApproval : undefined;
  const inlineApprovalBusy = inlineApproval?.status === "submitting";
  const shouldShowInlineApprovalActions =
    inlineApproval !== undefined && onAllowApproval !== undefined && onDenyApproval !== undefined;
  const shouldShowBubbleControls = !shouldShowInlineApprovalActions;
  const roleLabel = item.role === "agent" ? "Ciallo" : "You";
  const toneLabel = showLoadingState
    ? "Thinking"
    : shouldShowInlineApprovalActions
      ? "Approval"
      : item.bubble.type === "intent_confirm"
        ? "Confirm"
        : "Reply";

  const allowApprovalLabel = inlineApprovalBusy && inlineApproval?.pendingDecision === "allow_once" ? "Allowing..." : "Allow";
  const denyApprovalLabel = inlineApprovalBusy && inlineApproval?.pendingDecision === "deny_once" ? "Denying..." : "Deny";

  return (
    <div
      className={`shell-ball-bubble-zone__message-row shell-ball-bubble-zone__message-row--${item.role}`}
      data-role={item.role}
    >
      <div className={`shell-ball-bubble-message shell-ball-bubble-message--${item.role}`} data-message-id={bubbleId}>
        <div className="shell-ball-bubble-message__chrome">
          <div className="shell-ball-bubble-message__badges" aria-hidden="true">
            <span className="shell-ball-bubble-message__badge shell-ball-bubble-message__badge--role">{roleLabel}</span>
            <span className="shell-ball-bubble-message__badge shell-ball-bubble-message__badge--tone">{toneLabel}</span>
          </div>
          {shouldShowBubbleControls && (onPin || onDelete) ? (
            <div className="shell-ball-bubble-message__controls">
              {onPin ? (
                <button
                  type="button"
                  className="shell-ball-bubble-message__control shell-ball-bubble-message__pin-control"
                  data-bubble-action="pin"
                  data-bubble-id={bubbleId}
                  aria-label="Pin bubble"
                  onClick={() => {
                    onPin(bubbleId);
                  }}
                >
                  Pin
                </button>
              ) : null}
              {onDelete ? (
                <button
                  type="button"
                  className="shell-ball-bubble-message__control shell-ball-bubble-message__delete-control"
                  data-bubble-action="delete"
                  data-bubble-id={bubbleId}
                  aria-label="Delete bubble"
                  onClick={() => {
                    onDelete(bubbleId);
                  }}
                >
                  Hide
                </button>
              ) : null}
            </div>
          ) : null}
        </div>
        <div className="shell-ball-bubble-message__body">
          {showLoadingState ? (
            <div className="shell-ball-bubble-message__loading" aria-live="polite" aria-label={bubbleText || "Agent is thinking"}>
              <span className="shell-ball-bubble-message__loading-dots" aria-hidden="true">
                <span className="shell-ball-bubble-message__loading-dot" />
                <span className="shell-ball-bubble-message__loading-dot" />
                <span className="shell-ball-bubble-message__loading-dot" />
              </span>
              {bubbleText.trim() !== "" ? <span className="shell-ball-bubble-message__loading-label">{bubbleText}</span> : null}
            </div>
          ) : showMarkdown ? (
            <ShellBallMarkdown text={bubbleText} />
          ) : (
            <p className="shell-ball-bubble-message__text">{bubbleText}</p>
          )}
        </div>
        {shouldShowInlineApprovalActions ? (
          <div className="shell-ball-bubble-message__approval-actions">
            <button
              type="button"
              className="shell-ball-bubble-message__approval-action shell-ball-bubble-message__approval-action--deny"
              data-bubble-action="deny_approval"
              data-bubble-id={bubbleId}
              aria-label="Deny approval"
              disabled={inlineApprovalBusy}
              onClick={() => {
                onDenyApproval?.(bubbleId);
              }}
            >
              {denyApprovalLabel}
            </button>
            <button
              type="button"
              className="shell-ball-bubble-message__approval-action shell-ball-bubble-message__approval-action--allow"
              data-bubble-action="allow_approval"
              data-bubble-id={bubbleId}
              aria-label="Allow approval"
              disabled={inlineApprovalBusy}
              onClick={() => {
                onAllowApproval?.(bubbleId);
              }}
            >
              {allowApprovalLabel}
            </button>
          </div>
        ) : null}
      </div>
    </div>
  );
}
