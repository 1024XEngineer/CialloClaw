import type { ApprovalDecision, BubbleMessage, PageContext } from "@cialloclaw/protocol";

export type ShellBallBubbleRole = "user" | "agent";

export type ShellBallBubbleDesktopFreshnessHint = "fresh" | "stale";

export type ShellBallBubbleDesktopMotionHint = "settle";

export type ShellBallBubbleDesktopLifecycleState = "visible" | "fading" | "hidden";

export type ShellBallBubbleDesktopPresentationHint = "loading";

/**
 * Inline approval metadata is shell-ball-local UI state. It mirrors one active
 * approval request so the bubble can submit the formal decision RPC without
 * promoting extra approval objects into the protocol boundary.
 */
export type ShellBallBubbleInlineApprovalState = {
  approvalId: string;
  status: "idle" | "submitting";
  pendingDecision?: ApprovalDecision;
};

/**
 * Inline recommendation metadata stays shell-ball-local until the user accepts
 * one suggestion and promotes it into the formal task pipeline.
 */
export type ShellBallBubbleInlineRecommendationState = {
  recommendationId: string;
  pageContext: PageContext;
};

export type ShellBallBubbleDesktopState = {
  lifecycleState: ShellBallBubbleDesktopLifecycleState;
  freshnessHint?: ShellBallBubbleDesktopFreshnessHint;
  motionHint?: ShellBallBubbleDesktopMotionHint;
  presentationHint?: ShellBallBubbleDesktopPresentationHint;
  turnIndex?: number;
  turnPhase?: number;
  inlineApproval?: ShellBallBubbleInlineApprovalState;
  inlineRecommendation?: ShellBallBubbleInlineRecommendationState;
};

export type ShellBallBubbleItem = {
  bubble: BubbleMessage;
  role: ShellBallBubbleRole;
  desktop: ShellBallBubbleDesktopState;
};

function cloneShellBallBubbleInlineApprovalState(
  state: ShellBallBubbleInlineApprovalState,
): ShellBallBubbleInlineApprovalState {
  return { ...state };
}

function cloneShellBallBubbleInlineRecommendationState(
  state: ShellBallBubbleInlineRecommendationState,
): ShellBallBubbleInlineRecommendationState {
  return {
    recommendationId: state.recommendationId,
    pageContext: { ...state.pageContext },
  };
}

export function cloneShellBallBubbleDesktopState(state: ShellBallBubbleDesktopState): ShellBallBubbleDesktopState {
  return {
    ...state,
    ...(state.inlineApproval ? { inlineApproval: cloneShellBallBubbleInlineApprovalState(state.inlineApproval) } : {}),
    ...(state.inlineRecommendation
      ? { inlineRecommendation: cloneShellBallBubbleInlineRecommendationState(state.inlineRecommendation) }
      : {}),
  };
}

export function cloneShellBallBubbleItem(item: ShellBallBubbleItem): ShellBallBubbleItem {
  return {
    bubble: { ...item.bubble },
    role: item.role,
    desktop: cloneShellBallBubbleDesktopState(item.desktop),
  };
}

export function cloneShellBallBubbleItems(items: ShellBallBubbleItem[]): ShellBallBubbleItem[] {
  return items.map(cloneShellBallBubbleItem);
}
