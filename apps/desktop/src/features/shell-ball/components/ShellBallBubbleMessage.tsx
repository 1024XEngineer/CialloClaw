import { ChevronDown, ChevronUp, Trash2 } from "lucide-react";
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import type { ShellBallBubbleItem } from "../shellBall.bubble";
import { ShellBallMarkdown } from "./ShellBallMarkdown";

type ShellBallBubbleMessageProps = {
  item: ShellBallBubbleItem;
  onDelete?: (bubbleId: string) => void;
  onPin?: (bubbleId: string) => void;
  onAllowApproval?: (bubbleId: string) => void;
  onDenyApproval?: (bubbleId: string) => void;
};

const useIsomorphicLayoutEffect = typeof window === "undefined" ? useEffect : useLayoutEffect;

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
  const bodyRef = useRef<HTMLDivElement>(null);
  const measureRef = useRef<HTMLParagraphElement>(null);
  const userToggledCollapseRef = useRef(false);
  const bubbleId = item.bubble.bubble_id;
  const bubbleText = item.bubble.text;
  const showMarkdown = item.role === "agent" && item.bubble.type !== "intent_confirm";
  const showLoadingState = item.desktop.presentationHint === "loading";
  const inlineApproval = item.role === "agent" ? item.desktop.inlineApproval : undefined;
  const shouldShowBubbleControls = inlineApproval === undefined;
  const loadingText = bubbleText.trim() === "" ? "正在思考..." : bubbleText;
  const [isCollapsible, setIsCollapsible] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const collapsedContent = showLoadingState ? loadingText : bubbleText;

  const syncCollapsedState = useCallback(() => {
    const measure = measureRef.current;
    if (measure === null) {
      return;
    }

    const computedStyle = window.getComputedStyle(measure);
    const lineHeight = Number.parseFloat(computedStyle.lineHeight);
    if (!Number.isFinite(lineHeight) || lineHeight <= 0) {
      return;
    }

    const nextIsCollapsible = measure.scrollHeight > lineHeight * 2 + 1;
    setIsCollapsible(nextIsCollapsible);
    setCollapsed((current) => {
      if (!nextIsCollapsible) {
        return false;
      }

      if (userToggledCollapseRef.current) {
        return current;
      }

      return item.role === "user";
    });
  }, [item.role]);

  useIsomorphicLayoutEffect(() => {
    userToggledCollapseRef.current = false;
    syncCollapsedState();
  }, [bubbleId, bubbleText, syncCollapsedState]);

  useEffect(() => {
    const body = bodyRef.current;
    if (body === null || typeof ResizeObserver === "undefined") {
      return;
    }

    const observer = new ResizeObserver(() => {
      syncCollapsedState();
    });
    observer.observe(body);

    return () => {
      observer.disconnect();
    };
  }, [syncCollapsedState]);

  function handleToggleCollapsed() {
    userToggledCollapseRef.current = true;
    setCollapsed((current) => !current);
  }

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
        <div
          ref={bodyRef}
          className={`shell-ball-bubble-message__body${collapsed && isCollapsible ? " shell-ball-bubble-message__body--collapsed" : ""}`}
        >
          <p ref={measureRef} className="shell-ball-bubble-message__measure shell-ball-bubble-message__text" aria-hidden="true">
            {collapsedContent}
          </p>
          {collapsed && isCollapsible ? (
            <p className="shell-ball-bubble-message__text shell-ball-bubble-message__text--collapsed">{collapsedContent}</p>
          ) : showLoadingState ? (
            <p className="shell-ball-bubble-message__text" aria-live="polite" aria-label={loadingText}>
              {loadingText}
            </p>
          ) : showMarkdown ? (
            <ShellBallMarkdown text={bubbleText} />
          ) : (
            <p className="shell-ball-bubble-message__text">{bubbleText}</p>
          )}
        </div>
        {isCollapsible ? (
          <button
            type="button"
            className="shell-ball-bubble-message__collapse-control"
            aria-label={collapsed ? "展开气泡" : "折叠气泡"}
            title={collapsed ? "展开" : "折叠"}
            onClick={handleToggleCollapsed}
          >
            {collapsed ? (
              <ChevronDown className="shell-ball-bubble-message__collapse-control-icon" aria-hidden="true" />
            ) : (
              <ChevronUp className="shell-ball-bubble-message__collapse-control-icon" aria-hidden="true" />
            )}
          </button>
        ) : null}
      </div>
    </div>
  );
}
