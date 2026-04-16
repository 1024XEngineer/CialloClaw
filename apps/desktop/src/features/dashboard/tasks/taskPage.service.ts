import type { AgentTaskDetailGetResult, AgentTaskControlParams, RequestMeta, Task, TaskControlAction, TaskListGroup } from "@cialloclaw/protocol";
import { isRpcChannelUnavailable, logRpcMockFallback } from "@/rpc/fallback";
import { controlTask, getTaskDetail, listTasks } from "@/rpc/methods";
import { isActiveApprovalRequest, isApprovalRequest, isArtifact, isBinaryPendingAuthorizations, isMirrorReference, isRecoveryPoint, isTask, isTaskStep, normalizeArray, normalizeNullable } from "../shared/dashboardContractValidators";
import { RISK_LEVELS, SECURITY_STATUSES, TASK_STEP_STATUSES } from "@/rpc/protocolEnumerations";
import { getMockTaskBuckets, getMockTaskDetail, getTaskExperience, runMockTaskControl } from "./taskPage.mock";
import type { TaskBucketPageData, TaskBucketsData, TaskControlOutcome, TaskDetailData, TaskExperience, TaskListItem } from "./taskPage.types";

export type TaskPageDataMode = "rpc" | "mock";

const INITIAL_TASK_PAGE_LIMIT: Record<TaskListGroup, number> = {
  finished: 24,
  unfinished: 12,
};
const TASK_RPC_TIMEOUT_MS = 2_500;

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

function createFallbackExperience(task: Task): TaskExperience {
  return {
    acceptance: ["任务信息完整可读。", "当前状态与进度表达清晰。"],
    assistantState: {
      hint: "这是从真实 task 数据推断出的默认说明，后续可以补更细的上下文。",
      label: task.status === "processing" ? "正在思考" : task.finished_at ? "刚完成一步" : "待命",
    },
    background: "当前展示的是任务协议里的真实数据，补充说明采用了最小默认文案。",
    constraints: ["保持协议字段原样。", "避免猜测未返回的信息。"],
    dueAt: null,
    goal: task.title,
    nextAction: task.status === "processing" ? "继续沿着当前步骤推进。" : "等待下一次明确操作。",
    noteDraft: "当前任务基于真实协议返回，页面补充说明使用默认占位文案。",
    noteEntries: ["可在后续补充更具体的上下文摘要。"],
    outputs: [
      { id: `${task.task_id}_draft`, label: "当前草稿", content: "等待更多任务上下文后补齐。", tone: "draft" },
      { id: `${task.task_id}_result`, label: "已生成结果", content: "结果区会优先展示当前任务已经返回的产出与交付入口。", tone: "result" },
      { id: `${task.task_id}_editable`, label: "可继续编辑", content: "当前可先结合时间线与成果区继续查看已有上下文。", tone: "editable" },
    ],
    phase: `当前步骤：${task.current_step}`,
    priority: task.risk_level === "red" ? "critical" : task.risk_level === "yellow" ? "high" : "steady",
    progressHint: "真实任务数据已接入，页面补充文案为默认值。",
    quickContext: [
      { id: `${task.task_id}_ctx_1`, label: "来源", content: `当前任务来自 ${task.source_type}。` },
      { id: `${task.task_id}_ctx_2`, label: "风险等级", content: `当前风险等级为 ${task.risk_level}。` },
      { id: `${task.task_id}_ctx_3`, label: "建议动作", content: "可以先查看时间线，再决定是否继续推进。" },
    ],
    recentConversation: ["当前任务使用的是协议返回的真实数据。"],
    relatedFiles: [],
    stepTargets: {},
    suggestedNext: "优先查看当前步骤与时间线，再决定下一步动作。",
  };
}

function mapTasks(items: Task[]): TaskListItem[] {
  return items.map((task) => ({
    experience: getTaskExperience(task.task_id) ?? createFallbackExperience(task),
    task,
  }));
}

function getTaskListSortBy(group: TaskListGroup) {
  return group === "finished" ? "finished_at" : "updated_at";
}

function createFallbackTaskDetail(task: Task): AgentTaskDetailGetResult {
  return {
    approval_request: null,
    artifacts: [],
    mirror_references: [],
    security_summary: {
      latest_restore_point: null,
      pending_authorizations: 0,
      risk_level: task.risk_level,
      security_status: "normal",
    },
    task,
    timeline: [],
  };
}

const riskLevels = new Set<string>(RISK_LEVELS);
const securityStatuses = new Set<string>(SECURITY_STATUSES);
const taskStepStatuses = new Set<string>(TASK_STEP_STATUSES);

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

  const approvalRequest = normalizeNullable(detail.approval_request, isApprovalRequest, "task detail payload approval_request");
  const latestRestorePoint = normalizeNullable(detail.security_summary.latest_restore_point, isRecoveryPoint, "task detail payload restore point");
  const artifacts = normalizeArray(detail.artifacts, isArtifact, "task detail payload artifacts");
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
    artifacts,
    mirror_references: mirrorReferences,
    security_summary: {
      ...detail.security_summary,
      latest_restore_point: latestRestorePoint,
    },
    task: detail.task,
    timeline,
  };
}

export function buildFallbackTaskDetailData(item: TaskListItem): TaskDetailData {
  return {
    detailWarningMessage: null,
    detail: createFallbackTaskDetail(item.task),
    experience: item.experience,
    source: "fallback",
    task: item.task,
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
      const hasRecoveredMirrors = Array.isArray(candidate.mirror_references) && candidate.mirror_references.length === 0;

      if (
        nextError instanceof Error &&
        ((/artifacts/i.test(nextError.message) && hasRecoveredArtifacts) || (/mirror/i.test(nextError.message) && hasRecoveredMirrors))
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

function getMockTaskBucketPage(group: TaskListGroup, options?: { limit?: number; offset?: number }): TaskBucketPageData {
  const limit = options?.limit ?? INITIAL_TASK_PAGE_LIMIT[group];
  const offset = options?.offset ?? 0;
  const buckets = getMockTaskBuckets();
  const bucket = group === "unfinished" ? buckets.unfinished : buckets.finished;
  const items = bucket.items.slice(offset, offset + limit);

  return {
    items,
    page: {
      has_more: offset + limit < bucket.items.length,
      limit,
      offset,
      total: bucket.items.length,
    },
  };
}

export async function loadTaskBucketPage(group: TaskListGroup, options?: { limit?: number; offset?: number; source?: TaskPageDataMode }): Promise<TaskBucketPageData> {
  const source = options?.source ?? "rpc";

  if (source === "mock") {
    return getMockTaskBucketPage(group, options);
  }

  try {
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
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      logRpcMockFallback(`task bucket ${group}`, error);
      return getMockTaskBucketPage(group, options);
    }

    throw error;
  }
}

export async function loadTaskBuckets(options?: { unfinishedLimit?: number; finishedLimit?: number; source?: TaskPageDataMode }): Promise<TaskBucketsData> {
  const source = options?.source ?? "rpc";
  const [unfinishedResult, finishedResult] = await Promise.all([
    loadTaskBucketPage("unfinished", { limit: options?.unfinishedLimit, source }),
    loadTaskBucketPage("finished", { limit: options?.finishedLimit, source }),
  ]);

  return {
    finished: finishedResult,
    source,
    unfinished: unfinishedResult,
  };
}

export async function loadTaskDetailData(taskId: string, source: TaskPageDataMode = "rpc"): Promise<TaskDetailData> {
  if (source === "mock") {
    return getMockTaskDetail(taskId);
  }

  try {
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
      experience: getTaskExperience(taskId) ?? createFallbackExperience(normalized.detail.task),
      source: "rpc",
      task: normalized.detail.task,
    };
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      logRpcMockFallback(`task detail ${taskId}`, error);
      return getMockTaskDetail(taskId);
    }

    throw error;
  }
}

export async function controlTaskByAction(taskId: string, action: TaskControlAction, source: TaskPageDataMode = "rpc"): Promise<TaskControlOutcome> {
  const params: AgentTaskControlParams = {
    action,
    request_meta: createRequestMeta(`task_control_${action}`),
    task_id: taskId,
  };

  if (source === "mock") {
    return runMockTaskControl(taskId, action);
  }

  try {
    return {
      result: await withTimeout(controlTask(params), `task control ${action}`),
      source: "rpc",
    };
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      logRpcMockFallback(`task control ${action}`, error);
      return runMockTaskControl(taskId, action);
    }

    throw error;
  }
}
