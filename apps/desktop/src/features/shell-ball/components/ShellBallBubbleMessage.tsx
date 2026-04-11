import type { ShellBallBubbleItem } from "../shellBall.bubble";

type ShellBallBubbleMessageProps = {
  item: ShellBallBubbleItem;
  onDelete?: (bubbleId: string) => void;
  onPin?: (bubbleId: string, action: "pin" | "unpin") => void;
};

export function ShellBallBubbleMessage({ item, onDelete, onPin }: ShellBallBubbleMessageProps) {
  const bubbleId = item.bubble.bubble_id;
  const pinAction = item.bubble.pinned ? "unpin" : "pin";
  const pinLabel = pinAction === "pin" ? "Pin" : "Unpin";

  return (
    <div
      className={`shell-ball-bubble-zone__message-row shell-ball-bubble-zone__message-row--${item.role}`}
      data-role={item.role}
    >
      <div
        className={`shell-ball-bubble-message shell-ball-bubble-message--${item.role}`}
        data-lifecycle={item.desktop.lifecycleState}
        data-message-id={bubbleId}
        data-pinned={item.bubble.pinned ? "true" : "false"}
      >
        <button
          type="button"
          className="shell-ball-bubble-message__control shell-ball-bubble-message__pin-control"
          data-bubble-action={pinAction}
          data-bubble-id={bubbleId}
          aria-label={`${pinLabel} bubble`}
          onClick={() => {
            onPin?.(bubbleId, pinAction);
          }}
        >
          {pinLabel}
        </button>
        <button
          type="button"
          className="shell-ball-bubble-message__control shell-ball-bubble-message__delete-control"
          data-bubble-action="delete"
          data-bubble-id={bubbleId}
          aria-label="Delete bubble"
          onClick={() => {
            onDelete?.(bubbleId);
          }}
        >
          Delete
        </button>
        <p className="shell-ball-bubble-message__text">{item.bubble.text}</p>
      </div>
    </div>
  );
}
