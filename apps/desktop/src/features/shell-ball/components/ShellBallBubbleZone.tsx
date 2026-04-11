import { useEffect, useRef } from "react";
import type { ShellBallBubbleItem } from "../shellBall.bubble";
import type { ShellBallVisualState } from "../shellBall.types";
import { ShellBallBubbleMessage as ShellBallBubbleMessageView } from "./ShellBallBubbleMessage";

const SHELL_BALL_BUBBLE_ZONE_AUTO_SCROLL_THRESHOLD_PX = 24;

function updateShellBallBubbleZoneStickiness(
  scrollElement: Pick<HTMLDivElement, "clientHeight" | "scrollHeight" | "scrollTop">,
  shouldStickToBottomRef: { current: boolean },
) {
  const distanceFromBottom = scrollElement.scrollHeight - scrollElement.clientHeight - scrollElement.scrollTop;
  shouldStickToBottomRef.current = distanceFromBottom <= SHELL_BALL_BUBBLE_ZONE_AUTO_SCROLL_THRESHOLD_PX;
}

type ShellBallBubbleZoneProps = {
  visualState: ShellBallVisualState;
  bubbleItems?: ShellBallBubbleItem[];
  onDeleteBubble?: (bubbleId: string) => void;
  onInteractionActiveChange?: (active: boolean) => void;
  onPinBubble?: (bubbleId: string, action: "pin" | "unpin") => void;
};

export function ShellBallBubbleZone({
  visualState,
  bubbleItems = [],
  onDeleteBubble,
  onInteractionActiveChange,
  onPinBubble,
}: ShellBallBubbleZoneProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const previousBubbleCountRef = useRef(0);
  const shouldStickToBottomRef = useRef(true);
  const interactionActiveRef = useRef(false);

  function setShellBallBubbleZoneInteractionActive(active: boolean) {
    if (interactionActiveRef.current === active) {
      return;
    }

    interactionActiveRef.current = active;
    onInteractionActiveChange?.(active);
  }

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
    <section
      className="shell-ball-bubble-zone"
      data-state={visualState}
      onPointerEnter={() => {
        setShellBallBubbleZoneInteractionActive(true);
      }}
      onPointerLeave={() => {
        setShellBallBubbleZoneInteractionActive(false);
      }}
    >
      <div
        ref={scrollRef}
        className="shell-ball-bubble-zone__scroll"
        tabIndex={0}
        onPointerDown={() => {
          setShellBallBubbleZoneInteractionActive(true);
        }}
        onScroll={() => {
          const scrollElement = scrollRef.current;
          if (scrollElement === null) {
            return;
          }

          updateShellBallBubbleZoneStickiness(scrollElement, shouldStickToBottomRef);
        }}
        onWheel={(event) => {
          const scrollElement = scrollRef.current;
          if (scrollElement === null) {
            return;
          }

          setShellBallBubbleZoneInteractionActive(true);
          scrollElement.scrollTop += event.deltaY;
          updateShellBallBubbleZoneStickiness(scrollElement, shouldStickToBottomRef);
          event.preventDefault();
        }}
      >
        {bubbleItems.map((item) => (
          <div
            key={item.bubble.bubble_id}
            className="shell-ball-bubble-zone__message-entry"
            data-freshness={item.desktop.freshnessHint ?? "stale"}
            data-lifecycle={item.desktop.lifecycleState}
            data-motion={item.desktop.motionHint ?? "settle"}
            data-pinned={item.bubble.pinned ? "true" : "false"}
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
