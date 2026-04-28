import type {
  DeliveryPreference,
  InputContext,
  PageContext,
  RequestMeta,
  RequestSource,
  Task,
} from "@cialloclaw/protocol";
import { startTask } from "@/rpc/methods";
import { useTaskStore } from "@/stores/taskStore";
import { submitTextInput } from "./agentInputService";
import { getCurrentConversationSessionId, rememberConversationSessionFromTask } from "./conversationSessionService";

type StartTaskContext = {
  context?: InputContext;
  delivery?: DeliveryPreference;
  pageContext?: PageContext;
  sessionId?: string;
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

function resolveTaskPageContext(pageContext: PageContext | undefined) {
  const normalizedAppName = pageContext?.app_name?.trim();
  const normalizedTitle = pageContext?.title?.trim();
  const normalizedUrl = pageContext?.url?.trim();

  if (normalizedAppName && normalizedTitle && normalizedUrl) {
    return {
      app_name: normalizedAppName,
      title: normalizedTitle,
      url: normalizedUrl,
    };
  }

  return DEFAULT_TASK_PAGE_CONTEXT;
}

function resolveTaskSessionId(sessionId: string | undefined) {
  return sessionId?.trim() || getCurrentConversationSessionId();
}

export async function startTaskFromSelectedText(text: string, context: StartTaskContext = {}) {
  const normalizedText = text.trim();
  const resolvedSessionId = resolveTaskSessionId(context.sessionId);
  if (normalizedText === "") {
    throw new Error("selected text is empty");
  }

  const result = await startTask({
    request_meta: createRequestMeta("text_selected_click"),
    ...(resolvedSessionId ? { session_id: resolvedSessionId } : {}),
    source: context.source ?? "floating_ball",
    trigger: "text_selected_click",
    input: {
      type: "text_selection",
      text: normalizedText,
      page_context: resolveTaskPageContext(context.pageContext),
    },
    context: context.context,
    delivery: context.delivery ?? {
      preferred: "bubble",
      fallback: "task_detail",
    },
  });
  rememberConversationSessionFromTask(result.task);
  return result;
}

export async function startTaskFromFiles(files: string[], context: StartTaskContext = {}, text?: string) {
  const normalizedFiles = files.map((file) => file.trim()).filter(Boolean);
  const resolvedSessionId = resolveTaskSessionId(context.sessionId);
  if (normalizedFiles.length === 0) {
    throw new Error("dropped files are empty");
  }

  const normalizedText = normalizeTaskInputText(text);

  const result = await startTask({
    request_meta: createRequestMeta("file_drop"),
    ...(resolvedSessionId ? { session_id: resolvedSessionId } : {}),
    source: context.source ?? "floating_ball",
    trigger: "file_drop",
    input: {
      type: "file",
      ...(normalizedText === undefined ? {} : { text: normalizedText }),
      files: normalizedFiles,
      page_context: resolveTaskPageContext(context.pageContext),
    },
    context: context.context,
    delivery: context.delivery ?? {
      preferred: "bubble",
      fallback: "task_detail",
    },
    options: {
      confirm_required: normalizedText === undefined,
    },
  });
  rememberConversationSessionFromTask(result.task);
  return result;
}

export async function startTaskFromErrorSignal(errorMessage: string, context: StartTaskContext = {}) {
  const normalizedMessage = errorMessage.trim();
  const resolvedSessionId = resolveTaskSessionId(context.sessionId);
  if (normalizedMessage === "") {
    throw new Error("error signal is empty");
  }

  const result = await startTask({
    request_meta: createRequestMeta("error_detected"),
    ...(resolvedSessionId ? { session_id: resolvedSessionId } : {}),
    source: context.source ?? "floating_ball",
    trigger: "error_detected",
    input: {
      type: "error",
      error_message: normalizedMessage,
      page_context: resolveTaskPageContext(context.pageContext),
    },
    context: context.context,
    delivery: context.delivery ?? {
      preferred: "bubble",
      fallback: "task_detail",
    },
  });
  rememberConversationSessionFromTask(result.task);
  return result;
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
