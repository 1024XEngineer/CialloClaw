import { useEffect, useRef } from "react";
import type { ShellBallBubbleItem } from "../shellBall.bubble";
import type { ShellBallVisualState } from "../shellBall.types";
import { ShellBallBubbleMessage as ShellBallBubbleMessageView } from "./ShellBallBubbleMessage";

const SHELL_BALL_BUBBLE_ZONE_AUTO_SCROLL_THRESHOLD_PX = 24;

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
  const previousBubbleCountRef = useRef(0);
  const shouldStickToBottomRef = useRef(true);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (scrollElement === null) {
      return;
    }

    const nextBubbleCount = bubbleItems.length;
    const shouldAutoScroll =
      previousBubbleCountRef.current === 0 ||
      (nextBubbleCount > previousBubbleCountRef.current && shouldStickToBottomRef.current);

    if (shouldAutoScroll) {
      scrollElement.scrollTop = scrollElement.scrollHeight;
      shouldStickToBottomRef.current = true;
    }

    previousBubbleCountRef.current = nextBubbleCount;
  }, [bubbleItems]);

  return (
    <section className="shell-ball-bubble-zone" data-state={visualState}>
      <div
        ref={scrollRef}
        className="shell-ball-bubble-zone__scroll"
        onScroll={() => {
          const scrollElement = scrollRef.current;
          if (scrollElement === null) {
            return;
          }

          const distanceFromBottom = scrollElement.scrollHeight - scrollElement.clientHeight - scrollElement.scrollTop;
          shouldStickToBottomRef.current = distanceFromBottom <= SHELL_BALL_BUBBLE_ZONE_AUTO_SCROLL_THRESHOLD_PX;
        }}
      >
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
