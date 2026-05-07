import type {
  DeliveryPreference,
  InputContext,
  IntentPayload,
  PageContext,
  RequestMeta,
  RequestSource,
} from "@cialloclaw/protocol";
import { getActiveWindowContext, type DesktopWindowContextPayload } from "@/platform/desktopWindowContext";
import { startTask } from "@/rpc/methods";
import { submitTextInput } from "./agentInputService";
import {
  getConversationPageContextForSession,
  rememberConversationPageContextFromTask,
  rememberConversationSessionFromTask,
} from "./conversationSessionService";
import { compactPageContext, mapDesktopWindowSnapshotToPageContext } from "./pageContext";

type StartTaskContext = {
  context?: InputContext;
  delivery?: DeliveryPreference;
  intent?: IntentPayload;
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

function isShellBallIntakePageContext(pageContext: PageContext) {
  return pageContext.url === "local://shell-ball" && pageContext.app_name?.toLowerCase() === "desktop";
}

function hasTaskSpecificPageContextAnchor(pageContext: PageContext | undefined) {
  if (!pageContext || isShellBallIntakePageContext(pageContext)) {
    return false;
  }

  return Boolean(
    pageContext.url
      || pageContext.hover_target
      || (pageContext.app_name && (pageContext.title || pageContext.window_title)),
  );
}

function pageContextAnchorsMatch(left: PageContext | undefined, right: PageContext | undefined) {
  if (!left?.url || !right?.url) {
    return false;
  }

  return left.url === right.url;
}

function stripRememberedPageContextAttachHints(pageContext: PageContext): PageContext {
  return compactPageContext({
    app_name: pageContext.app_name,
    title: pageContext.title,
    url: pageContext.url,
    window_title: pageContext.window_title,
  }) ?? pageContext;
}

async function readForegroundPageContext(): Promise<PageContext | undefined> {
  try {
    const windowContext = await getActiveWindowContext();
    return mapDesktopWindowSnapshotToPageContext(windowContext as DesktopWindowContextPayload | null);
  } catch {
    return undefined;
  }
}

async function hydrateRememberedPageContext(rememberedPageContext: PageContext) {
  const stableRememberedPageContext = stripRememberedPageContextAttachHints(rememberedPageContext);
  const foregroundPageContext = await readForegroundPageContext();
  if (!pageContextAnchorsMatch(stableRememberedPageContext, foregroundPageContext)) {
    return stableRememberedPageContext;
  }

  // The remembered session anchor keeps stable page identity only. When the
  // current foreground snapshot still points at the same page, rehydrate fresh
  // attach hints so follow-up task starts do not replay stale process metadata.
  return compactPageContext({
    ...stableRememberedPageContext,
    ...foregroundPageContext,
  }) ?? stableRememberedPageContext;
}

async function resolveTaskPageContext(pageContext: PageContext | undefined, sessionId: string | undefined) {
  const compactedPageContext = compactPageContext(pageContext);

  if (hasTaskSpecificPageContextAnchor(compactedPageContext)) {
    return compactedPageContext;
  }

  const rememberedPageContext = getConversationPageContextForSession(sessionId);
  // URL-less remembered anchors are too weak to reuse safely across follow-up
  // tasks, so fall back to the shell-ball default context instead.
  if (rememberedPageContext?.url) {
    return hydrateRememberedPageContext(rememberedPageContext);
  }

  return DEFAULT_TASK_PAGE_CONTEXT;
}

function resolveTaskSessionId(sessionId: string | undefined) {
  // Task-entry helpers start fresh unless a caller deliberately pins a backend
  // session for an explicit continuation flow.
  return sessionId?.trim() || undefined;
}

export async function startTaskFromSelectedText(text: string, context: StartTaskContext = {}) {
  const normalizedText = text.trim();
  const resolvedSessionId = resolveTaskSessionId(context.sessionId);
  const pageContext = await resolveTaskPageContext(context.pageContext, resolvedSessionId);
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
      page_context: pageContext,
    },
    context: context.context,
    delivery: context.delivery ?? {
      preferred: "bubble",
      fallback: "task_detail",
    },
  });
  rememberConversationSessionFromTask(result.task);
  rememberConversationPageContextFromTask(result.task, pageContext);
  return result;
}

export async function startTaskFromFiles(files: string[], context: StartTaskContext = {}, text?: string) {
  const normalizedFiles = files.map((file) => file.trim()).filter(Boolean);
  const resolvedSessionId = resolveTaskSessionId(context.sessionId);
  const pageContext = await resolveTaskPageContext(context.pageContext, resolvedSessionId);
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
      page_context: pageContext,
    },
    context: context.context,
    delivery: context.delivery ?? {
      preferred: "bubble",
      fallback: "task_detail",
    },
    options: {
      // File drops do not force the confirmation gate; the backend decides
      // whether this is a new bare-file task or evidence for a pending task.
      confirm_required: false,
    },
  });
  rememberConversationSessionFromTask(result.task);
  rememberConversationPageContextFromTask(result.task, pageContext);
  return result;
}

export async function startTaskFromErrorSignal(errorMessage: string, context: StartTaskContext = {}) {
  const normalizedMessage = errorMessage.trim();
  const resolvedSessionId = resolveTaskSessionId(context.sessionId);
  const pageContext = await resolveTaskPageContext(context.pageContext, resolvedSessionId);
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
      page_context: pageContext,
    },
    context: context.context,
    delivery: context.delivery ?? {
      preferred: "bubble",
      fallback: "task_detail",
    },
  });
  rememberConversationSessionFromTask(result.task);
  rememberConversationPageContextFromTask(result.task, pageContext);
  return result;
}

/**
 * Starts a formal task from an accepted recommendation through the standard
 * `recommendation_click` task entrypoint.
 *
 * @param text Recommendation text accepted by the user.
 * @param context Optional desktop task-start metadata.
 * @returns The formal task-start response from the local service.
 */
export async function startTaskFromRecommendation(
  text: string,
  context: StartTaskContext = {},
) {
  const normalizedText = text.trim();
  const resolvedSessionId = resolveTaskSessionId(context.sessionId);
  const pageContext = await resolveTaskPageContext(context.pageContext, resolvedSessionId);
  if (normalizedText === "") {
    throw new Error("recommendation text is empty");
  }

  const result = await startTask({
    request_meta: createRequestMeta("recommendation_click"),
    ...(resolvedSessionId ? { session_id: resolvedSessionId } : {}),
    source: context.source ?? "floating_ball",
    trigger: "recommendation_click",
    ...(context.intent ? { intent: context.intent } : {}),
    input: {
      type: "text",
      text: normalizedText,
      page_context: pageContext,
    },
    context: context.context,
    delivery: context.delivery ?? {
      preferred: "bubble",
      fallback: "task_detail",
    },
  });
  rememberConversationSessionFromTask(result.task);
  rememberConversationPageContextFromTask(result.task, pageContext);
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
  if (taskResult.task === null) {
    throw new Error("hover text input did not create a task");
  }

  return taskResult.task;
}
