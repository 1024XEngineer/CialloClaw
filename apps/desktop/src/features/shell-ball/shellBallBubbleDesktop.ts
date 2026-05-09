import type { ApprovalDecision, BubbleMessage, InputContext, PageContext } from "@cialloclaw/protocol";

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

type ShellBallRecommendationRequestContext = InputContext & {
  app_name: string;
  page_title: string;
  error_text?: string;
  hover_target?: string;
  last_action?: string;
  page_url?: string;
  screen_summary?: string;
  selection_text?: string;
  visible_text?: string;
  window_title?: string;
};

/**
 * Inline recommendation metadata stays shell-ball-local until the user accepts
 * one suggestion and promotes it into the formal task pipeline.
 */
export type ShellBallBubbleInlineRecommendationState = {
  recommendationId: string;
  pageContext: PageContext;
  requestContext: ShellBallRecommendationRequestContext;
};

/**
 * Inline error-intake metadata keeps the explicit `error_detected` shortcut in
 * the local shell-ball bubble chrome until the user promotes it into a formal
 * task start.
 */
export type ShellBallBubbleInlineErrorSignalState = {
  errorText: string;
  status: "idle" | "submitting";
  pageContext?: PageContext;
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
  inlineErrorSignal?: ShellBallBubbleInlineErrorSignalState;
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
    requestContext: {
      ...state.requestContext,
      ...(state.requestContext.page ? { page: { ...state.requestContext.page } } : {}),
      ...(state.requestContext.screen ? { screen: { ...state.requestContext.screen } } : {}),
      ...(state.requestContext.behavior ? { behavior: { ...state.requestContext.behavior } } : {}),
      ...(state.requestContext.selection ? { selection: { ...state.requestContext.selection } } : {}),
      ...(state.requestContext.error ? { error: { ...state.requestContext.error } } : {}),
      ...(state.requestContext.clipboard ? { clipboard: { ...state.requestContext.clipboard } } : {}),
    },
  };
}

function cloneShellBallBubbleInlineErrorSignalState(
  state: ShellBallBubbleInlineErrorSignalState,
): ShellBallBubbleInlineErrorSignalState {
  return {
    ...state,
    ...(state.pageContext ? { pageContext: { ...state.pageContext } } : {}),
  };
}

export function cloneShellBallBubbleDesktopState(state: ShellBallBubbleDesktopState): ShellBallBubbleDesktopState {
  return {
    ...state,
    ...(state.inlineApproval ? { inlineApproval: cloneShellBallBubbleInlineApprovalState(state.inlineApproval) } : {}),
    ...(state.inlineRecommendation
      ? { inlineRecommendation: cloneShellBallBubbleInlineRecommendationState(state.inlineRecommendation) }
      : {}),
    ...(state.inlineErrorSignal
      ? { inlineErrorSignal: cloneShellBallBubbleInlineErrorSignalState(state.inlineErrorSignal) }
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
