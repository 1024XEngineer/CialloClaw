import type { TodoItem } from "@cialloclaw/protocol";
import type { NoteBucketsData, NoteConvertOutcome, NoteDetailExperience, NoteListItem, NoteUpdateOutcome } from "./notePage.types";

const HOUR = 1000 * 60 * 60;
const DAY = HOUR * 24;
const now = Date.now();

function iso(offset: number) {
  return new Date(now + offset).toISOString();
}

function clone<T>(value: T) {
  return structuredClone(value);
}

const baseItems: TodoItem[] = [
  {
    item_id: "note_upcoming_001",
    title: "今天下班前整理周报模板",
    bucket: "upcoming",
    status: "due_today",
    type: "template",
    due_at: iso(5 * HOUR),
    agent_suggestion: "先把上周模板打开，再补本周核心数据和结论。",
  },
  {
    item_id: "note_upcoming_002",
    title: "联系设计师确认首页球体交互排期",
    bucket: "upcoming",
    status: "normal",
    type: "follow_up",
    due_at: iso(2 * DAY),
    agent_suggestion: "先整理出三条要确认的问题，再发过去。",
  },
  {
    item_id: "note_upcoming_003",
    title: "给安全页补一版风险摘要文案",
    bucket: "upcoming",
    status: "overdue",
    type: "reminder",
    due_at: iso(-8 * HOUR),
    agent_suggestion: "先写三条更短的风险摘要，再挑一条放进页面。",
  },
  {
    item_id: "note_later_001",
    title: "月底前整理一次桌面端 UI token",
    bucket: "later",
    status: "normal",
    type: "reminder",
    due_at: iso(11 * DAY),
    agent_suggestion: "先继续积累样式片段，月底再统一整理。",
  },
  {
    item_id: "note_later_002",
    title: "镜子页稳定后补跨页联动演示",
    bucket: "later",
    status: "normal",
    type: "follow_up",
    due_at: iso(18 * DAY),
    agent_suggestion: "这条已经生成关联任务，后续只需要回看任务推进。",
    linked_task_id: "task_focus_001",
  },
  {
    item_id: "note_recurring_001",
    title: "每周一整理周报",
    bucket: "recurring_rule",
    status: "normal",
    type: "recurring",
    due_at: null,
    agent_suggestion: "周一早上汇总材料，中午前生成初稿。",
    recurring_enabled: true,
  },
  {
    item_id: "note_recurring_002",
    title: "工作日 09:00 巡检邮件",
    bucket: "recurring_rule",
    status: "normal",
    type: "recurring",
    due_at: null,
    agent_suggestion: "只保留重要邮件，其余自动归档。",
    recurring_enabled: true,
  },
  {
    item_id: "note_closed_001",
    title: "导出铃兰首页交互版截图",
    bucket: "closed",
    status: "completed",
    type: "archive",
    due_at: iso(-2 * DAY),
    agent_suggestion: "已经完成，可作为后续风格参考。",
    ended_at: iso(-2 * DAY + 2 * HOUR),
  },
  {
    item_id: "note_closed_002",
    title: "整理旧 prototype 的引用关系",
    bucket: "closed",
    status: "cancelled",
    type: "archive",
    due_at: iso(-4 * DAY),
    agent_suggestion: "已取消，后续如有需要再重新审计。",
    ended_at: iso(-4 * DAY + 3 * HOUR),
  },
];

const noteExperiences: Record<string, NoteDetailExperience> = {
  note_upcoming_001: {
    title: "今天下班前整理周报模板",
    previewStatus: "今天要做",
    timeHint: "今天 18:00 前",
    detailStatus: "即将到来",
    detailStatusTone: "warn",
    typeLabel: "模板整理",
    noteType: "template",
    noteText: "这次周报先把固定结构整理干净，后面每周只替换核心数据和结论。",
    prerequisite: "先确认本周重点结论和图表截图都齐了。",
    relatedResources: [
      { id: "res_001", label: "周报设计文档", path: "docs/dashboard-design.md", type: "文档", openAction: "open_file", taskId: null, url: null },
      { id: "res_002", label: "任务页目录", path: "apps/desktop/src/features/dashboard/tasks", type: "目录", openAction: "reveal_in_folder", taskId: null, url: null },
    ],
    agentSuggestion: {
      label: "下一步建议",
      detail: "先打开设计文档确认结构，再整理周报模板字段。",
    },
    nextOccurrenceAt: null,
    repeatRule: null,
    recentInstanceStatus: null,
    effectiveScope: null,
    plannedAt: iso(5 * HOUR),
    endedAt: null,
    isRecurringEnabled: false,
    canConvertToTask: true,
    summaryLabel: "今天待处理",
  },
  note_upcoming_002: {
    title: "联系设计师确认首页球体交互排期",
    previewStatus: "近期安排",
    timeHint: "还剩 2 天",
    detailStatus: "即将到来",
    detailStatusTone: "normal",
    typeLabel: "沟通跟进",
    noteType: "follow-up",
    noteText: "重点是确认长按语音、事件球调度和入口球拖动这三块的视觉排期。",
    prerequisite: "先整理出三条关键问题和预期结果。",
    relatedResources: [
      { id: "res_003", label: "首页实现入口", path: "apps/desktop/src/app/dashboard/DashboardHome.tsx", type: "页面", openAction: "open_file", taskId: null, url: null },
    ],
    agentSuggestion: {
      label: "下一步建议",
      detail: "如果确认问题已经整理好，可以直接转成任务让 Agent 帮你拟消息。",
    },
    nextOccurrenceAt: null,
    repeatRule: null,
    recentInstanceStatus: null,
    effectiveScope: null,
    plannedAt: iso(2 * DAY),
    endedAt: null,
    isRecurringEnabled: false,
    canConvertToTask: true,
    summaryLabel: "近期跟进",
  },
  note_upcoming_003: {
    title: "给安全页补一版风险摘要文案",
    previewStatus: "已逾期",
    timeHint: "逾期 8 小时",
    detailStatus: "已逾期",
    detailStatusTone: "overdue",
    typeLabel: "文案补充",
    noteType: "reminder",
    noteText: "当前风险摘要还偏技术，需要压缩成更容易理解的一版。",
    prerequisite: "先明确今天最想让用户看到的风险提醒语气。",
    relatedResources: [
      { id: "res_004", label: "安全页实现", path: "apps/desktop/src/features/dashboard/safety/SecurityApp.tsx", type: "页面", openAction: "open_file", taskId: null, url: null },
    ],
    agentSuggestion: {
      label: "下一步建议",
      detail: "先写三条简版风险提示，再决定是否交给 Agent 扩写。",
    },
    nextOccurrenceAt: null,
    repeatRule: null,
    recentInstanceStatus: null,
    effectiveScope: null,
    plannedAt: iso(-8 * HOUR),
    endedAt: null,
    isRecurringEnabled: false,
    canConvertToTask: true,
    summaryLabel: "需要优先处理",
  },
  note_later_001: {
    title: "月底前整理一次桌面端 UI token",
    previewStatus: "尚未到期",
    timeHint: "月底前",
    detailStatus: "尚未开始",
    detailStatusTone: "normal",
    typeLabel: "长期整理",
    noteType: "reminder",
    noteText: "等本轮 dashboard 子页稳定后，再统一梳理可复用的视觉 token。",
    prerequisite: "先继续积累任务页、便签页和镜子页的共用视觉片段。",
    relatedResources: [
      { id: "res_005", label: "镜子页样式", path: "apps/desktop/src/features/dashboard/memory/mirror.css", type: "样式", openAction: "open_file", taskId: null, url: null },
    ],
    agentSuggestion: {
      label: "下一步建议",
      detail: "暂时继续观察，月底前再把这条整理成正式任务。",
    },
    nextOccurrenceAt: null,
    repeatRule: null,
    recentInstanceStatus: null,
    effectiveScope: null,
    plannedAt: iso(11 * DAY),
    endedAt: null,
    isRecurringEnabled: false,
    canConvertToTask: false,
    summaryLabel: "后续安排",
  },
  note_later_002: {
    title: "镜子页稳定后补跨页联动演示",
    previewStatus: "尚未到期",
    timeHint: "后续安排",
    detailStatus: "尚未开始",
    detailStatusTone: "normal",
    typeLabel: "后续演示",
    noteType: "follow-up",
    noteText: "这条已经生成关联任务，当前保留在原分组里作为来源记录。",
    prerequisite: "等镜子页和任务页交互都收口后，再补演示。",
    relatedResources: [
      { id: "res_006", label: "查看关联任务", path: null, type: "任务", openAction: "task_detail", taskId: "task_focus_001", url: null },
      { id: "res_007", label: "镜子页入口", path: "apps/desktop/src/features/dashboard/memory/MirrorApp.tsx", type: "页面", openAction: "open_file", taskId: null, url: null },
    ],
    agentSuggestion: {
      label: "下一步建议",
      detail: "直接回看关联任务推进，不需要再转一次任务。",
    },
    nextOccurrenceAt: null,
    repeatRule: null,
    recentInstanceStatus: null,
    effectiveScope: null,
    plannedAt: iso(18 * DAY),
    endedAt: null,
    isRecurringEnabled: false,
    canConvertToTask: false,
    summaryLabel: "已关联任务",
  },
  note_recurring_001: {
    title: "每周一整理周报",
    previewStatus: "规则生效中",
    timeHint: "下次：下周一 09:00",
    detailStatus: "重复规则生效中",
    detailStatusTone: "normal",
    typeLabel: "重复事项",
    noteType: "recurring",
    noteText: "每周一固定整理一次周报，当天进入处理窗口时会在近期分组里出现实例。",
    prerequisite: "周日晚上或周一早上先汇总本周数据。",
    relatedResources: [
      { id: "res_008", label: "任务 mock 数据", path: "apps/desktop/src/features/dashboard/tasks/taskPage.mock.ts", type: "数据", openAction: "open_file", taskId: null, url: null },
    ],
    agentSuggestion: {
      label: "流程化建议",
      detail: "这类规则已经稳定，适合整理成固定模板。",
    },
    nextOccurrenceAt: iso(3 * DAY),
    repeatRule: "每周一 09:00",
    recentInstanceStatus: "上次已完成",
    effectiveScope: "工作周内持续生效",
    plannedAt: null,
    endedAt: null,
    isRecurringEnabled: true,
    canConvertToTask: false,
    summaryLabel: "规则本身",
  },
  note_recurring_002: {
    title: "工作日 09:00 巡检邮件",
    previewStatus: "规则生效中",
    timeHint: "下次：明天 09:00",
    detailStatus: "重复规则生效中",
    detailStatusTone: "normal",
    typeLabel: "重复事项",
    noteType: "recurring",
    noteText: "每天 09:00 处理一轮邮件，重要的进入近期，不重要的归档。",
    prerequisite: "邮箱筛选规则已经可用。",
    relatedResources: [
      { id: "res_009", label: "邮件巡检说明", path: null, type: "网页", openAction: "open_url", taskId: null, url: "https://example.test/mail-inspection" },
    ],
    agentSuggestion: {
      label: "流程化建议",
      detail: "如果这条长期保留，可以考虑把归档模板固定下来。",
    },
    nextOccurrenceAt: iso(DAY),
    repeatRule: "工作日 09:00",
    recentInstanceStatus: "今天实例已处理",
    effectiveScope: "仅工作日生效",
    plannedAt: null,
    endedAt: null,
    isRecurringEnabled: true,
    canConvertToTask: false,
    summaryLabel: "规则本身",
  },
  note_closed_001: {
    title: "导出铃兰首页交互版截图",
    previewStatus: "已完成",
    timeHint: "已结束",
    detailStatus: "已完成",
    detailStatusTone: "done",
    typeLabel: "已结束记录",
    noteType: "archive",
    noteText: "这条记录对应的首页截图已经归档，可继续作为视觉参考。",
    prerequisite: null,
    relatedResources: [
      { id: "res_010", label: "铃兰素材目录", path: "apps/desktop/src/assets/lily-of-the-valley", type: "目录", openAction: "reveal_in_folder", taskId: null, url: null },
    ],
    agentSuggestion: {
      label: "后续建议",
      detail: "如果还会复用这组素材，可以整理成模板或继续开新任务。",
    },
    nextOccurrenceAt: null,
    repeatRule: null,
    recentInstanceStatus: null,
    effectiveScope: null,
    plannedAt: iso(-2 * DAY),
    endedAt: iso(-2 * DAY + 2 * HOUR),
    isRecurringEnabled: false,
    canConvertToTask: true,
    summaryLabel: "可复用成果",
  },
  note_closed_002: {
    title: "整理旧 prototype 的引用关系",
    previewStatus: "已取消",
    timeHint: "已结束",
    detailStatus: "已取消",
    detailStatusTone: "done",
    typeLabel: "已结束记录",
    noteType: "archive",
    noteText: "当前阶段不适合扩大重构范围，所以先取消这条便签。",
    prerequisite: null,
    relatedResources: [
      { id: "res_011", label: "仪表盘设计文档", path: "docs/dashboard-design.md", type: "文档", openAction: "open_file", taskId: null, url: null },
    ],
    agentSuggestion: {
      label: "后续建议",
      detail: "如果未来重启这条线，先重新做一轮引用审计。",
    },
    nextOccurrenceAt: null,
    repeatRule: null,
    recentInstanceStatus: null,
    effectiveScope: null,
    plannedAt: iso(-4 * DAY),
    endedAt: iso(-4 * DAY + 3 * HOUR),
    isRecurringEnabled: false,
    canConvertToTask: true,
    summaryLabel: "后续可重启",
  },
};

let itemsState = clone(baseItems);

export function getMockNoteBuckets(): NoteBucketsData {
  const notes: NoteListItem[] = itemsState.map((item) => ({
    experience: getMockNoteExperience(item.item_id),
    item,
  }));

  return {
    closed: notes.filter((item) => item.item.bucket === "closed"),
    later: notes.filter((item) => item.item.bucket === "later"),
    recurring_rule: notes.filter((item) => item.item.bucket === "recurring_rule"),
    source: "mock",
    upcoming: notes.filter((item) => item.item.bucket === "upcoming"),
  };
}

export function getMockNoteExperience(itemId: string) {
  return clone(noteExperiences[itemId]);
}

export function runMockConvertNoteToTask(itemId: string): NoteConvertOutcome {
  const index = itemsState.findIndex((entry) => entry.item_id === itemId);
  const item = index >= 0 ? itemsState[index] : itemsState[0];

  if (!item) {
    throw new Error(`mock note not found: ${itemId}`);
  }

  const linkedTaskId = item.linked_task_id ?? `task_from_${item.item_id}`;
  const updatedItem = {
    ...item,
    linked_task_id: linkedTaskId,
  };

  if (index >= 0) {
    itemsState[index] = updatedItem;
  }

  return {
    result: {
      task: {
        current_step: "awaiting_confirmation",
        finished_at: null,
        intent: { name: "converted_note", arguments: { item_id: item.item_id } },
        risk_level: "green",
        source_type: "todo",
        started_at: new Date().toISOString(),
        status: "processing",
        task_id: linkedTaskId,
        title: item.title,
        updated_at: new Date().toISOString(),
      },
      notepad_item: updatedItem,
      refresh_groups: [item.bucket],
    },
    source: "mock",
  };
}

export function runMockUpdateNote(itemId: string, action: string): NoteUpdateOutcome {
  const index = itemsState.findIndex((entry) => entry.item_id === itemId);
  const item = index >= 0 ? itemsState[index] : itemsState[0];
  const nowIso = new Date().toISOString();

  if (!item) {
    throw new Error(`mock note not found: ${itemId}`);
  }

  let updatedItem: typeof item | null = { ...item };
  let refreshGroups: string[] = [item.bucket];
  let deletedItemId: string | null = null;

  switch (action) {
    case "complete":
      updatedItem = { ...updatedItem, bucket: "closed", status: "completed", ended_at: nowIso, due_at: null };
      refreshGroups = [item.bucket, "closed"];
      break;
    case "cancel":
      updatedItem = { ...updatedItem, bucket: "closed", status: "cancelled", ended_at: nowIso, due_at: null };
      refreshGroups = [item.bucket, "closed"];
      break;
    case "move_upcoming":
      updatedItem = { ...updatedItem, bucket: "upcoming", status: "normal" };
      refreshGroups = [item.bucket, "upcoming"];
      break;
    case "toggle_recurring":
      updatedItem = {
        ...updatedItem,
        recurring_enabled: !updatedItem.recurring_enabled,
        recent_instance_status: updatedItem.recurring_enabled ? "重复规则已暂停" : "重复规则已恢复",
        status: "normal",
      };
      break;
    case "cancel_recurring":
      updatedItem = { ...updatedItem, bucket: "closed", status: "cancelled", recurring_enabled: false, ended_at: nowIso };
      refreshGroups = [item.bucket, "closed"];
      break;
    case "restore":
      updatedItem = { ...updatedItem, bucket: "upcoming", status: "normal", ended_at: null };
      refreshGroups = [item.bucket, "upcoming"];
      break;
    case "delete":
      updatedItem = null;
      deletedItemId = itemId;
      refreshGroups = [item.bucket];
      break;
    default:
      throw new Error(`unsupported mock notepad action: ${action}`);
  }

  if (index >= 0) {
    if (updatedItem) {
      itemsState[index] = updatedItem;
    } else {
      itemsState.splice(index, 1);
    }
  }

  return {
    result: {
      deleted_item_id: deletedItemId,
      notepad_item: updatedItem,
      refresh_groups: refreshGroups as Array<"upcoming" | "later" | "recurring_rule" | "closed">,
    },
    source: "mock",
  };
}
