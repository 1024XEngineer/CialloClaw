import type { TodoItem } from "@cialloclaw/protocol";
import type { NoteBucketsData, NoteConvertOutcome, NoteDetailExperience, NoteListItem } from "./notePage.types";

const now = Date.now();
const HOUR = 1000 * 60 * 60;
const DAY = HOUR * 24;

function iso(offset: number) {
  return new Date(now + offset).toISOString();
}

function clone<T>(value: T) {
  return structuredClone(value);
}

const mockItems: TodoItem[] = [
  {
    item_id: "note_upcoming_001",
    title: "今天下班前把周报模板整理出来",
    bucket: "upcoming",
    status: "due_today",
    type: "template",
    due_at: iso(5 * HOUR),
    agent_suggestion: "先打开上周模板，再把这周数据补进去。",
  },
  {
    item_id: "note_upcoming_002",
    title: "联系设计师确认首页球体交互排期",
    bucket: "upcoming",
    status: "normal",
    type: "follow_up",
    due_at: iso(2 * DAY),
    agent_suggestion: "先整理你想确认的 3 个问题，再发过去。",
  },
  {
    item_id: "note_upcoming_003",
    title: "给安全页补一版风险摘要文案",
    bucket: "upcoming",
    status: "overdue",
    type: "reminder",
    due_at: iso(-8 * HOUR),
    agent_suggestion: "先把当前失败案例整理成 3 条摘要，再决定是否交给 Agent。",
  },
  {
    item_id: "note_later_001",
    title: "月底前整理一次桌面端 UI token",
    bucket: "later",
    status: "normal",
    type: "reminder",
    due_at: iso(11 * DAY),
    agent_suggestion: "先继续积累最近页面里的可复用 token，再统一整理。",
  },
  {
    item_id: "note_later_002",
    title: "等镜子页稳定后补跨页联动演示",
    bucket: "later",
    status: "normal",
    type: "follow_up",
    due_at: iso(18 * DAY),
    agent_suggestion: "等镜子页数据结构稳定，再开始串 dashboard 联动。",
  },
  {
    item_id: "note_recurring_001",
    title: "每周一整理周报",
    bucket: "recurring_rule",
    status: "normal",
    type: "recurring",
    due_at: null,
    agent_suggestion: "周一早上先汇总材料，中午前生成初稿。",
  },
  {
    item_id: "note_recurring_002",
    title: "工作日 09:00 巡检邮件",
    bucket: "recurring_rule",
    status: "normal",
    type: "recurring",
    due_at: null,
    agent_suggestion: "只保留重要邮件，其余自动归档。",
  },
  {
    item_id: "note_closed_001",
    title: "输出铃兰首页交互版截图",
    bucket: "closed",
    status: "completed",
    type: "archive",
    due_at: iso(-2 * DAY),
    agent_suggestion: "已完成，可作为后续任务的素材来源。",
  },
  {
    item_id: "note_closed_002",
    title: "整理旧 prototype 的引用关系",
    bucket: "closed",
    status: "cancelled",
    type: "archive",
    due_at: iso(-4 * DAY),
    agent_suggestion: "已取消，后续重新审计后再继续。",
  },
];

const noteExperiences: Record<string, NoteDetailExperience> = {
  note_upcoming_001: {
    title: "今天下班前把周报模板整理出来",
    previewStatus: "今天要做",
    timeHint: "今天 18:00 前",
    detailStatus: "即将到来",
    detailStatusTone: "warn",
    typeLabel: "模板整理",
    noteType: "template",
    noteText: "这次周报想先把固定结构整理干净，后续每周只需要替换数据与结论。",
    prerequisite: "先确认这周重点结论和图表截图已经齐全。",
    relatedResources: [
      { id: "res_001", label: "上周周报模板", path: "workspace/templates/weekly-report.md", type: "模板" },
      { id: "res_002", label: "本周数据草稿", path: "workspace/drafts/weekly-report-data.md", type: "草稿" },
    ],
    agentSuggestion: {
      label: "下一步建议",
      detail: "先打开上周模板，再把这周的重点数据补进去，确认后可直接转交给 Agent 起草。",
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
    previewStatus: "最近几天要处理",
    timeHint: "还剩 2 天",
    detailStatus: "即将到来",
    detailStatusTone: "normal",
    typeLabel: "沟通跟进",
    noteType: "follow-up",
    noteText: "主要是确认长按语音、事件球调度和入口球拖动这三块的视觉打磨排期。",
    prerequisite: "先整理出想确认的 3 个问题和期望结果。",
    relatedResources: [
      { id: "res_003", label: "首页交互说明", path: "docs/home-interaction-notes.md", type: "说明" },
    ],
    agentSuggestion: {
      label: "下一步建议",
      detail: "先把你想确认的问题整理成一条清晰消息，之后可以一键转任务。",
    },
    nextOccurrenceAt: null,
    repeatRule: null,
    recentInstanceStatus: null,
    effectiveScope: null,
    plannedAt: iso(2 * DAY),
    endedAt: null,
    isRecurringEnabled: false,
    canConvertToTask: true,
    summaryLabel: "近期沟通",
  },
  note_upcoming_003: {
    title: "给安全页补一版风险摘要文案",
    previewStatus: "已逾期",
    timeHint: "逾期 8 小时",
    detailStatus: "已逾期",
    detailStatusTone: "overdue",
    typeLabel: "文案补充",
    noteType: "reminder",
    noteText: "当前风险摘要仍然偏技术表达，需要再压缩成更容易理解的一版。",
    prerequisite: "先明确今天最重要的风险提示语气。",
    relatedResources: [
      { id: "res_004", label: "安全页草稿", path: "apps/desktop/src/features/dashboard/safety/SecurityApp.tsx", type: "页面" },
    ],
    agentSuggestion: {
      label: "下一步建议",
      detail: "先写 3 条简版风险摘要，再选择其中一条作为页面主文案。",
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
    previewStatus: "未到时间",
    timeHint: "还没到处理窗口",
    detailStatus: "尚未开始",
    detailStatusTone: "normal",
    typeLabel: "长期整理",
    noteType: "reminder",
    noteText: "这件事已记下，但不需要现在处理，等本月 dashboard 子页样式稳定后再统一梳理。",
    prerequisite: "先积累任务页、便签页、镜子页的视觉 token。",
    relatedResources: [
      { id: "res_005", label: "UI token 草稿", path: "docs/ui-token-notes.md", type: "草稿" },
    ],
    agentSuggestion: {
      label: "下一步建议",
      detail: "先继续积累页面样式，月底前再统一整理成设计 token。",
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
    title: "等镜子页稳定后补跨页联动演示",
    previewStatus: "未到时间",
    timeHint: "后续安排",
    detailStatus: "尚未开始",
    detailStatusTone: "normal",
    typeLabel: "后续演示",
    noteType: "follow-up",
    noteText: "跨页联动的演示要等镜子页和任务页都稳定之后再做。",
    prerequisite: "镜子页、任务页交互和样式都已收口。",
    relatedResources: [
      { id: "res_006", label: "镜子页入口", path: "apps/desktop/src/features/dashboard/memory/MirrorApp.tsx", type: "页面" },
    ],
    agentSuggestion: {
      label: "下一步建议",
      detail: "暂时继续观察，等镜子页状态稳定后再转成正式任务。",
    },
    nextOccurrenceAt: null,
    repeatRule: null,
    recentInstanceStatus: null,
    effectiveScope: null,
    plannedAt: iso(18 * DAY),
    endedAt: null,
    isRecurringEnabled: false,
    canConvertToTask: false,
    summaryLabel: "先记住即可",
  },
  note_recurring_001: {
    title: "每周一整理周报",
    previewStatus: "规则生效中",
    timeHint: "下次：下周一 09:00",
    detailStatus: "重复规则开启中",
    detailStatusTone: "normal",
    typeLabel: "重复事项",
    noteType: "recurring",
    noteText: "每周一固定整理一次周报，若当天进入处理窗口，就会在“近期要做”中生成当天实例。",
    prerequisite: "周日晚上或周一早上先汇总本周数据。",
    relatedResources: [
      { id: "res_007", label: "周报模板", path: "workspace/templates/weekly-report.md", type: "模板" },
    ],
    agentSuggestion: {
      label: "流程化建议",
      detail: "这类事项已经很稳定，适合进一步整理成固定模板并在周一自动提醒。",
    },
    nextOccurrenceAt: iso(DAY * 3),
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
    detailStatus: "重复规则开启中",
    detailStatusTone: "normal",
    typeLabel: "重复事项",
    noteType: "recurring",
    noteText: "每天 09:00 做一轮邮件巡检，重要邮件进入近期要做，其余归档。",
    prerequisite: "需要邮箱接入和当前白名单规则已生效。",
    relatedResources: [
      { id: "res_008", label: "邮件巡检说明", path: "docs/mail-inspection.md", type: "说明" },
    ],
    agentSuggestion: {
      label: "流程化建议",
      detail: "如果你长期保留这条规则，可以考虑把归档和提醒模板固定下来。",
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
    title: "输出铃兰首页交互版截图",
    previewStatus: "已完成",
    timeHint: "已结束",
    detailStatus: "已完成",
    detailStatusTone: "done",
    typeLabel: "已结束记录",
    noteType: "archive",
    noteText: "这条事项对应的首页交互截图已经完成并归档，可作为后续风格参考。",
    prerequisite: null,
    relatedResources: [
      { id: "res_009", label: "铃兰截图目录", path: "workspace/exports/lily-home", type: "文件夹" },
    ],
    agentSuggestion: {
      label: "后续建议",
      detail: "如果还会反复用到这组素材，可以整理成模板或直接转为新的任务。",
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
    noteText: "这件事已取消，原因是当前阶段不适合扩大重构范围。",
    prerequisite: null,
    relatedResources: [
      { id: "res_010", label: "原型文件清单", path: "docs/prototype-audit.md", type: "清单" },
    ],
    agentSuggestion: {
      label: "后续建议",
      detail: "下次如果重启，先做引用审计再继续推进。",
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

let itemsState = clone(mockItems);

export function getMockNoteBuckets(): NoteBucketsData {
  const notes: NoteListItem[] = itemsState.map((item) => ({
    experience: noteExperiences[item.item_id],
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
  const item = itemsState.find((entry) => entry.item_id === itemId) ?? itemsState[0];

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
        task_id: `task_from_${item.item_id}`,
        title: item.title,
        updated_at: new Date().toISOString(),
      },
      notepad_item: {
        ...item,
        linked_task_id: `task_from_${item.item_id}`,
      },
      refresh_groups: [item.bucket],
    },
    source: "mock",
  };
}
