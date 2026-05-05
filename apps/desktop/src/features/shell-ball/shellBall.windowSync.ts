import { cloneShellBallBubbleItems } from "./shellBall.bubble";
import type { ShellBallBubbleItem } from "./shellBall.bubble";
import type { ShellBallVoicePreview } from "./shellBall.interaction";
import { getShellBallInputBarMode } from "./shellBall.interaction";
import type { ShellBallSelectionSnapshot } from "./selection/selection.types";
import type { ShellBallInputBarMode, ShellBallVisualState, ShellBallVoiceHintMode } from "./shellBall.types";

export const shellBallWindowSyncEvents = Object.freeze({
  snapshot: "desktop-shell-ball:snapshot",
  geometry: "desktop-shell-ball:geometry",
  helperReady: "desktop-shell-ball:helper-ready",
  selectionSnapshot: "desktop-shell-ball:selection-snapshot",
  clipboardSnapshot: "desktop-shell-ball:clipboard-snapshot",
  pinnedWindowReady: "desktop-shell-ball:pinned-window-ready",
  pinnedWindowDetached: "desktop-shell-ball:pinned-window-detached",
  bubbleHover: "desktop-shell-ball:bubble-hover",
  inputHover: "desktop-shell-ball:input-hover",
  inputFocus: "desktop-shell-ball:input-focus",
  inputRequestFocus: "desktop-shell-ball:input-request-focus",
  inputDraft: "desktop-shell-ball:input-draft",
  primaryAction: "desktop-shell-ball:primary-action",
  pendingFileAction: "desktop-shell-ball:pending-file-action",
  intentDecision: "desktop-shell-ball:intent-decision",
  bubbleAction: "desktop-shell-ball:bubble-action",
});

export type ShellBallHelperWindowRole = "bubble" | "input" | "voice" | "pinned";

export type ShellBallPrimaryAction = "attach_file" | "submit" | "primary_click";

export type ShellBallPendingFileAction = "append" | "remove";

export type ShellBallIntentDecision = "confirm" | "cancel";

export type ShellBallBubbleAction = "pin" | "unpin" | "delete" | "allow_approval" | "deny_approval";

export type ShellBallBubbleActionSource = "bubble" | "pinned_window";

export type ShellBallHelperWindowVisibility = {
  bubble: boolean;
  input: boolean;
  voice: boolean;
};

export type ShellBallBubbleVisibilityPhase = "visible" | "fading" | "hidden";

export type ShellBallBubbleRegionState = {
  strategy: "persistent";
  hasVisibleItems: boolean;
  clickThrough: boolean;
  visibilityPhase: ShellBallBubbleVisibilityPhase;
};

export type ShellBallInputInteractionState = {
  clickThrough: boolean;
};

export type ShellBallWindowSnapshot = {
  visualState: ShellBallVisualState;
  voiceHintMode: ShellBallVoiceHintMode;
  inputBarMode: ShellBallInputBarMode;
  inputValue: string;
  pendingFiles: string[];
  voicePreview: ShellBallVoicePreview;
  bubbleItems: ShellBallBubbleItem[];
  bubbleRegion: ShellBallBubbleRegionState;
  inputInteraction: ShellBallInputInteractionState;
  visibility: ShellBallHelperWindowVisibility;
};

export type ShellBallWindowGeometry = {
  ballFrame: {
    x: number;
    y: number;
    width: number;
    height: number;
  };
  bounds: {
    minX: number;
    minY: number;
    maxX: number;
    maxY: number;
  };
  scaleFactor: number;
};

export type ShellBallHelperReadyPayload = {
  role: Exclude<ShellBallHelperWindowRole, "pinned">;
};

export type ShellBallPinnedWindowReadyPayload = {
  windowLabel: string;
  bubbleId: string;
};

export type ShellBallPinnedWindowDetachedPayload = {
  bubbleId: string;
};

export type ShellBallInputHoverPayload = {
  active: boolean;
};

export type ShellBallBubbleHoverPayload = {
  active: boolean;
};

export type ShellBallInputFocusPayload = {
  focused: boolean;
};

export type ShellBallInputDraftPayload = {
  value: string;
};

export type ShellBallInputRequestFocusPayload = {
  token: number;
};

export type ShellBallSelectionSnapshotPayload = {
  snapshot: ShellBallSelectionSnapshot | null;
};

export type ShellBallClipboardSnapshotPayload = {
  text: string;
};

export type ShellBallPrimaryActionPayload = {
  source: ShellBallHelperWindowRole;
  action: ShellBallPrimaryAction;
};

export type ShellBallPendingFileActionPayload =
  | {
      action: "append";
      paths: string[];
    }
  | {
      action: "remove";
      path: string;
    };

export type ShellBallIntentDecisionPayload = {
  source: ShellBallBubbleActionSource;
  taskId: string;
  decision: ShellBallIntentDecision;
};

export type ShellBallBubbleActionPayload = {
  source: ShellBallBubbleActionSource;
  action: ShellBallBubbleAction;
  bubbleId: string;
};

export function getShellBallHelperWindowVisibility(
  visualState: ShellBallVisualState,
  helpersVisible = true,
  bubbleVisibilityPhase: ShellBallBubbleVisibilityPhase = "hidden",
  voiceHintMode: ShellBallVoiceHintMode = "hidden",
): ShellBallHelperWindowVisibility {
  if (!helpersVisible) {
    return {
      bubble: false,
      input: false,
      voice: false,
    };
  }

  return {
    bubble: bubbleVisibilityPhase !== "hidden",
    input: getShellBallInputBarMode(visualState) !== "hidden",
    voice: voiceHintMode !== "hidden",
  };
}

export function getShellBallVisibleBubbleItems(items: ShellBallBubbleItem[]): ShellBallBubbleItem[] {
  return items.filter((item) => item.bubble.hidden === false && item.bubble.pinned === false);
}

export function getShellBallBubbleRegionState(
  items: ShellBallBubbleItem[],
  visibilityPhase: ShellBallBubbleVisibilityPhase = "hidden",
): ShellBallBubbleRegionState {
  const visibleItems = getShellBallVisibleBubbleItems(items);

  return {
    strategy: "persistent",
    hasVisibleItems: visibleItems.length > 0,
    clickThrough: visibleItems.length === 0 || visibilityPhase !== "visible",
    visibilityPhase,
  };
}

export function getShellBallInputInteractionState(input: {
  visualState: ShellBallVisualState;
  regionActive: boolean;
  inputFocused: boolean;
  inputHovered: boolean;
  hasDraft: boolean;
}): ShellBallInputInteractionState {
  const mode = getShellBallInputBarMode(input.visualState);
  if (mode === "hidden") {
    return {
      clickThrough: true,
    };
  }

  return {
    clickThrough: false,
  };
}

export function createShellBallWindowSnapshot(input: {
  visualState: ShellBallVisualState;
  voiceHintMode?: ShellBallVoiceHintMode;
  inputValue: string;
  pendingFiles?: string[];
  voicePreview: ShellBallVoicePreview;
  bubbleItems?: ShellBallBubbleItem[];
  helpersVisible?: boolean;
  bubbleVisibilityPhase?: ShellBallBubbleVisibilityPhase;
  inputInteraction?: ShellBallInputInteractionState;
}): ShellBallWindowSnapshot {
  const bubbleItems = cloneShellBallBubbleItems(input.bubbleItems ?? []);
  const bubbleVisibilityPhase = input.bubbleVisibilityPhase ?? "hidden";
  const pendingFiles = [...(input.pendingFiles ?? [])];

  return {
    visualState: input.visualState,
    voiceHintMode: input.voiceHintMode ?? "hidden",
    inputBarMode: getShellBallInputBarMode(input.visualState),
    inputValue: input.inputValue,
    pendingFiles,
    voicePreview: input.voicePreview,
    bubbleItems,
    bubbleRegion: getShellBallBubbleRegionState(bubbleItems, bubbleVisibilityPhase),
    inputInteraction: input.inputInteraction ?? { clickThrough: false },
    visibility: getShellBallHelperWindowVisibility(
      input.visualState,
      input.helpersVisible,
      bubbleVisibilityPhase,
      input.voiceHintMode ?? "hidden",
    ),
  };
}

export function createDefaultShellBallWindowSnapshot(): ShellBallWindowSnapshot {
  return createShellBallWindowSnapshot({
    visualState: "idle",
    voiceHintMode: "hidden",
    inputValue: "",
    pendingFiles: [],
    voicePreview: null,
    bubbleItems: [],
    helpersVisible: true,
    bubbleVisibilityPhase: "hidden",
  });
}
