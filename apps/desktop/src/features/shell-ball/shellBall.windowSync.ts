import {
  cloneShellBallBubbleItems,
  createLegacyShellBallBubbleMessages,
  createShellBallBubbleItemsFromLegacyMessages,
} from "./shellBall.bubble";
import type { ShellBallBubbleItem, ShellBallBubbleMessage } from "./shellBall.bubble";
import type { ShellBallVoicePreview } from "./shellBall.interaction";
import { getShellBallInputBarMode } from "./shellBall.interaction";
import type { ShellBallInputBarMode, ShellBallVisualState } from "./shellBall.types";

export const shellBallWindowSyncEvents = Object.freeze({
  snapshot: "desktop-shell-ball:snapshot",
  geometry: "desktop-shell-ball:geometry",
  helperReady: "desktop-shell-ball:helper-ready",
  inputHover: "desktop-shell-ball:input-hover",
  inputFocus: "desktop-shell-ball:input-focus",
  inputDraft: "desktop-shell-ball:input-draft",
  primaryAction: "desktop-shell-ball:primary-action",
});

export type ShellBallHelperWindowRole = "bubble" | "input";

export type ShellBallPrimaryAction = "attach_file" | "submit" | "primary_click";

export type ShellBallHelperWindowVisibility = {
  bubble: boolean;
  input: boolean;
};

export type ShellBallWindowSnapshot = {
  visualState: ShellBallVisualState;
  inputBarMode: ShellBallInputBarMode;
  inputValue: string;
  voicePreview: ShellBallVoicePreview;
  bubbleItems: ShellBallBubbleItem[];
  bubbleMessages?: ShellBallBubbleMessage[];
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
  role: ShellBallHelperWindowRole;
};

export type ShellBallInputHoverPayload = {
  active: boolean;
};

export type ShellBallInputFocusPayload = {
  focused: boolean;
};

export type ShellBallInputDraftPayload = {
  value: string;
};

export type ShellBallPrimaryActionPayload = {
  source: ShellBallHelperWindowRole;
  action: ShellBallPrimaryAction;
};

export function getShellBallHelperWindowVisibility(
  visualState: ShellBallVisualState,
): ShellBallHelperWindowVisibility {
  return {
    bubble: visualState !== "idle",
    input: getShellBallInputBarMode(visualState) !== "hidden",
  };
}

export function createShellBallWindowSnapshot(input: {
  visualState: ShellBallVisualState;
  inputValue: string;
  voicePreview: ShellBallVoicePreview;
  bubbleItems?: ShellBallBubbleItem[];
  bubbleMessages?: ShellBallBubbleMessage[];
}): ShellBallWindowSnapshot {
  const bubbleItems = cloneShellBallBubbleItems(
    input.bubbleItems ?? createShellBallBubbleItemsFromLegacyMessages(input.bubbleMessages ?? []),
  );
  const snapshot: ShellBallWindowSnapshot = {
    visualState: input.visualState,
    inputBarMode: getShellBallInputBarMode(input.visualState),
    inputValue: input.inputValue,
    voicePreview: input.voicePreview,
    bubbleItems,
    visibility: getShellBallHelperWindowVisibility(input.visualState),
  };

  Object.defineProperty(snapshot, "bubbleMessages", {
    enumerable: false,
    configurable: true,
    get() {
      return createLegacyShellBallBubbleMessages(bubbleItems);
    },
  });

  return snapshot;
}

export function createDefaultShellBallWindowSnapshot(): ShellBallWindowSnapshot {
  return createShellBallWindowSnapshot({
    visualState: "idle",
    inputValue: "",
    voicePreview: null,
    bubbleItems: [],
  });
}
