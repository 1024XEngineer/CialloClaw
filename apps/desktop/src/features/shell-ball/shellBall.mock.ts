import type { BubbleMessage, DeliveryResult, IntentPayload, Task } from "@cialloclaw/protocol";
import { createMockAgentInputSubmitResult } from "@/services/agentInputMock";

type ShellBallMockResult = {
  task: Task;
  bubble_message: BubbleMessage | null;
  delivery_result: DeliveryResult | null;
};

function createTimestamp() {
  return new Date().toISOString();
}

function buildTask(input: {
  taskId: string;
  title: string;
  sourceType: Task["source_type"];
  status: Task["status"];
  riskLevel: Task["risk_level"];
  intent: IntentPayload | null;
}) {
  const now = createTimestamp();
  return {
    task_id: input.taskId,
    title: input.title,
    source_type: input.sourceType,
    status: input.status,
    intent: input.intent,
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
  correctedIntent?: IntentPayload;
}): ShellBallMockResult {
  const hasCorrectedIntent = Boolean(input.correctedIntent?.name);
  const taskStatus = input.confirmed || hasCorrectedIntent ? "completed" : "confirming_intent";
  const intentName = input.confirmed
    ? "offline_mock_confirmed"
    : hasCorrectedIntent
      ? input.correctedIntent!.name
      : "offline_mock_reconfirm";
  const previewText = input.confirmed
    ? "JSON-RPC 当前未连通，已用 mock 模式继续执行，并生成了一条本地结果。"
    : hasCorrectedIntent
      ? "JSON-RPC 当前未连通，已按修正后的处理方式继续推进这条 mock 任务。"
      : "JSON-RPC 当前未连通，这条 mock 任务已回到确认态，请重新说明你的目标。";

  const task = buildTask({
    taskId: input.taskId,
    title: input.confirmed ? "Mock Confirmed Task" : hasCorrectedIntent ? "Mock Corrected Task" : "Mock Reconfirm Task",
    sourceType: "hover_input",
    status: taskStatus,
    riskLevel: input.confirmed || hasCorrectedIntent ? "yellow" : "green",
    intent: input.confirmed
      ? {
          name: intentName,
          arguments: {
            mode: "mock",
          },
        }
      : hasCorrectedIntent
        ? {
            name: intentName,
            arguments: {
              ...(input.correctedIntent?.arguments ?? {}),
              mode: "mock",
            },
          }
        : null,
  });

  return {
    task,
    bubble_message: buildBubble({
      taskId: input.taskId,
      text: previewText,
      type: taskStatus === "confirming_intent" ? "intent_confirm" : "result",
    }),
    delivery_result: taskStatus === "completed" ? buildDeliveryResult(input.taskId, previewText) : null,
  };
}
