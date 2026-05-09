import type { ShellBallBubbleItem } from "../shellBall.bubble";
import { ShellBallMarkdown } from "./ShellBallMarkdown";

type ShellBallBubbleMessageProps = {
  item: ShellBallBubbleItem;
  onDelete?: (bubbleId: string) => void;
  onPin?: (bubbleId: string) => void;
  onAllowApproval?: (bubbleId: string) => void;
  onDenyApproval?: (bubbleId: string) => void;
  onCancelTask?: (taskId: string) => void;
  onConfirmIntent?: (taskId: string) => void;
  onModifyIntent?: (taskId: string) => void;
};

export function ShellBallBubbleMessage({
  item,
  onDelete,
  onPin,
  onAllowApproval,
  onDenyApproval,
  onCancelTask,
  onConfirmIntent,
  onModifyIntent,
}: ShellBallBubbleMessageProps) {
  const bubbleId = item.bubble.bubble_id;
  const taskId = item.bubble.task_id.trim();
  const bubbleText = item.bubble.text;
  const showMarkdown = item.role === "agent" && item.bubble.type !== "intent_confirm";
  const showLoadingState = item.desktop.presentationHint === "loading";
  const inlineApproval = item.role === "agent" ? item.desktop.inlineApproval : undefined;
  const intentConfirm = item.role === "agent" ? item.desktop.intentConfirm : undefined;
  const inlineApprovalBusy = inlineApproval?.status === "submitting";
  const intentConfirmBusy = intentConfirm?.status === "submitting";
  const shouldShowInlineApprovalActions =
    inlineApproval !== undefined && onAllowApproval !== undefined && onDenyApproval !== undefined;
  const shouldShowIntentActions =
    item.role === "agent"
    && item.bubble.type === "intent_confirm"
    && taskId !== ""
    && onConfirmIntent !== undefined
    && onCancelTask !== undefined
    && onModifyIntent !== undefined;
  const shouldShowBubbleControls = !shouldShowInlineApprovalActions && !shouldShowIntentActions;

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
        {intentConfirm ? (
          <p className="shell-ball-bubble-message__intent-summary">
            当前意图：{intentConfirm.intentLabel}
          </p>
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
        ) : shouldShowIntentActions ? (
          <div className="shell-ball-bubble-message__intent-actions">
            <button
              type="button"
              className="shell-ball-bubble-message__intent-action shell-ball-bubble-message__intent-action--confirm"
              data-bubble-action="confirm_intent"
              data-bubble-id={bubbleId}
              aria-label="确认当前意图"
              disabled={intentConfirmBusy}
              onClick={() => {
                onConfirmIntent?.(taskId);
              }}
            >
              确认
            </button>
            <button
              type="button"
              className="shell-ball-bubble-message__intent-action shell-ball-bubble-message__intent-action--cancel"
              data-bubble-action="cancel_task"
              data-bubble-id={bubbleId}
              aria-label="取消当前任务"
              disabled={intentConfirmBusy}
              onClick={() => {
                onCancelTask?.(taskId);
              }}
            >
              取消任务
            </button>
            <button
              type="button"
              className="shell-ball-bubble-message__intent-action shell-ball-bubble-message__intent-action--modify"
              data-bubble-action="modify_intent"
              data-bubble-id={bubbleId}
              aria-label="修改当前意图"
              disabled={intentConfirmBusy}
              onClick={() => {
                onModifyIntent?.(taskId);
              }}
            >
              修改意图
            </button>
          </div>
        ) : null}
      </div>
    </div>
  );
}
