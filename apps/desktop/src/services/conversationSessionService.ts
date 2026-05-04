import type { PageContext, Task, TaskUpdatedNotification } from "@cialloclaw/protocol";
import { compactPageContext } from "./pageContext";

type BackendOwnedSessionCarrier = {
  task_id?: unknown;
  session_id?: unknown;
};

type RememberedConversationSession = {
  sessionId: string;
  observedAt: number;
};

type RememberedConversationPageContext = {
  pageContext: PageContext;
  observedAt: number;
};

const CONVERSATION_SESSION_FRESHNESS_MS = 15 * 60 * 1000;

let currentConversationSession: RememberedConversationSession | null = null;
const taskSessionIds = new Map<string, RememberedConversationSession>();
const sessionPageContexts = new Map<string, RememberedConversationPageContext>();

function normalizeSessionId(value: unknown) {
  if (typeof value !== "string") {
    return null;
  }

  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function normalizeTaskId(value: unknown) {
  if (typeof value !== "string") {
    return null;
  }

  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function isRememberedSessionFresh(session: RememberedConversationSession, now: number) {
  return now - session.observedAt <= CONVERSATION_SESSION_FRESHNESS_MS;
}

function isRememberedPageContextFresh(pageContext: RememberedConversationPageContext, now: number) {
  return now - pageContext.observedAt <= CONVERSATION_SESSION_FRESHNESS_MS;
}

function pruneExpiredConversationSessions(now = Date.now()) {
  if (currentConversationSession && !isRememberedSessionFresh(currentConversationSession, now)) {
    currentConversationSession = null;
  }

  for (const [taskId, session] of taskSessionIds.entries()) {
    if (!isRememberedSessionFresh(session, now)) {
      taskSessionIds.delete(taskId);
    }
  }

  for (const [sessionId, pageContext] of sessionPageContexts.entries()) {
    if (!isRememberedPageContextFresh(pageContext, now)) {
      sessionPageContexts.delete(sessionId);
    }
  }
}

function storeSession(taskId: unknown, sessionId: unknown) {
  const normalizedSessionId = normalizeSessionId(sessionId);
  if (normalizedSessionId === null) {
    return null;
  }

  const rememberedSession = {
    sessionId: normalizedSessionId,
    observedAt: Date.now(),
  } satisfies RememberedConversationSession;

  currentConversationSession = rememberedSession;
  const normalizedTaskId = normalizeTaskId(taskId);
  if (normalizedTaskId !== null) {
    taskSessionIds.set(normalizedTaskId, rememberedSession);
  }
  return normalizedSessionId;
}

function rememberConversationSession(value: BackendOwnedSessionCarrier | null | undefined) {
  if (value == null) {
    return null;
  }

  return storeSession(value.task_id, value.session_id);
}

function isShellBallIntakePageContext(pageContext: PageContext) {
  return pageContext.url === "local://shell-ball" && pageContext.app_name?.toLowerCase() === "desktop";
}

function hasTaskSpecificPageContextAnchor(pageContext: PageContext) {
  if (isShellBallIntakePageContext(pageContext)) {
    return false;
  }

  return Boolean(
    pageContext.url
      || pageContext.hover_target
      || (pageContext.app_name && (pageContext.title || pageContext.window_title)),
  );
}

function stripVolatileAttachHints(pageContext: PageContext): PageContext {
  return {
    ...(pageContext.app_name ? { app_name: pageContext.app_name } : {}),
    ...(pageContext.title ? { title: pageContext.title } : {}),
    ...(pageContext.url ? { url: pageContext.url } : {}),
    ...(pageContext.window_title ? { window_title: pageContext.window_title } : {}),
  };
}

function normalizePageContext(value: PageContext | null | undefined) {
  if (value == null) {
    return null;
  }

  const compactedPageContext = compactPageContext(value);
  if (!compactedPageContext) {
    return null;
  }

  const pageContext = stripVolatileAttachHints(compactedPageContext);

  if (!hasTaskSpecificPageContextAnchor(pageContext)) {
    return null;
  }

  return pageContext;
}

/**
 * Returns the latest hidden conversation session acknowledged by the backend.
 * The frontend does not generate session ids locally anymore, and it stops
 * reusing stale backend sessions once the continuation freshness window lapses.
 */
export function getCurrentConversationSessionId() {
  pruneExpiredConversationSessions();
  return currentConversationSession?.sessionId ?? undefined;
}

/**
 * Records the backend-owned session carried by a formal task payload.
 * The normalization path remains permissive so stale local services that still
 * omit `task.session_id` fail soft instead of breaking the desktop cache.
 */
export function rememberConversationSessionFromTask(task: Task | null | undefined) {
  return rememberConversationSession(task as BackendOwnedSessionCarrier | null | undefined);
}

/**
 * Remembers the real page or window anchor associated with a backend-owned
 * session. Synthetic shell-ball wrapper context is intentionally ignored so a
 * later attachment cannot pretend it matches a pending task. Volatile attach
 * hints are stripped here so follow-up reuse keeps a stable page anchor instead
 * of replaying stale process metadata.
 */
export function rememberConversationPageContextFromTask(task: Task | null | undefined, pageContext: PageContext | null | undefined) {
  const normalizedSessionId = normalizeSessionId((task as BackendOwnedSessionCarrier | null | undefined)?.session_id);
  const normalizedPageContext = normalizePageContext(pageContext);
  if (normalizedSessionId === null || normalizedPageContext === null) {
    return null;
  }

  sessionPageContexts.set(normalizedSessionId, {
    pageContext: normalizedPageContext,
    observedAt: Date.now(),
  });
  return normalizedPageContext;
}

/**
 * Returns the latest task-specific page or window anchor for the session that
 * will receive the next shell-ball input.
 */
export function getConversationPageContextForSession(sessionId: string | undefined) {
  pruneExpiredConversationSessions();
  const normalizedSessionId = normalizeSessionId(sessionId) ?? currentConversationSession?.sessionId ?? null;
  if (normalizedSessionId === null) {
    return undefined;
  }

  const rememberedPageContext = sessionPageContexts.get(normalizedSessionId);
  if (!rememberedPageContext) {
    return undefined;
  }

  return { ...rememberedPageContext.pageContext };
}

/**
 * Records the backend-owned session carried by task.updated.
 * The notification contract now includes `session_id`, but permissive
 * normalization keeps older backend builds from breaking session recall.
 */
export function rememberConversationSessionFromTaskUpdated(payload: TaskUpdatedNotification | null | undefined) {
  return rememberConversationSession(payload as BackendOwnedSessionCarrier | null | undefined);
}

export function getConversationSessionIdForTask(taskId: string | null | undefined) {
  pruneExpiredConversationSessions();
  const normalizedTaskId = normalizeTaskId(taskId);
  if (normalizedTaskId === null) {
    return undefined;
  }

  return taskSessionIds.get(normalizedTaskId)?.sessionId;
}
