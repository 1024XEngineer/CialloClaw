import type {
  AgentNotepadConvertToTaskParams,
  AgentNotepadListParams,
  RequestMeta,
  TodoBucket,
  TodoItem,
} from "@cialloclaw/protocol";
import { isRpcChannelUnavailable, logRpcMockFallback } from "@/rpc/fallback";
import { convertNotepadToTask, listNotepad } from "@/rpc/methods";
import { getMockNoteBuckets, getMockNoteExperience, runMockConvertNoteToTask } from "./notePage.mock";
import type { NoteConvertOutcome, NoteDetailExperience, NoteListItem, NoteResource } from "./notePage.types";

const NOTEPAD_RPC_TIMEOUT_MS = 2_500;
export type NotePageDataMode = "rpc" | "mock";

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

function getPreviewStatus(item: TodoItem) {
  if (item.bucket === "closed") {
    return item.status === "completed" ? "已完成" : "已取消";
  }

  if (item.bucket === "recurring_rule") {
    return "规则生效中";
  }

  if (item.status === "overdue") {
    return "已逾期";
  }

  if (item.status === "due_today") {
    return "今天要做";
  }

  return item.bucket === "later" ? "未到时间" : "近期要做";
}

function getDetailStatus(item: TodoItem) {
  if (item.bucket === "closed") {
    return item.status === "completed" ? "已结束" : "已取消";
  }

  if (item.bucket === "recurring_rule") {
    return "重复规则开启中";
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
  if (!item.due_at) {
    return item.bucket === "recurring_rule" ? "规则时间待补充" : "未设置时间";
  }

  if (item.bucket === "closed") {
    return formatAbsoluteTime(item.due_at);
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

  if (!normalizedType) {
    return item.bucket === "recurring_rule" ? "重复事项" : "便签事项";
  }

  return normalizedType
    .split(/\s+/)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

function createResourceHints(item: TodoItem) {
  if (item.related_resources && item.related_resources.length > 0) {
    return item.related_resources.map<NoteResource>((resource) => ({
      id: resource.resource_id,
      label: resource.label,
      path: resource.path,
      type: resource.resource_type,
    }));
  }

  const normalizedTitle = item.title.toLowerCase();
  const resources: NoteResource[] = [];

  if (normalizedTitle.includes("模板")) {
    resources.push({
      id: `${item.item_id}_template`,
      label: "关联模板",
      path: "workspace/templates",
      type: "模板目录",
    });
  }

  if (normalizedTitle.includes("周报") || normalizedTitle.includes("报告")) {
    resources.push({
      id: `${item.item_id}_report`,
      label: "文档草稿区",
      path: "workspace/drafts",
      type: "草稿目录",
    });
  }

  if (normalizedTitle.includes("设计") || normalizedTitle.includes("交互") || normalizedTitle.includes("页面")) {
    resources.push({
      id: `${item.item_id}_ui`,
      label: "Dashboard 前端目录",
      path: "apps/desktop/src/features/dashboard",
      type: "代码目录",
    });
  }

  return resources;
}

function createFallbackExperience(item: TodoItem): NoteDetailExperience {
  const previewStatus = getPreviewStatus(item);
  const detailStatus = getDetailStatus(item);

  return {
    agentSuggestion: {
      detail: item.agent_suggestion ?? "当前拿到的是协议中的基础便签数据，建议补一条更明确的上下文后再决定是否转交给 Agent。",
      label: "下一步建议",
    },
    canConvertToTask: item.bucket !== "closed" && !item.linked_task_id,
    detailStatus,
    detailStatusTone: item.status === "overdue" ? "overdue" : item.status === "completed" || item.status === "cancelled" ? "done" : "normal",
    effectiveScope: item.effective_scope ?? (item.bucket === "recurring_rule" ? "规则持续生效，直到手动暂停或取消。" : null),
    endedAt: item.ended_at ?? (item.status === "completed" || item.status === "cancelled" ? item.due_at : null),
    isRecurringEnabled: item.bucket === "recurring_rule",
    nextOccurrenceAt: item.next_occurrence_at ?? (item.bucket === "recurring_rule" ? item.due_at : null),
    noteText: item.note_text ?? (item.agent_suggestion
      ? `${item.title}。当前已同步到便签页，建议先按提示整理上下文，再视情况转成正式任务。`
      : `${item.title}。当前只返回了基础便签字段，页面用最小默认说明承接这条事项。`),
    noteType: item.bucket === "recurring_rule" ? "recurring" : item.bucket === "closed" ? "archive" : "reminder",
    plannedAt: item.due_at,
    previewStatus,
    prerequisite: item.prerequisite ?? (item.bucket === "later" ? "当前还没进入处理窗口，先保留上下文即可。" : item.bucket === "recurring_rule" ? "确认这条规则仍然需要继续生效。" : null),
    recentInstanceStatus: item.recent_instance_status ?? null,
    relatedResources: createResourceHints(item),
    repeatRule: item.repeat_rule ?? (item.bucket === "recurring_rule" ? "协议暂未返回具体重复规则，当前只展示规则条目。" : null),
    summaryLabel: getSummaryLabel(item),
    timeHint: getTimeHint(item),
    title: item.title,
    typeLabel: getTypeLabel(item),
  };
}

function mapItems(items: Awaited<ReturnType<typeof listNotepad>>["items"]): NoteListItem[] {
  return items.map((item) => ({
    experience: getMockNoteExperience(item.item_id) ?? createFallbackExperience(item),
    item,
  }));
}

async function withTimeout<T>(promise: Promise<T>, label: string): Promise<T> {
  return Promise.race([
    promise,
    new Promise<T>((_, reject) => {
      window.setTimeout(() => reject(new Error(`${label} request timed out`)), NOTEPAD_RPC_TIMEOUT_MS);
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

export async function loadNoteBucket(group: TodoBucket, source: NotePageDataMode = "rpc") {
  if (source === "mock") {
    return getMockNoteBucketPage(group);
  }

  try {
    const params: AgentNotepadListParams = {
      group,
      limit: group === "closed" ? 24 : 12,
      offset: 0,
      request_meta: createRequestMeta(`notepad_${group}`),
    };

    const result = await withTimeout(listNotepad(params), `notepad bucket ${group}`);
    return {
      items: mapItems(result.items),
      page: result.page,
    };
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      logRpcMockFallback(`notepad bucket ${group}`, error);
      return getMockNoteBucketPage(group);
    }

    throw error;
  }
}

export async function convertNoteToTask(itemId: string, source: NotePageDataMode = "rpc"): Promise<NoteConvertOutcome> {
  if (source === "mock") {
    return runMockConvertNoteToTask(itemId);
  }

  const params: AgentNotepadConvertToTaskParams = {
    confirmed: true,
    item_id: itemId,
    request_meta: createRequestMeta(`notepad_convert_${itemId}`),
  };

  try {
    const result = await withTimeout(convertNotepadToTask(params), `convert note ${itemId} to task`);
    return {
      result,
      source: "rpc",
    };
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      logRpcMockFallback(`convert note ${itemId} to task`, error);
      return runMockConvertNoteToTask(itemId);
    }

    throw error;
  }
}
