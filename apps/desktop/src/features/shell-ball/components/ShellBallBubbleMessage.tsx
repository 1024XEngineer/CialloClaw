import type { ShellBallBubbleItem } from "../shellBall.bubble";
import { ShellBallMarkdown } from "./ShellBallMarkdown";

type ShellBallBubbleMessageProps = {
  item: ShellBallBubbleItem;
  onDelete?: (bubbleId: string) => void;
  onPin?: (bubbleId: string) => void;
  onAllowApproval?: (bubbleId: string) => void;
  onDenyApproval?: (bubbleId: string) => void;
  onConfirmIntent?: (taskId: string) => void;
  onAcceptErrorSignal?: (bubbleId: string) => void;
  onIgnoreErrorSignal?: (bubbleId: string) => void;
  onAcceptRecommendation?: (bubbleId: string) => void;
  onIgnoreRecommendation?: (bubbleId: string) => void;
};

export function ShellBallBubbleMessage({
  item,
  onDelete,
  onPin,
  onAllowApproval,
  onDenyApproval,
  onConfirmIntent,
  onAcceptErrorSignal,
  onIgnoreErrorSignal,
  onAcceptRecommendation,
  onIgnoreRecommendation,
}: ShellBallBubbleMessageProps) {
  const bubbleId = item.bubble.bubble_id;
  const bubbleText = item.bubble.text;
  const taskId = item.bubble.task_id.trim();
  const showMarkdown = item.role === "agent" && item.bubble.type !== "intent_confirm";
  const showLoadingState = item.desktop.presentationHint === "loading";
  const inlineApproval = item.role === "agent" ? item.desktop.inlineApproval : undefined;
  const inlineErrorSignal = item.role === "agent" ? item.desktop.inlineErrorSignal : undefined;
  const inlineRecommendation = item.role === "agent" ? item.desktop.inlineRecommendation : undefined;
  const inlineApprovalBusy = inlineApproval?.status === "submitting";
  const inlineErrorSignalBusy = inlineErrorSignal?.status === "submitting";
  const shouldShowInlineApprovalActions =
    inlineApproval !== undefined && onAllowApproval !== undefined && onDenyApproval !== undefined;
  const shouldShowInlineErrorSignalActions =
    inlineErrorSignal !== undefined && onAcceptErrorSignal !== undefined && onIgnoreErrorSignal !== undefined;
  const shouldShowInlineRecommendationActions =
    inlineRecommendation !== undefined && onAcceptRecommendation !== undefined && onIgnoreRecommendation !== undefined;
  const shouldShowIntentConfirmAction =
    item.role === "agent" && item.bubble.type === "intent_confirm" && taskId !== "" && onConfirmIntent !== undefined;
  const shouldShowBubbleControls =
    !shouldShowInlineApprovalActions
    && !shouldShowInlineErrorSignalActions
    && !shouldShowInlineRecommendationActions
    && !shouldShowIntentConfirmAction;

  const allowApprovalLabel = inlineApprovalBusy && inlineApproval?.pendingDecision === "allow_once" ? "Allowing..." : "Allow";
  const denyApprovalLabel = inlineApprovalBusy && inlineApproval?.pendingDecision === "deny_once" ? "Denying..." : "Deny";

  return (
    <div
      className={`shell-ball-bubble-zone__message-row shell-ball-bubble-zone__message-row--${item.role}`}
      data-role={item.role}
    >
      <div className={`shell-ball-bubble-message shell-ball-bubble-message--${item.role}`} data-message-id={bubbleId}>
        {shouldShowBubbleControls && onPin ? (
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
        {shouldShowBubbleControls && onDelete ? (
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
            Delete
          </button>
        ) : null}
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
        ) : shouldShowInlineErrorSignalActions ? (
          <div className="shell-ball-bubble-message__recommendation-actions">
            <button
              type="button"
              className="shell-ball-bubble-message__recommendation-action shell-ball-bubble-message__recommendation-action--ignore"
              data-bubble-action="ignore_error_signal"
              data-bubble-id={bubbleId}
              aria-label="Dismiss error analysis"
              disabled={inlineErrorSignalBusy}
              onClick={() => {
                onIgnoreErrorSignal?.(bubbleId);
              }}
            >
              Not now
            </button>
            <button
              type="button"
              className="shell-ball-bubble-message__approval-action shell-ball-bubble-message__approval-action--allow"
              data-bubble-action="accept_error_signal"
              data-bubble-id={bubbleId}
              aria-label="Analyze error"
              disabled={inlineErrorSignalBusy}
              onClick={() => {
                onAcceptErrorSignal?.(bubbleId);
              }}
            >
              {inlineErrorSignalBusy ? "Starting..." : "Analyze error"}
            </button>
          </div>
        ) : shouldShowInlineRecommendationActions ? (
          <div className="shell-ball-bubble-message__recommendation-actions">
            <button
              type="button"
              className="shell-ball-bubble-message__recommendation-action shell-ball-bubble-message__recommendation-action--ignore"
              data-bubble-action="ignore_recommendation"
              data-bubble-id={bubbleId}
              aria-label="Dismiss recommendation"
              onClick={() => {
                onIgnoreRecommendation?.(bubbleId);
              }}
            >
              Not now
            </button>
            <button
              type="button"
              className="shell-ball-bubble-message__recommendation-action shell-ball-bubble-message__recommendation-action--accept"
              data-bubble-action="accept_recommendation"
              data-bubble-id={bubbleId}
              aria-label="Accept recommendation"
              onClick={() => {
                onAcceptRecommendation?.(bubbleId);
              }}
            >
              Try this
            </button>
          </div>
        ) : shouldShowIntentConfirmAction ? (
          <div className="shell-ball-bubble-message__recommendation-actions">
            <button
              type="button"
              className="shell-ball-bubble-message__approval-action shell-ball-bubble-message__approval-action--allow"
              data-bubble-action="confirm_intent"
              data-bubble-id={bubbleId}
              aria-label="Confirm intent"
              onClick={() => {
                onConfirmIntent?.(taskId);
              }}
            >
              OK
            </button>
          </div>
        ) : null}
      </div>
    </div>
  );
}
