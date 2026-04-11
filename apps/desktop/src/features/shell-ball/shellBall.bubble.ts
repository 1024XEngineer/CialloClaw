export type {
  ShellBallBubbleDesktopFreshnessHint,
  ShellBallBubbleDesktopMotionHint,
  ShellBallBubbleDesktopLifecycleState,
  ShellBallBubbleDesktopState,
  ShellBallBubbleItem,
  ShellBallBubbleRole,
  ShellBallLegacyBubbleMessage as ShellBallBubbleMessage,
} from "./shellBallBubbleDesktop";

export {
  cloneShellBallBubbleDesktopState,
  cloneShellBallBubbleItem,
  cloneShellBallBubbleItems,
  cloneShellBallBubbleMessages,
  createLegacyShellBallBubbleMessage,
  createLegacyShellBallBubbleMessages,
  createShellBallBubbleItemFromLegacyMessage,
  createShellBallBubbleItemsFromLegacyMessages,
} from "./shellBallBubbleDesktop";
