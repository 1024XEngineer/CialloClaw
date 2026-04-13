import type { AgentTaskDetailGetResult, AgentTaskControlParams, RequestMeta, Task, TaskControlAction, TaskListGroup, TaskStep } from "@cialloclaw/protocol";
import { RISK_LEVELS, SECURITY_STATUSES, TASK_STEP_STATUSES } from "@cialloclaw/protocol";
import { controlTask, getTaskDetail, listTasks } from "@/rpc/methods";
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
      { id: `${task.task_id}_result`, label: "已生成结果", content: "当前协议未返回更多结果摘要，先展示任务轨迹。", tone: "result" },
      { id: `${task.task_id}_editable`, label: "可继续编辑", content: "后续可把任务修改或产出打开能力接进来。", tone: "editable" },
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
    artifacts: [],
    mirror_references: [],
    security_summary: {
      latest_restore_point: null,
      pending_authorizations: task.status === "waiting_auth" ? 1 : 0,
      risk_level: task.risk_level,
      security_status: task.status === "waiting_auth" ? "pending_confirmation" : "normal",
    },
    task,
    timeline: [],
  };
}

const riskLevels = new Set<string>(RISK_LEVELS);
const securityStatuses = new Set<string>(SECURITY_STATUSES);
const taskStepStatuses = new Set<string>(TASK_STEP_STATUSES);

function isTaskStep(step: unknown): step is TaskStep {
  if (!step || typeof step !== "object") {
    return false;
  }

  const candidate = step as Partial<TaskStep>;
  return (
    typeof candidate.step_id === "string" &&
    typeof candidate.task_id === "string" &&
    typeof candidate.name === "string" &&
    typeof candidate.order_index === "number" &&
    typeof candidate.input_summary === "string" &&
    typeof candidate.output_summary === "string" &&
    typeof candidate.status === "string" &&
    taskStepStatuses.has(candidate.status)
  );
}

function hasValidSecuritySummary(detail: AgentTaskDetailGetResult): boolean {
  const summary = detail.security_summary as Partial<AgentTaskDetailGetResult["security_summary"]> | null | undefined;
  return Boolean(
    summary &&
      typeof summary.pending_authorizations === "number" &&
      typeof summary.risk_level === "string" &&
      typeof summary.security_status === "string" &&
      riskLevels.has(summary.risk_level) &&
      securityStatuses.has(summary.security_status) &&
      "latest_restore_point" in summary,
  );
}

function normalizeTaskDetailResult(detail: AgentTaskDetailGetResult): AgentTaskDetailGetResult {
  if (!detail || !detail.task || !detail.task.task_id) {
    throw new Error("task detail payload is missing task information");
  }

  if (!hasValidSecuritySummary(detail)) {
    throw new Error("task detail payload is missing security summary");
  }

  return {
    artifacts: Array.isArray(detail.artifacts) ? detail.artifacts : [],
    mirror_references: Array.isArray(detail.mirror_references) ? detail.mirror_references : [],
    security_summary: detail.security_summary,
    task: detail.task,
    timeline: Array.isArray(detail.timeline) ? detail.timeline.filter(isTaskStep) : [],
  };
}

export function buildFallbackTaskDetailData(item: TaskListItem): TaskDetailData {
  return {
    detail: createFallbackTaskDetail(item.task),
    experience: item.experience,
    source: "fallback",
    task: item.task,
  };
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

  const detail = normalizeTaskDetailResult(
    await withTimeout(
      getTaskDetail({
        request_meta: createRequestMeta(`task_detail_${taskId}`),
        task_id: taskId,
      }),
      `task detail ${taskId}`,
    ),
  );

  return {
    detail,
    experience: getTaskExperience(taskId) ?? createFallbackExperience(detail.task),
    source: "rpc",
    task: detail.task,
  };
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

  return {
    result: await withTimeout(controlTask(params), `task control ${action}`),
    source: "rpc",
  };
}
