import type { AgentMirrorOverviewGetResult, ApprovalRequest, RequestSource, Task, TokenCostSummary } from "@cialloclaw/protocol";
import type { MirrorConversationRecord } from "../../../services/mirrorMemoryService";

export type MirrorConversationSummary = {
  total_records: number;
  responded_records: number;
  failed_records: number;
  latest_at: string | null;
  latest_user_text: string | null;
  latest_agent_text: string | null;
  dominant_source: RequestSource | null;
  dominant_input_mode: MirrorConversationRecord["input_mode"] | null;
};

export type MirrorConversationDayGroup = {
  date_key: string;
  label: string;
  items: MirrorConversationRecord[];
};

export type MirrorConversationScopeFilter = "all" | "with_task" | "failed";

export type MirrorConversationSourceFilter = RequestSource | "all";

export type MirrorConversationInputModeFilter = MirrorConversationRecord["input_mode"] | "all";

export type MirrorConversationFilters = {
  scope: MirrorConversationScopeFilter;
  source: MirrorConversationSourceFilter;
  input_mode: MirrorConversationInputModeFilter;
  date_key: string | "all";
};

export type MirrorConversationDateOption = {
  date_key: string;
  label: string;
  count: number;
  latest_at: string;
};

export type MirrorConversationTaskMoment = {
  task_id: string;
  count: number;
  latest_at: string;
};

export type MirrorTaskDigest = {
  task_id: string;
  title: string;
  status: Task["status"];
  risk_level: Task["risk_level"];
  moment: string | null;
};

export type MirrorDailyStat = {
  id: string;
  label: string;
  value: string;
  detail: string;
  tone: string;
};

export type MirrorStageSnapshot = {
  id: string;
  label: string;
  description: string;
  tone: string;
  count: number;
  tasks: MirrorTaskDigest[];
};

export type MirrorContextNote = {
  id: string;
  label: string;
  value: string;
  detail: string;
  tone: string;
};

export type MirrorDailyDigest = {
  date: string;
  headline: string;
  lede: string;
  stats: MirrorDailyStat[];
  stage_snapshots: MirrorStageSnapshot[];
  context_notes: MirrorContextNote[];
};

export type MirrorProfileBaseItem = {
  id: string;
  label: string;
  value: string;
  hint: string;
  category: "style" | "preference" | "habit";
  source_kind: "backend_profile" | "local_stat";
  source_label: string;
};

export type MirrorProfileItemView = MirrorProfileBaseItem;

export type MirrorProfileView = {
  backend_items: MirrorProfileItemView[];
  local_stat_items: MirrorProfileItemView[];
  total_items: number;
};

type MirrorDailyDigestInput = {
  overview: AgentMirrorOverviewGetResult;
  unfinished_tasks: Task[];
  finished_tasks: Task[];
  pending_approvals: ApprovalRequest[];
  security_status: string | null;
  latest_restore_point_summary: string | null;
  token_cost_summary: TokenCostSummary | null;
  conversations: MirrorConversationRecord[];
};

const DISPLAY_DATE_LOCALE = "zh-CN";

function toCalendarDate(value: string) {
  return new Date(value).toLocaleDateString("sv-SE");
}

function toCalendarDateLabel(value: string) {
  return new Date(value).toLocaleDateString(DISPLAY_DATE_LOCALE, {
    month: "long",
    day: "numeric",
    weekday: "short",
  });
}

function formatCompactTime(value: string) {
  return new Date(value).toLocaleTimeString(DISPLAY_DATE_LOCALE, {
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatCompactDateTime(value: string) {
  return new Date(value).toLocaleString(DISPLAY_DATE_LOCALE, {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function clampList<T>(items: T[], limit: number) {
  return items.slice(0, limit);
}

function getDominantRecordValue<T extends string>(items: T[]) {
  const counts = new Map<T, number>();

  for (const item of items) {
    counts.set(item, (counts.get(item) ?? 0) + 1);
  }

  let winner: T | null = null;
  let bestCount = -1;

  for (const [item, count] of counts.entries()) {
    if (count > bestCount) {
      winner = item;
      bestCount = count;
    }
  }

  return winner;
}

function getTaskDigest(tasks: Task[]) {
  return clampList(
    tasks.map((task) => ({
      task_id: task.task_id,
      title: task.title,
      status: task.status,
      risk_level: task.risk_level,
      moment: task.finished_at ?? task.updated_at ?? task.started_at,
    } satisfies MirrorTaskDigest)),
    3,
  );
}

function getRiskTone(level: Task["risk_level"] | "green" | "yellow" | "red" | null) {
  if (level === "red") {
    return "red";
  }

  if (level === "yellow") {
    return "yellow";
  }

  return "green";
}

export function getMirrorConversationSourceLabel(source: RequestSource) {
  switch (source) {
    case "floating_ball":
      return "悬浮球";
    case "dashboard":
      return "仪表盘";
    case "tray_panel":
      return "托盘面板";
  }
}

export function getMirrorConversationInputModeLabel(mode: MirrorConversationRecord["input_mode"]) {
  return mode === "voice" ? "语音" : "文本";
}

export function getMirrorConversationTriggerLabel(trigger: MirrorConversationRecord["trigger"]) {
  return trigger === "voice_commit" ? "语音提交" : "悬停输入";
}

export function buildMirrorConversationSummary(records: MirrorConversationRecord[]): MirrorConversationSummary {
  const sortedRecords = [...records].sort((left, right) => right.updated_at.localeCompare(left.updated_at));
  const latestRecord = sortedRecords[0] ?? null;

  return {
    total_records: records.length,
    responded_records: records.filter((record) => record.status === "responded").length,
    failed_records: records.filter((record) => record.status === "failed").length,
    latest_at: latestRecord?.updated_at ?? null,
    latest_user_text: latestRecord?.user_text ?? null,
    latest_agent_text: latestRecord?.agent_text ?? null,
    dominant_source: getDominantRecordValue(records.map((record) => record.source)),
    dominant_input_mode: getDominantRecordValue(records.map((record) => record.input_mode)),
  };
}

export function filterMirrorConversationRecords(records: MirrorConversationRecord[], filters: MirrorConversationFilters) {
  return records.filter((record) => {
    if (filters.scope === "with_task" && !record.task_id) {
      return false;
    }

    if (filters.scope === "failed" && record.status !== "failed") {
      return false;
    }

    if (filters.source !== "all" && record.source !== filters.source) {
      return false;
    }

    if (filters.input_mode !== "all" && record.input_mode !== filters.input_mode) {
      return false;
    }

    if (filters.date_key !== "all" && toCalendarDate(record.updated_at) !== filters.date_key) {
      return false;
    }

    return true;
  });
}

export function buildMirrorConversationDateOptions(records: MirrorConversationRecord[]) {
  const dateMap = new Map<string, MirrorConversationDateOption>();

  for (const record of records) {
    const dateKey = toCalendarDate(record.updated_at);
    const current = dateMap.get(dateKey);

    if (!current) {
      dateMap.set(dateKey, {
        count: 1,
        date_key: dateKey,
        label: toCalendarDateLabel(dateKey),
        latest_at: record.updated_at,
      });
      continue;
    }

    dateMap.set(dateKey, {
      ...current,
      count: current.count + 1,
      latest_at: current.latest_at.localeCompare(record.updated_at) >= 0 ? current.latest_at : record.updated_at,
    });
  }

  return Array.from(dateMap.values()).sort((left, right) => {
    const dateOrder = right.date_key.localeCompare(left.date_key);
    if (dateOrder !== 0) {
      return dateOrder;
    }

    return right.latest_at.localeCompare(left.latest_at);
  });
}

export function buildMirrorConversationTaskMoments(records: MirrorConversationRecord[]) {
  const taskMap = new Map<string, MirrorConversationTaskMoment>();

  for (const record of records) {
    if (!record.task_id) {
      continue;
    }

    const current = taskMap.get(record.task_id);
    if (!current) {
      taskMap.set(record.task_id, {
        count: 1,
        latest_at: record.updated_at,
        task_id: record.task_id,
      });
      continue;
    }

    taskMap.set(record.task_id, {
      task_id: record.task_id,
      count: current.count + 1,
      latest_at: current.latest_at.localeCompare(record.updated_at) >= 0 ? current.latest_at : record.updated_at,
    });
  }

  return Array.from(taskMap.values()).sort((left, right) => {
    const latestOrder = left.latest_at.localeCompare(right.latest_at);
    if (latestOrder !== 0) {
      return latestOrder;
    }

    return left.task_id.localeCompare(right.task_id);
  });
}

export function groupMirrorConversationRecords(records: MirrorConversationRecord[]) {
  const groups = new Map<string, MirrorConversationRecord[]>();
  const sortedRecords = [...records].sort((left, right) => right.updated_at.localeCompare(left.updated_at));

  for (const record of sortedRecords) {
    const dateKey = toCalendarDate(record.updated_at);
    groups.set(dateKey, [...(groups.get(dateKey) ?? []), record]);
  }

  return Array.from(groups.entries())
    .sort(([leftDate], [rightDate]) => rightDate.localeCompare(leftDate))
    .map(([dateKey, items]) => ({
      date_key: dateKey,
      label: toCalendarDateLabel(dateKey),
      // Keep each day in chronological order so the local conversation history
      // reads like a timeline once the user narrows it down by date.
      items: [...items].sort((left, right) => left.updated_at.localeCompare(right.updated_at)),
    } satisfies MirrorConversationDayGroup));
}

export function buildMirrorDailyDigest(input: MirrorDailyDigestInput): MirrorDailyDigest {
  const digestDate = input.overview.daily_summary?.date ?? new Date().toISOString();
  const digestDateKey = toCalendarDate(digestDate);
  const completedTodayTasks = input.finished_tasks.filter(
    (task) => task.finished_at !== null && toCalendarDate(task.finished_at) === digestDateKey && task.status === "completed",
  );
  const completedCount = input.overview.daily_summary?.completed_tasks ?? completedTodayTasks.length;
  const generatedOutputs = input.overview.daily_summary?.generated_outputs ?? input.conversations.filter((record) => record.status === "responded").length;
  const inFlightTasks = input.unfinished_tasks.filter((task) => task.status === "processing");
  const waitingTasks = input.unfinished_tasks.filter((task) => task.status !== "processing");
  const latestPendingApproval = input.pending_approvals[0] ?? null;
  const latestConversation = input.conversations[0] ?? null;
  const todayCost = input.token_cost_summary?.today_cost ?? 0;
  const todayTokens = input.token_cost_summary?.today_tokens ?? 0;

  const stageSnapshots: MirrorStageSnapshot[] = [
    {
      id: "in-flight",
      label: "正在推进",
      description: inFlightTasks.length > 0 ? "列出当前状态为 processing 的任务。" : "当前没有 processing 任务。",
      tone: "processing",
      count: inFlightTasks.length,
      tasks: getTaskDigest(inFlightTasks),
    },
    {
      id: "waiting",
      label: "等待确认 / 停顿",
      description:
        waitingTasks.length > 0
          ? "列出等待授权、等待输入、暂停或阻塞的任务。"
          : "当前没有等待中的任务。",
      tone: waitingTasks.some((task) => task.status === "waiting_auth") ? "yellow" : "status",
      count: waitingTasks.length,
      tasks: getTaskDigest(waitingTasks),
    },
    {
      id: "completed",
      label: "今日收束",
      description: completedCount > 0 ? "列出今天 finished_at 落在统计日期内、且状态为 completed 的任务。" : "今天还没有状态为 completed 的任务。",
      tone: "green",
      count: completedCount,
      tasks: getTaskDigest(completedTodayTasks),
    },
  ];

  const contextNotes: MirrorContextNote[] = [
    {
      id: "approvals",
      label: "权限与风险",
      value: `${input.pending_approvals.length} 项待处理`,
      detail: latestPendingApproval ? `${latestPendingApproval.operation_name} · ${latestPendingApproval.reason}` : "当前没有待授权动作。",
      tone: getRiskTone(latestPendingApproval?.risk_level ?? null),
    },
    {
      id: "restore-point",
      label: "恢复点",
      value: input.latest_restore_point_summary ? "可回退" : "暂无",
      detail: input.latest_restore_point_summary ?? "当前没有恢复点说明。",
      tone: input.latest_restore_point_summary ? "processing" : "status",
    },
    {
      id: "cost",
      label: "结果与成本",
      value: `${generatedOutputs} 条结果记录`,
      detail:
        todayTokens > 0 || todayCost > 0
          ? `今日累计 ${todayTokens.toLocaleString("zh-CN")} tokens · ¥${todayCost.toFixed(2)}`
          : "当前没有今日成本统计。",
      tone: "processing",
    },
    {
      id: "continuity",
      label: "本地对话记录",
      value: latestConversation ? formatCompactDateTime(latestConversation.updated_at) : "暂无本地连续对话",
      detail: latestConversation?.agent_text ?? latestConversation?.user_text ?? "当前没有本地记录。",
      tone: latestConversation?.status === "failed" ? "red" : "green",
    },
  ];

  const stats: MirrorDailyStat[] = [
    {
      id: "completed",
      label: "今日完成",
      value: String(completedCount),
      detail: completedCount > 0 ? "统计日期内状态为 completed 的任务数。" : "统计日期内没有 completed 任务。",
      tone: "green",
    },
    {
      id: "active",
      label: "仍在推进",
      value: String(input.unfinished_tasks.length),
      detail: input.unfinished_tasks.length > 0 ? "processing、waiting、paused、blocked 等未结束任务总数。" : "当前没有未结束任务。",
      tone: "processing",
    },
    {
      id: "approvals",
      label: "待确认",
      value: String(input.pending_approvals.length),
      detail: input.security_status ? `安全状态：${input.security_status}` : "当前没有安全状态说明。",
      tone: getRiskTone(latestPendingApproval?.risk_level ?? null),
    },
    {
      id: "continuity",
      label: "本地对话",
      value: String(input.conversations.length),
      detail: input.conversations.length > 0 ? "仅统计最近 100 条本地输入与前端可见回应记录。" : "当前还没有本地对话记录。",
      tone: "processing",
    },
  ];

  const headline = completedCount > 0
    ? `今日完成 ${completedCount} 条任务，未结束 ${input.unfinished_tasks.length} 条。`
    : `今日完成 0 条任务，未结束 ${input.unfinished_tasks.length} 条。`;
  const lede = latestPendingApproval
    ? `当前待确认动作：${latestPendingApproval.operation_name}。`
    : "当前没有待确认动作。";

  return {
    date: digestDate,
    headline,
    lede,
    stats,
    stage_snapshots: stageSnapshots,
    context_notes: contextNotes,
  };
}

export function buildMirrorProfileBaseItems(input: {
  profile: AgentMirrorOverviewGetResult["profile"];
  conversations: MirrorConversationRecord[];
}) {
  const items: MirrorProfileBaseItem[] = [];
  const summary = buildMirrorConversationSummary(input.conversations);
  const voiceCount = input.conversations.filter((record) => record.input_mode === "voice").length;
  const textCount = summary.total_records - voiceCount;

  if (input.profile) {
    items.push(
      {
        id: "profile-work-style",
        label: "工作风格",
        value: input.profile.work_style,
        hint: "来自你的画像概览。",
        category: "style",
        source_kind: "backend_profile",
        source_label: "你的画像",
      },
      {
        id: "profile-preferred-output",
        label: "偏好交付",
        value: input.profile.preferred_output,
        hint: "来自你的画像概览。",
        category: "preference",
        source_kind: "backend_profile",
        source_label: "你的画像",
      },
      {
        id: "profile-active-hours",
        label: "活跃时段",
        value: input.profile.active_hours,
        hint: "来自你的画像概览。",
        category: "habit",
        source_kind: "backend_profile",
        source_label: "你的画像",
      },
    );
  }

  if (summary.total_records > 0 && summary.dominant_input_mode) {
    items.push({
      id: "local-stat-input-mode",
      label: "最近输入方式统计",
      value: `语音 ${voiceCount} 条 / 文本 ${textCount} 条`,
      hint: `按最近 ${summary.total_records} 条本地记录机械统计。`,
      category: "habit",
      source_kind: "local_stat",
      source_label: "最近本地统计",
    });
  }

  if (summary.total_records > 0 && summary.dominant_source) {
    const dominantSourceCount = input.conversations.filter((record) => record.source === summary.dominant_source).length;

    items.push({
      id: "local-stat-entry-surface",
      label: "最近入口统计",
      value: `${getMirrorConversationSourceLabel(summary.dominant_source)} ${dominantSourceCount} 条`,
      hint: `按最近 ${summary.total_records} 条本地记录机械统计。`,
      category: "habit",
      source_kind: "local_stat",
      source_label: "最近本地统计",
    });
  }

  if (summary.total_records > 0) {
    items.push({
      id: "local-stat-continuity",
      label: "前端可见回应统计",
      value: `${summary.responded_records}/${summary.total_records}`,
      hint: summary.failed_records > 0 ? `其中 ${summary.failed_records} 条记录标记为失败。` : "最近本地记录中没有失败条目。",
      category: "preference",
      source_kind: "local_stat",
      source_label: "最近本地统计",
    });
  }

  return items;
}

export function buildMirrorProfileView(items: MirrorProfileBaseItem[]): MirrorProfileView {
  return {
    backend_items: items.filter((item) => item.source_kind === "backend_profile"),
    local_stat_items: items.filter((item) => item.source_kind === "local_stat"),
    total_items: items.length,
  };
}

export function formatMirrorConversationRecordMoment(record: MirrorConversationRecord) {
  return `${toCalendarDateLabel(record.updated_at)} · ${formatCompactTime(record.updated_at)}`;
}
