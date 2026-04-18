import type {
  DeliveryPreference,
  InputContext,
  RequestMeta,
  RequestSource,
  Task,
} from "@cialloclaw/protocol";
import { startTask } from "@/rpc/methods";
import { useTaskStore } from "@/stores/taskStore";
import { submitTextInput } from "./agentInputService";

type StartTaskContext = {
  context?: InputContext;
  delivery?: DeliveryPreference;
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

function normalizeTaskInputText(value: string | undefined) {
  const trimmed = value?.trim() ?? "";
  return trimmed === "" ? undefined : trimmed;
}

export async function startTaskFromSelectedText(text: string, context: StartTaskContext = {}) {
  const normalizedText = text.trim();
  if (normalizedText === "") {
    throw new Error("selected text is empty");
  }

  return startTask({
    request_meta: createRequestMeta("text_selected_click"),
    source: context.source ?? "floating_ball",
    trigger: "text_selected_click",
    input: {
      type: "text_selection",
      text: normalizedText,
      page_context: context.pageContext ?? DEFAULT_TASK_PAGE_CONTEXT,
    },
    context: context.context,
    delivery: context.delivery ?? {
      preferred: "bubble",
      fallback: "task_detail",
    },
  });
}

export async function startTaskFromFiles(files: string[], context: StartTaskContext = {}, text?: string) {
  const normalizedFiles = files.map((file) => file.trim()).filter(Boolean);
  if (normalizedFiles.length === 0) {
    throw new Error("dropped files are empty");
  }

  const normalizedText = normalizeTaskInputText(text);

  return startTask({
    request_meta: createRequestMeta("file_drop"),
    source: context.source ?? "floating_ball",
    trigger: "file_drop",
    input: {
      type: "file",
      ...(normalizedText === undefined ? {} : { text: normalizedText }),
      files: normalizedFiles,
      page_context: context.pageContext ?? DEFAULT_TASK_PAGE_CONTEXT,
    },
    context: context.context,
    delivery: context.delivery ?? {
      preferred: "bubble",
      fallback: "task_detail",
    },
  });
}

export async function startTaskFromErrorSignal(errorMessage: string, context: StartTaskContext = {}) {
  const normalizedMessage = errorMessage.trim();
  if (normalizedMessage === "") {
    throw new Error("error signal is empty");
  }

  return startTask({
    request_meta: createRequestMeta("error_detected"),
    source: context.source ?? "floating_ball",
    trigger: "error_detected",
    input: {
      type: "error",
      error_message: normalizedMessage,
      page_context: context.pageContext ?? DEFAULT_TASK_PAGE_CONTEXT,
    },
    context: context.context,
    delivery: context.delivery ?? {
      preferred: "bubble",
      fallback: "task_detail",
    },
  });
}

export async function bootstrapTask(title: string) {
  const taskResult = await submitTextInput({
    text: title,
    source: "floating_ball",
    trigger: "hover_text_input",
    inputMode: "text",
    options: {
      preferred_delivery: "bubble",
    },
  });

  if (taskResult === null) {
    throw new Error("hover text input is empty");
  }

  return taskResult.task;
}

export function listActiveTasks(): Task[] {
  return useTaskStore.getState().tasks;
}
