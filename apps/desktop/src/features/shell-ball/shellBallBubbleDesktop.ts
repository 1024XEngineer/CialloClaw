import type { ApprovalDecision, BubbleMessage } from "@cialloclaw/protocol";

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
 * Intent-confirm metadata is shell-ball-local UI state used to render the
 * current inferred intent and to disable actions while a confirm/correction
 * request is in flight.
 */
export type ShellBallBubbleIntentConfirmState = {
  intentName: string;
  intentLabel: string;
  status?: "idle" | "submitting";
};

export type ShellBallBubbleDesktopState = {
  lifecycleState: ShellBallBubbleDesktopLifecycleState;
  freshnessHint?: ShellBallBubbleDesktopFreshnessHint;
  motionHint?: ShellBallBubbleDesktopMotionHint;
  presentationHint?: ShellBallBubbleDesktopPresentationHint;
  turnIndex?: number;
  turnPhase?: number;
  inlineApproval?: ShellBallBubbleInlineApprovalState;
  intentConfirm?: ShellBallBubbleIntentConfirmState;
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

function cloneShellBallBubbleIntentConfirmState(
  state: ShellBallBubbleIntentConfirmState,
): ShellBallBubbleIntentConfirmState {
  return { ...state };
}

export function cloneShellBallBubbleDesktopState(state: ShellBallBubbleDesktopState): ShellBallBubbleDesktopState {
  return {
    ...state,
    ...(state.inlineApproval ? { inlineApproval: cloneShellBallBubbleInlineApprovalState(state.inlineApproval) } : {}),
    ...(state.intentConfirm ? { intentConfirm: cloneShellBallBubbleIntentConfirmState(state.intentConfirm) } : {}),
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
