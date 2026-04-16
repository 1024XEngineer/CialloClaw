import type { BubbleMessage, DeliveryResult, Task } from "@cialloclaw/protocol";
import { createMockAgentInputSubmitResult } from "@/services/agentInputMock";

type ShellBallMockResult = {
  task: Task;
  bubble_message: BubbleMessage | null;
  delivery_result: DeliveryResult | null;
};

function createTimestamp() {
  return new Date().toISOString();
}

function createTaskId() {
  return typeof globalThis.crypto?.randomUUID === "function"
    ? `task_mock_${globalThis.crypto.randomUUID()}`
    : `task_mock_${Date.now()}_${Math.random().toString(16).slice(2)}`;
}

function buildTask(input: {
  taskId: string;
  title: string;
  sourceType: Task["source_type"];
  status: Task["status"];
  riskLevel: Task["risk_level"];
  intentName: string;
}) {
  const now = createTimestamp();
  return {
    task_id: input.taskId,
    title: input.title,
    source_type: input.sourceType,
    status: input.status,
    intent: {
      name: input.intentName,
      arguments: {
        mode: "mock",
      },
    },
    current_step: input.status === "confirming_intent" ? "等待确认" : "本地 mock 结果已生成",
    risk_level: input.riskLevel,
    started_at: now,
    updated_at: now,
    finished_at: input.status === "completed" || input.status === "cancelled" ? now : null,
  } satisfies Task;
}

function buildBubble(input: {
  taskId: string;
  text: string;
  type: BubbleMessage["type"];
}) {
  return {
    bubble_id: `bubble_${input.taskId}`,
    task_id: input.taskId,
    type: input.type,
    text: input.text,
    pinned: false,
    hidden: false,
    created_at: createTimestamp(),
  } satisfies BubbleMessage;
}

function buildDeliveryResult(taskId: string, previewText: string): DeliveryResult {
  return {
    type: "bubble",
    title: "Mock Delivery",
    payload: {
      path: null,
      task_id: taskId,
      url: null,
    },
    preview_text: previewText,
  };
}

export function createMockShellBallSubmitResult(input: {
  text: string;
  inputMode: "voice" | "text";
}): ShellBallMockResult {
  return createMockAgentInputSubmitResult(input);
}

export function createMockShellBallConfirmResult(input: {
  taskId: string;
  confirmed: boolean;
}): ShellBallMockResult {
  const previewText = input.confirmed
    ? "JSON-RPC 当前未连通，已用 mock 模式继续执行，并生成了一条本地结果。"
    : "JSON-RPC 当前未连通，这条 mock 任务已取消，不会继续执行。";

  const task = buildTask({
    taskId: input.taskId,
    title: input.confirmed ? "Mock Confirmed Task" : "Mock Cancelled Task",
    sourceType: "hover_input",
    status: input.confirmed ? "completed" : "cancelled",
    riskLevel: input.confirmed ? "yellow" : "green",
    intentName: input.confirmed ? "offline_mock_confirmed" : "offline_mock_cancelled",
  });

  return {
    task,
    bubble_message: buildBubble({
      taskId: input.taskId,
      text: previewText,
      type: "result",
    }),
    delivery_result: input.confirmed ? buildDeliveryResult(input.taskId, previewText) : null,
  };
}
