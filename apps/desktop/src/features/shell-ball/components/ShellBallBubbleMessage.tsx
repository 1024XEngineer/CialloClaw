import type { ShellBallBubbleItem } from "../shellBall.bubble";
import { ShellBallMarkdown } from "./ShellBallMarkdown";

type ShellBallBubbleMessageProps = {
  item: ShellBallBubbleItem;
  onDelete?: (bubbleId: string) => void;
  onPin?: (bubbleId: string) => void;
  onCancelIntent?: (taskId: string) => void;
  onConfirmIntent?: (taskId: string) => void;
};

export function ShellBallBubbleMessage({ item, onDelete, onPin, onCancelIntent, onConfirmIntent }: ShellBallBubbleMessageProps) {
  const bubbleId = item.bubble.bubble_id;
  const bubbleText = item.bubble.text;
  const showMarkdown = item.role === "agent" && item.bubble.type !== "intent_confirm";

  void onDelete;
  void onPin;

  return (
    <div
      className={`shell-ball-bubble-zone__message-row shell-ball-bubble-zone__message-row--${item.role}`}
      data-role={item.role}
    >
      <div className={`shell-ball-bubble-message shell-ball-bubble-message--${item.role}`} data-message-id={bubbleId}>
        {item.bubble.type === "intent_confirm" ? (
          <div className="shell-ball-bubble-message__intent-actions">
            <button
              className="shell-ball-bubble-message__intent-button shell-ball-bubble-message__intent-button--muted"
              onClick={() => {
                onCancelIntent?.(item.bubble.task_id);
              }}
              type="button"
            >
              取消
            </button>
            <button
              className="shell-ball-bubble-message__intent-button shell-ball-bubble-message__intent-button--primary"
              onClick={() => {
                onConfirmIntent?.(item.bubble.task_id);
              }}
              type="button"
            >
              确认继续
            </button>
          </div>
        ) : null}
        {showMarkdown ? (
          <ShellBallMarkdown text={bubbleText} />
        ) : (
          <p className="shell-ball-bubble-message__text">{bubbleText}</p>
        )}
      </div>
    </div>
  );
}
