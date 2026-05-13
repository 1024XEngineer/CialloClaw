import type {
  AgentTaskDetailGetResult,
  AgentTaskControlParams,
  AgentTaskEventsListParams,
  AgentTaskSteerParams,
  RequestMeta,
  Task,
  TaskControlAction,
  TaskEvent,
  TaskListGroup,
  TaskRuntimeSummary,
} from "@cialloclaw/protocol";
import { controlTask, getTaskDetail, listTaskEvents, listTasks, steerTask } from "@/rpc/methods";
import { isActiveApprovalRequest, isApprovalRequest, isArtifact, isAuditRecord, isAuthorizationRecord, isBinaryPendingAuthorizations, isCitation, isDeliveryResult, isMirrorReference, isRecoveryPoint, isTask, isTaskEvent, isTaskStep, normalizeArray, normalizeNullable } from "../shared/dashboardContractValidators";
import { RISK_LEVELS, SECURITY_STATUSES, TASK_STEP_STATUSES } from "@/rpc/protocolEnumerations";
import { formatTaskSourceLabel, getTaskPreviewStatusLabel } from "./taskPage.mapper";
import type { TaskBucketPageData, TaskBucketsData, TaskControlOutcome, TaskDetailData, TaskEventFilters, TaskEventPageData, TaskEventTimeRange, TaskExperience, TaskListItem } from "./taskPage.types";

export type TaskPageDataMode = "rpc";

const INITIAL_TASK_PAGE_LIMIT: Record<TaskListGroup, number> = {
  finished: 24,
  unfinished: 12,
};
const TASK_RPC_TIMEOUT_MS = 2_500;

export const DEFAULT_TASK_EVENT_FILTERS: TaskEventFilters = {
  eventType: "",
  runId: "",
  timeRange: "all",
};

function createRequestMeta(scope: string): RequestMeta {
  return {
    client_time: new Date().toISOString(),
    trace_id: `trace_${scope}_${Date.now()}`,
  };
}

async function withTimeout<T>(promise: Promise<T>, label: string): Promise<T> {
  return Promise.race([
    promise,
    new Promise<T>((_, reject) => {
      window.setTimeout(() => reject(new Error(`${label} request timed out`)), TASK_RPC_TIMEOUT_MS);
    }),
  ]);
}

function getTaskPriority(task: Task): TaskExperience["priority"] {
  if (task.risk_level === "red") {
    return "critical";
  }

  if (task.risk_level === "yellow") {
    return "high";
  }

  return "steady";
}

function getTaskPhase(task: Task) {
  if (task.current_step.trim() !== "") {
    return task.current_step.trim();
  }

  if (task.status === "processing") {
    return "等待时间线返回当前步骤。";
  }

  return `当前状态：${getTaskPreviewStatusLabel(task.status)}`;
}

function getTaskWaitingReason(task: Task, detail?: AgentTaskDetailGetResult) {
  if (task.status === "confirming_intent") {
    return "等待确认当前处理方式后继续执行。";
  }

  if (task.status === "waiting_auth") {
    const operationName = detail?.approval_request?.operation_name?.trim();
    return operationName ? `等待你确认是否允许 ${operationName}。` : "等待授权确认后继续执行。";
  }

  if (task.status === "waiting_input") {
    return "等待补充信息后继续执行。";
  }

  if (task.status === "paused") {
    return "任务已暂停，恢复后会继续推进。";
  }

  return undefined;
}

function getTaskBlockedReason(task: Task, detail?: AgentTaskDetailGetResult) {
  if (task.status !== "blocked" && task.status !== "failed") {
    return undefined;
  }

  const runtimeSummary = detail?.runtime_summary;
  const failureSummary = runtimeSummary?.latest_failure_summary?.trim();
  if (failureSummary) {
    return failureSummary;
  }

  const stopReason = runtimeSummary?.loop_stop_reason?.trim();
  if (stopReason) {
    return `最近一次停止原因：${stopReason}`;
  }

  return "当前任务遇到阻塞，需要先补齐条件或恢复后再继续。";
}

function getTaskEndedSummary(task: Task, detail?: AgentTaskDetailGetResult) {
  if (task.status === "completed") {
    return "任务已完成。";
  }

  if (task.status === "cancelled") {
    return "任务已取消。";
  }

  if (task.status === "failed" || task.status === "ended_unfinished") {
    return getTaskBlockedReason(task, detail) ?? "任务已结束但未完整收束，请先查看失败原因与恢复点。";
  }

  return undefined;
}

function getTaskNextAction(task: Task, detail?: AgentTaskDetailGetResult) {
  const waitingReason = getTaskWaitingReason(task, detail);
  if (waitingReason) {
    return waitingReason;
  }

  if (task.status === "processing") {
    return "等待时间线、交付结果或运行时通知继续推进。";
  }

  const blockedReason = getTaskBlockedReason(task, detail);
  if (blockedReason) {
    return blockedReason;
  }

  return getTaskEndedSummary(task, detail) ?? "查看交付或重新启动任务。";
}

function buildProtocolTaskExperience(task: Task, detail?: AgentTaskDetailGetResult): TaskExperience {
  const waitingReason = getTaskWaitingReason(task, detail);
  const blockedReason = getTaskBlockedReason(task, detail);
  const endedSummary = getTaskEndedSummary(task, detail);
  const nextAction = getTaskNextAction(task, detail);

  return {
    acceptance: [],
    assistantState: {
      hint: "当前面板只消费任务详情返回内容，不再复用示意说明。",
      label: getTaskPreviewStatusLabel(task.status),
    },
    background: "当前说明直接基于任务详情数据生成，只保留已返回的任务事实与本地展示性文案。",
    constraints: ["不推断未返回的上下文。"],
    dueAt: null,
    goal: task.title,
    nextAction,
    noteDraft: "当前笔记草稿仍保留在本地，不会替代任务与交付对象。",
    noteEntries: [],
    outputs: [],
    phase: getTaskPhase(task),
    priority: getTaskPriority(task),
    progressHint: "当前页面说明来自任务详情；更细上下文需等待后端返回。",
    quickContext: [
      { id: `${task.task_id}_ctx_source`, label: "来源", content: formatTaskSourceLabel(task.source_type) },
      { id: `${task.task_id}_ctx_status`, label: "当前状态", content: getTaskPreviewStatusLabel(task.status) },
      { id: `${task.task_id}_ctx_risk`, label: "风险等级", content: task.risk_level },
    ],
    recentConversation: [],
    relatedFiles: [],
    stepTargets: {},
    suggestedNext: nextAction,
    ...(endedSummary ? { endedSummary } : {}),
    ...(waitingReason ? { waitingReason } : {}),
    ...(blockedReason ? { blockedReason } : {}),
  };
}

function mapTasks(items: Task[]): TaskListItem[] {
  return items.map((task) => ({
    experience: buildProtocolTaskExperience(task),
    task,
  }));
}

function getTaskListSortBy(group: TaskListGroup) {
  return group === "finished" ? "finished_at" : "updated_at";
}

function parseTaskEventPayload(event: TaskEvent): Record<string, unknown> | null {
  const source = event.payload_json.trim();
  if (!source) {
    return null;
  }

  try {
    const parsed = JSON.parse(source) as unknown;
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? (parsed as Record<string, unknown>) : null;
  } catch {
    return null;
  }
}

function createFallbackRuntimeSummary(): AgentTaskDetailGetResult["runtime_summary"] {
  return {
    active_steering_count: 0,
    events_count: 0,
    latest_failure_code: null,
    latest_failure_category: null,
    latest_failure_summary: null,
    latest_event_type: null,
    loop_stop_reason: null,
    observation_signals: [],
  };
}

function normalizeRuntimeSummary(detail: AgentTaskDetailGetResult): AgentTaskDetailGetResult["runtime_summary"] {
  const candidate = detail.runtime_summary as Partial<AgentTaskDetailGetResult["runtime_summary"]> | null | undefined;
  if (!candidate || typeof candidate !== "object") {
    return createFallbackRuntimeSummary();
  }

  return {
    active_steering_count: typeof candidate.active_steering_count === "number" ? candidate.active_steering_count : 0,
    events_count: typeof candidate.events_count === "number" ? candidate.events_count : 0,
    latest_failure_code: typeof candidate.latest_failure_code === "string" ? candidate.latest_failure_code : null,
    latest_failure_category: typeof candidate.latest_failure_category === "string" ? candidate.latest_failure_category : null,
    latest_failure_summary: typeof candidate.latest_failure_summary === "string" ? candidate.latest_failure_summary : null,
    latest_event_type: typeof candidate.latest_event_type === "string" ? candidate.latest_event_type : null,
    loop_stop_reason: typeof candidate.loop_stop_reason === "string" ? candidate.loop_stop_reason : null,
    observation_signals: Array.isArray(candidate.observation_signals) ? candidate.observation_signals.filter((item): item is string => typeof item === "string") : [],
  };
}

function normalizeTaskEventPage(result: { items: TaskEvent[]; page: TaskEventPageData["page"] }): TaskEventPageData {
  return {
    items: normalizeArray(result.items, isTaskEvent, "task events payload items").map((event) => ({
      ...event,
      payload: parseTaskEventPayload(event),
    })),
    page: result.page,
  };
}

function sanitizeTaskEventFilters(filters?: Partial<TaskEventFilters>): TaskEventFilters {
  const timeRange = filters?.timeRange ?? DEFAULT_TASK_EVENT_FILTERS.timeRange;
  return {
    eventType: filters?.eventType?.trim() ?? DEFAULT_TASK_EVENT_FILTERS.eventType,
    runId: filters?.runId?.trim() ?? DEFAULT_TASK_EVENT_FILTERS.runId,
    timeRange: isTaskEventTimeRange(timeRange) ? timeRange : DEFAULT_TASK_EVENT_FILTERS.timeRange,
  };
}

function isTaskEventTimeRange(value: string): value is TaskEventTimeRange {
  return value === "all" || value === "1h" || value === "24h" || value === "7d";
}

function resolveTaskEventTimeBounds(timeRange: TaskEventTimeRange, nowProvider: () => Date) {
  if (timeRange === "all") {
    return {};
  }

  const now = nowProvider();
  const start = new Date(now.getTime());
  switch (timeRange) {
    case "1h":
      start.setHours(start.getHours() - 1);
      break;
    case "24h":
      start.setDate(start.getDate() - 1);
      break;
    case "7d":
      start.setDate(start.getDate() - 7);
      break;
    default:
      return {};
  }

  return {
    created_at_from: start.toISOString(),
    created_at_to: now.toISOString(),
  };
}

const riskLevels = new Set<string>(RISK_LEVELS);
const securityStatuses = new Set<string>(SECURITY_STATUSES);
const taskStepStatuses = new Set<string>(TASK_STEP_STATUSES);

function isValidRuntimeSummary(summary: Partial<TaskRuntimeSummary> | null | undefined): summary is TaskRuntimeSummary {
  return Boolean(
    summary &&
      typeof summary.events_count === "number" &&
      Number.isFinite(summary.events_count) &&
      typeof summary.active_steering_count === "number" &&
      Number.isFinite(summary.active_steering_count) &&
      (typeof summary.latest_failure_code === "string" || summary.latest_failure_code === null || typeof summary.latest_failure_code === "undefined") &&
      (typeof summary.latest_failure_category === "string" || summary.latest_failure_category === null || typeof summary.latest_failure_category === "undefined") &&
      (typeof summary.latest_failure_summary === "string" || summary.latest_failure_summary === null || typeof summary.latest_failure_summary === "undefined") &&
      (typeof summary.latest_event_type === "string" || summary.latest_event_type === null || typeof summary.latest_event_type === "undefined") &&
      (typeof summary.loop_stop_reason === "string" || summary.loop_stop_reason === null || typeof summary.loop_stop_reason === "undefined") &&
      (Array.isArray(summary.observation_signals) ? summary.observation_signals.every((item) => typeof item === "string") : typeof summary.observation_signals === "undefined"),
  );
}

function hasValidSecuritySummary(detail: AgentTaskDetailGetResult): boolean {
  const summary = detail.security_summary as Partial<AgentTaskDetailGetResult["security_summary"]> | null | undefined;
  return Boolean(
    summary &&
      isBinaryPendingAuthorizations(summary.pending_authorizations) &&
      typeof summary.risk_level === "string" &&
      typeof summary.security_status === "string" &&
      riskLevels.has(summary.risk_level) &&
      securityStatuses.has(summary.security_status) &&
      "latest_restore_point" in summary,
  );
}

export function normalizeTaskDetailResult(detail: AgentTaskDetailGetResult): AgentTaskDetailGetResult {
  if (!detail || !isTask(detail.task)) {
    throw new Error("task detail payload is missing task information");
  }

  const taskId = detail.task.task_id;

  if (!hasValidSecuritySummary(detail)) {
    throw new Error("task detail payload is missing security summary");
  }

  if (detail.runtime_summary !== undefined && !isValidRuntimeSummary(detail.runtime_summary)) {
    throw new Error("task detail payload is missing runtime summary");
  }
  const runtimeSummary = detail.runtime_summary === undefined ? createFallbackRuntimeSummary() : normalizeRuntimeSummary(detail);

  const approvalRequest = normalizeNullable(detail.approval_request, isApprovalRequest, "task detail payload approval_request");
  const authorizationRecord = detail.authorization_record === undefined ? null : normalizeNullable(detail.authorization_record, isAuthorizationRecord, "task detail payload authorization_record");
  const auditRecord = detail.audit_record === undefined ? null : normalizeNullable(detail.audit_record, isAuditRecord, "task detail payload audit_record");
  const deliveryResult = detail.delivery_result === undefined ? null : normalizeNullable(detail.delivery_result, isDeliveryResult, "task detail payload delivery_result");
  const latestRestorePoint = normalizeNullable(detail.security_summary.latest_restore_point, isRecoveryPoint, "task detail payload restore point");
  const artifacts = normalizeArray(detail.artifacts, isArtifact, "task detail payload artifacts");
  const citations = normalizeArray(detail.citations, isCitation, "task detail payload citations");
  const mirrorReferences = normalizeArray(detail.mirror_references, isMirrorReference, "task detail payload mirror_references");
  const timeline = normalizeArray(detail.timeline, (value): value is (typeof detail.timeline)[number] => isTaskStep(value, taskStepStatuses), "task detail payload timeline");

  if (approvalRequest === null && detail.security_summary.pending_authorizations !== 0) {
    throw new Error("task detail payload pending authorization summary does not match approval_request");
  }

  if (approvalRequest !== null && detail.security_summary.pending_authorizations !== 1) {
    throw new Error("task detail payload pending authorization summary does not match approval_request");
  }

  if (approvalRequest !== null && detail.task.status !== "waiting_auth") {
    throw new Error("task detail payload approval_request requires task.status waiting_auth");
  }

  if (approvalRequest !== null && !isActiveApprovalRequest(approvalRequest)) {
    throw new Error("task detail payload approval_request is not active pending");
  }

  if (approvalRequest !== null && approvalRequest.task_id !== taskId) {
    throw new Error("task detail payload approval_request task_id does not match task.task_id");
  }

  if (latestRestorePoint !== null && latestRestorePoint.task_id !== taskId) {
    throw new Error("task detail payload restore point task_id does not match task.task_id");
  }

  return {
    approval_request: approvalRequest,
    audit_record: auditRecord,
    artifacts,
    authorization_record: authorizationRecord,
    citations,
    delivery_result: deliveryResult,
    mirror_references: mirrorReferences,
    runtime_summary: runtimeSummary,
    security_summary: {
      ...detail.security_summary,
      latest_restore_point: latestRestorePoint,
    },
    task: detail.task,
    timeline,
  };
}

function recoverTaskDetailFromInvalidCollections(detail: AgentTaskDetailGetResult, error: unknown) {
  if (!(error instanceof Error)) {
    throw error;
  }

  const warnings: string[] = [];
  let candidate = detail;
  let currentError: unknown = error;

  for (;;) {
    if (!(currentError instanceof Error)) {
      throw currentError;
    }

    if (/artifacts/i.test(currentError.message)) {
      warnings.push("任务成果信息暂时无法完整展示，已先隐藏格式不符合要求的产物。");
      candidate = {
        ...candidate,
        artifacts: [],
      };
    } else if (/citations/i.test(currentError.message)) {
      warnings.push("任务引用信息暂时无法完整展示，已先隐藏格式不符合要求的引用。");
      candidate = {
        ...candidate,
        citations: [],
      };
    } else if (/mirror/i.test(currentError.message)) {
      warnings.push("镜子命中信息暂时无法完整展示，已先隐藏格式不符合要求的上下文引用。");
      candidate = {
        ...candidate,
        mirror_references: [],
      };
    } else {
      throw currentError;
    }

    try {
      return {
        detail: normalizeTaskDetailResult(candidate),
        detailWarningMessage: warnings.join(" "),
      };
    } catch (nextError) {
      const hasRecoveredArtifacts = Array.isArray(candidate.artifacts) && candidate.artifacts.length === 0;
      const hasRecoveredCitations = Array.isArray(candidate.citations) && candidate.citations.length === 0;
      const hasRecoveredMirrors = Array.isArray(candidate.mirror_references) && candidate.mirror_references.length === 0;

      if (
        nextError instanceof Error &&
        ((/artifacts/i.test(nextError.message) && hasRecoveredArtifacts) ||
          (/citations/i.test(nextError.message) && hasRecoveredCitations) ||
          (/mirror/i.test(nextError.message) && hasRecoveredMirrors))
      ) {
        throw nextError;
      }

      currentError = nextError;
    }
  }
}

export function normalizeTaskDetailData(detail: AgentTaskDetailGetResult) {
  try {
    return {
      detailWarningMessage: null,
      detail: normalizeTaskDetailResult(detail),
    };
  } catch (error) {
    return recoverTaskDetailFromInvalidCollections(detail, error);
  }
}

export async function loadTaskBucketPage(group: TaskListGroup, options?: { limit?: number; offset?: number; source?: TaskPageDataMode }): Promise<TaskBucketPageData> {
  const limit = options?.limit ?? INITIAL_TASK_PAGE_LIMIT[group];
  const offset = options?.offset ?? 0;
  const result = await withTimeout(
    listTasks({
      group,
      limit,
      offset,
      request_meta: createRequestMeta(`task_list_${group}_${offset}_${limit}`),
      sort_by: getTaskListSortBy(group),
      sort_order: "desc",
    }),
    `task bucket ${group}`,
  );

  return {
    items: mapTasks(result.items),
    page: result.page,
  };
}

export async function loadTaskEventPage(taskId: string, _source: TaskPageDataMode = "rpc", filters: Partial<TaskEventFilters> = DEFAULT_TASK_EVENT_FILTERS, nowProvider: () => Date = () => new Date()): Promise<TaskEventPageData> {
  const normalizedFilters = sanitizeTaskEventFilters(filters);
  const params: AgentTaskEventsListParams = {
    limit: 20,
    offset: 0,
    request_meta: createRequestMeta(`task_events_${taskId}`),
    task_id: taskId,
    ...(normalizedFilters.runId ? { run_id: normalizedFilters.runId } : {}),
    ...(normalizedFilters.eventType ? { type: normalizedFilters.eventType } : {}),
    ...resolveTaskEventTimeBounds(normalizedFilters.timeRange, nowProvider),
  };

  return normalizeTaskEventPage(await withTimeout(listTaskEvents(params), `task events ${taskId}`));
}

export async function steerTaskByMessage(taskId: string, message: string, _source: TaskPageDataMode = "rpc") {
  const trimmed = message.trim();
  if (!trimmed) {
    throw new Error("补充要求不能为空");
  }

  const params: AgentTaskSteerParams = {
    message: trimmed,
    request_meta: createRequestMeta(`task_steer_${taskId}`),
    task_id: taskId,
  };

  return withTimeout(steerTask(params), `task steer ${taskId}`);
}

export async function loadTaskBuckets(options?: { unfinishedLimit?: number; finishedLimit?: number; source?: TaskPageDataMode }): Promise<TaskBucketsData> {
  const [unfinishedResult, finishedResult] = await Promise.all([
    loadTaskBucketPage("unfinished", { limit: options?.unfinishedLimit, source: "rpc" }),
    loadTaskBucketPage("finished", { limit: options?.finishedLimit, source: "rpc" }),
  ]);

  return {
    finished: finishedResult,
    source: "rpc",
    unfinished: unfinishedResult,
  };
}

export async function loadTaskDetailData(taskId: string, _source: TaskPageDataMode = "rpc"): Promise<TaskDetailData> {
  const normalized = normalizeTaskDetailData(
    await withTimeout(
      getTaskDetail({
        request_meta: createRequestMeta(`task_detail_${taskId}`),
        task_id: taskId,
      }),
      `task detail ${taskId}`,
    ),
  );

  return {
    detailWarningMessage: normalized.detailWarningMessage,
    detail: normalized.detail,
    experience: buildProtocolTaskExperience(normalized.detail.task, normalized.detail),
    source: "rpc",
    task: normalized.detail.task,
  };
}

export async function controlTaskByAction(taskId: string, action: TaskControlAction, source: TaskPageDataMode = "rpc"): Promise<TaskControlOutcome> {
  const params: AgentTaskControlParams = {
    action,
    request_meta: createRequestMeta(`task_control_${action}`),
    task_id: taskId,
  };

  return {
    result: await withTimeout(controlTask(params), `task control ${action}`),
    source,
  };
}
