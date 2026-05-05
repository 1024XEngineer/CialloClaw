import { useCallback, useEffect, useRef } from "react";
import type { ShellBallBubbleItem } from "../shellBall.bubble";
import type { ShellBallVisualState } from "../shellBall.types";
import { ShellBallBubbleMessage as ShellBallBubbleMessageView } from "./ShellBallBubbleMessage";

type ShellBallBubbleZoneProps = {
  visualState: ShellBallVisualState;
  bubbleItems?: ShellBallBubbleItem[];
  onDeleteBubble?: (bubbleId: string) => void;
  onPinBubble?: (bubbleId: string) => void;
  onAllowApprovalBubble?: (bubbleId: string) => void;
  onDenyApprovalBubble?: (bubbleId: string) => void;
  onConfirmIntentBubble?: (taskId: string) => void;
  onRefineIntentBubble?: (taskId: string) => void;
  onAcceptRecommendationBubble?: (bubbleId: string) => void;
  onIgnoreRecommendationBubble?: (bubbleId: string) => void;
};

export function ShellBallBubbleZone({
  visualState,
  bubbleItems = [],
  onDeleteBubble,
  onPinBubble,
  onAllowApprovalBubble,
  onDenyApprovalBubble,
  onConfirmIntentBubble,
  onRefineIntentBubble,
  onAcceptRecommendationBubble,
  onIgnoreRecommendationBubble,
}: ShellBallBubbleZoneProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const shouldAutoScrollRef = useRef(true);

  const syncAutoScrollState = useCallback(() => {
    const scrollElement = scrollRef.current;
    if (scrollElement === null) {
      return;
    }

    const distanceFromBottom = scrollElement.scrollHeight - scrollElement.scrollTop - scrollElement.clientHeight;
    shouldAutoScrollRef.current = distanceFromBottom <= 24;
  }, []);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    const nextMessageCount = bubbleItems.length;
    if (scrollElement === null) {
      return;
    }

    if (nextMessageCount === 0) {
      shouldAutoScrollRef.current = true;
      return;
    }

    if (shouldAutoScrollRef.current) {
      scrollElement.scrollTop = scrollElement.scrollHeight;
    }
  }, [bubbleItems]);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (scrollElement === null) {
      return;
    }

    const handleNativeWheel = (event: WheelEvent) => {
      scrollElement.scrollTop += event.deltaY;
      syncAutoScrollState();
      event.preventDefault();
      event.stopPropagation();
    };

    scrollElement.addEventListener("wheel", handleNativeWheel, { passive: false });

    return () => {
      scrollElement.removeEventListener("wheel", handleNativeWheel);
    };
  }, [syncAutoScrollState]);

  return (
    <section className="shell-ball-bubble-zone" data-state={visualState}>
      <div
        ref={scrollRef}
        className="shell-ball-bubble-zone__scroll"
        data-shell-ball-interactive="true"
        onScroll={syncAutoScrollState}
      >
        <div className="shell-ball-bubble-zone__spacer" aria-hidden="true" />
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
              onAllowApproval={onAllowApprovalBubble}
              onDenyApproval={onDenyApprovalBubble}
              onConfirmIntent={onConfirmIntentBubble}
              onRefineIntent={onRefineIntentBubble}
              onAcceptRecommendation={onAcceptRecommendationBubble}
              onIgnoreRecommendation={onIgnoreRecommendationBubble}
            />
          </div>
        ))}
        <div className="shell-ball-bubble-zone__bottom-anchor" aria-hidden="true" />
      </div>
    </section>
  );
}
