import type { BubbleMessage } from "@cialloclaw/protocol";

export type ShellBallBubbleRole = "user" | "agent";

export type ShellBallBubbleDesktopFreshnessHint = "fresh" | "stale";

export type ShellBallBubbleDesktopMotionHint = "settle";

export type ShellBallBubbleDesktopLifecycleState = "visible" | "fading" | "hidden";

export type ShellBallBubbleDesktopState = {
  detachedWindowId?: string;
  lifecycleState: ShellBallBubbleDesktopLifecycleState;
  freshnessHint?: ShellBallBubbleDesktopFreshnessHint;
  motionHint?: ShellBallBubbleDesktopMotionHint;
};

export type ShellBallBubbleItem = {
  bubble: BubbleMessage;
  role: ShellBallBubbleRole;
  desktop: ShellBallBubbleDesktopState;
};

export type ShellBallLegacyBubbleMessage = {
  id: string;
  role: ShellBallBubbleRole;
  text: string;
  createdAt: string;
  freshnessHint?: ShellBallBubbleDesktopFreshnessHint;
  motionHint?: ShellBallBubbleDesktopMotionHint;
};

export function cloneShellBallBubbleDesktopState(state: ShellBallBubbleDesktopState): ShellBallBubbleDesktopState {
  return { ...state };
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

export function createShellBallBubbleItemFromLegacyMessage(
  message: ShellBallLegacyBubbleMessage,
): ShellBallBubbleItem {
  return {
    bubble: {
      bubble_id: message.id,
      task_id: "",
      type: "status",
      text: message.text,
      pinned: false,
      hidden: false,
      created_at: message.createdAt,
    },
    role: message.role,
    desktop: {
      lifecycleState: "visible",
      freshnessHint: message.freshnessHint,
      motionHint: message.motionHint,
    },
  };
}

export function createLegacyShellBallBubbleMessage(item: ShellBallBubbleItem): ShellBallLegacyBubbleMessage {
  return {
    id: item.bubble.bubble_id,
    role: item.role,
    text: item.bubble.text,
    createdAt: item.bubble.created_at,
    freshnessHint: item.desktop.freshnessHint,
    motionHint: item.desktop.motionHint,
  };
}

export function cloneShellBallBubbleMessages(messages: ShellBallLegacyBubbleMessage[]): ShellBallLegacyBubbleMessage[] {
  return messages.map((message) => ({ ...message }));
}

export function createShellBallBubbleItemsFromLegacyMessages(
  messages: ShellBallLegacyBubbleMessage[],
): ShellBallBubbleItem[] {
  return messages.map(createShellBallBubbleItemFromLegacyMessage);
}

export function createLegacyShellBallBubbleMessages(items: ShellBallBubbleItem[]): ShellBallLegacyBubbleMessage[] {
  return items.map(createLegacyShellBallBubbleMessage);
}
