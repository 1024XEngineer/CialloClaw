import { useEffect, useRef } from "react";
import type { ShellBallBubbleItem } from "../shellBall.bubble";
import type { ShellBallVisualState } from "../shellBall.types";
import { ShellBallBubbleMessage as ShellBallBubbleMessageView } from "./ShellBallBubbleMessage";

type ShellBallBubbleZoneProps = {
  visualState: ShellBallVisualState;
  bubbleItems?: ShellBallBubbleItem[];
  onDeleteBubble?: (bubbleId: string) => void;
  onPinBubble?: (bubbleId: string) => void;
};

export function ShellBallBubbleZone({
  visualState,
  bubbleItems = [],
  onDeleteBubble,
  onPinBubble,
}: ShellBallBubbleZoneProps) {
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (scrollElement === null) {
      return;
    }

    scrollElement.scrollTop = scrollElement.scrollHeight;
  }, [bubbleItems]);

  return (
    <section className="shell-ball-bubble-zone" data-state={visualState}>
      <div ref={scrollRef} className="shell-ball-bubble-zone__scroll">
        {bubbleItems.map((item) => (
          <div
            key={item.bubble.bubble_id}
            className="shell-ball-bubble-zone__message-entry"
            data-freshness={item.desktop.freshnessHint ?? "stale"}
            data-motion={item.desktop.motionHint ?? "settle"}
          >
            <ShellBallBubbleMessageView
              item={item}
              onDelete={onDeleteBubble}
              onPin={onPinBubble}
            />
          </div>
        ))}
        <div className="shell-ball-bubble-zone__bottom-anchor" aria-hidden="true" />
      </div>
    </section>
  );
}
