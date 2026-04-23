import type { Task, TaskUpdatedNotification } from "@cialloclaw/protocol";

type BackendOwnedSessionCarrier = {
  task_id?: unknown;
  session_id?: unknown;
};

type RememberedConversationSession = {
  sessionId: string;
  observedAt: number;
};

const CONVERSATION_SESSION_FRESHNESS_MS = 15 * 60 * 1000;

let currentConversationSession: RememberedConversationSession | null = null;
const taskSessionIds = new Map<string, RememberedConversationSession>();

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

function pruneExpiredConversationSessions(now = Date.now()) {
  if (currentConversationSession && !isRememberedSessionFresh(currentConversationSession, now)) {
    currentConversationSession = null;
  }

  for (const [taskId, session] of taskSessionIds.entries()) {
    if (!isRememberedSessionFresh(session, now)) {
      taskSessionIds.delete(taskId);
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
