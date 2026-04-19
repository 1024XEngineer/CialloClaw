import type {
  AgentNotepadConvertToTaskParams,
  AgentNotepadListParams,
  AgentNotepadUpdateParams,
  DeliveryPayload,
  DeliveryType,
  NotepadAction,
  RequestMeta,
  TodoBucket,
  TodoItem,
} from "@cialloclaw/protocol";
import {
  createDashboardOpenPlan,
  isAllowedDashboardOpenUrl,
  performDashboardOpenPlan,
  type DashboardOpenExecutionResult,
  type DashboardOpenPlan,
} from "@/features/dashboard/shared/dashboardOpen";
import { convertNotepadToTask, listNotepad, updateNotepad } from "@/rpc/methods";
import { getMockNoteBuckets, getMockNoteExperience, runMockConvertNoteToTask, runMockUpdateNote } from "./notePage.mock";
import type { NoteConvertOutcome, NoteDetailExperience, NoteListItem, NoteResource, NoteUpdateOutcome } from "./notePage.types";

const NOTEPAD_RPC_TIMEOUT_MS = 2_500;

export type NotePageDataMode = "rpc" | "mock";
export type NoteResourceOpenExecutionPlan = DashboardOpenPlan;
export type NoteResourceOpenExecutionResult = DashboardOpenExecutionResult;
export type NoteResourceOpenExecutionOptions = {
  approveOutsideWorkspace?: boolean;
};

function createRequestMeta(scope: string): RequestMeta {
  return {
    client_time: new Date().toISOString(),
    trace_id: `trace_${scope}_${Date.now()}`,
  };
}

function formatAbsoluteTime(value: string) {
  return new Date(value).toLocaleString("zh-CN", {
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    month: "numeric",
  });
}

function formatRelativeTime(value: string) {
  const targetTime = new Date(value).getTime();
  const diffMs = targetTime - Date.now();
  const absHours = Math.round(Math.abs(diffMs) / (1000 * 60 * 60));
  const absDays = Math.round(Math.abs(diffMs) / (1000 * 60 * 60 * 24));

  if (absHours < 1) {
    return diffMs >= 0 ? "1 小时内" : "刚刚超时";
  }

  if (absHours < 24) {
    return diffMs >= 0 ? `还剩 ${absHours} 小时` : `逾期 ${absHours} 小时`;
  }

  return diffMs >= 0 ? `还剩 ${absDays} 天` : `逾期 ${absDays} 天`;
}

export function isAllowedNoteOpenUrl(url: string): boolean {
  return isAllowedDashboardOpenUrl(url);
}

function resolveResourceOpenPayload(resource: NonNullable<TodoItem["related_resources"]>[number]): DeliveryPayload | null {
  if (!resource?.open_payload) {
    return null;
  }

  return {
    path: resource.open_payload.path ?? null,
    task_id: resource.open_payload.task_id ?? null,
    url: resource.open_payload.url ?? null,
  };
}

function getPreviewStatus(item: TodoItem) {
  if (item.bucket === "closed") {
    return item.status === "completed" ? "已完成" : "已取消";
  }

  if (item.bucket === "recurring_rule") {
    return item.recurring_enabled === false ? "规则已暂停" : "规则生效中";
  }

  if (item.status === "overdue") {
    return "已逾期";
  }

  if (item.status === "due_today") {
    return "今天要做";
  }

  return item.bucket === "later" ? "尚未到期" : "近期安排";
}

function getDetailStatus(item: TodoItem) {
  if (item.bucket === "closed") {
    return item.status === "completed" ? "已结束" : "已取消";
  }

  if (item.bucket === "recurring_rule") {
    return item.recurring_enabled === false ? "重复规则已暂停" : "重复规则生效中";
  }

  if (item.status === "overdue") {
    return "已逾期";
  }

  if (item.status === "due_today") {
    return "今日待处理";
  }

  return item.bucket === "later" ? "尚未开始" : "即将到来";
}

function getTimeHint(item: TodoItem) {
  const endedTime = item.ended_at ?? item.due_at;

  if (item.bucket === "closed") {
    return endedTime ? formatAbsoluteTime(endedTime) : "未设置时间";
  }

  if (!item.due_at) {
    return item.bucket === "recurring_rule" ? "等待补充规则时间" : "未设置时间";
  }

  if (item.bucket === "recurring_rule") {
    return formatAbsoluteTime(item.due_at);
  }

  if (item.status === "due_today" || item.status === "overdue") {
    return formatRelativeTime(item.due_at);
  }

  return formatAbsoluteTime(item.due_at);
}

function getSummaryLabel(item: TodoItem) {
  if (item.bucket === "closed") {
    return item.status === "completed" ? "已归档" : "已取消";
  }

  if (item.bucket === "recurring_rule") {
    return "重复提醒";
  }

  if (item.bucket === "later") {
    return "后续安排";
  }

  return item.status === "overdue" ? "优先处理" : "待进入执行";
}

function getTypeLabel(item: TodoItem) {
  const normalizedType = item.type.replace(/[_-]/g, " ").trim();
  const normalizedKey = normalizedType.toLowerCase();

  const typeLabelMap: Record<string, string> = {
    archive: "已结束记录",
    "follow up": "跟进行项",
    note: "便签事项",
    recurring: "重复事项",
    reminder: "提醒事项",
    task: "任务事项",
    template: "模板事项",
  };

  if (!normalizedType) {
    return item.bucket === "recurring_rule" ? "重复事项" : "便签事项";
  }

  if (typeLabelMap[normalizedKey]) {
    return typeLabelMap[normalizedKey];
  }

  if (/[\u4e00-\u9fff]/.test(normalizedType)) {
    return normalizedType;
  }

  return item.bucket === "recurring_rule" ? "重复事项" : "便签事项";
}

function normalizeResourceOpenAction(action: DeliveryType | null, payload: DeliveryPayload | null): NoteResource["openAction"] {
  if (action === "task_detail") {
    return "task_detail";
  }

  if (action === "result_page" && payload?.url) {
    return "open_url";
  }

  if (action === "open_file") {
    return "open_file";
  }

  if (action === "reveal_in_folder") {
    return "reveal_in_folder";
  }

  if (action === "workspace_document" && payload?.path) {
    return "open_file";
  }

  if (payload?.url) {
    return "open_url";
  }

  return payload?.path ? "open_file" : null;
}

function createResourceHints(item: TodoItem) {
  if (item.related_resources && item.related_resources.length > 0) {
    return item.related_resources.map<NoteResource>((resource) => {
      const payload = resolveResourceOpenPayload(resource);

      return {
        id: resource.resource_id,
        label: resource.label,
        openAction: normalizeResourceOpenAction(resource.open_action ?? null, payload),
        path: resource.path ?? payload?.path ?? null,
        taskId: payload?.task_id ?? null,
        type: resource.resource_type,
        url: payload?.url ?? null,
      };
    });
  }

  const normalizedTitle = item.title.toLowerCase();
  const resources: NoteResource[] = [];

  if (normalizedTitle.includes("template") || normalizedTitle.includes("模板")) {
    resources.push({
      id: `${item.item_id}_template`,
      label: "模板目录",
      openAction: "open_file",
      path: "workspace/templates",
      type: "模板目录",
    });
  }

  if (normalizedTitle.includes("report") || normalizedTitle.includes("周报") || normalizedTitle.includes("报告")) {
    resources.push({
      id: `${item.item_id}_draft`,
      label: "草稿目录",
      openAction: "open_file",
      path: "workspace/drafts",
      type: "草稿目录",
    });
  }

  if (normalizedTitle.includes("design") || normalizedTitle.includes("设计") || normalizedTitle.includes("page") || normalizedTitle.includes("页面")) {
    resources.push({
      id: `${item.item_id}_ui`,
      label: "仪表盘目录",
      openAction: "reveal_in_folder",
      path: "apps/desktop/src/features/dashboard",
      type: "代码目录",
    });
  }

  return resources;
}

function createFallbackExperience(item: TodoItem): NoteDetailExperience {
  const fallbackNoteType =
    item.bucket === "recurring_rule"
      ? "recurring"
      : item.bucket === "closed"
        ? "archive"
        : item.type === "follow_up"
          ? "follow-up"
          : item.type === "template"
            ? "template"
            : "reminder";

  return {
    agentSuggestion: {
      detail:
        item.agent_suggestion ??
        "当前只拿到了基础便签字段。建议先补一条更明确的上下文，再决定是否转成任务。",
      label: "下一步建议",
    },
    canConvertToTask: item.bucket !== "closed" && !item.linked_task_id,
    detailStatus: getDetailStatus(item),
    detailStatusTone:
      item.status === "overdue"
        ? "overdue"
        : item.status === "completed" || item.status === "cancelled"
          ? "done"
          : "normal",
    effectiveScope:
      item.effective_scope ??
      (item.bucket === "recurring_rule" ? "规则持续生效，直到手动暂停或取消。" : null),
    endedAt: item.ended_at ?? (item.status === "completed" || item.status === "cancelled" ? item.due_at : null),
    isRecurringEnabled: item.bucket === "recurring_rule" ? item.recurring_enabled !== false : false,
    nextOccurrenceAt: item.next_occurrence_at ?? (item.bucket === "recurring_rule" ? item.due_at : null),
    noteText:
      item.note_text ??
      (item.agent_suggestion
        ? `${item.title}。${item.agent_suggestion}`
        : `${item.title}。当前只有基础便签字段，页面会用最小说明承接这条记录。`),
    noteType: fallbackNoteType,
    plannedAt: item.due_at,
    prerequisite:
      item.prerequisite ??
      (item.bucket === "later"
        ? "当前还没有进入处理窗口，先保留上下文即可。"
        : item.bucket === "recurring_rule"
          ? "确认这条规则仍然需要继续生效。"
          : null),
    previewStatus: getPreviewStatus(item),
    recentInstanceStatus: item.recent_instance_status ?? null,
    relatedResources: createResourceHints(item),
    repeatRule:
      item.repeat_rule ??
      (item.bucket === "recurring_rule" ? "协议暂未返回具体重复规则，当前只展示规则条目。" : null),
    summaryLabel: getSummaryLabel(item),
    timeHint: getTimeHint(item),
    title: item.title,
    typeLabel: getTypeLabel(item),
  };
}

function mapItems(items: TodoItem[]): NoteListItem[] {
  return items.map((item) => {
    const experience = getMockNoteExperience(item.item_id) ?? createFallbackExperience(item);

    return {
      experience: {
        ...experience,
        canConvertToTask: experience.canConvertToTask && item.bucket !== "closed" && !item.linked_task_id,
      },
      item,
    };
  });
}

async function withTimeout<T>(promise: Promise<T>, label: string): Promise<T> {
  return Promise.race([
    promise,
    new Promise<T>((_, reject) => {
      window.setTimeout(() => reject(new Error(`${label} 请求超时`)), NOTEPAD_RPC_TIMEOUT_MS);
    }),
  ]);
}

function getMockNoteBucketPage(group: TodoBucket) {
  const buckets = getMockNoteBuckets();
  const items = buckets[group] ?? [];

  return {
    items,
    page: {
      has_more: false,
      limit: items.length,
      offset: 0,
      total: items.length,
    },
  };
}

/**
 * Loads one formal note bucket and maps it into the local note workbench model.
 *
 * @param group Formal note bucket key.
 * @param source Data source mode for the page.
 * @returns The normalized bucket payload.
 */
export async function loadNoteBucket(group: TodoBucket, source: NotePageDataMode = "rpc") {
  if (source === "mock") {
    return getMockNoteBucketPage(group);
  }

  const params: AgentNotepadListParams = {
    group,
    limit: group === "closed" ? 24 : 12,
    offset: 0,
    request_meta: createRequestMeta(`notepad_${group}`),
  };

  const result = await withTimeout(listNotepad(params), `便签分组 ${group}`);
  return {
    items: mapItems(result.items),
    page: result.page,
  };
}

/**
 * Converts a note into a formal task while preserving the note bucket source.
 *
 * @param itemId Stable note id.
 * @param source Data source mode for the page.
 * @returns The convert outcome from RPC or mock mode.
 */
export async function convertNoteToTask(itemId: string, source: NotePageDataMode = "rpc"): Promise<NoteConvertOutcome> {
  if (source === "mock") {
    return runMockConvertNoteToTask(itemId);
  }

  const params: AgentNotepadConvertToTaskParams = {
    confirmed: true,
    item_id: itemId,
    request_meta: createRequestMeta(`notepad_convert_${itemId}`),
  };

  const result = await withTimeout(convertNotepadToTask(params), `将便签 ${itemId} 转成任务`);
  return {
    result,
    source: "rpc",
  };
}

/**
 * Applies a stable notepad mutation action to a note item.
 *
 * @param itemId Stable note id.
 * @param action Stable notepad action.
 * @param source Data source mode for the page.
 * @returns The update outcome from RPC or mock mode.
 */
export async function updateNote(itemId: string, action: NotepadAction, source: NotePageDataMode = "rpc"): Promise<NoteUpdateOutcome> {
  if (source === "mock") {
    return runMockUpdateNote(itemId, action);
  }

  const params: AgentNotepadUpdateParams = {
    action,
    item_id: itemId,
    request_meta: createRequestMeta(`notepad_update_${action}_${itemId}`),
  };

  const result = await withTimeout(updateNotepad(params), `更新便签 ${itemId}（${action}）`);
  return {
    result,
    source: "rpc",
  };
}

/**
 * Maps a note resource descriptor to the shared dashboard open plan shape.
 *
 * @param resource Note resource definition from the page model.
 * @returns A normalized dashboard open execution plan.
 */
export function resolveNoteResourceOpenExecutionPlan(resource: NoteResource): NoteResourceOpenExecutionPlan {
  if (resource.openAction === "task_detail" && resource.taskId) {
    return createDashboardOpenPlan({
      feedback: `已定位到任务 ${resource.label}。`,
      label: resource.label,
      missingTargetMessage: `资源“${resource.label}”缺少可定位的任务。`,
      mode: "task_detail",
      path: resource.path,
      taskId: resource.taskId,
      url: resource.url ?? null,
    });
  }

  if (resource.openAction === "open_url" && resource.url) {
    return createDashboardOpenPlan({
      feedback: `已打开 ${resource.label}。`,
      label: resource.label,
      missingTargetMessage: `资源“${resource.label}”缺少可打开的链接。`,
      mode: "open_url",
      path: resource.path,
      taskId: resource.taskId ?? null,
      url: resource.url,
    });
  }

  if (resource.openAction === "reveal_in_folder") {
    return createDashboardOpenPlan({
      feedback: `已在文件夹中定位 ${resource.label}。`,
      label: resource.label,
      missingTargetMessage: `资源“${resource.label}”缺少可定位的路径。`,
      mode: "reveal_in_folder",
      path: resource.path,
      taskId: resource.taskId ?? null,
      url: resource.url ?? null,
    });
  }

  return createDashboardOpenPlan({
    feedback: `已打开 ${resource.label}。`,
    label: resource.label,
    missingTargetMessage: `当前资源“${resource.label}”没有可打开的地址。`,
    mode: "open_file",
    path: resource.path,
    taskId: resource.taskId ?? null,
    url: resource.url ?? null,
  });
}

/**
 * Executes a normalized note resource open plan.
 *
 * @param plan Shared dashboard open plan.
 * @param options Optional execution overrides.
 * @returns The normalized execution result for the note page.
 */
export async function performNoteResourceOpenExecution(
  plan: NoteResourceOpenExecutionPlan,
  options: NoteResourceOpenExecutionOptions = {},
): Promise<NoteResourceOpenExecutionResult> {
  return performDashboardOpenPlan(plan, options);
}
