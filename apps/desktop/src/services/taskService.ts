// 该文件封装前端任务服务调用。
import type {
  AgentTaskStartResult,
  DeliveryPreference,
  InputType,
  IntentPayload,
  RequestMeta,
  RequestSource,
  Task,
  TaskStatus,
} from "@cialloclaw/protocol";
import { isRpcChannelUnavailable, logRpcMockFallback } from "@/rpc/fallback";
import { startTask } from "@/rpc/methods";
import { useTaskStore } from "@/stores/taskStore";

type StartTaskContext = {
  delivery?: DeliveryPreference;
  intent?: IntentPayload;
  pageContext?: {
    app_name: string;
    title: string;
    url: string;
  };
  source?: RequestSource;
};

const DEFAULT_TASK_PAGE_CONTEXT = {
  app_name: "desktop",
  title: "Quick Intake",
  url: "local://shell-ball",
} as const;

function createRequestMeta(scope: string): RequestMeta {
  return {
    trace_id: `trace_${scope}_${Date.now()}`,
    client_time: new Date().toISOString(),
  };
}

function normalizeTaskTitle(value: string) {
  const trimmed = value.trim();
  if (trimmed.length <= 24) {
    return trimmed;
  }

  return `${trimmed.slice(0, 24)}...`;
}

function createMockTaskStartResult(input: {
  currentStep: string;
  inputType: InputType;
  previewText: string;
  riskLevel: Task["risk_level"];
  scope: string;
  sourceType: Task["source_type"];
  status: TaskStatus;
  title: string;
}): AgentTaskStartResult {
  const now = new Date().toISOString();
  const taskId = `task_mock_${input.scope}_${Date.now()}_${Math.random().toString(16).slice(2, 8)}`;

  return {
    task: {
      task_id: taskId,
      title: input.title,
      source_type: input.sourceType,
      status: input.status,
      intent: {
        name: `mock_${input.inputType}`,
        arguments: {
          scope: input.scope,
        },
      },
      current_step: input.currentStep,
      risk_level: input.riskLevel,
      started_at: now,
      updated_at: now,
      finished_at: input.status === "completed" || input.status === "cancelled" ? now : null,
    },
    bubble_message: {
      bubble_id: `bubble_mock_${taskId}`,
      task_id: taskId,
      type: input.status === "confirming_intent" ? "intent_confirm" : "result",
      text: input.previewText,
      pinned: false,
      hidden: false,
      created_at: now,
    },
    delivery_result:
      input.status === "completed"
        ? {
            type: "bubble",
            title: input.title,
            payload: {
              path: null,
              task_id: taskId,
              url: null,
            },
            preview_text: input.previewText,
          }
        : null,
  };
}

async function startTaskWithFallback(input: {
  mock: Omit<Parameters<typeof createMockTaskStartResult>[0], "scope">;
  scope: string;
  params: Parameters<typeof startTask>[0];
}) {
  try {
    return await startTask(input.params);
  } catch (error) {
    if (!isRpcChannelUnavailable(error)) {
      throw error;
    }

    logRpcMockFallback(`task.start ${input.scope}`, error);
    return createMockTaskStartResult({
      ...input.mock,
      scope: input.scope,
    });
  }
}

export async function startTaskFromSelectedText(text: string, context: StartTaskContext = {}) {
  const normalizedText = text.trim();
  if (normalizedText === "") {
    throw new Error("selected text is empty");
  }

  return startTaskWithFallback({
    scope: "text_selected_click",
    params: {
      request_meta: createRequestMeta("text_selected_click"),
      source: context.source ?? "floating_ball",
      trigger: "text_selected_click",
      input: {
        type: "text_selection",
        text: normalizedText,
        page_context: context.pageContext ?? DEFAULT_TASK_PAGE_CONTEXT,
      },
      intent: context.intent,
      delivery: context.delivery ?? {
        preferred: "bubble",
        fallback: "task_detail",
      },
    },
    mock: {
      currentStep: "本地 mock 已承接选中文本",
      inputType: "text_selection",
      previewText: `JSON-RPC 当前未连通，已用 mock 模式承接选中文本：${normalizeTaskTitle(normalizedText)}`,
      riskLevel: "green",
      sourceType: "selected_text",
      status: "completed",
      title: normalizeTaskTitle(normalizedText),
    },
  });
}

export async function startTaskFromFiles(files: string[], context: StartTaskContext = {}) {
  const normalizedFiles = files.map((file) => file.trim()).filter(Boolean);
  if (normalizedFiles.length === 0) {
    throw new Error("dropped files are empty");
  }

  const leadFile = normalizedFiles[0].split(/[\\/]/).pop() ?? normalizedFiles[0];
  const title = normalizedFiles.length === 1 ? `处理文件：${leadFile}` : `处理 ${normalizedFiles.length} 个文件`;

  return startTaskWithFallback({
    scope: "file_drop",
    params: {
      request_meta: createRequestMeta("file_drop"),
      source: context.source ?? "floating_ball",
      trigger: "file_drop",
      input: {
        type: "file",
        files: normalizedFiles,
        page_context: context.pageContext ?? DEFAULT_TASK_PAGE_CONTEXT,
      },
      intent: context.intent,
      delivery: context.delivery ?? {
        preferred: "bubble",
        fallback: "task_detail",
      },
    },
    mock: {
      currentStep: "本地 mock 已承接文件拖拽",
      inputType: "file",
      previewText:
        normalizedFiles.length === 1
          ? `JSON-RPC 当前未连通，已用 mock 模式承接文件：${leadFile}`
          : `JSON-RPC 当前未连通，已用 mock 模式承接 ${normalizedFiles.length} 个文件。`,
      riskLevel: "yellow",
      sourceType: "dragged_file",
      status: "completed",
      title,
    },
  });
}

export async function startTaskFromErrorSignal(errorMessage: string, context: StartTaskContext = {}) {
  const normalizedMessage = errorMessage.trim();
  if (normalizedMessage === "") {
    throw new Error("error signal is empty");
  }

  return startTaskWithFallback({
    scope: "error_detected",
    params: {
      request_meta: createRequestMeta("error_detected"),
      source: context.source ?? "floating_ball",
      trigger: "error_detected",
      input: {
        type: "error",
        error_message: normalizedMessage,
        page_context: context.pageContext ?? DEFAULT_TASK_PAGE_CONTEXT,
      },
      intent: context.intent,
      delivery: context.delivery ?? {
        preferred: "bubble",
        fallback: "task_detail",
      },
    },
    mock: {
      currentStep: "本地 mock 已承接错误信号",
      inputType: "error",
      previewText: `JSON-RPC 当前未连通，已用 mock 模式承接错误信息：${normalizeTaskTitle(normalizedMessage)}`,
      riskLevel: "yellow",
      sourceType: "error_signal",
      status: "completed",
      title: normalizeTaskTitle(normalizedMessage),
    },
  });
}

// bootstrapTask 处理当前模块的相关逻辑。
export async function bootstrapTask(title: string) {
  const taskResult = await startTaskWithFallback({
    scope: "hover_text_input",
    params: {
      request_meta: createRequestMeta("hover_text_input"),
      source: "floating_ball",
      trigger: "hover_text_input",
      input: {
        type: "text",
        text: title,
        page_context: {
          title: "Quick Input",
          url: "local://shell-ball",
          app_name: "desktop",
        },
      },
      intent: {
        name: "summarize",
        arguments: {
          style: "key_points",
        },
      },
      delivery: {
        preferred: "bubble",
        fallback: "workspace_document",
      },
    },
    mock: {
      currentStep: "本地 mock 已承接悬浮输入",
      inputType: "text",
      previewText: `JSON-RPC 当前未连通，已用 mock 模式承接输入：${normalizeTaskTitle(title)}`,
      riskLevel: "green",
      sourceType: "hover_input",
      status: "completed",
      title: normalizeTaskTitle(title),
    },
  });

  return taskResult.task;
}

// listActiveTasks 处理当前模块的相关逻辑。
export function listActiveTasks(): Task[] {
  return useTaskStore.getState().tasks;
}
