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
import { openDesktopLocalPath, revealDesktopLocalPath } from "@/platform/desktopLocalPath";
import { convertNotepadToTask, listNotepad, updateNotepad } from "@/rpc/methods";
import { getMockNoteBuckets, getMockNoteExperience, runMockConvertNoteToTask, runMockUpdateNote } from "./notePage.mock";
import type { NoteConvertOutcome, NoteDetailExperience, NoteListItem, NoteResource, NoteUpdateOutcome, SourceNoteDocument } from "./notePage.types";

const NOTEPAD_RPC_TIMEOUT_MS = 2_500;
export type NotePageDataMode = "rpc" | "mock";

export type NoteResourceOpenExecutionPlan = {
  mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
  feedback: string;
  path: string | null;
  taskId: string | null;
  url: string | null;
};

export type NoteResourceOpenExecutionOptions = {
  onOpenTaskDetail?: (input: {
    plan: NoteResourceOpenExecutionPlan;
    taskId: string;
  }) => Promise<string | void> | string | void;
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

function formatAbsoluteTimestamp(value: number) {
  return new Date(value).toLocaleString("zh-CN", {
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    month: "numeric",
  });
}

function createSourceNoteFallbackId(path: string) {
  let hash = 2166136261;

  for (let index = 0; index < path.length; index += 1) {
    hash ^= path.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }

  return `source_note_${(hash >>> 0).toString(16)}`;
}

function extractSourceNotePreview(content: string) {
  const lines = content
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  const previewLine = lines.find((line) => !/^#+\s*/.test(line)) ?? lines[0] ?? "";

  return previewLine
    .replace(/^[-*+]\s+\[[ xX]\]\s*/, "")
    .replace(/^[-*+]\s*/, "")
    .trim();
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
  try {
    const parsed = new URL(url);
    return parsed.protocol === "https:" || parsed.protocol === "http:";
  } catch {
    return false;
  }
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

  return item.bucket === "later" ? "未到时间" : "近期要做";
}

function getDetailStatus(item: TodoItem) {
  if (item.bucket === "closed") {
    return item.status === "completed" ? "已结束" : "已取消";
  }

  if (item.bucket === "recurring_rule") {
    return item.recurring_enabled === false ? "重复规则已暂停" : "重复规则开启中";
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
  const completedTime = item.ended_at ?? item.due_at;

  if (item.bucket === "closed") {
    return completedTime ? formatAbsoluteTime(completedTime) : "未设置时间";
  }

  if (!item.due_at) {
    return item.bucket === "recurring_rule" ? "规则时间待补充" : "未设置时间";
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
    "follow up": "跟进事项",
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

  if (action === "reveal_in_folder" && payload?.path) {
    return "reveal_in_folder";
  }

  if ((action === "open_file" || action === "workspace_document") && payload?.path) {
    return "open_file";
  }

  return "copy_path";
}

function createResourceHints(item: TodoItem) {
  if (item.related_resources && item.related_resources.length > 0) {
    return item.related_resources.map<NoteResource>((resource) => {
      const payload = resolveResourceOpenPayload(resource);

      return {
        id: resource.resource_id,
        label: resource.label,
        openAction: normalizeResourceOpenAction(resource.open_action ?? null, payload),
        path: resource.path,
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
      label: "关联模板",
      path: "workspace/templates",
      type: "模板目录",
    });
  }

  if (normalizedTitle.includes("report") || normalizedTitle.includes("周报") || normalizedTitle.includes("报告")) {
    resources.push({
      id: `${item.item_id}_report`,
      label: "文档草稿区",
      path: "workspace/drafts",
      type: "草稿目录",
    });
  }

  if (normalizedTitle.includes("design") || normalizedTitle.includes("设计") || normalizedTitle.includes("page") || normalizedTitle.includes("页面")) {
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
        "当前拿到的是协议中的基础便签数据，建议补一条更明确的上下文后再决定是否转交给 Agent。",
      label: "下一步建议",
    },
    canConvertToTask: item.bucket !== "closed" && !item.linked_task_id,
    detailStatus,
    detailStatusTone: item.status === "overdue" ? "overdue" : item.status === "completed" || item.status === "cancelled" ? "done" : "normal",
    effectiveScope:
      item.effective_scope ?? (item.bucket === "recurring_rule" ? "规则持续生效，直到手动暂停或取消。" : null),
    endedAt: item.ended_at ?? (item.status === "completed" || item.status === "cancelled" ? item.due_at : null),
    isRecurringEnabled: item.bucket === "recurring_rule" ? item.recurring_enabled !== false : false,
    nextOccurrenceAt: item.next_occurrence_at ?? (item.bucket === "recurring_rule" ? item.due_at : null),
    noteText:
      item.note_text ??
      (item.agent_suggestion
        ? `${item.title}。${item.agent_suggestion}`
        : `${item.title}。当前只返回了基础便签字段，页面用最小默认说明承接这条事项。`),
    noteType: fallbackNoteType,
    plannedAt: item.due_at,
    previewStatus,
    prerequisite:
      item.prerequisite ??
      (item.bucket === "later"
        ? "当前还没进入处理窗口，先保留上下文即可。"
        : item.bucket === "recurring_rule"
          ? "确认这条规则仍然需要继续生效。"
          : null),
    recentInstanceStatus: item.recent_instance_status ?? null,
    relatedResources: createResourceHints(item),
    repeatRule:
      item.repeat_rule ?? (item.bucket === "recurring_rule" ? "协议暂未返回具体重复规则，当前只展示规则条目。" : null),
    summaryLabel: getSummaryLabel(item),
    timeHint: getTimeHint(item),
    title: item.title,
    typeLabel: getTypeLabel(item),
  };
}

function mapItems(items: TodoItem[]): NoteListItem[] {
  return items.map((item) => ({
    experience: getMockNoteExperience(item.item_id) ?? createFallbackExperience(item),
    item,
  }));
}

/**
 * Groups renderer-local note cards by their formal bucket so source-note
 * fallbacks render in the same columns as synced inspector results.
 *
 * @param items Note cards prepared for the dashboard.
 * @returns Note cards partitioned by notepad bucket.
 */
export function partitionNoteItemsByBucket(items: NoteListItem[]): Record<TodoBucket, NoteListItem[]> {
  return items.reduce<Record<TodoBucket, NoteListItem[]>>(
    (groups, item) => {
      groups[item.item.bucket].push(item);
      return groups;
    },
    {
      closed: [],
      later: [],
      recurring_rule: [],
      upcoming: [],
    },
  );
}

function parseSourceChecklistLine(line: string) {
  const trimmed = line.trimEnd();
  if (/^\s/.test(trimmed)) {
    return null;
  }

  const match = /^[-*]\s+\[( |x|X)\]\s+(.+)$/.exec(trimmed);
  if (!match) {
    return null;
  }

  return {
    checked: match[1].toLowerCase() === "x",
    title: match[2].trim(),
  };
}

function isIndentedSourceLine(line: string) {
  return /^\s/.test(line);
}

function normalizeSourceChecklistBodyLine(line: string) {
  const trimmedRight = line.trimEnd();
  if (trimmedRight.startsWith("  ")) {
    return trimmedRight.slice(2);
  }
  if (trimmedRight.startsWith("\t")) {
    return trimmedRight.slice(1);
  }
  return trimmedRight;
}

function normalizeSourceNaturalNoteLine(line: string) {
  const trimmedRight = line.trimEnd();
  const trimmed = trimmedRight.trim();
  if (trimmed.startsWith("#")) {
    return trimmed.replace(/^#+\s*/, "").trim();
  }
  return trimmedRight;
}

function isSourceNaturalHeadingLine(line: string) {
  return line.trim().startsWith("#") && normalizeSourceNaturalNoteLine(line) !== "";
}

function hasSourceNaturalNoteContent(lines: string[]) {
  return lines.some((line) => line.trim() !== "");
}

// Blank lines inside note bodies are content; only outer blank boundaries are trimmed.
function trimSourceBoundaryBlankLines(lines: string[]) {
  let start = 0;
  while (start < lines.length && lines[start]?.trim() === "") {
    start += 1;
  }

  let end = lines.length;
  while (end > start && lines[end - 1]?.trim() === "") {
    end -= 1;
  }

  return lines.slice(start, end);
}

function joinSourceNoteText(metadataText: string | null, bodyText: string) {
  if (!metadataText) {
    return bodyText;
  }
  if (bodyText === "") {
    return metadataText;
  }
  return `${metadataText}\n\n${bodyText}`;
}

function splitSourceNaturalNoteContent(lines: string[]) {
  const normalized = trimSourceBoundaryBlankLines(lines.map(normalizeSourceNaturalNoteLine));
  if (normalized.length === 0) {
    return null;
  }

  return {
    noteText: trimSourceBoundaryBlankLines(normalized.slice(1)).join("\n"),
    title: normalized[0]?.trim() ?? "",
  };
}

function splitSourceMetadataLine(line: string) {
  const separatorIndex = line.indexOf(":");
  if (separatorIndex <= 0 || separatorIndex >= line.length - 1) {
    return null;
  }

  const key = line.slice(0, separatorIndex).trim().toLowerCase();
  const value = line.slice(separatorIndex + 1).trim();
  if (key === "" || value === "") {
    return null;
  }

  return { key, value };
}

function getDefaultBucketForSourcePath(sourcePath: string | null, checked: boolean, recurring: boolean): TodoBucket {
  if (checked) {
    return "closed";
  }

  const normalized = sourcePath?.trim().toLowerCase() ?? "";
  if (recurring || normalized.includes("recurring") || normalized.includes("weekly") || normalized.includes("repeat")) {
    return "recurring_rule";
  }
  if (normalized.includes("later") || normalized.includes("backlog")) {
    return "later";
  }
  if (normalized.includes("inbox") || normalized.includes("upcoming") || normalized.includes("today") || normalized.includes("urgent")) {
    return "upcoming";
  }
  return "upcoming";
}

type SourceNaturalFallbackFields = {
  bucket: TodoBucket;
  dueAt: string | null;
  nextOccurrenceAt: string | null;
  repeatRule: string | null;
};

function hasSourceNaturalRepeatHint(lowerText: string) {
  return ["every ", "daily", "weekly", "monthly", "repeat", "recurring", "每天", "每日", "每周", "每月", "工作日", "定期", "重复"].some((hint) =>
    lowerText.includes(hint),
  );
}

function hasSourceNaturalLaterHint(lowerText: string) {
  return ["later", "someday", "backlog", "以后", "稍后", "有空", "之后"].some((hint) => lowerText.includes(hint));
}

function extractSourceNaturalRepeatRuleText(text: string) {
  for (const line of text.split("\n")) {
    const normalized = normalizeSourceNaturalNoteLine(line).trim();
    if (normalized === "") {
      continue;
    }
    if (hasSourceNaturalRepeatHint(normalized.toLowerCase())) {
      return normalized;
    }
  }
  return null;
}

function formatSourceNaturalDateTime(now: Date, years: number, months: number, days: number) {
  const target = new Date(now);
  target.setFullYear(target.getFullYear() + years);
  target.setMonth(target.getMonth() + months);
  target.setDate(target.getDate() + days);
  target.setSeconds(0, 0);
  return target.toISOString();
}

function inferSourceNaturalDueTime(lowerText: string, now: Date) {
  if (lowerText.includes("今天") || lowerText.includes("today")) {
    return formatSourceNaturalDateTime(now, 0, 0, 0);
  }
  if (lowerText.includes("明天") || lowerText.includes("tomorrow")) {
    return formatSourceNaturalDateTime(now, 0, 0, 1);
  }
  if (lowerText.includes("后天")) {
    return formatSourceNaturalDateTime(now, 0, 0, 2);
  }
  if (lowerText.includes("下周") || lowerText.includes("next week")) {
    return formatSourceNaturalDateTime(now, 0, 0, 7);
  }
  if (lowerText.includes("下个月") || lowerText.includes("next month")) {
    return formatSourceNaturalDateTime(now, 0, 1, 0);
  }
  return null;
}

function deriveSourceRecurringNextOccurrence(dueAt: string | null, repeatRule: string | null) {
  if (!dueAt) {
    return null;
  }

  const parsedDueAt = new Date(dueAt);
  if (Number.isNaN(parsedDueAt.getTime())) {
    return dueAt;
  }

  const normalizedRule = repeatRule?.toLowerCase() ?? "";
  if (normalizedRule.includes("2 week") || normalizedRule.includes("两周")) {
    parsedDueAt.setDate(parsedDueAt.getDate() + 14);
  } else if (normalizedRule.includes("month") || normalizedRule.includes("每月")) {
    parsedDueAt.setMonth(parsedDueAt.getMonth() + 1);
  } else if (normalizedRule.includes("day") || normalizedRule.includes("每天")) {
    parsedDueAt.setDate(parsedDueAt.getDate() + 1);
  } else {
    parsedDueAt.setDate(parsedDueAt.getDate() + 7);
  }

  return parsedDueAt.toISOString();
}

function normalizeFallbackMetadataTime(value: string | null, now = new Date()) {
  const trimmed = value?.trim() ?? "";
  if (trimmed === "") {
    return null;
  }

  const parsed = new Date(trimmed);
  if (!Number.isNaN(parsed.getTime())) {
    return parsed.toISOString();
  }

  const dateTimeMatch = /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2})$/.exec(trimmed);
  if (dateTimeMatch) {
    const [, year, month, day, hour, minute] = dateTimeMatch;
    const normalized = new Date(now);
    normalized.setFullYear(Number(year), Number(month) - 1, Number(day));
    normalized.setHours(Number(hour), Number(minute), 0, 0);
    return normalized.toISOString();
  }

  const dateMatch = /^(\d{4})-(\d{2})-(\d{2})$/.exec(trimmed);
  if (dateMatch) {
    const [, year, month, day] = dateMatch;
    const normalized = new Date(now);
    normalized.setFullYear(Number(year), Number(month) - 1, Number(day));
    normalized.setSeconds(0, 0);
    return normalized.toISOString();
  }

  return trimmed;
}

function extractSourceNaturalHintScopeText(text: string) {
  for (const line of text.split("\n")) {
    const normalized = normalizeSourceNaturalNoteLine(line).trim();
    if (normalized !== "") {
      return normalized;
    }
  }

  return "";
}

// Renderer fallback cards are local state, but they must infer the same natural
// scheduling hints as the backend inspector to avoid visible pre-sync drift.
function inferSourceNaturalFallbackFields(text: string, sourcePath: string | null, now = new Date()): SourceNaturalFallbackFields {
  const hintScopeText = extractSourceNaturalHintScopeText(text);
  const lowerText = hintScopeText.toLowerCase();
  const dueAt = inferSourceNaturalDueTime(lowerText, now);
  const repeatRule = extractSourceNaturalRepeatRuleText(hintScopeText);

  if (repeatRule) {
    return {
      bucket: "recurring_rule",
      dueAt,
      nextOccurrenceAt: deriveSourceRecurringNextOccurrence(dueAt, repeatRule),
      repeatRule,
    };
  }

  if (dueAt) {
    return {
      bucket: "upcoming",
      dueAt,
      nextOccurrenceAt: null,
      repeatRule: null,
    };
  }

  return {
    bucket: hasSourceNaturalLaterHint(lowerText) ? "later" : getDefaultBucketForSourcePath(sourcePath, false, false),
    dueAt: null,
    nextOccurrenceAt: null,
    repeatRule: null,
  };
}

function normalizeFallbackBucket(value: string | null, checked: boolean, sourcePath: string | null) {
  if (checked) {
    return "closed";
  }
  if (value === "upcoming" || value === "later" || value === "recurring_rule" || value === "closed") {
    return value;
  }

  return getDefaultBucketForSourcePath(sourcePath, checked, false);
}

function normalizeFallbackCheckedState(bucket: string | null, checked: boolean) {
  return checked || valueIsClosedBucket(bucket);
}

function valueIsClosedBucket(bucket: string | null) {
  return bucket?.trim().toLowerCase() === "closed";
}

function inferFallbackStatus(dueAt: string | null, checked: boolean): TodoItem["status"] {
  if (checked) {
    return "completed";
  }

  if (!dueAt) {
    return "normal";
  }

  const parsedDueAt = new Date(dueAt);
  if (Number.isNaN(parsedDueAt.getTime())) {
    return "normal";
  }

  const now = new Date();
  if (parsedDueAt.getTime() < now.getTime()) {
    return "overdue";
  }

  if (
    parsedDueAt.getFullYear() === now.getFullYear()
    && parsedDueAt.getMonth() === now.getMonth()
    && parsedDueAt.getDate() === now.getDate()
  ) {
    return "due_today";
  }

  return "normal";
}

function buildSourceNoteResource(itemId: string, path: string) {
  return {
    label: "源文件",
    open_action: "open_file" as const,
    open_payload: {
      path,
      task_id: null,
      url: null,
    },
    path,
    resource_id: `${itemId}_source`,
    resource_type: "来源文件",
  };
}

/**
 * Builds one renderer-local note card from a markdown source file when the
 * source note exists on disk but has not been surfaced as a formal notepad
 * item yet.
 *
 * @param note Markdown source note from the desktop bridge.
 * @returns A later-bucket note card that keeps the source file visible.
 */
export function buildSourceNoteFallbackItem(note: SourceNoteDocument): NoteListItem {
  const itemId = createSourceNoteFallbackId(note.path);
  const previewText = extractSourceNotePreview(note.content);

  return {
    experience: {
      agentSuggestion: {
        detail: "这张便签已经保存在任务来源目录里。你可以继续编辑它，巡检识别后会切换成正式便签项。",
        label: "下一步建议",
      },
      canConvertToTask: false,
      detailStatus: "等待巡检同步",
      detailStatusTone: "normal",
      effectiveScope: note.sourceRoot,
      endedAt: null,
      isRecurringEnabled: false,
      nextOccurrenceAt: null,
      noteText: previewText || "这张源便签还没有提炼出正式事项，当前先作为本地便签卡片显示。",
      noteType: "reminder",
      plannedAt: null,
      prerequisite: "当前还没有正式巡检结果，这张卡片直接来自任务来源目录中的便签文件。",
      previewStatus: "待巡检",
      recentInstanceStatus: null,
      relatedResources: [
        {
          id: `${itemId}_source`,
          label: "源文件",
          openAction: "open_file",
          path: note.path,
          taskId: null,
          type: "来源文件",
          url: null,
        },
      ],
      repeatRule: null,
      summaryLabel: "源便签",
      timeHint: note.modifiedAtMs ? `最后修改 ${formatAbsoluteTimestamp(note.modifiedAtMs)}` : "刚创建",
      title: note.title,
      typeLabel: "源便签",
    },
    item: {
      agent_suggestion: "等待巡检同步后再进入正式事项流。",
      bucket: "later",
      due_at: null,
      item_id: itemId,
      linked_task_id: null,
      note_text: previewText || note.content.trim() || "新建源便签",
      related_resources: [
        {
          label: "源文件",
          open_action: "open_file",
          open_payload: {
            path: note.path,
            task_id: null,
            url: null,
          },
          path: note.path,
          resource_id: `${itemId}_source`,
          resource_type: "来源文件",
        },
      ],
      status: "normal",
      title: note.title,
      type: "note",
      ended_at: null,
      effective_scope: note.sourceRoot,
      next_occurrence_at: null,
      prerequisite: "等待巡检把源文件识别成正式事项。",
      recent_instance_status: null,
      recurring_enabled: false,
      repeat_rule: null,
    },
    sourceNote: {
      localOnly: true,
      path: note.path,
      sourceLine: null,
      title: note.title,
    },
  };
}

/**
 * Builds one local fallback card for every checklist or natural block found in
 * the source markdown file. The page uses these cards before the backend
 * inspector has finished translating the file into formal notepad items.
 *
 * @param note Markdown source note from the desktop bridge.
 * @returns Renderer-local cards derived from checklist or natural blocks in the file.
 */
export function buildSourceNoteFallbackItems(note: SourceNoteDocument): NoteListItem[] {
  const lines = note.content.replace(/\r\n/g, "\n").split("\n");
  const items: NoteListItem[] = [];
  let naturalLines: string[] = [];
  let naturalStartLine: number | null = null;
  let naturalStartedWithHeading = false;
  let current:
    | {
        agentSuggestion: string | null;
        bodyLines: string[];
        bucket: string | null;
        checked: boolean;
        createdAt: string | null;
        dueAt: string | null;
        effectiveScope: string | null;
        endedAt: string | null;
        nextOccurrenceAt: string | null;
        noteMetadataText: string | null;
        prerequisite: string | null;
        recentInstanceStatus: string | null;
        repeatRule: string | null;
        sourceLine: number;
        title: string;
        updatedAt: string | null;
      }
    | null = null;
  let currentPendingBodySpacer = false;

  const flushCurrent = () => {
    if (!current) {
      return;
    }

    const itemId = createSourceNoteFallbackId(`${note.path}:${current.sourceLine}:${current.title}`);
    const bodyText = trimSourceBoundaryBlankLines(current.bodyLines).join("\n");
    const noteText = joinSourceNoteText(current.noteMetadataText, bodyText) || current.title;
    const checked = normalizeFallbackCheckedState(current.bucket, current.checked);
    const bucket = normalizeFallbackBucket(current.bucket, checked, note.path);
    const dueAt = normalizeFallbackMetadataTime(current.dueAt);
    const endedAt = normalizeFallbackMetadataTime(current.endedAt) ?? (checked ? dueAt : null);
    const nextOccurrenceAt = normalizeFallbackMetadataTime(current.nextOccurrenceAt);
    const item = {
      agent_suggestion: current.agentSuggestion ?? "等待巡检把这张便签同步成正式事项。",
      bucket,
      created_at: current.createdAt,
      due_at: dueAt,
      effective_scope: current.effectiveScope ?? note.sourceRoot,
      ended_at: endedAt,
      item_id: itemId,
      linked_task_id: null,
      next_occurrence_at: nextOccurrenceAt,
      note_text: noteText,
      prerequisite: current.prerequisite,
      recent_instance_status: current.recentInstanceStatus,
      recurring_enabled: bucket === "recurring_rule",
      related_resources: [buildSourceNoteResource(itemId, note.path)],
      repeat_rule: current.repeatRule,
      source_line: current.sourceLine,
      source_path: note.path,
      status: inferFallbackStatus(dueAt, checked),
      title: current.title,
      type: bucket === "recurring_rule" ? "recurring" : "note",
      updated_at: current.updatedAt,
    } as TodoItem;

    items.push({
      experience: createFallbackExperience(item),
      item,
      sourceNote: {
        localOnly: true,
        path: note.path,
        sourceLine: current.sourceLine,
        title: current.title,
      },
    });

    current = null;
    currentPendingBodySpacer = false;
  };
  const flushNatural = () => {
    const naturalContent = splitSourceNaturalNoteContent(naturalLines);
    if (naturalContent && naturalStartLine !== null) {
      const inferred = inferSourceNaturalFallbackFields(naturalLines.join("\n"), note.path);
      current = {
        agentSuggestion: null,
        bodyLines: naturalContent.noteText ? [naturalContent.noteText] : [],
        bucket: inferred.bucket,
        checked: false,
        createdAt: null,
        dueAt: inferred.dueAt,
        effectiveScope: null,
        endedAt: null,
        nextOccurrenceAt: inferred.nextOccurrenceAt,
        noteMetadataText: null,
        prerequisite: null,
        recentInstanceStatus: null,
        repeatRule: inferred.repeatRule,
        sourceLine: naturalStartLine,
        title: naturalContent.title,
        updatedAt: null,
      };
      flushCurrent();
    }
    naturalLines = [];
    naturalStartLine = null;
    naturalStartedWithHeading = false;
  };

  lines.forEach((line, index) => {
    const checklist = parseSourceChecklistLine(line);
    // Only top-level checklist rows start fallback cards; indented rows stay in
    // the current card body to match editor serialization and backend parsing.
    if (checklist) {
      flushNatural();
      flushCurrent();
      current = {
        agentSuggestion: null,
        bodyLines: [],
        bucket: null,
        checked: checklist.checked,
        createdAt: null,
        dueAt: null,
        effectiveScope: null,
        endedAt: null,
        nextOccurrenceAt: null,
        noteMetadataText: null,
        prerequisite: null,
        recentInstanceStatus: null,
        repeatRule: null,
        sourceLine: index + 1,
        title: checklist.title,
        updatedAt: null,
      };
      currentPendingBodySpacer = false;
      return;
    }

    if (!current) {
      if (isSourceNaturalHeadingLine(line) && hasSourceNaturalNoteContent(naturalLines)) {
        flushNatural();
      }
      const naturalLine = normalizeSourceNaturalNoteLine(line);
      if (naturalLine.trim() === "") {
        if (hasSourceNaturalNoteContent(naturalLines)) {
          if (naturalStartedWithHeading) {
            naturalLines.push("");
          } else {
            // Renderer-local fallback cards must follow the same paragraph
            // boundary rule as the backend inspector for plain-text notes.
            flushNatural();
          }
        }
        return;
      }
      if (naturalStartLine === null) {
        naturalStartLine = index + 1;
        naturalStartedWithHeading = isSourceNaturalHeadingLine(line);
      }
      naturalLines.push(naturalLine);
      return;
    }

    const trimmed = line.trim();
    if (trimmed === "") {
      if (current.bodyLines.length > 0) {
        current.bodyLines.push("");
      } else {
        // Checklist blocks may use one blank spacer before metadata; keep that
        // spacer outside note_text until real body content starts.
        currentPendingBodySpacer = true;
      }
      return;
    }

    const metadata =
      current.bodyLines.length === 0 && (!currentPendingBodySpacer || !isIndentedSourceLine(line))
        ? splitSourceMetadataLine(trimmed)
        : null;
    if (!metadata) {
      currentPendingBodySpacer = false;
      current.bodyLines.push(normalizeSourceChecklistBodyLine(line));
      return;
    }

    currentPendingBodySpacer = false;
    switch (metadata.key) {
      case "agent":
      case "suggest":
        current.agentSuggestion = metadata.value;
        return;
      case "bucket":
        current.bucket = metadata.value;
        return;
      case "created_at":
        current.createdAt = normalizeFallbackMetadataTime(metadata.value) ?? metadata.value;
        return;
      case "due":
        current.dueAt = normalizeFallbackMetadataTime(metadata.value) ?? metadata.value;
        return;
      case "ended_at":
        current.endedAt = normalizeFallbackMetadataTime(metadata.value) ?? metadata.value;
        return;
      case "next":
        current.nextOccurrenceAt = normalizeFallbackMetadataTime(metadata.value) ?? metadata.value;
        return;
      case "note":
        current.noteMetadataText = metadata.value;
        return;
      case "prerequisite":
        current.prerequisite = metadata.value;
        return;
      case "repeat":
        current.repeatRule = metadata.value;
        current.bucket = "recurring_rule";
        return;
      case "scope":
        current.effectiveScope = metadata.value;
        return;
      case "status":
        current.recentInstanceStatus = metadata.value;
        return;
      case "updated_at":
        current.updatedAt = normalizeFallbackMetadataTime(metadata.value) ?? metadata.value;
        return;
      default:
        current.bodyLines.push(normalizeSourceChecklistBodyLine(line));
    }
  });

  flushCurrent();
  flushNatural();
  return items.length > 0 ? items : [buildSourceNoteFallbackItem(note)];
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

export async function convertNoteToTask(itemId: string, source: NotePageDataMode = "rpc"): Promise<NoteConvertOutcome> {
  if (source === "mock") {
    return runMockConvertNoteToTask(itemId);
  }

  const params: AgentNotepadConvertToTaskParams = {
    confirmed: true,
    item_id: itemId,
    request_meta: createRequestMeta(`notepad_convert_${itemId}`),
  };

  const result = await withTimeout(convertNotepadToTask(params), `将便签 ${itemId} 转为任务`);
  return {
    result,
    source: "rpc",
  };
}

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
 * Converts a note resource reference into a renderer-side execution plan that
 * stays aligned with the formal task/detail open semantics.
 *
 * @param resource Note resource reference prepared for the dashboard view.
 * @returns The renderer execution plan for the selected resource.
 */
export function resolveNoteResourceOpenExecutionPlan(resource: NoteResource): NoteResourceOpenExecutionPlan {
  if (resource.openAction === "task_detail" && resource.taskId) {
    return {
      feedback: `已定位到任务 ${resource.label}。`,
      mode: "task_detail",
      path: resource.path,
      taskId: resource.taskId,
      url: resource.url ?? null,
    };
  }

  if (resource.openAction === "open_url" && resource.url) {
    return {
      feedback: `已打开 ${resource.label}。`,
      mode: "open_url",
      path: resource.path || null,
      taskId: resource.taskId ?? null,
      url: resource.url,
    };
  }

  if (resource.openAction === "reveal_in_folder" && resource.path) {
    return {
      feedback: `已在文件夹中定位 ${resource.label}。`,
      mode: "reveal_local_path",
      path: resource.path || null,
      taskId: resource.taskId ?? null,
      url: resource.url ?? null,
    };
  }

  if (resource.openAction === "open_file" && resource.path) {
    return {
      feedback: `已打开 ${resource.label}。`,
      mode: "open_local_path",
      path: resource.path || null,
      taskId: resource.taskId ?? null,
      url: resource.url ?? null,
    };
  }

  return {
    feedback: resource.path || resource.url ? `已准备 ${resource.label} 的地址。` : `当前资源 ${resource.label} 缺少可打开地址。`,
    mode: "copy_path",
    path: resource.path || resource.url || null,
    taskId: resource.taskId ?? null,
    url: resource.url ?? null,
  };
}

async function copyPreparedPath(feedback: string, path: string | null) {
  if (!path) {
    return feedback;
  }

  if (globalThis.navigator?.clipboard?.writeText) {
    try {
      await globalThis.navigator.clipboard.writeText(path);
      return `${feedback} 已复制路径。`;
    } catch {
      return `${feedback} 路径：${path}`;
    }
  }

  return `${feedback} 路径：${path}`;
}

function localPathExecutionFailure(message: string, error: unknown) {
  const detail = error instanceof Error ? error.message.trim() : "";
  if (!detail) {
    return message;
  }

  return `${message}（${detail}）`;
}

/**
 * Executes a note resource open plan while preserving task-detail routing and
 * copy-path fallback inside the same renderer entry.
 *
 * @param plan Renderer-side execution plan for the selected note resource.
 * @param options Optional task-detail delegate for callers that need to route into a view.
 * @returns User-facing feedback describing the completed action or fallback.
 */
export async function performNoteResourceOpenExecution(
  plan: NoteResourceOpenExecutionPlan,
  options: NoteResourceOpenExecutionOptions = {},
): Promise<string> {
  if (plan.mode === "task_detail" && plan.taskId) {
    const detailFeedback = await options.onOpenTaskDetail?.({
      plan,
      taskId: plan.taskId,
    });

    return typeof detailFeedback === "string" && detailFeedback.trim() !== ""
      ? detailFeedback
      : plan.feedback;
  }

  if (plan.mode === "open_url" && plan.url) {
    if (!isAllowedNoteOpenUrl(plan.url)) {
      return "已拦截不受支持的便签资源链接。";
    }

    window.open(plan.url, "_blank", "noopener,noreferrer");
    return plan.feedback;
  }

  if (plan.mode === "open_local_path" && plan.path) {
    try {
      await openDesktopLocalPath(plan.path);
      return plan.feedback;
    } catch (error) {
      return copyPreparedPath(localPathExecutionFailure("无法直接打开本地资源，已准备复制路径", error), plan.path);
    }
  }

  if (plan.mode === "reveal_local_path" && plan.path) {
    try {
      await revealDesktopLocalPath(plan.path);
      return plan.feedback;
    } catch (error) {
      return copyPreparedPath(localPathExecutionFailure("无法在文件夹中定位资源，已准备复制路径", error), plan.path);
    }
  }

  if (plan.mode === "copy_path" && plan.path) {
    return copyPreparedPath(plan.feedback, plan.path);
  }

  return plan.feedback;
}
