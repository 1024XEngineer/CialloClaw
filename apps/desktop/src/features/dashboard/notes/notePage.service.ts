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
import { openDesktopExternalUrl } from "@/platform/desktopExternalUrl";
import { openDesktopLocalPath, revealDesktopLocalPath } from "@/platform/desktopLocalPath";
import { convertNotepadToTask, listNotepad, updateNotepad } from "@/rpc/methods";
import { isDashboardTaskDeliveryHref } from "../tasks/taskDeliveryNavigation";
import type { NoteConvertOutcome, NoteDetailExperience, NoteListItem, NoteResource, NoteUpdateOutcome, SourceNoteDocument } from "./notePage.types";
import { sanitizeSourceNoteBodyText } from "./sourceNoteEditor";

const NOTEPAD_RPC_TIMEOUT_MS = 2_500;
const NOTE_HIDDEN_METADATA_KEYS = new Set([
  "agent",
  "bucket",
  "created_at",
  "due",
  "ended_at",
  "next",
  "note",
  "prerequisite",
  "recent_instance_status",
  "reminder",
  "repeat",
  "resource",
  "recurring_enabled",
  "scope",
  "status",
  "suggest",
  "tags",
  "updated_at",
]);

export type NotePageDataMode = "rpc";

export type NoteResourceOpenExecutionPlan = {
  mode: "task_detail" | "open_result_page" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
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
  onOpenResultPage?: (input: {
    plan: NoteResourceOpenExecutionPlan;
    taskId: string | null;
    url: string;
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

function trimBoundaryBlankLines(lines: string[]) {
  let start = 0;
  let end = lines.length;

  while (start < end && lines[start]?.trim() === "") {
    start += 1;
  }

  while (end > start && lines[end - 1]?.trim() === "") {
    end -= 1;
  }

  return lines.slice(start, end);
}

function createSourceNoteFallbackId(path: string) {
  let hash = 2166136261;

  for (let index = 0; index < path.length; index += 1) {
    hash ^= path.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }

  return `source_note_${(hash >>> 0).toString(16)}`;
}

/**
 * Converts note text that may still contain hidden markdown header metadata
 * into the content-only text that should be rendered in note cards and detail
 * panels. Header metadata stays hidden until the body begins.
 *
 * @param value Raw note text from protocol or markdown-derived fallback data.
 * @param options Optional title context used to collapse title-only echoes.
 * @returns User-visible note content with hidden header metadata removed.
 */
export function buildVisibleNoteText(
  value: string | null | undefined,
  options: {
    title?: string | null;
  } = {},
) {
  if (!value) {
    return "";
  }

  const normalizedLines = value.replace(/\r\n/g, "\n").split("\n");
  const visibleLines: string[] = [];
  let bodyStarted = false;

  normalizedLines.forEach((rawLine) => {
    const line = rawLine.trimEnd();
    const trimmed = line.trim();

    if (bodyStarted) {
      visibleLines.push(line);
      return;
    }

    if (trimmed === "") {
      return;
    }

    const metadata = splitSourceMetadataLine(trimmed);
    if (metadata && NOTE_HIDDEN_METADATA_KEYS.has(metadata.key)) {
      if (metadata.key === "note") {
        visibleLines.push(metadata.value);
        bodyStarted = true;
      }
      return;
    }

    visibleLines.push(line);
    bodyStarted = true;
  });

  return sanitizeSourceNoteBodyText(trimBoundaryBlankLines(visibleLines).join("\n"), options);
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

function isAllowedNoteResultPageUrl(url: string): boolean {
  return isDashboardTaskDeliveryHref(url) || isAllowedNoteOpenUrl(url);
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
    return "result_page";
  }

  if (payload?.url) {
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

  return [];
}

function createFallbackExperience(item: TodoItem): NoteDetailExperience {
  const previewStatus = getPreviewStatus(item);
  const detailStatus = getDetailStatus(item);
  const visibleNoteText = buildVisibleNoteText(item.note_text, { title: item.title });
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
        "这条便签会按当前正文直接生成任务；如果还想补充路径、时间或说明，可以继续写在正文里后再转交给 Agent。",
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
      visibleNoteText !== ""
        ? visibleNoteText
        : item.note_text
          ? ""
          : item.agent_suggestion
            ? `${item.title}。${item.agent_suggestion}`
            : `${item.title}。当前只返回了基础便签字段，页面用最小默认说明承接这条事项。`,
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
    experience: createFallbackExperience(item),
    item,
  }));
}

function parseSourceChecklistLine(line: string) {
  const trimmed = line.trim();
  const match = /^[-*]\s+\[( |x|X)\]\s+(.+)$/.exec(trimmed);
  if (!match) {
    return null;
  }

  return {
    checked: match[1].toLowerCase() === "x",
    title: match[2].trim(),
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

function normalizeFallbackBucket(value: string | null) {
  if (value === "upcoming" || value === "later" || value === "recurring_rule" || value === "closed") {
    return value;
  }

  return "later";
}

function createSourceNoteFallbackExperience(input: {
  agentSuggestion: string;
  checked: boolean;
  effectiveScope: string | null;
  noteText: string;
  plannedAt: string | null;
  recentInstanceStatus: string | null;
  relatedResources: NoteDetailExperience["relatedResources"];
  repeatRule: string | null;
  summaryLabel: string;
  timeHint: string;
  title: string;
}) {
  const detailStatus = input.checked ? "已勾选，等待巡检同步" : "等待巡检同步";

  return {
    agentSuggestion: {
      detail: input.agentSuggestion,
      label: "下一步建议",
    },
    canConvertToTask: false,
    detailStatus,
    detailStatusTone: input.checked ? "done" : "normal",
    effectiveScope: input.effectiveScope,
    endedAt: null,
    isRecurringEnabled: input.repeatRule !== null,
    nextOccurrenceAt: input.repeatRule ? input.plannedAt : null,
    noteText: input.noteText,
    noteType: input.repeatRule ? "recurring" : "reminder",
    plannedAt: input.plannedAt,
    prerequisite: "当前还没有正式巡检结果，这条记录仍停留在桌面本地同步态。",
    previewStatus: input.checked ? "已勾选" : "待巡检",
    recentInstanceStatus: input.recentInstanceStatus,
    relatedResources: input.relatedResources,
    repeatRule: input.repeatRule,
    summaryLabel: input.summaryLabel,
    timeHint: input.timeHint,
    title: input.title,
    typeLabel: input.repeatRule ? "源重复事项" : "源便签",
  } satisfies NoteDetailExperience;
}

function buildSourceNoteResource(itemId: string, path: string) {
  return {
    label: "源 markdown",
    open_action: "open_file" as const,
    open_payload: {
      path,
      task_id: null,
      url: null,
    },
    path,
    resource_id: `${itemId}_source`,
    resource_type: "Markdown 文件",
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
  const sourceResources: NoteDetailExperience["relatedResources"] = [
    {
      id: `${itemId}_source`,
      label: "源 markdown",
      openAction: "open_file",
      path: note.path,
      taskId: null,
      type: "Markdown 文件",
      url: null,
    },
  ];

  return {
    experience: createSourceNoteFallbackExperience({
      agentSuggestion: "这张便签已经保存在任务来源目录里。你可以继续编辑它，巡检识别后会切换成正式便签项。",
      checked: false,
      effectiveScope: note.sourceRoot,
      noteText: previewText || "这张源便签还没有提炼出正式事项，当前先作为本地便签卡片显示。",
      plannedAt: null,
      recentInstanceStatus: null,
      relatedResources: sourceResources,
      repeatRule: null,
      summaryLabel: "源便签",
      timeHint: note.modifiedAtMs ? `最后修改 ${formatAbsoluteTimestamp(note.modifiedAtMs)}` : "刚创建",
      title: note.title,
    }),
    item: {
      agent_suggestion: "等待巡检同步后再进入正式事项流。",
      bucket: "later",
      due_at: null,
      item_id: itemId,
      linked_task_id: null,
      note_text: previewText || note.content.trim() || "新建源便签",
      related_resources: [
        {
          label: "源 markdown",
          open_action: "open_file",
          open_payload: {
            path: note.path,
            task_id: null,
            url: null,
          },
          path: note.path,
          resource_id: `${itemId}_source`,
          resource_type: "Markdown 文件",
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
 * Builds one local fallback card for every checklist block found in the source
 * markdown file. These cards stay renderer-local and intentionally avoid
 * recreating formal bucket/status inference before the backend inspector has
 * translated the source file into stable notepad items.
 *
 * @param note Markdown source note from the desktop bridge.
 * @returns Renderer-local cards derived from checklist blocks in the file.
 */
export function buildSourceNoteFallbackItems(note: SourceNoteDocument): NoteListItem[] {
  const lines = note.content.replace(/\r\n/g, "\n").split("\n");
  const items: NoteListItem[] = [];
  let current:
    | {
        agentSuggestion: string | null;
        bodyLines: string[];
        bucket: string | null;
        checked: boolean;
        dueAt: string | null;
        effectiveScope: string | null;
        nextOccurrenceAt: string | null;
        noteText: string | null;
        prerequisite: string | null;
        recentInstanceStatus: string | null;
        repeatRule: string | null;
        sourceLine: number;
        title: string;
      }
    | null = null;

  const flushCurrent = () => {
    if (!current) {
      return;
    }

    const itemId = createSourceNoteFallbackId(`${note.path}:${current.sourceLine}:${current.title}`);
    const noteText = current.noteText ?? (current.bodyLines.join("\n").trim() || current.title);
    const bucket = normalizeFallbackBucket(current.bucket);
    const dueAt = current.nextOccurrenceAt ?? current.dueAt;
    const sourceResources: NoteDetailExperience["relatedResources"] = [
      {
        id: `${itemId}_source`,
        label: "源 markdown",
        openAction: "open_file",
        path: note.path,
        taskId: null,
        type: "Markdown 文件",
        url: null,
      },
    ];
    const item = {
      agent_suggestion: current.agentSuggestion ?? "等待巡检把这个 markdown 便签块同步成正式事项。",
      bucket,
      due_at: dueAt,
      effective_scope: current.effectiveScope ?? note.sourceRoot,
      ended_at: null,
      item_id: itemId,
      linked_task_id: null,
      next_occurrence_at: current.nextOccurrenceAt,
      note_text: noteText,
      prerequisite: current.prerequisite,
      recent_instance_status: current.recentInstanceStatus,
      recurring_enabled: current.repeatRule !== null,
      related_resources: [buildSourceNoteResource(itemId, note.path)],
      repeat_rule: current.repeatRule,
      source_line: current.sourceLine,
      source_path: note.path,
      status: current.checked ? "completed" : "normal",
      title: current.title,
      type: current.repeatRule !== null ? "recurring" : "note",
    } as TodoItem;

    items.push({
      experience: createSourceNoteFallbackExperience({
        agentSuggestion: current.agentSuggestion ?? "这条 markdown 事项还在等待巡检同步，正式识别后才会进入便签流程。",
        checked: current.checked,
        effectiveScope: current.effectiveScope ?? note.sourceRoot,
        noteText,
        plannedAt: dueAt,
        recentInstanceStatus: current.recentInstanceStatus,
        relatedResources: sourceResources,
        repeatRule: current.repeatRule,
        summaryLabel: current.repeatRule ? "源规则" : "源便签",
        timeHint: dueAt ? `计划 ${formatAbsoluteTime(dueAt)}` : `来自 ${note.fileName}`,
        title: current.title,
      }),
      item,
      sourceNote: {
        localOnly: true,
        path: note.path,
        sourceLine: current.sourceLine,
        title: current.title,
      },
    });

    current = null;
  };

  lines.forEach((line, index) => {
    const checklist = parseSourceChecklistLine(line);
    if (checklist) {
      flushCurrent();
      current = {
        agentSuggestion: null,
        bodyLines: [],
        bucket: null,
        checked: checklist.checked,
        dueAt: null,
        effectiveScope: null,
        nextOccurrenceAt: null,
        noteText: null,
        prerequisite: null,
        recentInstanceStatus: null,
        repeatRule: null,
        sourceLine: index + 1,
        title: checklist.title,
      };
      return;
    }

    if (!current) {
      return;
    }

    const trimmed = line.trim();
    if (trimmed === "") {
      return;
    }

    const metadata = splitSourceMetadataLine(trimmed);
    if (!metadata) {
      current.bodyLines.push(trimmed);
      return;
    }

    switch (metadata.key) {
      case "agent":
      case "suggest":
        current.agentSuggestion = metadata.value;
        return;
      case "bucket":
        current.bucket = metadata.value;
        return;
      case "due":
        current.dueAt = metadata.value;
        return;
      case "next":
        current.nextOccurrenceAt = metadata.value;
        return;
      case "note":
        current.noteText = metadata.value;
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
      default:
        current.bodyLines.push(trimmed);
    }
  });

  flushCurrent();
  return items.length > 0 ? items : [buildSourceNoteFallbackItem(note)];
}

async function withTimeout<T>(promise: Promise<T>, label: string): Promise<T> {
  return Promise.race([
    promise,
    new Promise<T>((_, reject) => {
      globalThis.setTimeout(() => reject(new Error(`${label} 请求超时`)), NOTEPAD_RPC_TIMEOUT_MS);
    }),
  ]);
}

export async function loadNoteBucket(group: TodoBucket, _source: NotePageDataMode = "rpc") {
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

export async function convertNoteToTask(itemId: string, _source: NotePageDataMode = "rpc"): Promise<NoteConvertOutcome> {
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

export async function updateNote(itemId: string, action: NotepadAction, _source: NotePageDataMode = "rpc"): Promise<NoteUpdateOutcome> {
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

  if (resource.openAction === "result_page" && resource.url) {
    return {
      feedback: `已打开 ${resource.label} 结果页。`,
      mode: "open_result_page",
      path: resource.path || null,
      taskId: resource.taskId ?? null,
      url: resource.url,
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

<<<<<<< HEAD
function externalUrlExecutionFailure(message: string, error: unknown) {
  const detail = error instanceof Error ? error.message.trim() : "";
  if (!detail) {
    return message;
  }

  return `${message}（${detail}）`;
=======
async function copyPreparedUrl(feedback: string, url: string) {
  if (globalThis.navigator?.clipboard?.writeText) {
    try {
      await globalThis.navigator.clipboard.writeText(url);
      return `${feedback} 已复制链接。`;
    } catch {
      return `${feedback} 链接：${url}`;
    }
  }

  return `${feedback} 链接：${url}`;
>>>>>>> b7062f9e (fix(desktop): harden external link open flows)
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

  if (plan.mode === "open_result_page" && plan.url) {
    if (!isAllowedNoteResultPageUrl(plan.url)) {
      return "已拦截不受支持的便签结果页链接。";
    }

    const resultPageFeedback = await options.onOpenResultPage?.({
      plan,
      taskId: plan.taskId,
      url: plan.url,
    });

    if (typeof resultPageFeedback === "string" && resultPageFeedback.trim() !== "") {
      return resultPageFeedback;
    }

    try {
      await openDesktopExternalUrl(plan.url);
      return plan.feedback;
    } catch (error) {
      return externalUrlExecutionFailure("无法通过系统浏览器打开便签结果页链接", error);
    }
  }

  if (plan.mode === "open_url" && plan.url) {
    if (!isAllowedNoteOpenUrl(plan.url)) {
      return "已拦截不受支持的便签资源链接。";
    }

    try {
      await openDesktopExternalUrl(plan.url);
      return plan.feedback;
    } catch (error) {
      return externalUrlExecutionFailure("无法通过系统浏览器打开便签资源链接", error);
    }
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
