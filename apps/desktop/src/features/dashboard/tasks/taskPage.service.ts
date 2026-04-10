import type {
  AgentTaskControlParams,
  AgentTaskListResult,
  RequestMeta,
  Task,
  TaskControlAction,
  TaskListGroup,
} from "@cialloclaw/protocol";
import { controlTask, getTaskDetail, listTasks } from "@/rpc/methods";
import { getMockTaskBuckets, getMockTaskDetail, getTaskExperience, hasMockTaskData, runMockTaskControl } from "./taskPage.mock";
import type { TaskBucketsData, TaskControlOutcome, TaskDetailData, TaskExperience, TaskListItem } from "./taskPage.types";

const TASK_LIST_PAGE_SIZE = 50;
const MAX_TASK_LIST_PAGES = 100;
const TASK_PAGE_MOCK_MODE = import.meta.env.DEV || import.meta.env.VITE_TASK_PAGE_USE_MOCKS === "true";

function createRequestMeta(scope: string): RequestMeta {
  return {
    client_time: new Date().toISOString(),
    trace_id: `trace_${scope}_${Date.now()}`,
  };
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

async function listTasksByGroup(group: TaskListGroup, offset: number): Promise<AgentTaskListResult> {
  return listTasks({
    group,
    limit: TASK_LIST_PAGE_SIZE,
    offset,
    request_meta: createRequestMeta(`task_list_${group}`),
    sort_by: group === "finished" ? "finished_at" : "updated_at",
    sort_order: "desc",
  });
}

async function listAllTasksByGroup(group: TaskListGroup): Promise<Task[]> {
  const items: Task[] = [];
  let offset = 0;

  for (let pageIndex = 0; pageIndex < MAX_TASK_LIST_PAGES; pageIndex += 1) {
    const result = await listTasksByGroup(group, offset);
    items.push(...result.items);

    if (!result.page.has_more || result.items.length === 0) {
      return items;
    }

    offset = result.page.offset + result.items.length;
  }

  throw new Error(`Task list pagination exceeded ${MAX_TASK_LIST_PAGES} pages for group ${group}.`);
}

export async function loadTaskBuckets(): Promise<TaskBucketsData> {
  try {
    const [unfinishedTasks, finishedTasks] = await Promise.all([
      listAllTasksByGroup("unfinished"),
      listAllTasksByGroup("finished"),
    ]);

    return {
      finished: mapTasks(finishedTasks),
      source: "rpc",
      unfinished: mapTasks(unfinishedTasks),
    };
  } catch (error) {
    if (TASK_PAGE_MOCK_MODE) {
      console.warn("Task buckets RPC unavailable, using local mock fallback.", error);
      return getMockTaskBuckets();
    }

    throw error;
  }
}

export async function loadTaskDetailData(taskId: string): Promise<TaskDetailData> {
  try {
    const detail = await getTaskDetail({
      request_meta: createRequestMeta(`task_detail_${taskId}`),
      task_id: taskId,
    });

    return {
      detail,
      experience: getTaskExperience(taskId) ?? createFallbackExperience(detail.task),
      source: "rpc",
      task: detail.task,
    };
  } catch (error) {
    if (TASK_PAGE_MOCK_MODE && hasMockTaskData(taskId)) {
      console.warn("Task detail RPC unavailable, using local mock fallback.", error);
      return getMockTaskDetail(taskId);
    }

    throw error;
  }
}

export async function controlTaskByAction(taskId: string, action: TaskControlAction): Promise<TaskControlOutcome> {
  const params: AgentTaskControlParams = {
    action,
    request_meta: createRequestMeta(`task_control_${action}`),
    task_id: taskId,
  };

  try {
    return {
      result: await controlTask(params),
      source: "rpc",
    };
  } catch (error) {
    if (TASK_PAGE_MOCK_MODE && hasMockTaskData(taskId)) {
      console.warn(`Task control RPC unavailable for ${action}, using local mock fallback.`, error);
      return runMockTaskControl(taskId, action);
    }

    throw error;
  }
}
