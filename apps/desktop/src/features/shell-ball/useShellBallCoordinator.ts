import type { BubbleMessage, DeliveryResult } from "@cialloclaw/protocol";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { subscribeDeliveryReady } from "@/rpc/subscriptions";
import {
  SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME,
  closeShellBallPinnedBubbleWindow,
  emitToShellBallWindowLabel,
  getShellBallPinnedBubbleIdFromLabel,
  getShellBallPinnedBubbleWindowAnchor,
  getShellBallPinnedBubbleWindowLabel,
  openShellBallPinnedBubbleWindow,
  setShellBallPinnedBubbleWindowVisible,
  shellBallWindowLabels,
} from "../../platform/shellBallWindowController";
import { cloneShellBallBubbleItems, type ShellBallBubbleItem } from "./shellBall.bubble";
import type { ShellBallVoicePreview } from "./shellBall.interaction";
import type { ShellBallInputBarMode, ShellBallVisualState, ShellBallVoiceHintMode } from "./shellBall.types";
import type { ShellBallInputSubmitResult } from "./useShellBallInteraction";
import { isRpcChannelUnavailable, logRpcMockFallback } from "@/rpc/fallback";
import { readClipboardText } from "@/services/clipboardService";
import { startTaskFromFiles } from "@/services/taskService";
import {
  createDefaultShellBallWindowSnapshot,
  createShellBallWindowSnapshot,
  getShellBallVisibleBubbleItems,
  type ShellBallBubbleAction,
  type ShellBallBubbleActionPayload,
  type ShellBallBubbleHoverPayload,
  type ShellBallBubbleVisibilityPhase,
  type ShellBallIntentDecisionPayload,
  shellBallWindowSyncEvents,
  type ShellBallHelperReadyPayload,
  type ShellBallHelperWindowRole,
  type ShellBallInputDraftPayload,
  type ShellBallInputFocusPayload,
  type ShellBallInputHoverPayload,
  type ShellBallPendingFileActionPayload,
  type ShellBallInputRequestFocusPayload,
  type ShellBallPinnedWindowDetachedPayload,
  type ShellBallPinnedWindowReadyPayload,
  type ShellBallPrimaryAction,
  type ShellBallPrimaryActionPayload,
} from "./shellBall.windowSync";
import { getShellBallBubbleAnchor } from "./useShellBallWindowMetrics";
import { getShellBallVisualStateForTaskStatus } from "./shellBall.interaction";
import { createMockShellBallConfirmResult } from "./shellBall.mock";
import { useShellBallStore } from "../../stores/shellBallStore";

type ShellBallCoordinatorInput = {
  visualState: ShellBallVisualState;
  helperWindowsVisible?: boolean;
  inputValue: string;
  pendingFiles?: string[];
  finalizedSpeechPayload: string | null;
  voicePreview: ShellBallVoicePreview;
  voiceHintMode: ShellBallVoiceHintMode;
  setInputValue: (value: string) => void;
  onAppendPendingFiles?: (paths: string[]) => void;
  onRemovePendingFile?: (path: string) => void;
  onFinalizedSpeechHandled: () => void;
  onRegionEnter: () => void;
  onRegionLeave: () => void;
  onInputFocusChange: (focused: boolean) => void;
  onSubmitText: () => Promise<ShellBallInputSubmitResult | null> | ShellBallInputSubmitResult | null | void;
  onAttachFile: () => void;
  onPrimaryClick: () => void;
};

type ShellBallHelperSnapshotInput = {
  role: ShellBallHelperWindowRole;
  windowLabel?: string;
};

const SHELL_BALL_LOCAL_BUBBLE_ITEMS: ShellBallBubbleItem[] = [];
const SHELL_BALL_BUBBLE_HIDE_DELAY_MS = 5_000;
const SHELL_BALL_BUBBLE_FADE_DURATION_MS = 420;
const SHELL_BALL_CLIPBOARD_COMMAND = "粘贴板";

function createShellBallRequestMeta() {
  const now = new Date().toISOString();
  const traceId = typeof globalThis.crypto?.randomUUID === "function"
    ? globalThis.crypto.randomUUID()
    : `trace_${Date.now()}_${Math.random().toString(16).slice(2)}`;

  return {
    trace_id: traceId,
    client_time: now,
  };
}

export function compareShellBallBubbleItemsByTimestamp(left: ShellBallBubbleItem, right: ShellBallBubbleItem) {
  const createdAtOrder = left.bubble.created_at.localeCompare(right.bubble.created_at);

  if (createdAtOrder !== 0) {
    return createdAtOrder;
  }

  return left.bubble.bubble_id.localeCompare(right.bubble.bubble_id);
}

export function sortShellBallBubbleItemsByTimestamp(items: ShellBallBubbleItem[]) {
  return [...items].sort(compareShellBallBubbleItemsByTimestamp);
}

function isShellBallInputSubmitResult(value: ShellBallInputSubmitResult | null | void): value is ShellBallInputSubmitResult {
  return value !== null && value !== undefined && typeof value === "object" && "task" in value;
}

export function createShellBallFinalizedSpeechBubbleItem(input: {
  text: string;
  sequence: number;
  createdAt: string;
}): ShellBallBubbleItem {
  return {
    bubble: {
      bubble_id: `shell-ball-local-user-voice-${input.sequence}`,
      task_id: "",
      type: "result",
      text: input.text,
      pinned: false,
      hidden: false,
      created_at: input.createdAt,
    },
    role: "user",
    desktop: {
      lifecycleState: "visible",
      freshnessHint: "fresh",
      motionHint: "settle",
    },
  };
}

function createShellBallTextBubbleItem(input: {
  role: "user" | "agent";
  text: string;
  bubbleType: BubbleMessage["type"];
  createdAt: string;
  taskId?: string;
}) {
  const prefix = input.role === "user" ? "shell-ball-local-user-text" : "shell-ball-local-agent-text";

  return {
    bubble: {
      bubble_id: `${prefix}-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`,
      task_id: input.taskId ?? "",
      type: input.bubbleType,
      text: input.text,
      pinned: false,
      hidden: false,
      created_at: input.createdAt,
    },
    role: input.role,
    desktop: {
      lifecycleState: "visible",
      freshnessHint: "fresh",
      motionHint: "settle",
    },
  } satisfies ShellBallBubbleItem;
}

function getShellBallPendingFileName(filePath: string) {
  const normalizedPath = filePath.replace(/\\/g, "/").trim();
  if (normalizedPath === "") {
    return "未命名文件";
  }

  const segments = normalizedPath.split("/").filter((segment) => segment !== "");
  return segments.at(-1) ?? normalizedPath;
}

function summarizeShellBallPendingFiles(filePaths: string[]) {
  const fileNames = filePaths.map(getShellBallPendingFileName).filter((fileName) => fileName !== "");
  if (fileNames.length === 0) {
    return "";
  }

  const visibleNames = fileNames.slice(0, 3).join("、");
  if (fileNames.length <= 3) {
    return visibleNames;
  }

  return `${visibleNames} 等 ${fileNames.length} 个文件`;
}

function createShellBallSubmittedContentPreview(input: {
  text: string;
  files: string[];
}) {
  const lines: string[] = [];
  const fileSummary = summarizeShellBallPendingFiles(input.files);
  const trimmedText = input.text.trim();

  if (fileSummary !== "") {
    lines.push(`附件：${fileSummary}`);
  }
  if (trimmedText !== "") {
    lines.push(fileSummary === "" ? trimmedText : `说明：${trimmedText}`);
  }

  return lines.join("\n");
}

function createShellBallDeliveryResultBubbleItem(input: {
  taskId: string;
  deliveryResult: DeliveryResult;
  createdAt: string;
}) {
  return createShellBallTextBubbleItem({
    role: "agent",
    text: input.deliveryResult.preview_text.trim() || input.deliveryResult.title,
    bubbleType: "result",
    createdAt: input.createdAt,
    taskId: input.taskId,
  });
}

function syncShellBallVisualStateFromTaskStatus(status: Parameters<typeof getShellBallVisualStateForTaskStatus>[0]) {
  const currentState = useShellBallStore.getState().visualState;
  const nextState = getShellBallVisualStateForTaskStatus(status, currentState);
  useShellBallStore.getState().setVisualState(nextState);
}

export function createShellBallAgentBubbleItem(result: ShellBallInputSubmitResult, fallbackCreatedAt: string) {
  const deliveryPreview = result.delivery_result?.type === "bubble" ? result.delivery_result.preview_text?.trim() ?? "" : "";
  const bubbleMessage = result.bubble_message;

  if (deliveryPreview !== "") {
    return createShellBallTextBubbleItem({
      role: "agent",
      text: deliveryPreview,
      bubbleType: "result",
      createdAt: result.delivery_result?.payload?.task_id ? fallbackCreatedAt : bubbleMessage?.created_at ?? fallbackCreatedAt,
      taskId: result.task.task_id,
    });
  }

  if (bubbleMessage?.text.trim()) {
    return {
      bubble: {
        ...bubbleMessage,
        hidden: false,
        pinned: false,
      },
      role: "agent",
      desktop: {
        lifecycleState: "visible",
        freshnessHint: "fresh",
        motionHint: "settle",
      },
    } satisfies ShellBallBubbleItem;
  }

  return createShellBallTextBubbleItem({
    role: "agent",
    text: "已收到，正在处理。",
    bubbleType: "status",
    createdAt: fallbackCreatedAt,
    taskId: result.task.task_id,
  });
}

export function applyShellBallBubbleAction(
  items: ShellBallBubbleItem[],
  payload: Pick<ShellBallBubbleActionPayload, "action" | "bubbleId">,
): ShellBallBubbleItem[] {
  if (payload.action === "delete") {
    return sortShellBallBubbleItemsByTimestamp(items.filter((item) => item.bubble.bubble_id !== payload.bubbleId));
  }

  return sortShellBallBubbleItemsByTimestamp(
    items.map((item) => {
      if (item.bubble.bubble_id !== payload.bubbleId) {
        return item;
      }

      return {
        ...item,
        bubble: {
          ...item.bubble,
          pinned: payload.action === "pin",
        },
      };
    }),
  );
}

export function useShellBallCoordinator(input: ShellBallCoordinatorInput) {
  const [bubbleItems, setBubbleItems] = useState(() => sortShellBallBubbleItemsByTimestamp(cloneShellBallBubbleItems(SHELL_BALL_LOCAL_BUBBLE_ITEMS)));
  const appendedVoiceBubbleSequenceRef = useRef(0);
  const handledFinalizedSpeechPayloadRef = useRef<string | null>(null);
  const [bubbleVisibilityPhase, setBubbleVisibilityPhase] = useState<ShellBallBubbleVisibilityPhase>("hidden");
  const helpersVisible = input.helperWindowsVisible ?? true;
  const snapshot = useMemo(
    () =>
      createShellBallWindowSnapshot({
        visualState: input.visualState,
        helpersVisible,
        inputValue: input.inputValue,
        pendingFiles: input.pendingFiles ?? [],
        voicePreview: input.voicePreview,
        voiceHintMode: input.voiceHintMode,
        bubbleItems,
        bubbleVisibilityPhase,
      }),
    [bubbleItems, bubbleVisibilityPhase, helpersVisible, input.inputValue, input.pendingFiles, input.visualState, input.voiceHintMode, input.voicePreview],
  );
  const snapshotRef = useRef(snapshot);
  const bubbleItemsRef = useRef(bubbleItems);
  const bubbleVisibilityPhaseRef = useRef<ShellBallBubbleVisibilityPhase>(bubbleVisibilityPhase);
  const visibleBubbleCountRef = useRef(getShellBallVisibleBubbleItems(bubbleItems).length);
  const previousVisibleBubbleCountRef = useRef(visibleBubbleCountRef.current);
  const detachedPinnedBubbleIdsRef = useRef(new Set<string>());
  const deliveryReadyBubbleKeysRef = useRef(new Set<string>());
  const shellBallTaskIdsRef = useRef(new Set<string>());
  const helperWindowsVisibleRef = useRef(input.helperWindowsVisible ?? true);
  const regionActiveRef = useRef(false);
  const bubbleHoveredRef = useRef(false);
  const inputFocusedRef = useRef(false);
  const bubbleHideDelayTimeoutRef = useRef<number | null>(null);
  const bubbleHideCompleteTimeoutRef = useRef<number | null>(null);
  helperWindowsVisibleRef.current = helpersVisible;
  const handlersRef = useRef({
    setInputValue: input.setInputValue,
    onAppendPendingFiles: input.onAppendPendingFiles ?? (() => {}),
    onRemovePendingFile: input.onRemovePendingFile ?? (() => {}),
    onFinalizedSpeechHandled: input.onFinalizedSpeechHandled,
    onRegionEnter: input.onRegionEnter,
    onRegionLeave: input.onRegionLeave,
    onInputFocusChange: input.onInputFocusChange,
    onSubmitText: input.onSubmitText,
    onAttachFile: input.onAttachFile,
    onPrimaryClick: input.onPrimaryClick,
  });

  snapshotRef.current = snapshot;
  bubbleItemsRef.current = bubbleItems;
  bubbleVisibilityPhaseRef.current = bubbleVisibilityPhase;
  handlersRef.current = {
    setInputValue: input.setInputValue,
    onAppendPendingFiles: input.onAppendPendingFiles ?? (() => {}),
    onRemovePendingFile: input.onRemovePendingFile ?? (() => {}),
    onFinalizedSpeechHandled: input.onFinalizedSpeechHandled,
    onRegionEnter: input.onRegionEnter,
    onRegionLeave: input.onRegionLeave,
    onInputFocusChange: input.onInputFocusChange,
    onSubmitText: input.onSubmitText,
    onAttachFile: input.onAttachFile,
    onPrimaryClick: input.onPrimaryClick,
  };

  const clearBubbleVisibilityTimers = useCallback(() => {
    if (bubbleHideDelayTimeoutRef.current !== null) {
      window.clearTimeout(bubbleHideDelayTimeoutRef.current);
      bubbleHideDelayTimeoutRef.current = null;
    }

    if (bubbleHideCompleteTimeoutRef.current !== null) {
      window.clearTimeout(bubbleHideCompleteTimeoutRef.current);
      bubbleHideCompleteTimeoutRef.current = null;
    }
  }, []);

  const applyBubbleVisibilityPhase = useCallback((nextPhase: ShellBallBubbleVisibilityPhase) => {
    bubbleVisibilityPhaseRef.current = nextPhase;
    setBubbleVisibilityPhase((currentPhase) => (currentPhase === nextPhase ? currentPhase : nextPhase));
  }, []);

  const revealBubbleRegion = useCallback(() => {
    clearBubbleVisibilityTimers();

    if (!helperWindowsVisibleRef.current || visibleBubbleCountRef.current === 0) {
      applyBubbleVisibilityPhase("hidden");
      return;
    }

    applyBubbleVisibilityPhase("visible");
  }, [applyBubbleVisibilityPhase, clearBubbleVisibilityTimers]);

  const scheduleBubbleRegionHide = useCallback(() => {
    clearBubbleVisibilityTimers();

    if (!helperWindowsVisibleRef.current || visibleBubbleCountRef.current === 0) {
      applyBubbleVisibilityPhase("hidden");
      return;
    }

    if (regionActiveRef.current || bubbleHoveredRef.current || inputFocusedRef.current) {
      applyBubbleVisibilityPhase("visible");
      return;
    }

    bubbleHideDelayTimeoutRef.current = window.setTimeout(() => {
      if (!helperWindowsVisibleRef.current || visibleBubbleCountRef.current === 0) {
        applyBubbleVisibilityPhase("hidden");
        return;
      }

      if (regionActiveRef.current || bubbleHoveredRef.current || inputFocusedRef.current) {
        applyBubbleVisibilityPhase("visible");
        return;
      }

      applyBubbleVisibilityPhase("fading");
      bubbleHideCompleteTimeoutRef.current = window.setTimeout(() => {
        if (regionActiveRef.current || bubbleHoveredRef.current || inputFocusedRef.current) {
          applyBubbleVisibilityPhase("visible");
          return;
        }

        applyBubbleVisibilityPhase("hidden");
      }, SHELL_BALL_BUBBLE_FADE_DURATION_MS);
    }, SHELL_BALL_BUBBLE_HIDE_DELAY_MS);
  }, [applyBubbleVisibilityPhase, clearBubbleVisibilityTimers]);

  const handleDroppedFiles = useCallback(async (paths: string[]) => {
    const normalizedPaths = paths.map((path) => path.trim()).filter(Boolean);

    if (normalizedPaths.length === 0) {
      return;
    }

    const createdAt = new Date().toISOString();
    const leadFile = normalizedPaths[0].split(/[\\/]/).pop() ?? normalizedPaths[0];
    const userText = normalizedPaths.length === 1 ? `拖入文件：${leadFile}` : `拖入 ${normalizedPaths.length} 个文件`;

    setBubbleItems((currentItems) =>
      sortShellBallBubbleItemsByTimestamp([
        ...currentItems,
        createShellBallTextBubbleItem({
          role: "user",
          text: userText,
          bubbleType: "status",
          createdAt,
        }),
      ]),
    );
    revealBubbleRegion();

    try {
      const result = await startTaskFromFiles(normalizedPaths, {
        delivery: {
          preferred: "bubble",
          fallback: "task_detail",
        },
        source: "floating_ball",
      });
      shellBallTaskIdsRef.current.add(result.task.task_id);

      syncShellBallVisualStateFromTaskStatus(result.task.status);

      setBubbleItems((currentItems) =>
        sortShellBallBubbleItemsByTimestamp([
          ...currentItems,
          createShellBallAgentBubbleItem(result, new Date().toISOString()),
        ]),
      );
      revealBubbleRegion();
    } catch (error) {
      console.warn("shell-ball file drop start failed", error);
      setBubbleItems((currentItems) =>
        sortShellBallBubbleItemsByTimestamp([
          ...currentItems,
          createShellBallTextBubbleItem({
            role: "agent",
            text: error instanceof Error ? error.message : "文件承接失败，请稍后再试。",
            bubbleType: "status",
            createdAt: new Date().toISOString(),
          }),
        ]),
      );
      revealBubbleRegion();
    }
  }, [revealBubbleRegion]);

  const handleSelectedTextPrompt = useCallback((text: string) => {
    setBubbleItems((currentItems) =>
      sortShellBallBubbleItemsByTimestamp([
        ...currentItems,
        createShellBallTextBubbleItem({
          role: "agent",
          text: createShellBallSelectedTextPreview(text),
          bubbleType: "status",
          createdAt: new Date().toISOString(),
        }),
      ]),
    );
    revealBubbleRegion();
  }, [revealBubbleRegion]);

  useEffect(() => {
    const visibleBubbleCount = getShellBallVisibleBubbleItems(bubbleItems).length;
    const previousVisibleBubbleCount = previousVisibleBubbleCountRef.current;

    visibleBubbleCountRef.current = visibleBubbleCount;
    previousVisibleBubbleCountRef.current = visibleBubbleCount;

    if (!helperWindowsVisibleRef.current || visibleBubbleCount === 0) {
      clearBubbleVisibilityTimers();
      applyBubbleVisibilityPhase("hidden");
      return;
    }

    if (regionActiveRef.current || bubbleHoveredRef.current || inputFocusedRef.current) {
      revealBubbleRegion();
      return;
    }

    if (visibleBubbleCount > previousVisibleBubbleCount) {
      revealBubbleRegion();
      scheduleBubbleRegionHide();
    }
  }, [applyBubbleVisibilityPhase, bubbleItems, clearBubbleVisibilityTimers, revealBubbleRegion, scheduleBubbleRegionHide]);

  useEffect(() => {
    if (!helpersVisible) {
      clearBubbleVisibilityTimers();
      applyBubbleVisibilityPhase("hidden");
      return;
    }

    if (visibleBubbleCountRef.current === 0) {
      applyBubbleVisibilityPhase("hidden");
      return;
    }

    if (regionActiveRef.current || bubbleHoveredRef.current || inputFocusedRef.current) {
      revealBubbleRegion();
      return;
    }

    scheduleBubbleRegionHide();
  }, [applyBubbleVisibilityPhase, clearBubbleVisibilityTimers, helpersVisible, revealBubbleRegion, scheduleBubbleRegionHide]);

  useEffect(() => {
    const hoverDrivenState =
      input.visualState === "hover_input" || input.visualState === "voice_listening" || input.visualState === "voice_locked";

    if (hoverDrivenState) {
      regionActiveRef.current = true;
      revealBubbleRegion();
      return;
    }

    if (input.visualState === "idle") {
      regionActiveRef.current = false;

      if (!inputFocusedRef.current) {
        scheduleBubbleRegionHide();
      }
    }
  }, [input.visualState, revealBubbleRegion, scheduleBubbleRegionHide]);

  useEffect(() => {
    return () => {
      clearBubbleVisibilityTimers();
    };
  }, [clearBubbleVisibilityTimers]);

  useEffect(() => {
    const finalizedSpeechPayload = input.finalizedSpeechPayload;

    if (finalizedSpeechPayload === null) {
      handledFinalizedSpeechPayloadRef.current = null;
      return;
    }

    if (handledFinalizedSpeechPayloadRef.current === finalizedSpeechPayload) {
      return;
    }

    handledFinalizedSpeechPayloadRef.current = finalizedSpeechPayload;
    appendedVoiceBubbleSequenceRef.current += 1;

    setBubbleItems((currentItems) =>
      sortShellBallBubbleItemsByTimestamp([
        ...currentItems,
        createShellBallFinalizedSpeechBubbleItem({
          text: finalizedSpeechPayload,
          sequence: appendedVoiceBubbleSequenceRef.current,
          createdAt: new Date().toISOString(),
        }),
      ]),
    );
    handlersRef.current.onFinalizedSpeechHandled();
  }, [input.finalizedSpeechPayload]);

  useEffect(() => {
    return subscribeDeliveryReady((payload) => {
      if (!shellBallTaskIdsRef.current.has(payload.task_id)) {
        return;
      }

      const bubbleText = payload.delivery_result.preview_text.trim() || payload.delivery_result.title;
      const bubbleKey = `${payload.task_id}:${payload.delivery_result.type}:${bubbleText}`;

      if (deliveryReadyBubbleKeysRef.current.has(bubbleKey)) {
        return;
      }

      deliveryReadyBubbleKeysRef.current.add(bubbleKey);

      setBubbleItems((currentItems) => {
        if (
          currentItems.some(
            (item) =>
              item.bubble.task_id === payload.task_id &&
              item.bubble.type === "result" &&
              item.bubble.text === bubbleText,
          )
        ) {
          return currentItems;
        }

        return sortShellBallBubbleItemsByTimestamp([
          ...currentItems,
          createShellBallDeliveryResultBubbleItem({
            createdAt: new Date().toISOString(),
            deliveryResult: payload.delivery_result,
            taskId: payload.task_id,
          }),
        ]);
      });
      revealBubbleRegion();
    });
  }, [revealBubbleRegion]);

  useEffect(() => {
    const currentWindow = getCurrentWindow();
    const latestSnapshot = snapshot;

    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }

    async function emitSnapshotToLabel(label: string) {
      await emitToShellBallWindowLabel(label, shellBallWindowSyncEvents.snapshot, latestSnapshot);
    }

    const pinnedBubbleLabels = latestSnapshot.bubbleItems
      .filter((item) => item.bubble.pinned)
      .map((item) => getShellBallPinnedBubbleWindowLabel(item.bubble.bubble_id));

    void Promise.all([
      emitSnapshotToLabel(shellBallWindowLabels.bubble),
      emitSnapshotToLabel(shellBallWindowLabels.input),
      emitSnapshotToLabel(shellBallWindowLabels.voice),
      ...pinnedBubbleLabels.map((label) => emitSnapshotToLabel(label)),
      ...latestSnapshot.bubbleItems
        .filter((item) => item.bubble.pinned)
        .map((item) => setShellBallPinnedBubbleWindowVisible(item.bubble.bubble_id, latestSnapshot.visibility.bubble)),
    ]);
  }, [snapshot]);

  useEffect(() => {
    const currentWindow = getCurrentWindow();

    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }

    let disposed = false;
    let cleanupFns: Array<() => void> = [];

    async function emitSnapshotTo(role: Exclude<ShellBallHelperWindowRole, "pinned">) {
      await emitToShellBallWindowLabel(shellBallWindowLabels[role], shellBallWindowSyncEvents.snapshot, snapshotRef.current);
    }

    async function syncPinnedBubbleWindowAnchor(bubbleId: string) {
      if (detachedPinnedBubbleIdsRef.current.has(bubbleId)) {
        return;
      }

      const bubbleItem = bubbleItemsRef.current.find((item) => item.bubble.bubble_id === bubbleId && item.bubble.pinned);

      if (bubbleItem === undefined) {
        return;
      }

      const outerPosition = await currentWindow.outerPosition();
      const outerSize = await currentWindow.outerSize();
      const scaleFactor = await currentWindow.scaleFactor();
      const logicalPosition = outerPosition.toLogical(scaleFactor);
      const logicalSize = outerSize.toLogical(scaleFactor);
      const bubbleAnchor = getShellBallBubbleAnchor({
        ballFrame: {
          x: logicalPosition.x,
          y: logicalPosition.y,
          width: logicalSize.width,
          height: logicalSize.height,
        },
        helperFrame: SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME,
      });

      await openShellBallPinnedBubbleWindow({
        bubbleId,
        position: getShellBallPinnedBubbleWindowAnchor({ bubbleAnchor }),
        size: SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME,
      });
    }

    async function syncAnchoredPinnedBubbleWindows() {
      await Promise.all(
        bubbleItemsRef.current
          .filter((item) => item.bubble.pinned)
          .map((item) => syncPinnedBubbleWindowAnchor(item.bubble.bubble_id)),
      );
    }

    function handleCoordinatorRegionEnter() {
      regionActiveRef.current = true;
      revealBubbleRegion();
      handlersRef.current.onRegionEnter();
    }

    function handleCoordinatorRegionLeave() {
      regionActiveRef.current = false;
      scheduleBubbleRegionHide();
      handlersRef.current.onRegionLeave();
    }

    function handleCoordinatorInputFocusChange(focused: boolean) {
      inputFocusedRef.current = focused;

      if (focused) {
        revealBubbleRegion();
      } else if (!regionActiveRef.current && !bubbleHoveredRef.current) {
        scheduleBubbleRegionHide();
      }

      handlersRef.current.onInputFocusChange(focused);
    }

    function handleCoordinatorBubbleHoverChange(active: boolean) {
      bubbleHoveredRef.current = active;

      if (active) {
        revealBubbleRegion();
        return;
      }

      if (!regionActiveRef.current && !inputFocusedRef.current) {
        scheduleBubbleRegionHide();
      }
    }

    async function handlePrimaryAction(action: ShellBallPrimaryAction) {
      switch (action) {
        case "attach_file":
          handlersRef.current.onAttachFile();
          setBubbleItems((currentItems) =>
            sortShellBallBubbleItemsByTimestamp([
              ...currentItems,
              createShellBallTextBubbleItem({
                role: "agent",
                text: "把文件拖到悬浮球上，就会按 issue #187 的 file_drop 入口创建任务。",
                bubbleType: "status",
                createdAt: new Date().toISOString(),
              }),
            ]),
          );
          revealBubbleRegion();
          break;
        case "submit": {
          const submittedText = snapshotRef.current.inputValue.trim();
          const submittedFiles = snapshotRef.current.pendingFiles;

          if (shouldHandleShellBallClipboardCommand({
            text: submittedText,
            files: submittedFiles,
          })) {
            const createdAt = new Date().toISOString();
            setBubbleItems((currentItems) =>
              sortShellBallBubbleItemsByTimestamp([
                ...currentItems,
                createShellBallTextBubbleItem({
                  role: "user",
                  text: SHELL_BALL_CLIPBOARD_COMMAND,
                  bubbleType: "result",
                  createdAt,
                }),
              ]),
            );
            revealBubbleRegion();

            try {
              const clipboardText = await readClipboardText();
              setBubbleItems((currentItems) =>
                sortShellBallBubbleItemsByTimestamp([
                  ...currentItems,
                  createShellBallTextBubbleItem({
                    role: "agent",
                    text: createShellBallClipboardReply(clipboardText),
                    bubbleType: "result",
                    createdAt: new Date().toISOString(),
                  }),
                ]),
              );
            } catch (error) {
              console.warn("shell-ball clipboard read failed", error);
              setBubbleItems((currentItems) =>
                sortShellBallBubbleItemsByTimestamp([
                  ...currentItems,
                  createShellBallTextBubbleItem({
                    role: "agent",
                    text: "Clipboard is unavailable right now.",
                    bubbleType: "status",
                    createdAt: new Date().toISOString(),
                  }),
                ]),
              );
            }

            handlersRef.current.setInputValue("");
            handlersRef.current.onInputFocusChange(false);
            revealBubbleRegion();
            break;
          }

          const submittedPreview = createShellBallSubmittedContentPreview({
            text: submittedText,
            files: submittedFiles,
          });

          if (submittedPreview === "") {
            await handlersRef.current.onSubmitText();
            break;
          }

          const createdAt = new Date().toISOString();
          setBubbleItems((currentItems) =>
            sortShellBallBubbleItemsByTimestamp([
              ...currentItems,
              createShellBallTextBubbleItem({
                role: "user",
                text: submittedPreview,
                bubbleType: "result",
                createdAt,
              }),
            ]),
          );
          revealBubbleRegion();

          const result = await handlersRef.current.onSubmitText();
          if (isShellBallInputSubmitResult(result)) {
            shellBallTaskIdsRef.current.add(result.task.task_id);
            setBubbleItems((currentItems) =>
              sortShellBallBubbleItemsByTimestamp([
                ...currentItems,
                createShellBallAgentBubbleItem(result, new Date().toISOString()),
              ]),
            );
            revealBubbleRegion();
          }

          break;
        }
        case "primary_click":
          handlersRef.current.onPrimaryClick();
          break;
      }
    }

    async function handleIntentDecision(payload: ShellBallIntentDecisionPayload) {
      const importRpcMethods = new Function("return import('../../rpc/methods')") as () => Promise<{
        confirmTask: (request: {
          confirmed: boolean;
          request_meta: ReturnType<typeof createShellBallRequestMeta>;
          task_id: string;
        }) => Promise<ShellBallInputSubmitResult>;
      }>;

      try {
        const rpcMethods = await importRpcMethods();
        const createdAt = new Date().toISOString();
        const decisionText = payload.decision === "confirm" ? "确认继续" : "取消";

        setBubbleItems((currentItems) =>
          sortShellBallBubbleItemsByTimestamp([
            ...currentItems,
            createShellBallTextBubbleItem({
              createdAt,
              role: "user",
              text: decisionText,
              bubbleType: "status",
              taskId: payload.taskId,
            }),
          ]),
        );

        let result: ShellBallInputSubmitResult;

        try {
          result = await rpcMethods.confirmTask({
            confirmed: payload.decision === "confirm",
            request_meta: createShellBallRequestMeta(),
            task_id: payload.taskId,
          });
        } catch (error) {
          if (!isRpcChannelUnavailable(error)) {
            throw error;
          }

          logRpcMockFallback("shell-ball confirm", error);
          result = createMockShellBallConfirmResult({
            confirmed: payload.decision === "confirm",
            taskId: payload.taskId,
          });
        }

        syncShellBallVisualStateFromTaskStatus(result.task.status);
        shellBallTaskIdsRef.current.add(result.task.task_id);

        setBubbleItems((currentItems) =>
          sortShellBallBubbleItemsByTimestamp([
            ...currentItems,
            createShellBallAgentBubbleItem(result, new Date().toISOString()),
          ]),
        );
        revealBubbleRegion();
      } catch (error) {
        console.warn("shell-ball intent decision failed", error);
      }
    }

    function handleBubbleAction(payload: ShellBallBubbleActionPayload) {
      setBubbleItems((currentItems) => applyShellBallBubbleAction(currentItems, payload));

      if (payload.action === "pin") {
        detachedPinnedBubbleIdsRef.current.delete(payload.bubbleId);
        void syncPinnedBubbleWindowAnchor(payload.bubbleId);
        return;
      }

      detachedPinnedBubbleIdsRef.current.delete(payload.bubbleId);
      void closeShellBallPinnedBubbleWindow(payload.bubbleId);
    }

    void Promise.all([
      currentWindow.listen<ShellBallHelperReadyPayload>(
        shellBallWindowSyncEvents.helperReady,
        ({ payload }) => {
          void emitSnapshotTo(payload.role);
        },
      ),
      currentWindow.listen<ShellBallPinnedWindowReadyPayload>(
        shellBallWindowSyncEvents.pinnedWindowReady,
        ({ payload }) => {
          void emitToShellBallWindowLabel(payload.windowLabel, shellBallWindowSyncEvents.snapshot, snapshotRef.current);
          void syncPinnedBubbleWindowAnchor(payload.bubbleId);
        },
      ),
      currentWindow.listen<ShellBallPinnedWindowDetachedPayload>(
        shellBallWindowSyncEvents.pinnedWindowDetached,
        ({ payload }) => {
          detachedPinnedBubbleIdsRef.current.add(payload.bubbleId);
        },
      ),
      currentWindow.listen<ShellBallInputHoverPayload>(shellBallWindowSyncEvents.inputHover, ({ payload }) => {
        if (payload.active) {
          handleCoordinatorRegionEnter();
          return;
        }

        handleCoordinatorRegionLeave();
      }),
      currentWindow.listen<ShellBallBubbleHoverPayload>(shellBallWindowSyncEvents.bubbleHover, ({ payload }) => {
        handleCoordinatorBubbleHoverChange(payload.active);
      }),
      currentWindow.listen<ShellBallInputFocusPayload>(shellBallWindowSyncEvents.inputFocus, ({ payload }) => {
        handleCoordinatorInputFocusChange(payload.focused);
      }),
      currentWindow.listen<ShellBallInputDraftPayload>(shellBallWindowSyncEvents.inputDraft, ({ payload }) => {
        handlersRef.current.setInputValue(payload.value);
      }),
      currentWindow.listen<ShellBallPendingFileActionPayload>(shellBallWindowSyncEvents.pendingFileAction, ({ payload }) => {
        if (payload.action === "append") {
          handlersRef.current.onAppendPendingFiles(payload.paths);
          return;
        }

        if (payload.action === "remove") {
          handlersRef.current.onRemovePendingFile(payload.path);
        }
      }),
      currentWindow.listen<ShellBallPrimaryActionPayload>(
        shellBallWindowSyncEvents.primaryAction,
        ({ payload }) => {
          void handlePrimaryAction(payload.action);
        },
      ),
      currentWindow.listen<ShellBallIntentDecisionPayload>(shellBallWindowSyncEvents.intentDecision, ({ payload }) => {
        void handleIntentDecision(payload);
      }),
      currentWindow.listen<ShellBallBubbleActionPayload>(shellBallWindowSyncEvents.bubbleAction, ({ payload }) => {
        handleBubbleAction(payload);
      }),
      currentWindow.onMoved(() => {
        void syncAnchoredPinnedBubbleWindows();
      }),
      currentWindow.onResized(() => {
        void syncAnchoredPinnedBubbleWindows();
      }),
    ]).then((unlisteners) => {
      if (disposed) {
        for (const unlisten of unlisteners) {
          unlisten();
        }
        return;
      }

      cleanupFns = unlisteners;
    });

    return () => {
      disposed = true;
      for (const cleanup of cleanupFns) {
        cleanup();
      }
    };
  }, [revealBubbleRegion, scheduleBubbleRegionHide]);

  return { snapshot, handleDroppedFiles, handleSelectedTextPrompt };
}

export function useShellBallHelperWindowSnapshot({ role }: ShellBallHelperSnapshotInput) {
  const [snapshot, setSnapshot] = useState(createDefaultShellBallWindowSnapshot);

  useEffect(() => {
    const currentWindow = getCurrentWindow();

    const targetLabel = role === "pinned" ? currentWindow.label : shellBallWindowLabels[role];

    if (role === "pinned" && getShellBallPinnedBubbleIdFromLabel(targetLabel) === null) {
      return;
    }

    if (role !== "pinned" && currentWindow.label !== targetLabel) {
      return;
    }

    let cleanup: (() => void) | null = null;
    let disposed = false;

    void currentWindow
      .listen(shellBallWindowSyncEvents.snapshot, ({ payload }) => {
        setSnapshot(payload as ReturnType<typeof createDefaultShellBallWindowSnapshot>);
      })
      .then((unlisten) => {
        if (disposed) {
          unlisten();
          return;
        }

        cleanup = unlisten;

        if (role === "pinned") {
          const bubbleId = getShellBallPinnedBubbleIdFromLabel(targetLabel);

          if (bubbleId !== null) {
            void currentWindow.emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.pinnedWindowReady, {
              windowLabel: targetLabel,
              bubbleId,
            });
          }

          return;
        }

        void currentWindow.emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.helperReady, { role });
      });

    return () => {
      disposed = true;
      cleanup?.();
    };
  }, [role]);

  return snapshot;
}

export async function emitShellBallInputHover(active: boolean) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.inputHover, { active });
}

export async function emitShellBallBubbleHover(active: boolean) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.bubbleHover, { active });
}

export async function emitShellBallInputFocus(focused: boolean) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.inputFocus, {
    focused,
  });
}

export async function emitShellBallInputDraft(value: string) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.inputDraft, { value });
}

export async function emitShellBallInputRequestFocus(token: number) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.input, shellBallWindowSyncEvents.inputRequestFocus, { token });
}

export async function emitShellBallPrimaryAction(action: ShellBallPrimaryAction, source: ShellBallHelperWindowRole) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.primaryAction, {
    action,
    source,
  });
}

export async function emitShellBallPendingFileAction(payload: ShellBallPendingFileActionPayload) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.pendingFileAction, payload);
}

export async function emitShellBallIntentDecision(
  decision: ShellBallIntentDecisionPayload["decision"],
  taskId: string,
  source: ShellBallIntentDecisionPayload["source"],
) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.intentDecision, {
    decision,
    source,
    taskId,
  });
}

export async function emitShellBallBubbleAction(
  action: ShellBallBubbleAction,
  bubbleId: string,
  source: ShellBallBubbleActionPayload["source"] = "bubble",
) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.bubbleAction, {
    action,
    bubbleId,
    source,
  });
}

export async function emitShellBallPinnedWindowDetached(bubbleId: string) {
  await getCurrentWindow().emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.pinnedWindowDetached, {
    bubbleId,
  });
}
/**
 * Builds a compact selection preview so shell-ball can acknowledge the exact
 * text that was detected without overwhelming the bubble region.
 *
 * @param text Selected text captured from the current DOM scene.
 * @returns A short preview string for the acknowledgement bubble.
 */
function createShellBallSelectedTextPreview(text: string) {
  const normalizedText = text.replace(/\s+/g, " ").trim();

  if (normalizedText === "") {
    return "识别到选中了文字";
  }

  if (normalizedText.length <= 28) {
    return `识别到选中了文字：${normalizedText}`;
  }

  return `识别到选中了文字：${normalizedText.slice(0, 28)}…`;
}

/**
 * Determines whether the current shell-ball draft should be handled by the
 * frontend-only clipboard shortcut instead of the normal submit path.
 *
 * @param input Current text draft and pending file attachments.
 * @returns Whether the clipboard shortcut should run locally.
 */
function shouldHandleShellBallClipboardCommand(input: {
  text: string;
  files: string[];
}) {
  return input.files.length === 0 && input.text.trim() === SHELL_BALL_CLIPBOARD_COMMAND;
}

/**
 * Resolves the fixed assistant reply used by the clipboard shortcut.
 *
 * @param text Clipboard text returned by the desktop clipboard service.
 * @returns The user-facing reply bubble content.
 */
function createShellBallClipboardReply(text: string) {
  if (text.trim() === "") {
    return "Clipboard is empty.";
  }

  return text;
}
