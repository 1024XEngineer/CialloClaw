import type { ApprovalDecision, ApprovalRequest, BubbleMessage, DeliveryResult, InputContext, TaskUpdatedNotification } from "@cialloclaw/protocol";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { getTaskDetail, respondSecurityDetailed } from "@/rpc/methods";
import { subscribeApprovalPending, subscribeDeliveryReady, subscribeTaskUpdated } from "@/rpc/subscriptions";
import { submitTextInput } from "@/services/agentInputService";
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
import type { ShellBallSelectionSnapshot } from "./selection/selection.types";
import type { ShellBallVisualState, ShellBallVoiceHintMode } from "./shellBall.types";
import type { ShellBallInputSubmitResult } from "./useShellBallInteraction";
import { isRpcChannelUnavailable } from "@/rpc/fallback";
import { readClipboardText } from "@/services/clipboardService";
import { startTaskFromSelectedText } from "@/services/taskService";
import { requestDashboardTaskDetailOpen } from "@/features/dashboard/shared/dashboardTaskDetailNavigation";
import {
  createDefaultShellBallWindowSnapshot,
  createShellBallWindowSnapshot,
  getShellBallVisibleBubbleItems,
  getShellBallInputInteractionState,
  type ShellBallBubbleAction,
  type ShellBallBubbleActionPayload,
  type ShellBallBubbleVisibilityPhase,
  type ShellBallIntentDecisionPayload,
  shellBallWindowSyncEvents,
  type ShellBallPinnedWindowDetachedPayload,
  type ShellBallPinnedWindowReadyPayload,
  type ShellBallPrimaryAction,
} from "./shellBall.windowSync";
import { getShellBallBubbleAnchor } from "./useShellBallWindowMetrics";
import { getShellBallVisualStateForTaskStatus } from "./shellBall.interaction";
import { useShellBallStore } from "../../stores/shellBallStore";
import {
  getShellBallTaskTerminalStatusText,
  isTerminalShellBallTaskStatus,
  resolveShellBallTaskPollingFailure,
} from "./shellBallTaskPolling";

type ShellBallCoordinatorInput = {
  visualState: ShellBallVisualState;
  helperWindowsVisible?: boolean;
  getBallClientRect?: () => DOMRect | null;
  regionActive: boolean;
  inputValue: string;
  inputFocused: boolean;
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
  onInputHoverChange: (active: boolean) => void;
  onInputFocusChange: (focused: boolean) => void;
  onSubmitText: () => Promise<ShellBallInputSubmitResult | null> | ShellBallInputSubmitResult | null | void;
  onSubmitVoiceText?: (text: string) => Promise<ShellBallInputSubmitResult | null> | ShellBallInputSubmitResult | null;
  getCurrentConversationSessionId?: () => string | undefined;
  onAttachFile: () => void;
  onPrimaryClick: () => void;
};

type QueuedApprovalPendingNotification = {
  approvalRequest: ApprovalRequest;
  taskId: string;
};

type QueuedDeliveryReadyNotification = {
  deliveryResult: DeliveryResult;
  taskId: string;
};

type QueuedTaskUpdatedNotification = TaskUpdatedNotification;
type ShellBallTaskOutputServiceModule = {
  openTaskDeliveryForTask: (taskId: string, artifactId: string | undefined, source?: "rpc" | "mock") => Promise<unknown>;
  performTaskOpenExecution: (
    plan: {
      feedback: string;
      mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
      path: string | null;
      taskId: string | null;
      url: string | null;
    },
    options?: {
      onOpenTaskDetail?: (input: {
        plan: {
          feedback: string;
          mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
          path: string | null;
          taskId: string | null;
          url: string | null;
        };
        taskId: string;
      }) => Promise<string | void> | string | void;
    },
  ) => Promise<string>;
  resolveTaskOpenExecutionPlan: (result: unknown) => {
    feedback: string;
    mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
    path: string | null;
    taskId: string | null;
    url: string | null;
  };
};

const defaultSubmitVoiceText: NonNullable<ShellBallCoordinatorInput["onSubmitVoiceText"]> = () => null;
let shellBallTaskOutputServicePromise: Promise<ShellBallTaskOutputServiceModule> | null = null;

// Lazy-load the dashboard delivery-open helpers so shell-ball can reuse the
// formal desktop open path without creating a hard startup dependency.
function loadShellBallTaskOutputService() {
  if (shellBallTaskOutputServicePromise === null) {
    if (typeof require === "function") {
      const requireTaskOutputService = new Function(
        "loader",
        "return loader('@/features/dashboard/tasks/taskOutput.service')",
      ) as (loader: NodeRequire) => ShellBallTaskOutputServiceModule;
      shellBallTaskOutputServicePromise = Promise.resolve(requireTaskOutputService(require));
    } else {
      const importTaskOutputService = new Function(
        "return import('../dashboard/tasks/taskOutput.service')",
      ) as () => Promise<ShellBallTaskOutputServiceModule>;
      shellBallTaskOutputServicePromise = importTaskOutputService();
    }
  }

  return shellBallTaskOutputServicePromise;
}

const SHELL_BALL_LOCAL_BUBBLE_ITEMS: ShellBallBubbleItem[] = [];
const SHELL_BALL_BUBBLE_HIDE_DELAY_MS = 5_000;
const SHELL_BALL_BUBBLE_FADE_DURATION_MS = 420;
const SHELL_BALL_SUBMIT_STALL_NOTICE_MS = 15_000;
const SHELL_BALL_TASK_POLL_INTERVAL_MS = 1_500;
const SHELL_BALL_CLIPBOARD_COMMAND = "粘贴板";
const SHELL_BALL_SCREENSHOT_COMMAND = "截屏";
const SHELL_BALL_WINDOW_COMMAND = "窗口";

const scheduleShellBallTimeout = globalThis.setTimeout.bind(globalThis);
const clearShellBallTimeout = globalThis.clearTimeout.bind(globalThis);
const scheduleShellBallInterval = globalThis.setInterval.bind(globalThis);
const clearShellBallInterval = globalThis.clearInterval.bind(globalThis);
const SHELL_BALL_SCREENSHOT_PROMPT_TEXT = "帮我看看当前屏幕";
const SHELL_BALL_WINDOW_PROMPT_TEXT = "帮我看看当前窗口";
const SHELL_BALL_SCREENSHOT_SUMMARY = "Current screen inspection requested from the shell-ball screenshot shortcut.";
const SHELL_BALL_WINDOW_SUMMARY = "Foreground window inspection requested from the shell-ball window shortcut.";

type ShellBallBubbleTurnOrder = {
  turnIndex?: number;
  turnPhase?: number;
};

type TerminalTaskBubbleDeduper = {
  markAndCheckShouldDisplay: (taskId: string, status: string) => boolean;
  clearTask: (taskId: string) => void;
};

/**
 * Creates a task-scoped terminal bubble deduper that can be cleaned up when the
 * shell-ball task mapping is detached.
 */
export function createTerminalTaskBubbleDeduper(): TerminalTaskBubbleDeduper {
  const taskStatusMap = new Map<string, Set<string>>();

  return {
    markAndCheckShouldDisplay(taskId, status) {
      const knownStatuses = taskStatusMap.get(taskId);
      if (knownStatuses?.has(status)) {
        return false;
      }

      if (knownStatuses === undefined) {
        taskStatusMap.set(taskId, new Set([status]));
      } else {
        knownStatuses.add(status);
      }
      return true;
    },
    clearTask(taskId) {
      taskStatusMap.delete(taskId);
    },
  };
}

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

function createShellBallBubbleDesktopState(turnOrder: ShellBallBubbleTurnOrder = {}) {
  return {
    lifecycleState: "visible" as const,
    freshnessHint: "fresh" as const,
    motionHint: "settle" as const,
    turnIndex: turnOrder.turnIndex,
    turnPhase: turnOrder.turnPhase,
  };
}

function createShellBallAgentLoadingBubbleItem(input: {
  createdAt: string;
  taskId?: string;
  turnIndex?: number;
  turnPhase?: number;
}) {
  const bubbleItem = createShellBallTextBubbleItem({
    role: "agent",
    text: "正在思考…",
    bubbleType: "status",
    createdAt: input.createdAt,
    taskId: input.taskId,
    turnIndex: input.turnIndex,
    turnPhase: input.turnPhase,
  });

  return {
    ...bubbleItem,
    desktop: {
      ...bubbleItem.desktop,
      presentationHint: "loading" as const,
    },
  } satisfies ShellBallBubbleItem;
}

function replaceShellBallPendingBubble(
  items: ShellBallBubbleItem[],
  pendingBubbleId: string,
  nextItem?: ShellBallBubbleItem,
) {
  const nextItems = items.filter((item) => item.bubble.bubble_id !== pendingBubbleId);
  return nextItem === undefined ? sortShellBallBubbleItemsByTimestamp(nextItems) : sortShellBallBubbleItemsByTimestamp([...nextItems, nextItem]);
}

function markShellBallPendingBubbleAsStalled(items: ShellBallBubbleItem[], pendingBubbleId: string) {
  return sortShellBallBubbleItemsByTimestamp(items.map((item) => {
    if (item.bubble.bubble_id !== pendingBubbleId) {
      return item;
    }

    return {
      ...item,
      bubble: {
        ...item.bubble,
        text: "还在处理中…如果长时间没有结果，请重试或到任务面板查看。",
      },
      desktop: {
        ...item.desktop,
        presentationHint: undefined,
      },
    } satisfies ShellBallBubbleItem;
  }));
}

export function compareShellBallBubbleItemsByTimestamp(left: ShellBallBubbleItem, right: ShellBallBubbleItem) {
  // Anchor late agent replies to the user turn that created them before falling back to timestamps.
  const leftTurnIndex = left.desktop.turnIndex;
  const rightTurnIndex = right.desktop.turnIndex;

  if (leftTurnIndex !== undefined && rightTurnIndex !== undefined) {
    if (leftTurnIndex !== rightTurnIndex) {
      return leftTurnIndex - rightTurnIndex;
    }

    const leftTurnPhase = left.desktop.turnPhase ?? 0;
    const rightTurnPhase = right.desktop.turnPhase ?? 0;

    if (leftTurnPhase !== rightTurnPhase) {
      return leftTurnPhase - rightTurnPhase;
    }
  }

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
  turnIndex?: number;
  turnPhase?: number;
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
    desktop: createShellBallBubbleDesktopState(input),
  };
}

function createShellBallTextBubbleItem(input: {
  role: "user" | "agent";
  text: string;
  bubbleType: BubbleMessage["type"];
  createdAt: string;
  taskId?: string;
  turnIndex?: number;
  turnPhase?: number;
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
    desktop: createShellBallBubbleDesktopState(input),
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
  turnIndex?: number;
  turnPhase?: number;
  textOverride?: string;
}) {
  return createShellBallTextBubbleItem({
    role: "agent",
    text: input.textOverride?.trim() || input.deliveryResult.preview_text.trim() || input.deliveryResult.title,
    bubbleType: "result",
    createdAt: input.createdAt,
    taskId: input.taskId,
    turnIndex: input.turnIndex,
    turnPhase: input.turnPhase,
  });
}

function buildShellBallDeliveryResultKey(taskId: string, deliveryResult: DeliveryResult) {
  return [
    taskId,
    deliveryResult.type,
    deliveryResult.title,
    deliveryResult.preview_text,
    deliveryResult.payload.path ?? "",
    deliveryResult.payload.url ?? "",
  ].join("::");
}

/**
 * Shell-ball only auto-opens formal results that the desktop can immediately
 * hand off to the OS or browser. Bubble-only replies remain in the local chat.
 *
 * @param deliveryResult The formal result returned by task creation or delivery.ready.
 * @returns Whether shell-ball should resolve and execute the formal open flow.
 */
export function shouldAutoOpenShellBallDeliveryResult(
  deliveryResult: DeliveryResult | null | undefined,
): deliveryResult is DeliveryResult {
  if (!deliveryResult) {
    return false;
  }

  switch (deliveryResult.type) {
    case "task_detail":
    case "workspace_document":
    case "open_file":
    case "reveal_in_folder":
    case "result_page":
      return true;
    default:
      return false;
  }
}

function syncShellBallVisualStateFromTaskStatus(status: Parameters<typeof getShellBallVisualStateForTaskStatus>[0]) {
  const currentState = useShellBallStore.getState().visualState;
  const nextState = getShellBallVisualStateForTaskStatus(status, currentState);
  useShellBallStore.getState().setVisualState(nextState);
}

function createShellBallApprovalPendingReply(approvalRequest: ApprovalRequest) {
  const operationName = approvalRequest.operation_name.trim();
  const targetObject = approvalRequest.target_object.trim();
  const reason = approvalRequest.reason.trim();

  if (operationName !== "" && targetObject !== "" && reason !== "") {
    return `Waiting for approval: ${operationName} on ${targetObject}. ${reason}`;
  }

  if (operationName !== "" && targetObject !== "") {
    return `Waiting for approval: ${operationName} on ${targetObject}.`;
  }

  if (reason !== "") {
    return `Waiting for approval: ${reason}`;
  }

  return "Waiting for approval before the task can continue.";
}

/**
 * Pending approval bubbles keep one approval id in shell-ball-local state so
 * the floating surface can submit the formal decision RPC without inventing a
 * second approval object outside the backend contract.
 */
function createShellBallApprovalPendingBubbleItem(input: {
  approvalRequest: ApprovalRequest;
  createdAt: string;
  taskId: string;
  turnIndex?: number;
  turnPhase?: number;
}) {
  const bubbleItem = createShellBallTextBubbleItem({
    role: "agent",
    text: createShellBallApprovalPendingReply(input.approvalRequest),
    bubbleType: "status",
    createdAt: input.createdAt,
    taskId: input.taskId,
    turnIndex: input.turnIndex,
    turnPhase: input.turnPhase,
  });

  return {
    ...bubbleItem,
    desktop: {
      ...bubbleItem.desktop,
      inlineApproval: {
        approvalId: input.approvalRequest.approval_id,
        status: "idle" as const,
      },
    },
  } satisfies ShellBallBubbleItem;
}

function createShellBallApprovalResponseBubbleItem(input: {
  createdAt: string;
  decision: ApprovalDecision;
  response: Awaited<ReturnType<typeof respondSecurityDetailed>>["data"];
  taskId: string;
  turnIndex?: number;
  turnPhase?: number;
}) {
  const bubbleMessage = input.response.bubble_message;
  const bubbleText = bubbleMessage?.text.trim() ?? "";

  if (bubbleMessage !== null && bubbleText !== "") {
    return {
      bubble: {
        ...bubbleMessage,
        task_id: input.taskId,
        hidden: false,
        pinned: false,
      },
      role: "agent",
      desktop: createShellBallBubbleDesktopState({
        turnIndex: input.turnIndex,
        turnPhase: input.turnPhase,
      }),
    } satisfies ShellBallBubbleItem;
  }

  return createShellBallTextBubbleItem({
    role: "agent",
    text: input.decision === "allow_once"
      ? "Approval granted. The task is continuing."
      : "Approval denied. The task will stay paused.",
    bubbleType: "status",
    createdAt: input.createdAt,
    taskId: input.taskId,
    turnIndex: input.turnIndex,
    turnPhase: input.turnPhase,
  });
}

export function createShellBallAgentBubbleItem(
  result: ShellBallInputSubmitResult,
  fallbackCreatedAt: string,
  turnOrder: ShellBallBubbleTurnOrder = {},
) {
  const bubbleMessage = result.bubble_message;
  const bubbleText = bubbleMessage?.text.trim() ?? "";
  const deliveryPreview = result.delivery_result?.preview_text?.trim() ?? "";

  if (bubbleMessage !== null && bubbleText !== "") {
    const bubbleType = bubbleMessage.type;

    if (bubbleType === "result" && result.delivery_result !== null) {
      return createShellBallDeliveryResultBubbleItem({
        taskId: result.task.task_id,
        deliveryResult: result.delivery_result,
        createdAt: bubbleMessage.created_at || fallbackCreatedAt,
        turnIndex: turnOrder.turnIndex,
        turnPhase: turnOrder.turnPhase,
        textOverride: bubbleText,
      });
    }

    return {
      bubble: {
        ...bubbleMessage,
        hidden: false,
        pinned: false,
      },
      role: "agent",
      desktop: createShellBallBubbleDesktopState(turnOrder),
    } satisfies ShellBallBubbleItem;
  }

  if (deliveryPreview !== "") {
    return createShellBallTextBubbleItem({
      role: "agent",
      text: deliveryPreview,
      bubbleType: "result",
      createdAt: result.delivery_result?.payload?.task_id ? fallbackCreatedAt : bubbleMessage?.created_at ?? fallbackCreatedAt,
      taskId: result.task.task_id,
      turnIndex: turnOrder.turnIndex,
      turnPhase: turnOrder.turnPhase,
    });
  }

  return createShellBallTextBubbleItem({
    role: "agent",
    text: "已收到，正在处理。",
    bubbleType: "status",
    createdAt: fallbackCreatedAt,
    taskId: result.task.task_id,
    turnIndex: turnOrder.turnIndex,
    turnPhase: turnOrder.turnPhase,
  });
}

function getShellBallTaskErrorText(error: unknown) {
  if (isRpcChannelUnavailable(error)) {
    return "任务入口未连通，请先确认本地服务可用后再重试。";
  }

  if (error instanceof Error) {
    const message = error.message.trim();
    const normalizedMessage = message.toLowerCase();
    if (normalizedMessage.includes("timed out waiting for local service response") || normalizedMessage.includes("named pipe response wait timed out")) {
      return "任务提交超时：本地服务长时间未返回结果，请重试。";
    }
    if (message !== "") {
      return `任务提交失败：${message}`;
    }
  }

  return "任务提交失败，请稍后重试。";
}

function getShellBallApprovalErrorText(error: unknown) {
  if (isRpcChannelUnavailable(error)) {
    return "Approval response could not reach the local service. Please retry.";
  }

  if (error instanceof Error) {
    const message = error.message.trim();
    if (message !== "") {
      return `Approval response failed: ${message}`;
    }
  }

  return "Approval response failed. Please try again.";
}

// Submission failures stay as local shell-ball status bubbles until the backend
// accepts a formal task.
function createShellBallTaskErrorBubbleItem(input: {
  createdAt: string;
  error: unknown;
  taskId?: string;
  turnIndex?: number;
  turnPhase?: number;
}) {
  return createShellBallTextBubbleItem({
    role: "agent",
    text: getShellBallTaskErrorText(input.error),
    bubbleType: "status",
    createdAt: input.createdAt,
    taskId: input.taskId,
    turnIndex: input.turnIndex,
    turnPhase: input.turnPhase,
  });
}

function createShellBallApprovalErrorBubbleItem(input: {
  createdAt: string;
  error: unknown;
  taskId?: string;
  turnIndex?: number;
  turnPhase?: number;
}) {
  return createShellBallTextBubbleItem({
    role: "agent",
    text: getShellBallApprovalErrorText(input.error),
    bubbleType: "status",
    createdAt: input.createdAt,
    taskId: input.taskId,
    turnIndex: input.turnIndex,
    turnPhase: input.turnPhase,
  });
}

function setShellBallInlineApprovalState(
  items: ShellBallBubbleItem[],
  bubbleId: string,
  inlineApproval?: ShellBallBubbleItem["desktop"]["inlineApproval"],
) {
  return sortShellBallBubbleItemsByTimestamp(
    items.map((item) => {
      if (item.bubble.bubble_id !== bubbleId) {
        return item;
      }

      const desktopState = { ...item.desktop };
      Reflect.deleteProperty(desktopState, "inlineApproval");

      return {
        ...item,
        desktop: inlineApproval === undefined
          ? desktopState
          : {
              ...desktopState,
              inlineApproval: { ...inlineApproval },
            },
      };
    }),
  );
}

export function applyShellBallBubbleAction(
  items: ShellBallBubbleItem[],
  payload: Pick<ShellBallBubbleActionPayload, "action" | "bubbleId">,
): ShellBallBubbleItem[] {
  if (payload.action === "delete") {
    return sortShellBallBubbleItemsByTimestamp(items.filter((item) => item.bubble.bubble_id !== payload.bubbleId));
  }

  if (payload.action === "allow_approval" || payload.action === "deny_approval") {
    return sortShellBallBubbleItemsByTimestamp(items);
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
  const bubbleTurnIndexRef = useRef(0);
  const [bubbleVisibilityPhase, setBubbleVisibilityPhase] = useState<ShellBallBubbleVisibilityPhase>("hidden");
  const [inputHovered, setInputHovered] = useState(false);
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
        inputInteraction: getShellBallInputInteractionState({
          visualState: input.visualState,
          regionActive: input.regionActive,
          inputFocused: input.inputFocused,
          inputHovered,
          hasDraft: input.inputValue.trim() !== "" || (input.pendingFiles ?? []).length > 0,
        }),
      }),
    [bubbleItems, bubbleVisibilityPhase, helpersVisible, input.inputFocused, input.inputValue, input.pendingFiles, input.regionActive, input.visualState, input.voiceHintMode, input.voicePreview, inputHovered],
  );
  const snapshotRef = useRef(snapshot);
  const bubbleItemsRef = useRef(bubbleItems);
  const bubbleVisibilityPhaseRef = useRef<ShellBallBubbleVisibilityPhase>(bubbleVisibilityPhase);
  const visibleBubbleCountRef = useRef(getShellBallVisibleBubbleItems(bubbleItems).length);
  const previousVisibleBubbleCountRef = useRef(visibleBubbleCountRef.current);
  const detachedPinnedBubbleIdsRef = useRef(new Set<string>());
  const deliveryReadyBubbleKeysRef = useRef(new Set<string>());
  const approvalPendingBubbleKeysRef = useRef(new Set<string>());
  // Approval notifications can win the race against `agent.input.submit`.
  // Keep them task-scoped until the submit result binds the formal task id to
  // this shell-ball turn, then replay them into the local bubble timeline.
  const queuedApprovalPendingNotificationsRef = useRef(new Map<string, QueuedApprovalPendingNotification[]>());
  // Fast task status updates can also win that race. Buffer the latest status
  // per task id so shell-ball still reflects waiting_auth/processing as soon as
  // the formal task id becomes known locally.
  const queuedTaskUpdatedNotificationsRef = useRef(new Map<string, QueuedTaskUpdatedNotification>());
  // Delivery notifications can also arrive before the submit response exposes
  // the formal task id locally. Buffer them with the same task-scoped replay
  // path so shell-ball still shows the result bubble and open flow.
  const queuedDeliveryReadyNotificationsRef = useRef(new Map<string, QueuedDeliveryReadyNotification[]>());
  const pollingTaskIdsRef = useRef(new Set<string>());
  const pollingTaskFailureCountRef = useRef(new Map<string, number>());
  const terminalTaskBubbleDeduperRef = useRef(createTerminalTaskBubbleDeduper());
  // Only shell-ball submissions that are still waiting for their formal task id
  // are allowed to buffer approval notifications. This keeps unrelated desktop
  // approvals from lingering in shell-ball memory forever.
  const pendingShellBallTaskRegistrationsRef = useRef(0);
  const autoOpenedDeliveryKeysRef = useRef(new Set<string>());
  const shellBallTaskIdsRef = useRef(new Set<string>());
  const shellBallTaskTurnIndexRef = useRef(new Map<string, number>());
  const activeShellBallTaskIdRef = useRef<string | null>(null);
  const revealBubbleRegionRef = useRef<() => void>(() => {});
  const unbindShellBallTaskRef = useRef<(taskId: string) => void>(() => {});
  const autoOpenShellBallDeliveryResultRef = useRef<(taskId: string, deliveryResult: DeliveryResult | null | undefined) => Promise<void>>(
    () => Promise.resolve(),
  );
  unbindShellBallTaskRef.current = (taskId) => {
    shellBallTaskIdsRef.current.delete(taskId);
    terminalTaskBubbleDeduperRef.current.clearTask(taskId);
    pollingTaskFailureCountRef.current.delete(taskId);
  };
  const syncPinnedBubbleWindowAnchorRef = useRef<(bubbleId: string) => Promise<void>>(() => Promise.resolve());
  const syncAnchoredPinnedBubbleWindowsRef = useRef<() => Promise<void>>(() => Promise.resolve());
  const handleBubbleActionRef = useRef<(payload: ShellBallBubbleActionPayload) => void>(() => {});
  const helperWindowsVisibleRef = useRef(input.helperWindowsVisible ?? true);
  const getBallClientRect = input.getBallClientRect;
  const regionActiveRef = useRef(false);
  const bubbleHoveredRef = useRef(false);
  const inputFocusedRef = useRef(false);
  const inputHoveredRef = useRef(false);
  const bubbleHideDelayTimeoutRef = useRef<ReturnType<typeof globalThis.setTimeout> | null>(null);
  const bubbleHideCompleteTimeoutRef = useRef<ReturnType<typeof globalThis.setTimeout> | null>(null);
  helperWindowsVisibleRef.current = helpersVisible;
  const handlersRef = useRef({
    setInputValue: input.setInputValue,
    onAppendPendingFiles: input.onAppendPendingFiles ?? (() => {}),
    onRemovePendingFile: input.onRemovePendingFile ?? (() => {}),
    onFinalizedSpeechHandled: input.onFinalizedSpeechHandled,
    onRegionEnter: input.onRegionEnter,
    onRegionLeave: input.onRegionLeave,
    onInputHoverChange: input.onInputHoverChange,
    onInputFocusChange: input.onInputFocusChange,
    onSubmitText: input.onSubmitText,
    onSubmitVoiceText: input.onSubmitVoiceText ?? defaultSubmitVoiceText,
    getCurrentConversationSessionId: input.getCurrentConversationSessionId,
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
    onInputHoverChange: input.onInputHoverChange,
    onInputFocusChange: input.onInputFocusChange,
    onSubmitText: input.onSubmitText,
    onSubmitVoiceText: input.onSubmitVoiceText ?? defaultSubmitVoiceText,
    getCurrentConversationSessionId: input.getCurrentConversationSessionId,
    onAttachFile: input.onAttachFile,
    onPrimaryClick: input.onPrimaryClick,
  };

  const allocateBubbleTurnIndex = useCallback(() => {
    bubbleTurnIndexRef.current += 1;
    return bubbleTurnIndexRef.current;
  }, []);

  const bindTaskToBubbleTurn = useCallback((taskId: string, turnIndex: number) => {
    shellBallTaskTurnIndexRef.current.set(taskId, turnIndex);
  }, []);

  const getTaskBubbleTurnIndex = useCallback((taskId: string) => {
    return shellBallTaskTurnIndexRef.current.get(taskId);
  }, []);

  const appendApprovalPendingBubble = useCallback((input: QueuedApprovalPendingNotification) => {
    const bubbleKey = `${input.taskId}:${input.approvalRequest.approval_id}`;
    if (approvalPendingBubbleKeysRef.current.has(bubbleKey)) {
      return;
    }

    approvalPendingBubbleKeysRef.current.add(bubbleKey);

    if (activeShellBallTaskIdRef.current === input.taskId) {
      syncShellBallVisualStateFromTaskStatus("waiting_auth");
    }

    const nextTurnIndex = shellBallTaskTurnIndexRef.current.get(input.taskId) ?? (() => {
      bubbleTurnIndexRef.current += 1;
      return bubbleTurnIndexRef.current;
    })();
    shellBallTaskTurnIndexRef.current.set(input.taskId, nextTurnIndex);

    setBubbleItems((currentItems) =>
      sortShellBallBubbleItemsByTimestamp([
        ...currentItems,
        createShellBallApprovalPendingBubbleItem({
          approvalRequest: input.approvalRequest,
          createdAt: new Date().toISOString(),
          taskId: input.taskId,
          turnIndex: nextTurnIndex,
          turnPhase: 2,
        }),
      ]),
    );
    revealBubbleRegionRef.current();
  }, []);

  const appendDeliveryReadyBubble = useCallback((input: QueuedDeliveryReadyNotification) => {
    const bubbleText = input.deliveryResult.preview_text.trim() || input.deliveryResult.title;
    const bubbleKey = `${input.taskId}:${input.deliveryResult.type}:${bubbleText}`;

    if (deliveryReadyBubbleKeysRef.current.has(bubbleKey)) {
      return;
    }

    deliveryReadyBubbleKeysRef.current.add(bubbleKey);

    setBubbleItems((currentItems) => {
      if (
        currentItems.some(
          (item) =>
            item.bubble.task_id === input.taskId &&
            item.bubble.type === "result" &&
            item.role === "agent",
        )
      ) {
        return currentItems;
      }

      const turnIndex = getTaskBubbleTurnIndex(input.taskId) ?? allocateBubbleTurnIndex();
      bindTaskToBubbleTurn(input.taskId, turnIndex);

      return sortShellBallBubbleItemsByTimestamp([
        ...currentItems,
        createShellBallDeliveryResultBubbleItem({
          createdAt: new Date().toISOString(),
          deliveryResult: input.deliveryResult,
          taskId: input.taskId,
          turnIndex,
          turnPhase: 2,
        }),
      ]);
    });
    revealBubbleRegionRef.current();
    void autoOpenShellBallDeliveryResultRef.current(input.taskId, input.deliveryResult);
  }, [allocateBubbleTurnIndex, bindTaskToBubbleTurn, getTaskBubbleTurnIndex]);

  const registerShellBallTask = useCallback((
    taskId: string,
    turnIndex?: number,
    fallbackStatus?: QueuedTaskUpdatedNotification["status"],
  ) => {
    shellBallTaskIdsRef.current.add(taskId);
    activeShellBallTaskIdRef.current = taskId;

    if (turnIndex !== undefined) {
      shellBallTaskTurnIndexRef.current.set(taskId, turnIndex);
    }

    const queuedTaskUpdatedNotification = queuedTaskUpdatedNotificationsRef.current.get(taskId);
    queuedTaskUpdatedNotificationsRef.current.delete(taskId);

    if (queuedTaskUpdatedNotification !== undefined) {
      syncShellBallVisualStateFromTaskStatus(queuedTaskUpdatedNotification.status);
    } else if (fallbackStatus !== undefined) {
      syncShellBallVisualStateFromTaskStatus(fallbackStatus);
    }

    const queuedNotifications = queuedApprovalPendingNotificationsRef.current.get(taskId) ?? [];
    queuedApprovalPendingNotificationsRef.current.delete(taskId);

    queuedNotifications.forEach((notification) => {
      appendApprovalPendingBubble(notification);
    });

    const queuedDeliveryNotifications = queuedDeliveryReadyNotificationsRef.current.get(taskId) ?? [];
    queuedDeliveryReadyNotificationsRef.current.delete(taskId);

    queuedDeliveryNotifications.forEach((notification) => {
      appendDeliveryReadyBubble(notification);
    });
  }, [appendApprovalPendingBubble, appendDeliveryReadyBubble]);

  const beginPendingShellBallTaskRegistration = useCallback(() => {
    pendingShellBallTaskRegistrationsRef.current += 1;
    let completed = false;

    return () => {
      if (completed) {
        return;
      }

      completed = true;
      pendingShellBallTaskRegistrationsRef.current = Math.max(0, pendingShellBallTaskRegistrationsRef.current - 1);

      if (pendingShellBallTaskRegistrationsRef.current === 0) {
        queuedApprovalPendingNotificationsRef.current.clear();
        queuedTaskUpdatedNotificationsRef.current.clear();
        queuedDeliveryReadyNotificationsRef.current.clear();
      }
    };
  }, []);

  const clearBubbleVisibilityTimers = useCallback(() => {
    if (bubbleHideDelayTimeoutRef.current !== null) {
      clearShellBallTimeout(bubbleHideDelayTimeoutRef.current);
      bubbleHideDelayTimeoutRef.current = null;
    }

    if (bubbleHideCompleteTimeoutRef.current !== null) {
      clearShellBallTimeout(bubbleHideCompleteTimeoutRef.current);
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

  const appendShellBallAutoOpenFeedback = useCallback((input: {
    taskId: string;
    text: string;
  }): void => {
    const turnIndex = getTaskBubbleTurnIndex(input.taskId) ?? allocateBubbleTurnIndex();
    bindTaskToBubbleTurn(input.taskId, turnIndex);

    setBubbleItems((currentItems) =>
      sortShellBallBubbleItemsByTimestamp([
        ...currentItems,
        createShellBallTextBubbleItem({
          role: "agent",
          text: input.text,
          bubbleType: "status",
          createdAt: new Date().toISOString(),
          taskId: input.taskId,
          turnIndex,
          turnPhase: 3,
        }),
      ]),
    );
    revealBubbleRegion();
  }, [allocateBubbleTurnIndex, bindTaskToBubbleTurn, getTaskBubbleTurnIndex, revealBubbleRegion]);

  /**
   * Shell-ball only resolves and executes the formal delivery-open flow after
   * a task has already produced a formal delivery result. The actual open
   * action still comes from `agent.delivery.open`.
   */
  const autoOpenShellBallDeliveryResult = useCallback(async (taskId: string, deliveryResult: DeliveryResult | null | undefined): Promise<void> => {
    if (!shouldAutoOpenShellBallDeliveryResult(deliveryResult)) {
      return;
    }

    const deliveryKey = buildShellBallDeliveryResultKey(taskId, deliveryResult);

    if (autoOpenedDeliveryKeysRef.current.has(deliveryKey)) {
      return;
    }

    autoOpenedDeliveryKeysRef.current.add(deliveryKey);

    try {
      const taskOutputService = await loadShellBallTaskOutputService();
      const openResult = await taskOutputService.openTaskDeliveryForTask(taskId, undefined, "rpc");
      const plan = taskOutputService.resolveTaskOpenExecutionPlan(openResult);
      const feedback = await taskOutputService.performTaskOpenExecution(plan, {
        onOpenTaskDetail: async ({ taskId: resolvedTaskId }) => {
          await requestDashboardTaskDetailOpen(resolvedTaskId);
          return plan.feedback;
        },
      });

      if (plan.mode === "copy_path" || feedback !== plan.feedback) {
        appendShellBallAutoOpenFeedback({
          taskId,
          text: feedback,
        });
      }
    } catch (error) {
      autoOpenedDeliveryKeysRef.current.delete(deliveryKey);
      console.warn("shell-ball delivery auto-open failed", error);
      appendShellBallAutoOpenFeedback({
        taskId,
        text: "结果已生成，但自动打开失败，请从任务详情里重新打开。",
      });
    }
  }, [appendShellBallAutoOpenFeedback]);
  autoOpenShellBallDeliveryResultRef.current = autoOpenShellBallDeliveryResult;

  const scheduleBubbleRegionHide = useCallback(() => {
    clearBubbleVisibilityTimers();

    if (!helperWindowsVisibleRef.current || visibleBubbleCountRef.current === 0) {
      applyBubbleVisibilityPhase("hidden");
      return;
    }

    if (regionActiveRef.current || bubbleHoveredRef.current || inputFocusedRef.current || inputHoveredRef.current) {
      applyBubbleVisibilityPhase("visible");
      return;
    }

    bubbleHideDelayTimeoutRef.current = scheduleShellBallTimeout(() => {
      if (!helperWindowsVisibleRef.current || visibleBubbleCountRef.current === 0) {
        applyBubbleVisibilityPhase("hidden");
        return;
      }

      if (regionActiveRef.current || bubbleHoveredRef.current || inputFocusedRef.current || inputHoveredRef.current) {
        applyBubbleVisibilityPhase("visible");
        return;
      }

      applyBubbleVisibilityPhase("fading");
      bubbleHideCompleteTimeoutRef.current = scheduleShellBallTimeout(() => {
        if (regionActiveRef.current || bubbleHoveredRef.current || inputFocusedRef.current || inputHoveredRef.current) {
          applyBubbleVisibilityPhase("visible");
          return;
        }

        applyBubbleVisibilityPhase("hidden");
      }, SHELL_BALL_BUBBLE_FADE_DURATION_MS);
    }, SHELL_BALL_BUBBLE_HIDE_DELAY_MS);
  }, [applyBubbleVisibilityPhase, clearBubbleVisibilityTimers]);

  /**
   * Desktop file drops should reuse the same pending attachment queue as the
   * picker so the user can review files and send them explicitly.
   */
  const handleDroppedFiles = useCallback(async (paths: string[]) => {
    const normalizedPaths = paths.map((path) => path.trim()).filter(Boolean);

    if (normalizedPaths.length === 0) {
      return;
    }

    handlersRef.current.onAppendPendingFiles(normalizedPaths);
  }, []);

  /**
   * Selected-text intake should enter the same formal task pipeline as other
   * shell-ball entries. The orb click is only the acceptance gesture; the
   * actual selected content must continue through `agent.task.start`.
   */
  const handleSelectedTextPrompt = useCallback(async (selection: ShellBallSelectionSnapshot | string) => {
    const text = typeof selection === "string" ? selection : selection.text;
    const pageContext = typeof selection === "string" ? undefined : selection.page_context;
    const normalizedText = text.trim();
    const createdAt = new Date().toISOString();
    const turnIndex = allocateBubbleTurnIndex();
    const previewBubbleItem = createShellBallTextBubbleItem({
      role: "agent",
      text: createShellBallSelectedTextPreview(text),
      bubbleType: "status",
      createdAt,
      turnIndex,
      turnPhase: 0,
    });

    if (normalizedText === "") {
      setBubbleItems((currentItems) =>
        sortShellBallBubbleItemsByTimestamp([
          ...currentItems,
          previewBubbleItem,
        ]),
      );
      revealBubbleRegion();
      return;
    }

    const pendingAgentBubbleItem = createShellBallAgentLoadingBubbleItem({
      createdAt: new Date().toISOString(),
      turnIndex,
      turnPhase: 1,
    });

    setBubbleItems((currentItems) =>
      sortShellBallBubbleItemsByTimestamp([
        ...currentItems,
        previewBubbleItem,
        pendingAgentBubbleItem,
      ]),
    );
    revealBubbleRegion();

    const finishPendingTaskRegistration = beginPendingShellBallTaskRegistration();

    try {
      const result = await startTaskFromSelectedText(normalizedText, {
        delivery: {
          preferred: "bubble",
          fallback: "task_detail",
        },
        pageContext,
        sessionId: handlersRef.current.getCurrentConversationSessionId?.(),
        source: "floating_ball",
      });

      if (!isShellBallInputSubmitResult(result)) {
        setBubbleItems((currentItems) =>
          replaceShellBallPendingBubble(currentItems, pendingAgentBubbleItem.bubble.bubble_id),
        );
        return;
      }

      registerShellBallTask(result.task.task_id, turnIndex, result.task.status);
      setBubbleItems((currentItems) =>
        replaceShellBallPendingBubble(
          currentItems,
          pendingAgentBubbleItem.bubble.bubble_id,
          createShellBallAgentBubbleItem(result, new Date().toISOString(), {
            turnIndex,
            turnPhase: 1,
          }),
        ),
      );
      revealBubbleRegion();
      void autoOpenShellBallDeliveryResult(result.task.task_id, result.delivery_result);
    } catch (error) {
      console.warn("shell-ball selected text submit failed", error);
      setBubbleItems((currentItems) =>
        replaceShellBallPendingBubble(
          currentItems,
          pendingAgentBubbleItem.bubble.bubble_id,
          createShellBallTaskErrorBubbleItem({
            createdAt: new Date().toISOString(),
            error,
            turnIndex,
            turnPhase: 1,
          }),
        ),
      );
      revealBubbleRegion();
    } finally {
      finishPendingTaskRegistration();
    }
  }, [allocateBubbleTurnIndex, autoOpenShellBallDeliveryResult, beginPendingShellBallTaskRegistration, registerShellBallTask, revealBubbleRegion]);

  /**
   * Submits clipboard text through the formal shell-ball text input path while
   * preserving the local bubble turn ordering used by hover-input submissions.
   *
   * @param text Clipboard text captured by the desktop clipboard prompt.
   * @returns A promise that resolves after the bubble timeline has been updated.
   */
  const handleClipboardPrompt = useCallback(async (text: string) => {
    const normalizedText = text.trim();
    if (normalizedText === "") {
      return;
    }

    const createdAt = new Date().toISOString();
    const turnIndex = allocateBubbleTurnIndex();
    const userBubbleItem = createShellBallTextBubbleItem({
      role: "user",
      text: normalizedText,
      bubbleType: "result",
      createdAt,
      turnIndex,
      turnPhase: 0,
    });

    setBubbleItems((currentItems) =>
      sortShellBallBubbleItemsByTimestamp([
        ...currentItems,
        userBubbleItem,
      ]),
    );
    revealBubbleRegion();

    const finishPendingTaskRegistration = beginPendingShellBallTaskRegistration();

    try {
      const result = await submitTextInput({
        text: normalizedText,
        source: "floating_ball",
        trigger: "hover_text_input",
        inputMode: "text",
        sessionId: handlersRef.current.getCurrentConversationSessionId?.(),
        options: {
          confirm_required: false,
          preferred_delivery: "bubble",
        },
      });

      if (!isShellBallInputSubmitResult(result)) {
        return;
      }

      registerShellBallTask(result.task.task_id, turnIndex, result.task.status);
      setBubbleItems((currentItems) => {
        const nextItems = currentItems.map((item) =>
          item.bubble.bubble_id === userBubbleItem.bubble.bubble_id
            ? {
                ...item,
                bubble: {
                  ...item.bubble,
                  task_id: result.task.task_id,
                },
              }
            : item,
        );

        return sortShellBallBubbleItemsByTimestamp([
          ...nextItems,
          createShellBallAgentBubbleItem(result, new Date().toISOString(), {
            turnIndex,
            turnPhase: 1,
          }),
        ]);
      });
      revealBubbleRegion();
      void autoOpenShellBallDeliveryResult(result.task.task_id, result.delivery_result);
    } catch (error) {
      console.warn("shell-ball clipboard prompt submit failed", error);
      setBubbleItems((currentItems) =>
        sortShellBallBubbleItemsByTimestamp([
          ...currentItems,
          createShellBallTextBubbleItem({
            role: "agent",
            text: "Clipboard request failed.",
            bubbleType: "status",
            createdAt: new Date().toISOString(),
            turnIndex,
            turnPhase: 1,
          }),
        ]),
      );
      revealBubbleRegion();
    } finally {
      finishPendingTaskRegistration();
    }
  }, [allocateBubbleTurnIndex, autoOpenShellBallDeliveryResult, beginPendingShellBallTaskRegistration, registerShellBallTask, revealBubbleRegion]);

  /**
   * Shortcut keywords such as `截屏` and `窗口` still enter the formal task
   * pipeline. The shell-ball only keeps the local bubble ordering while the
   * backend remains the source of truth for authorization and evidence capture.
   *
   * @param input Shortcut metadata and the explicit visual context hints to send.
   * @returns A promise that resolves after the shell-ball bubble timeline updates.
   */
  const submitShellBallScreenShortcut = useCallback(async (input: {
    commandText: string;
    promptText: string;
    failureText: string;
    context: InputContext;
  }) => {
    const createdAt = new Date().toISOString();
    const turnIndex = allocateBubbleTurnIndex();
    const userBubbleItem = createShellBallTextBubbleItem({
      role: "user",
      text: input.commandText,
      bubbleType: "result",
      createdAt,
      turnIndex,
      turnPhase: 0,
    });
    const pendingAgentBubbleItem = createShellBallAgentLoadingBubbleItem({
      createdAt: new Date().toISOString(),
      turnIndex,
      turnPhase: 1,
    });

    setBubbleItems((currentItems) =>
      sortShellBallBubbleItemsByTimestamp([
        ...currentItems,
        userBubbleItem,
        pendingAgentBubbleItem,
      ]),
    );
    revealBubbleRegion();

    const finishPendingTaskRegistration = beginPendingShellBallTaskRegistration();

    try {
      const result = await submitTextInput({
        text: input.promptText,
        source: "floating_ball",
        trigger: "hover_text_input",
        inputMode: "text",
        context: input.context,
        sessionId: handlersRef.current.getCurrentConversationSessionId?.(),
        options: {
          confirm_required: false,
          preferred_delivery: "bubble",
        },
      });

      if (!isShellBallInputSubmitResult(result)) {
        setBubbleItems((currentItems) =>
          replaceShellBallPendingBubble(currentItems, pendingAgentBubbleItem.bubble.bubble_id),
        );
        return;
      }

      registerShellBallTask(result.task.task_id, turnIndex, result.task.status);
      setBubbleItems((currentItems) => {
        const nextItems = currentItems.map((item) =>
          item.bubble.bubble_id === userBubbleItem.bubble.bubble_id
            ? {
                ...item,
                bubble: {
                  ...item.bubble,
                  task_id: result.task.task_id,
                },
              }
            : item,
        );

        return replaceShellBallPendingBubble(
          nextItems,
          pendingAgentBubbleItem.bubble.bubble_id,
          createShellBallAgentBubbleItem(result, new Date().toISOString(), {
            turnIndex,
            turnPhase: 1,
          }),
        );
      });
      revealBubbleRegion();
      void autoOpenShellBallDeliveryResult(result.task.task_id, result.delivery_result);
    } catch (error) {
      console.warn("shell-ball screen shortcut submit failed", error);
      setBubbleItems((currentItems) =>
        replaceShellBallPendingBubble(
          currentItems,
          pendingAgentBubbleItem.bubble.bubble_id,
          createShellBallTextBubbleItem({
            role: "agent",
            text: input.failureText,
            bubbleType: "status",
            createdAt: new Date().toISOString(),
            turnIndex,
            turnPhase: 1,
          }),
        ),
      );
      revealBubbleRegion();
    } finally {
      finishPendingTaskRegistration();
      handlersRef.current.setInputValue("");
      handlersRef.current.onInputFocusChange(false);
      revealBubbleRegion();
    }
  }, [allocateBubbleTurnIndex, autoOpenShellBallDeliveryResult, beginPendingShellBallTaskRegistration, registerShellBallTask, revealBubbleRegion]);

  /**
   * Maps the shell-ball screenshot keyword to the formal visual-task pipeline.
   *
   * @returns A promise that resolves after the task shortcut bubble turn updates.
   */
  const handleScreenshotPrompt = useCallback(async () => {
    await submitShellBallScreenShortcut({
      commandText: SHELL_BALL_SCREENSHOT_COMMAND,
      promptText: SHELL_BALL_SCREENSHOT_PROMPT_TEXT,
      failureText: "Screen inspection request failed.",
      context: {
        screen: {
          summary: SHELL_BALL_SCREENSHOT_SUMMARY,
        },
        behavior: {
          last_action: "review_screen",
        },
      },
    });
  }, [submitShellBallScreenShortcut]);

  /**
   * Maps the shell-ball window keyword to the formal visual-task pipeline.
   *
   * @returns A promise that resolves after the task shortcut bubble turn updates.
   */
  const handleWindowPrompt = useCallback(async () => {
    await submitShellBallScreenShortcut({
      commandText: SHELL_BALL_WINDOW_COMMAND,
      promptText: SHELL_BALL_WINDOW_PROMPT_TEXT,
      failureText: "Window inspection request failed.",
      context: {
        screen: {
          summary: SHELL_BALL_WINDOW_SUMMARY,
        },
        behavior: {
          last_action: "review_window",
        },
      },
    });
  }, [submitShellBallScreenShortcut]);

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

    if (regionActiveRef.current || bubbleHoveredRef.current || inputFocusedRef.current || inputHoveredRef.current) {
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

    if (regionActiveRef.current || bubbleHoveredRef.current || inputFocusedRef.current || inputHoveredRef.current) {
      revealBubbleRegion();
      return;
    }

    scheduleBubbleRegionHide();
  }, [applyBubbleVisibilityPhase, clearBubbleVisibilityTimers, helpersVisible, revealBubbleRegion, scheduleBubbleRegionHide]);

  const handleCoordinatorRegionEnter = useCallback(() => {
    regionActiveRef.current = true;
    revealBubbleRegion();
    handlersRef.current.onRegionEnter();
  }, [revealBubbleRegion]);

  const handleCoordinatorRegionLeave = useCallback(() => {
    regionActiveRef.current = false;
    scheduleBubbleRegionHide();
    handlersRef.current.onRegionLeave();
  }, [scheduleBubbleRegionHide]);

  const syncPinnedBubbleWindowAnchor = useCallback(async (bubbleId: string) => {
    const currentWindow = getCurrentWindow();

    if (currentWindow.label !== shellBallWindowLabels.ball) {
      return;
    }

    if (detachedPinnedBubbleIdsRef.current.has(bubbleId)) {
      return;
    }

    const bubbleItem = bubbleItemsRef.current.find((item) => item.bubble.bubble_id === bubbleId && item.bubble.pinned);

    if (bubbleItem === undefined) {
      return;
    }

    const outerPosition = await currentWindow.outerPosition();
    const scaleFactor = await currentWindow.scaleFactor();
    const logicalPosition = outerPosition.toLogical(scaleFactor);
    const ballClientRect = getBallClientRect?.();
    const ballFrame = ballClientRect === null || ballClientRect === undefined
      ? (() => {
          const outerSize = currentWindow.outerSize();
          return outerSize.then((size) => {
            const logicalSize = size.toLogical(scaleFactor);

            return {
              x: logicalPosition.x,
              y: logicalPosition.y,
              width: logicalSize.width,
              height: logicalSize.height,
            };
          });
        })()
      : Promise.resolve({
          x: logicalPosition.x + ballClientRect.left,
          y: logicalPosition.y + ballClientRect.top,
          width: ballClientRect.width,
          height: ballClientRect.height,
        });
    const bubbleAnchor = getShellBallBubbleAnchor({
      ballFrame: await ballFrame,
      helperFrame: SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME,
    });

    await openShellBallPinnedBubbleWindow({
      bubbleId,
      position: getShellBallPinnedBubbleWindowAnchor({ bubbleAnchor }),
      size: SHELL_BALL_PINNED_BUBBLE_WINDOW_FRAME,
    });
  }, [getBallClientRect]);

  const syncAnchoredPinnedBubbleWindows = useCallback(async () => {
    await Promise.all(
      bubbleItemsRef.current
        .filter((item) => item.bubble.pinned)
        .map((item) => syncPinnedBubbleWindowAnchor(item.bubble.bubble_id)),
    );
  }, [syncPinnedBubbleWindowAnchor]);

  revealBubbleRegionRef.current = revealBubbleRegion;
  syncPinnedBubbleWindowAnchorRef.current = syncPinnedBubbleWindowAnchor;
  syncAnchoredPinnedBubbleWindowsRef.current = syncAnchoredPinnedBubbleWindows;

  const handleCoordinatorInputFocusChange = useCallback((focused: boolean) => {
    inputFocusedRef.current = focused;

    if (focused) {
      revealBubbleRegion();
    } else if (!regionActiveRef.current && !bubbleHoveredRef.current && !inputHoveredRef.current) {
      scheduleBubbleRegionHide();
    }

    handlersRef.current.onInputFocusChange(focused);
  }, [revealBubbleRegion, scheduleBubbleRegionHide]);

  const handleCoordinatorInputHoverChange = useCallback((active: boolean) => {
    inputHoveredRef.current = active;
    setInputHovered(active);

    if (active) {
      revealBubbleRegion();
    } else if (!regionActiveRef.current && !bubbleHoveredRef.current && !inputFocusedRef.current) {
      scheduleBubbleRegionHide();
    }

    handlersRef.current.onInputHoverChange(active);
  }, [revealBubbleRegion, scheduleBubbleRegionHide]);

  const handleCoordinatorBubbleHoverChange = useCallback((active: boolean) => {
    bubbleHoveredRef.current = active;

    if (active) {
      revealBubbleRegion();
      return;
    }

    if (!regionActiveRef.current && !inputFocusedRef.current && !inputHoveredRef.current) {
      scheduleBubbleRegionHide();
    }
  }, [revealBubbleRegion, scheduleBubbleRegionHide]);

  useEffect(() => {
    if (snapshot.visibility.input) {
      return;
    }

    inputHoveredRef.current = false;
    setInputHovered(false);
  }, [snapshot.visibility.input]);

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

  const handleInlineApprovalBubbleAction = useCallback(async (payload: ShellBallBubbleActionPayload) => {
    const bubbleItem = bubbleItemsRef.current.find((item) => item.bubble.bubble_id === payload.bubbleId);
    const inlineApproval = bubbleItem?.desktop.inlineApproval;
    const taskId = bubbleItem?.bubble.task_id ?? "";

    if (bubbleItem === undefined || inlineApproval === undefined || inlineApproval.status === "submitting" || taskId === "") {
      return;
    }

    const decision: ApprovalDecision = payload.action === "allow_approval" ? "allow_once" : "deny_once";
    const turnIndex = bubbleItem.desktop.turnIndex ?? getTaskBubbleTurnIndex(taskId) ?? allocateBubbleTurnIndex();

    bindTaskToBubbleTurn(taskId, turnIndex);
    setBubbleItems((currentItems) =>
      setShellBallInlineApprovalState(currentItems, payload.bubbleId, {
        ...inlineApproval,
        status: "submitting",
        pendingDecision: decision,
      }),
    );
    revealBubbleRegion();

    try {
      const response = await respondSecurityDetailed({
        request_meta: createShellBallRequestMeta(),
        task_id: taskId,
        approval_id: inlineApproval.approvalId,
        decision,
        remember_rule: false,
      });

      const shouldFallbackToResponseStatus = !shellBallTaskIdsRef.current.has(response.data.task.task_id);

      // Live task subscriptions remain authoritative after the task has been
      // registered. The RPC response only supplies a fallback status for the
      // narrow first-registration path where no subscription update exists yet.
      registerShellBallTask(
        response.data.task.task_id,
        turnIndex,
        shouldFallbackToResponseStatus ? response.data.task.status : undefined,
      );

      setBubbleItems((currentItems) =>
        replaceShellBallPendingBubble(
          currentItems,
          payload.bubbleId,
          createShellBallApprovalResponseBubbleItem({
            createdAt: new Date().toISOString(),
            decision,
            response: response.data,
            taskId: response.data.task.task_id,
            turnIndex,
            turnPhase: 2,
          }),
        ),
      );
      revealBubbleRegion();
    } catch (error) {
      console.warn("shell-ball approval response failed", error);
      setBubbleItems((currentItems) => {
        const resetItems = setShellBallInlineApprovalState(currentItems, payload.bubbleId, {
          approvalId: inlineApproval.approvalId,
          status: "idle",
        });

        return sortShellBallBubbleItemsByTimestamp([
          ...resetItems,
          createShellBallApprovalErrorBubbleItem({
            createdAt: new Date().toISOString(),
            error,
            taskId,
            turnIndex,
            turnPhase: 3,
          }),
        ]);
      });
      revealBubbleRegion();
    }
  }, [allocateBubbleTurnIndex, bindTaskToBubbleTurn, getTaskBubbleTurnIndex, registerShellBallTask, revealBubbleRegion]);

  const handleBubbleAction = useCallback((payload: ShellBallBubbleActionPayload) => {
    if (payload.action === "allow_approval" || payload.action === "deny_approval") {
      void handleInlineApprovalBubbleAction(payload);
      return;
    }

    setBubbleItems((currentItems) => applyShellBallBubbleAction(currentItems, payload));

    if (payload.action === "pin") {
      detachedPinnedBubbleIdsRef.current.delete(payload.bubbleId);
      void syncPinnedBubbleWindowAnchor(payload.bubbleId);
      return;
    }

    detachedPinnedBubbleIdsRef.current.delete(payload.bubbleId);
    void closeShellBallPinnedBubbleWindow(payload.bubbleId);
  }, [handleInlineApprovalBubbleAction, syncPinnedBubbleWindowAnchor]);

  handleBubbleActionRef.current = handleBubbleAction;

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
    const turnIndex = allocateBubbleTurnIndex();
    const userBubbleItem = createShellBallFinalizedSpeechBubbleItem({
      text: finalizedSpeechPayload,
      sequence: appendedVoiceBubbleSequenceRef.current,
      createdAt: new Date().toISOString(),
      turnIndex,
      turnPhase: 0,
    });
    const pendingAgentBubbleItem = createShellBallAgentLoadingBubbleItem({
      createdAt: new Date().toISOString(),
      turnIndex,
      turnPhase: 1,
    });

    setBubbleItems((currentItems) =>
      sortShellBallBubbleItemsByTimestamp([
        ...currentItems,
        userBubbleItem,
        pendingAgentBubbleItem,
      ]),
    );
    revealBubbleRegion();

    /**
     * Voice submissions should reuse the same task/bubble/delivery pipeline as
     * hover-text submissions so the shell-ball can track task detail routing and
     * formal delivery auto-open consistently.
     */
    const finishPendingTaskRegistration = beginPendingShellBallTaskRegistration();
    const pendingBubbleId = pendingAgentBubbleItem.bubble.bubble_id;
    const stallTimeout = scheduleShellBallTimeout(() => {
      setBubbleItems((currentItems) => markShellBallPendingBubbleAsStalled(currentItems, pendingBubbleId));
      revealBubbleRegionRef.current();
    }, SHELL_BALL_SUBMIT_STALL_NOTICE_MS);

    void Promise.resolve(handlersRef.current.onSubmitVoiceText(finalizedSpeechPayload))
      .then((result) => {
        if (!isShellBallInputSubmitResult(result)) {
          setBubbleItems((currentItems) =>
            replaceShellBallPendingBubble(currentItems, pendingAgentBubbleItem.bubble.bubble_id),
          );
          return;
        }

        registerShellBallTask(result.task.task_id, turnIndex, result.task.status);
        setBubbleItems((currentItems) => {
          const nextItems = currentItems.map((item) =>
            item.bubble.bubble_id === userBubbleItem.bubble.bubble_id
              ? {
                  ...item,
                  bubble: {
                    ...item.bubble,
                    task_id: result.task.task_id,
                  },
                }
              : item,
          );

          return replaceShellBallPendingBubble(
            nextItems,
            pendingAgentBubbleItem.bubble.bubble_id,
            createShellBallAgentBubbleItem(result, new Date().toISOString(), {
              turnIndex,
              turnPhase: 1,
            }),
          );
        });
        revealBubbleRegion();
        void autoOpenShellBallDeliveryResult(result.task.task_id, result.delivery_result);
      })
      .catch((error) => {
        console.warn("shell-ball voice submit failed", error);
        setBubbleItems((currentItems) =>
          replaceShellBallPendingBubble(
            currentItems,
            pendingAgentBubbleItem.bubble.bubble_id,
            createShellBallTaskErrorBubbleItem({
              createdAt: new Date().toISOString(),
              error,
              turnIndex,
              turnPhase: 1,
            }),
          ),
        );
        revealBubbleRegion();
      })
      .finally(() => {
        clearShellBallTimeout(stallTimeout);
        finishPendingTaskRegistration();
        handlersRef.current.onFinalizedSpeechHandled();
      });
  }, [allocateBubbleTurnIndex, autoOpenShellBallDeliveryResult, beginPendingShellBallTaskRegistration, input.finalizedSpeechPayload, registerShellBallTask, revealBubbleRegion]);

  useEffect(() => {
    const clearTaskSubscription = subscribeTaskUpdated((payload) => {
      if (!shellBallTaskIdsRef.current.has(payload.task_id)) {
        if (pendingShellBallTaskRegistrationsRef.current === 0) {
          return;
        }

        queuedTaskUpdatedNotificationsRef.current.set(payload.task_id, payload);
        return;
      }

      if (activeShellBallTaskIdRef.current === payload.task_id) {
        syncShellBallVisualStateFromTaskStatus(payload.status);
      }
    });

    const clearApprovalSubscription = subscribeApprovalPending((payload) => {
      if (!shellBallTaskIdsRef.current.has(payload.task_id)) {
        if (pendingShellBallTaskRegistrationsRef.current === 0) {
          return;
        }

        const queuedNotifications = queuedApprovalPendingNotificationsRef.current.get(payload.task_id) ?? [];
        queuedNotifications.push({
          approvalRequest: payload.approval_request,
          taskId: payload.task_id,
        });
        queuedApprovalPendingNotificationsRef.current.set(payload.task_id, queuedNotifications);
        return;
      }

      appendApprovalPendingBubble({
        approvalRequest: payload.approval_request,
        taskId: payload.task_id,
      });
    });

    return () => {
      clearTaskSubscription();
      clearApprovalSubscription();
    };
  }, [appendApprovalPendingBubble]);

  useEffect(() => {
    return subscribeDeliveryReady((payload) => {
      if (!shellBallTaskIdsRef.current.has(payload.task_id)) {
        if (pendingShellBallTaskRegistrationsRef.current === 0) {
          return;
        }

        const queuedNotifications = queuedDeliveryReadyNotificationsRef.current.get(payload.task_id) ?? [];
        queuedNotifications.push({
          deliveryResult: payload.delivery_result,
          taskId: payload.task_id,
        });
        queuedDeliveryReadyNotificationsRef.current.set(payload.task_id, queuedNotifications);
        return;
      }

      appendDeliveryReadyBubble({
        deliveryResult: payload.delivery_result,
        taskId: payload.task_id,
      });
    });
  }, [appendDeliveryReadyBubble]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    let disposed = false;
    const pollingTaskIds = pollingTaskIdsRef.current;
    const pollingTaskFailureCount = pollingTaskFailureCountRef.current;

    const pollShellBallTasks = () => {
      for (const taskId of shellBallTaskIdsRef.current) {
        if (pollingTaskIds.has(taskId)) {
          continue;
        }

        pollingTaskIds.add(taskId);
        void getTaskDetail({
          request_meta: createShellBallRequestMeta(),
          task_id: taskId,
        })
          .then((detail) => {
            if (disposed) {
              return;
            }

            syncShellBallVisualStateFromTaskStatus(detail.task.status);

            if (detail.approval_request) {
              appendApprovalPendingBubble({
                approvalRequest: detail.approval_request,
                taskId: detail.task.task_id,
              });
            }

            pollingTaskFailureCount.delete(detail.task.task_id);

            if (detail.delivery_result) {
              appendDeliveryReadyBubble({
                deliveryResult: detail.delivery_result,
                taskId: detail.task.task_id,
              });
            }

            if (isTerminalShellBallTaskStatus(detail.task.status)) {
              if (detail.delivery_result == null) {
                if (terminalTaskBubbleDeduperRef.current.markAndCheckShouldDisplay(detail.task.task_id, detail.task.status)) {
                  const turnIndex = getTaskBubbleTurnIndex(detail.task.task_id) ?? allocateBubbleTurnIndex();
                  bindTaskToBubbleTurn(detail.task.task_id, turnIndex);
                  setBubbleItems((currentItems) =>
                    sortShellBallBubbleItemsByTimestamp([
                      ...currentItems,
                      createShellBallTextBubbleItem({
                        role: "agent",
                        text: getShellBallTaskTerminalStatusText({
                          status: detail.task.status,
                          failureSummary: detail.runtime_summary.latest_failure_summary,
                        }),
                        bubbleType: "status",
                        createdAt: new Date().toISOString(),
                        taskId: detail.task.task_id,
                        turnIndex,
                        turnPhase: 2,
                      }),
                    ]),
                  );
                  revealBubbleRegionRef.current();
                }
              }

              unbindShellBallTaskRef.current(detail.task.task_id);
            }
          })
          .catch((error) => {
            if (!disposed) {
              console.warn("shell-ball task polling failed", error);

              const { nextCount: nextFailureCount, shouldStopPolling } = resolveShellBallTaskPollingFailure(
                pollingTaskFailureCount.get(taskId) ?? 0,
              );
              pollingTaskFailureCount.set(taskId, nextFailureCount);
              if (shouldStopPolling) {
                unbindShellBallTaskRef.current(taskId);
              }
            }
          })
          .finally(() => {
            pollingTaskIds.delete(taskId);
          });
      }
    };

    const intervalId = scheduleShellBallInterval(pollShellBallTasks, SHELL_BALL_TASK_POLL_INTERVAL_MS);
    pollShellBallTasks();

    return () => {
      disposed = true;
      clearShellBallInterval(intervalId);
      pollingTaskIds.clear();
      pollingTaskFailureCount.clear();
    };
  }, [allocateBubbleTurnIndex, appendApprovalPendingBubble, appendDeliveryReadyBubble, bindTaskToBubbleTurn, getTaskBubbleTurnIndex]);

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

    async function handleIntentDecision(payload: ShellBallIntentDecisionPayload) {
      const importRpcMethods = new Function("return import('../../rpc/methods')") as () => Promise<{
        confirmTask: (request: {
          confirmed: boolean;
          corrected_intent?: ShellBallIntentDecisionPayload["correctedIntent"];
          request_meta: ReturnType<typeof createShellBallRequestMeta>;
          task_id: string;
        }) => Promise<ShellBallInputSubmitResult>;
      }>;
      const createdAt = new Date().toISOString();
      const turnIndex = allocateBubbleTurnIndex();
      const decisionText = payload.decision === "confirm" ? "确认继续" : "取消";

      bindTaskToBubbleTurn(payload.taskId, turnIndex);

      setBubbleItems((currentItems) =>
        sortShellBallBubbleItemsByTimestamp([
          ...currentItems,
          createShellBallTextBubbleItem({
            createdAt,
            role: "user",
            text: decisionText,
            bubbleType: "status",
            taskId: payload.taskId,
            turnIndex,
            turnPhase: 0,
          }),
        ]),
      );

      const finishPendingTaskRegistration = beginPendingShellBallTaskRegistration();

      try {
        const rpcMethods = await importRpcMethods();
        const result = await rpcMethods.confirmTask({
          confirmed: payload.decision === "confirm",
          corrected_intent: payload.correctedIntent,
          request_meta: createShellBallRequestMeta(),
          task_id: payload.taskId,
        });

        syncShellBallVisualStateFromTaskStatus(result.task.status);
        registerShellBallTask(result.task.task_id, turnIndex, result.task.status);

        setBubbleItems((currentItems) =>
          sortShellBallBubbleItemsByTimestamp([
            ...currentItems,
            createShellBallAgentBubbleItem(result, new Date().toISOString(), {
              turnIndex,
              turnPhase: 1,
            }),
          ]),
        );
        revealBubbleRegionRef.current();
        void autoOpenShellBallDeliveryResult(result.task.task_id, result.delivery_result);
      } catch (error) {
        console.warn("shell-ball intent decision failed", error);
        setBubbleItems((currentItems) =>
          sortShellBallBubbleItemsByTimestamp([
            ...currentItems,
            createShellBallTaskErrorBubbleItem({
              createdAt: new Date().toISOString(),
              error,
              taskId: payload.taskId,
              turnIndex,
              turnPhase: 1,
            }),
          ]),
        );
        revealBubbleRegionRef.current();
      } finally {
        finishPendingTaskRegistration();
      }
    }

    void Promise.all([
      currentWindow.listen<ShellBallPinnedWindowReadyPayload>(
        shellBallWindowSyncEvents.pinnedWindowReady,
        ({ payload }) => {
          void emitToShellBallWindowLabel(payload.windowLabel, shellBallWindowSyncEvents.snapshot, snapshotRef.current);
          void syncPinnedBubbleWindowAnchorRef.current(payload.bubbleId);
        },
      ),
      currentWindow.listen<ShellBallPinnedWindowDetachedPayload>(
        shellBallWindowSyncEvents.pinnedWindowDetached,
        ({ payload }) => {
          detachedPinnedBubbleIdsRef.current.add(payload.bubbleId);
        },
      ),
      currentWindow.listen<ShellBallIntentDecisionPayload>(shellBallWindowSyncEvents.intentDecision, ({ payload }) => {
        void handleIntentDecision(payload);
      }),
      currentWindow.listen<ShellBallBubbleActionPayload>(shellBallWindowSyncEvents.bubbleAction, ({ payload }) => {
        handleBubbleActionRef.current(payload);
      }),
      currentWindow.onMoved(() => {
        void syncAnchoredPinnedBubbleWindowsRef.current();
      }),
      currentWindow.onResized(() => {
        void syncAnchoredPinnedBubbleWindowsRef.current();
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
  }, [allocateBubbleTurnIndex, autoOpenShellBallDeliveryResult, beginPendingShellBallTaskRegistration, bindTaskToBubbleTurn, registerShellBallTask]);

  const handlePrimaryAction = useCallback(async (action: ShellBallPrimaryAction) => {
    switch (action) {
      case "attach_file": {
        const turnIndex = allocateBubbleTurnIndex();
        setBubbleItems((currentItems) =>
          sortShellBallBubbleItemsByTimestamp([
            ...currentItems,
            createShellBallTextBubbleItem({
              role: "agent",
              text: "文件选择失败，请重试；也可以把文件拖到悬浮球上先加入附件，再手动发送。",
              bubbleType: "status",
              createdAt: new Date().toISOString(),
              turnIndex,
              turnPhase: 0,
            }),
          ]),
        );
        revealBubbleRegion();
        break;
      }
      case "submit": {
        const submittedText = snapshotRef.current.inputValue.trim();
        const submittedFiles = snapshotRef.current.pendingFiles;

        if (shouldHandleShellBallWindowCommand({ text: submittedText, files: submittedFiles })) {
          void handleWindowPrompt();
          break;
        }

        if (shouldHandleShellBallScreenshotCommand({ text: submittedText, files: submittedFiles })) {
          void handleScreenshotPrompt();
          break;
        }

        if (shouldHandleShellBallClipboardCommand({ text: submittedText, files: submittedFiles })) {
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
          const immediateResult = await handlersRef.current.onSubmitText();

          if (isShellBallInputSubmitResult(immediateResult)) {
            registerShellBallTask(immediateResult.task.task_id, undefined, immediateResult.task.status);
            void autoOpenShellBallDeliveryResult(immediateResult.task.task_id, immediateResult.delivery_result);
          }

          break;
        }

        const createdAt = new Date().toISOString();
        const turnIndex = allocateBubbleTurnIndex();
        const userBubbleItem = createShellBallTextBubbleItem({
          role: "user",
          text: submittedPreview,
          bubbleType: "result",
          createdAt,
          turnIndex,
          turnPhase: 0,
        });
        const pendingAgentBubbleItem = createShellBallAgentLoadingBubbleItem({
          createdAt,
          turnIndex,
          turnPhase: 1,
        });
        setBubbleItems((currentItems) =>
          sortShellBallBubbleItemsByTimestamp([...currentItems, userBubbleItem, pendingAgentBubbleItem]),
        );
        revealBubbleRegion();

		let result: ShellBallInputSubmitResult | null | void;

		const finishPendingTaskRegistration = beginPendingShellBallTaskRegistration();
		const pendingBubbleId = pendingAgentBubbleItem.bubble.bubble_id;
		const stallTimeout = scheduleShellBallTimeout(() => {
		  setBubbleItems((currentItems) => markShellBallPendingBubbleAsStalled(currentItems, pendingBubbleId));
		  revealBubbleRegionRef.current();
		}, SHELL_BALL_SUBMIT_STALL_NOTICE_MS);

		try {
		  result = await handlersRef.current.onSubmitText();
		} catch (error) {
		  clearShellBallTimeout(stallTimeout);
		  console.warn("shell-ball text submit failed", error);
          setBubbleItems((currentItems) =>
            replaceShellBallPendingBubble(
              currentItems,
              pendingAgentBubbleItem.bubble.bubble_id,
              createShellBallTaskErrorBubbleItem({
                createdAt: new Date().toISOString(),
                error,
                turnIndex,
                turnPhase: 1,
              }),
            ),
          );
          revealBubbleRegion();
          finishPendingTaskRegistration();
          break;
        }

		if (isShellBallInputSubmitResult(result)) {
		  clearShellBallTimeout(stallTimeout);
		  registerShellBallTask(result.task.task_id, turnIndex, result.task.status);
          setBubbleItems((currentItems) => {
            const nextItems = currentItems.map((item) =>
              item.bubble.bubble_id === userBubbleItem.bubble.bubble_id
                ? {
                    ...item,
                    bubble: {
                      ...item.bubble,
                      task_id: result.task.task_id,
                    },
                  }
                : item,
            );

            return replaceShellBallPendingBubble(
              nextItems,
              pendingAgentBubbleItem.bubble.bubble_id,
              createShellBallAgentBubbleItem(result, new Date().toISOString(), {
                turnIndex,
                turnPhase: 1,
              }),
            );
          });
          revealBubbleRegion();
          void autoOpenShellBallDeliveryResult(result.task.task_id, result.delivery_result);
          finishPendingTaskRegistration();
          break;
        }

		setBubbleItems((currentItems) =>
		  replaceShellBallPendingBubble(currentItems, pendingAgentBubbleItem.bubble.bubble_id),
		);
		clearShellBallTimeout(stallTimeout);
		finishPendingTaskRegistration();
		break;
      }
      case "primary_click":
        handlersRef.current.onPrimaryClick();
        break;
    }
  }, [allocateBubbleTurnIndex, autoOpenShellBallDeliveryResult, beginPendingShellBallTaskRegistration, handleScreenshotPrompt, handleWindowPrompt, registerShellBallTask, revealBubbleRegion]);

  return {
    snapshot,
    handleDroppedFiles,
    handleSelectedTextPrompt,
    handleClipboardPrompt,
    handlePrimaryAction,
    handleBubbleAction,
    handleBubbleHoverChange: handleCoordinatorBubbleHoverChange,
    handleInputHoverChange: handleCoordinatorInputHoverChange,
    handleInputFocusChange: handleCoordinatorInputFocusChange,
    handleRegionEnter: handleCoordinatorRegionEnter,
    handleRegionLeave: handleCoordinatorRegionLeave,
  };
}

export function useShellBallPinnedBubbleSnapshot() {
  const [snapshot, setSnapshot] = useState(createDefaultShellBallWindowSnapshot);

  useEffect(() => {
    const currentWindow = getCurrentWindow();

    const targetLabel = currentWindow.label;

    if (getShellBallPinnedBubbleIdFromLabel(targetLabel) === null) {
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

        const bubbleId = getShellBallPinnedBubbleIdFromLabel(targetLabel);

        if (bubbleId !== null) {
          void currentWindow.emitTo(shellBallWindowLabels.ball, shellBallWindowSyncEvents.pinnedWindowReady, {
            windowLabel: targetLabel,
            bubbleId,
          });
        }
      });

    return () => {
      disposed = true;
      cleanup?.();
    };
  }, []);

  return snapshot;
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

/**
 * Determines whether the current shell-ball draft should trigger the formal
 * screenshot shortcut instead of the normal submit path.
 *
 * @param input Current text draft and pending file attachments.
 * @returns Whether the screenshot shortcut should route into the formal task flow.
 */
function shouldHandleShellBallScreenshotCommand(input: {
  text: string;
  files: string[];
}) {
  return input.files.length === 0 && input.text.trim() === SHELL_BALL_SCREENSHOT_COMMAND;
}

/**
 * Determines whether the current shell-ball draft should trigger the formal
 * foreground-window shortcut.
 *
 * @param input Current text draft and pending file attachments.
 * @returns Whether the foreground-window shortcut should route into the formal task flow.
 */
function shouldHandleShellBallWindowCommand(input: {
  text: string;
  files: string[];
}) {
  return input.files.length === 0 && input.text.trim() === SHELL_BALL_WINDOW_COMMAND;
}
