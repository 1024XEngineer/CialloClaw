import type {
  AgentDashboardModuleGetResult,
  AgentDashboardOverviewGetResult,
  AgentMirrorOverviewGetResult,
  AgentRecommendationGetResult,
  RecommendationFeedback,
  RecommendationItem,
  RequestMeta,
  RiskLevel,
  TaskStatus,
} from "@cialloclaw/protocol";
import {
  getDashboardModule,
  getDashboardOverview,
  getMirrorOverview,
  getRecommendations,
  submitRecommendationFeedback,
} from "@/rpc/methods";
import { describeNotePreview, buildNoteSummary } from "@/features/dashboard/notes/notePage.mapper";
import { loadNoteBucket } from "@/features/dashboard/notes/notePage.service";
import type { NoteListItem } from "@/features/dashboard/notes/notePage.types";
import {
  dashboardHomeStates,
} from "./dashboardHome.presets";
import type {
  DashboardHomeContextItem,
  DashboardHomeEventStateKey,
  DashboardHomeInsightItem,
  DashboardHomeNavigationTarget,
  DashboardHomeModuleKey,
  DashboardHomeNoteItem,
  DashboardHomeSignalItem,
  DashboardHomeStateData,
  DashboardHomeStateGroup,
  DashboardHomeSummonEvent,
  DashboardVoiceSequence,
} from "./dashboardHome.types";

const dashboardModuleLabels: Record<DashboardHomeModuleKey, string> = {
  memory: "镜子",
  notes: "便签",
  safety: "安全",
  tasks: "任务",
};

const dashboardModuleTabs: Record<DashboardHomeModuleKey, string> = {
  memory: "overview",
  notes: "queue",
  safety: "guard",
  tasks: "focus",
};

const dashboardModuleActionLabels: Record<DashboardHomeModuleKey, string> = {
  memory: "打开镜子页",
  notes: "打开便签页",
  safety: "打开安全页",
  tasks: "打开任务页",
};

const dashboardModuleNextSteps: Record<DashboardHomeModuleKey, string> = {
  memory: "打开镜子页查看本周总结",
  notes: "打开便签页继续整理事项",
  safety: "打开安全页确认风险摘要",
  tasks: "打开任务页继续推进",
};

const dashboardVoiceExecutionSteps: Record<DashboardHomeModuleKey, string[]> = {
  memory: ["正在读取镜子概览…", "整理近期协作总结…", "准备切换到镜子页…", "马上打开"],
  notes: ["正在读取便签列表…", "整理待办与提醒…", "准备切换到便签页…", "马上打开"],
  safety: ["正在读取安全摘要…", "整理待授权与恢复点…", "准备切换到安全页…", "马上打开"],
  tasks: ["正在读取任务列表…", "定位当前焦点任务…", "准备切换到任务页…", "马上打开"],
};

export type DashboardHomeData = {
  focusLine: {
    headline: string;
    reason: string;
  };
  loadWarnings: string[];
  stateGroups: DashboardHomeStateGroup[];
  stateMap: Record<DashboardHomeEventStateKey, DashboardHomeStateData>;
  summonTemplates: Array<Omit<DashboardHomeSummonEvent, "id">>;
  voiceSequences: DashboardVoiceSequence[];
};

type DashboardTaskRuntimeSummary = {
  processingTasks: number;
  waitingAuthTasks: number;
  blockedTasks: number;
  focusRuntimeSummary: {
    active_steering_count: number;
    events_count: number;
    latest_failure_code: string | null;
    latest_failure_summary: string | null;
    latest_event_type: string | null;
    loop_stop_reason: string | null;
    observation_signals: string[];
  };
};

type DashboardHomeNoteBuckets = {
  closed: NoteListItem[];
  later: NoteListItem[];
  primaryItem: NoteListItem | null;
  recurring: NoteListItem[];
  summary: ReturnType<typeof buildNoteSummary>;
  upcoming: NoteListItem[];
  warnings: string[];
};

const emptyFocusRuntimeSummary: DashboardTaskRuntimeSummary["focusRuntimeSummary"] = {
  active_steering_count: 0,
  events_count: 0,
  latest_failure_code: null,
  latest_failure_summary: null,
  latest_event_type: null,
  loop_stop_reason: null,
  observation_signals: [],
};

function createRequestMeta(scope: string): RequestMeta {
  return {
    client_time: new Date().toISOString(),
    trace_id: `trace_${scope}_${Date.now()}`,
  };
}

function cloneStateData(state: DashboardHomeStateData): DashboardHomeStateData {
  return {
    ...state,
    anomaly: state.anomaly ? { ...state.anomaly } : undefined,
    context: state.context.map((item) => ({ ...item })),
    insights: state.insights?.map((item) => ({ ...item })),
    navigationTarget: state.navigationTarget ? { ...state.navigationTarget } : undefined,
    notes: state.notes?.map((item) => ({ ...item })),
    progressSteps: state.progressSteps?.map((item) => ({ ...item })),
    signals: state.signals?.map((item) => ({ ...item })),
  };
}

function createBaseStateMap() {
  return Object.fromEntries(
    Object.entries(dashboardHomeStates).map(([key, value]) => [key, cloneStateData(value)]),
  ) as Record<DashboardHomeEventStateKey, DashboardHomeStateData>;
}

function formatDashboardTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "刚刚";
  }

  return date.toLocaleString("zh-CN", {
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    month: "numeric",
  });
}

function formatRiskLabel(riskLevel: RiskLevel) {
  switch (riskLevel) {
    case "green":
      return "低";
    case "yellow":
      return "中";
    case "red":
      return "高";
  }
}

function formatTaskStatusLabel(status: TaskStatus) {
  switch (status) {
    case "confirming_intent":
      return "等待确认";
    case "processing":
      return "处理中";
    case "waiting_auth":
      return "等待授权";
    case "waiting_input":
      return "等待补充";
    case "paused":
      return "已暂停";
    case "blocked":
      return "被阻塞";
    case "failed":
      return "执行失败";
    case "completed":
      return "已完成";
    case "cancelled":
      return "已取消";
    case "ended_unfinished":
      return "未完成结束";
  }
}

function formatTaskTag(status: TaskStatus) {
  switch (status) {
    case "confirming_intent":
      return "待确认";
    case "processing":
      return "处理中";
    case "waiting_auth":
      return "待授权";
    case "waiting_input":
      return "待补充";
    case "completed":
      return "已完成";
    default:
      return formatTaskStatusLabel(status);
  }
}

function getSignalLevel(riskLevel: RiskLevel): DashboardHomeSignalItem["level"] {
  if (riskLevel === "red") {
    return "critical";
  }

  if (riskLevel === "yellow") {
    return "warn";
  }

  return "normal";
}

function buildModuleNavigationTarget(
  module: DashboardHomeModuleKey,
  label = dashboardModuleActionLabels[module],
): DashboardHomeNavigationTarget {
  return {
    kind: "module",
    label,
    module,
  };
}

function buildMirrorDetailNavigationTarget(
  activeDetailKey: "profile" | "memory" | "history",
  label: string,
  focusMemoryId?: string,
): DashboardHomeNavigationTarget {
  return {
    activeDetailKey,
    focusMemoryId,
    kind: "mirror_detail",
    label,
    module: "memory",
  };
}

function buildTaskDetailNavigationTarget(taskId: string, label = "打开任务详情"): DashboardHomeNavigationTarget {
  return {
    kind: "task_detail",
    label,
    module: "tasks",
    taskId,
  };
}

function buildTaskNavigationTarget(
  focusSummary: NonNullable<AgentDashboardOverviewGetResult["overview"]["focus_summary"]>,
): DashboardHomeNavigationTarget {
  if (focusSummary.status === "waiting_auth") {
    return buildModuleNavigationTarget("safety", "前往授权");
  }

  if (focusSummary.status === "completed") {
    return buildTaskDetailNavigationTarget(focusSummary.task_id, "查看交付结果");
  }

  return buildTaskDetailNavigationTarget(focusSummary.task_id);
}

function getOverviewSignals(overview: AgentDashboardOverviewGetResult) {
  return Array.isArray(overview.overview.high_value_signal)
    ? overview.overview.high_value_signal.filter((signal): signal is string => typeof signal === "string" && signal.trim() !== "")
    : [];
}

function getOverviewQuickActions(overview: AgentDashboardOverviewGetResult) {
  return Array.isArray(overview.overview.quick_actions)
    ? overview.overview.quick_actions.filter((action): action is string => typeof action === "string" && action.trim() !== "")
    : [];
}

function matchesNavigationActionLabel(state: DashboardHomeStateData, actionLabel: string) {
  return state.navigationTarget?.label.trim() === actionLabel.trim();
}

function getSummonNextStep(state: DashboardHomeStateData, quickActions?: string[]) {
  if (
    state.module === "tasks" &&
    state.navigationTarget?.kind === "task_detail" &&
    quickActions?.[0] &&
    matchesNavigationActionLabel(state, quickActions[0])
  ) {
    return quickActions[0];
  }

  return state.navigationTarget?.label ?? dashboardModuleNextSteps[state.module];
}

function appendDistinctContextItem(
  items: DashboardHomeContextItem[],
  candidate: DashboardHomeContextItem | null,
) {
  if (!candidate) {
    return items;
  }

  const normalizedCandidate = candidate.text.trim();
  if (normalizedCandidate === "") {
    return items;
  }

  if (items.some((item) => item.text.trim() === normalizedCandidate)) {
    return items;
  }

  items.push(candidate);
  return items;
}

function normalizeDashboardHomeCopy(value: string | null | undefined) {
  if (typeof value !== "string") {
    return null;
  }

  const normalized = value
    .replace(/\uFFFD/g, "")
    .replace(/\s+/g, " ")
    .trim();

  return normalized === "" ? null : normalized;
}

function dedupeSummonTemplates(templates: Array<Omit<DashboardHomeSummonEvent, "id">>) {
  const seen = new Set<string>();

  return templates.filter((template) => {
    const key = `${template.stateKey}::${template.message}::${template.reason}`;
    if (seen.has(key)) {
      return false;
    }

    seen.add(key);
    return true;
  });
}

function getModuleHighlights(result: AgentDashboardModuleGetResult | null | undefined) {
  return Array.isArray(result?.highlights) ? result.highlights.filter(Boolean) : [];
}

async function loadDashboardHomeNoteBuckets(): Promise<DashboardHomeNoteBuckets> {
  const [upcomingResult, laterResult, recurringResult, closedResult] = await Promise.allSettled([
    loadNoteBucket("upcoming"),
    loadNoteBucket("later"),
    loadNoteBucket("recurring_rule"),
    loadNoteBucket("closed"),
  ]);
  const warnings: string[] = [];
  const upcoming = upcomingResult.status === "fulfilled"
    ? upcomingResult.value.items
    : (warnings.push(formatDashboardHomeLoadWarning("便签近期要做", upcomingResult.reason)), []);
  const later = laterResult.status === "fulfilled"
    ? laterResult.value.items
    : (warnings.push(formatDashboardHomeLoadWarning("便签后续安排", laterResult.reason)), []);
  const recurring = recurringResult.status === "fulfilled"
    ? recurringResult.value.items
    : (warnings.push(formatDashboardHomeLoadWarning("便签重复事项", recurringResult.reason)), []);
  const closed = closedResult.status === "fulfilled"
    ? closedResult.value.items
    : (warnings.push(formatDashboardHomeLoadWarning("便签已结束", closedResult.reason)), []);
  const primaryItem = upcoming[0] ?? later[0] ?? recurring[0] ?? null;

  return {
    closed,
    later,
    primaryItem,
    recurring,
    summary: buildNoteSummary({ recurring_rule: recurring, upcoming }),
    upcoming,
    warnings,
  };
}

function isCrossDomainGovernanceHighlight(text: string) {
  return /恢复点|审计|授权|回显|风险|generate_text|openai_responses|tool_call|workspace/i.test(text);
}

function getNotesHighlights(result: AgentDashboardModuleGetResult | null | undefined) {
  return getModuleHighlights(result).filter((item) => !isCrossDomainGovernanceHighlight(item));
}

function getMemoryHighlights(result: AgentDashboardModuleGetResult | null | undefined) {
  return getModuleHighlights(result).filter((item) => !isCrossDomainGovernanceHighlight(item));
}

function getModuleSummaryNumber(result: AgentDashboardModuleGetResult | null | undefined, key: string) {
  const value = result?.summary?.[key];
  return typeof value === "number" ? value : 0;
}

function getTaskModuleRuntimeSummary(
  result: AgentDashboardModuleGetResult | null | undefined,
  expectedFocusTaskId?: string | null,
): DashboardTaskRuntimeSummary {
  const candidate = result?.summary?.focus_runtime_summary;
  const focusRuntimeSummary = candidate && typeof candidate === "object" ? (candidate as Record<string, unknown>) : null;
  const focusTaskId = typeof result?.summary?.focus_task_id === "string" ? result.summary.focus_task_id : null;
  const shouldUseFocusRuntimeSummary = !expectedFocusTaskId || (focusTaskId !== null && focusTaskId === expectedFocusTaskId);

  return {
    blockedTasks: getModuleSummaryNumber(result, "blocked_tasks"),
    processingTasks: getModuleSummaryNumber(result, "processing_tasks"),
    waitingAuthTasks: getModuleSummaryNumber(result, "waiting_auth_tasks"),
    // Dashboard overview and module payloads are fetched independently. Only
    // surface per-task runtime cues when both responses still point at the same
    // focus task so mixed snapshots do not blend two different tasks together.
    focusRuntimeSummary: shouldUseFocusRuntimeSummary
      ? {
          active_steering_count:
            typeof focusRuntimeSummary?.active_steering_count === "number" ? focusRuntimeSummary.active_steering_count : 0,
          events_count: typeof focusRuntimeSummary?.events_count === "number" ? focusRuntimeSummary.events_count : 0,
          latest_failure_code: typeof focusRuntimeSummary?.latest_failure_code === "string" ? focusRuntimeSummary.latest_failure_code : null,
          latest_failure_summary: typeof focusRuntimeSummary?.latest_failure_summary === "string" ? focusRuntimeSummary.latest_failure_summary : null,
          latest_event_type: typeof focusRuntimeSummary?.latest_event_type === "string" ? focusRuntimeSummary.latest_event_type : null,
          loop_stop_reason: typeof focusRuntimeSummary?.loop_stop_reason === "string" ? focusRuntimeSummary.loop_stop_reason : null,
          observation_signals: Array.isArray(focusRuntimeSummary?.observation_signals)
            ? focusRuntimeSummary.observation_signals.filter((item): item is string => typeof item === "string")
            : [],
        }
      : emptyFocusRuntimeSummary,
  };
}

function inferModuleFromRecommendation(item: RecommendationItem): DashboardHomeModuleKey {
  const corpus = `${item.intent.name} ${item.text}`.toLowerCase();

  if (
    corpus.includes("memory") ||
    corpus.includes("mirror") ||
    corpus.includes("summary") ||
    corpus.includes("habit") ||
    item.text.includes("镜") ||
    item.text.includes("总结")
  ) {
    return "memory";
  }

  if (
    corpus.includes("safety") ||
    corpus.includes("security") ||
    corpus.includes("approval") ||
    corpus.includes("authorize") ||
    corpus.includes("audit") ||
    item.text.includes("授权") ||
    item.text.includes("安全")
  ) {
    return "safety";
  }

  if (
    corpus.includes("todo") ||
    corpus.includes("note") ||
    corpus.includes("reminder") ||
    corpus.includes("schedule") ||
    item.text.includes("便签") ||
    item.text.includes("提醒")
  ) {
    return "notes";
  }

  return "tasks";
}

function getTaskStateKey(overview: AgentDashboardOverviewGetResult, taskModule: AgentDashboardModuleGetResult) {
  const status = overview.overview.focus_summary?.status;
  const runtimeSummary = getTaskModuleRuntimeSummary(taskModule, overview.overview.focus_summary?.task_id).focusRuntimeSummary;
  if (status === "confirming_intent") {
    return "task_completing";
  }

  if (status === "waiting_auth") {
    return "task_error_permission";
  }

  if (status === "waiting_input") {
    return "task_error_missing_info";
  }

  if (status === "blocked" || status === "paused" || status === "failed" || status === "ended_unfinished") {
    return "task_error_blocked";
  }

  if (status === "completed") {
    return "task_done";
  }

  if (runtimeSummary.active_steering_count > 0 || runtimeSummary.latest_event_type === "loop.retrying") {
    return "task_highlight";
  }

  if (getModuleHighlights(taskModule).length > 1) {
    return "task_highlight";
  }

  return "task_working";
}

function getNotesStateKey(notesModule: AgentDashboardModuleGetResult, recommendations: RecommendationItem[]) {
  const recommendationCount = recommendations.filter((item) => inferModuleFromRecommendation(item) === "notes").length;
  const highlights = getModuleHighlights(notesModule);

  if (highlights.some((item) => item.includes("重复") || item.includes("周期") || item.includes("习惯"))) {
    return "notes_reminder";
  }

  if (recommendationCount > 0) {
    return "notes_processing";
  }

  return "notes_scheduled";
}

function getNotesStateKeyFromBuckets(noteBuckets: DashboardHomeNoteBuckets | null) {
  if (!noteBuckets) {
    return null;
  }

  if (noteBuckets.primaryItem?.item.bucket === "recurring_rule") {
    return "notes_reminder" as const;
  }

  if (noteBuckets.primaryItem) {
    return "notes_processing" as const;
  }

  if (noteBuckets.recurring.length > 0) {
    return "notes_reminder" as const;
  }

  return "notes_scheduled" as const;
}

function getMemoryStateKey(memoryModule: AgentDashboardModuleGetResult) {
  return getModuleHighlights(memoryModule).length > 1 ? "memory_summary" : "memory_habit";
}

function getSafetyStateKey(overview: AgentDashboardOverviewGetResult) {
  const trustSummary = overview.overview.trust_summary;
  return trustSummary.pending_authorizations > 0 || trustSummary.risk_level !== "green" ? "safety_alert" : "safety_guard";
}

function buildTaskContext(
  overview: AgentDashboardOverviewGetResult,
  taskModule: AgentDashboardModuleGetResult,
): DashboardHomeContextItem[] {
  const focusSummary = overview.overview.focus_summary;
  const highlights = getModuleHighlights(taskModule);
  const taskRuntime = getTaskModuleRuntimeSummary(taskModule, focusSummary?.task_id);
  const runtimeSummary = taskRuntime.focusRuntimeSummary;

  if (!focusSummary) {
    return highlights.slice(0, 3).map((item, index) => ({
      iconKey: index === 0 ? "sparkles" : index === 1 ? "flag" : "info",
      text: item,
      type: index === 0 ? "active" : "hint",
    }));
  }

  const context: DashboardHomeContextItem[] = [
    {
      iconKey: focusSummary.status === "processing" ? "loader" : "check",
      text: `当前步骤：${focusSummary.current_step}`,
      time: formatDashboardTime(focusSummary.updated_at),
      type: focusSummary.status === "processing" ? "active" : "normal",
    },
    {
      iconKey: "flag",
      text: `下一步：${focusSummary.next_action}`,
      type: "hint",
    },
  ];

  if (runtimeSummary.latest_event_type) {
    context.push({
      iconKey: runtimeSummary.latest_event_type === "loop.retrying" ? "refresh" : "sparkles",
      text: `最近运行事件：${runtimeSummary.latest_event_type}`,
      type: runtimeSummary.latest_event_type === "loop.retrying" ? "warn" : "normal",
    });
  }
  if (runtimeSummary.active_steering_count > 0) {
    context.push({
      iconKey: "send",
      text: `待消费追加要求：${runtimeSummary.active_steering_count} 条`,
      type: "active",
    });
  }
  if (runtimeSummary.loop_stop_reason) {
    context.push({
      iconKey: "alert",
      text: `最近停止原因：${runtimeSummary.loop_stop_reason}`,
      type: "warn",
    });
  }
  if (taskRuntime.waitingAuthTasks > 0 && focusSummary.status !== "waiting_auth") {
    context.push({
      iconKey: "lock",
      text: `仍有 ${taskRuntime.waitingAuthTasks} 条任务等待授权`,
      type: "warn",
    });
  }
  if (context.length < 4 && highlights[0]) {
    context.push({
      iconKey: "sparkles",
      text: highlights[0],
      type: "normal",
    });
  }

  return context.slice(0, 4);
}

function buildTaskState(
  stateKey: DashboardHomeEventStateKey,
  overview: AgentDashboardOverviewGetResult,
  taskModule: AgentDashboardModuleGetResult,
) {
  const state = cloneStateData(dashboardHomeStates[stateKey]);
  const focusSummary = overview.overview.focus_summary;
  const runtimeSummary = getTaskModuleRuntimeSummary(taskModule, focusSummary?.task_id).focusRuntimeSummary;
  state.navigationTarget = buildModuleNavigationTarget("tasks");

  if (!focusSummary) {
    const highlights = getModuleHighlights(taskModule);
    if (highlights[0]) {
      state.headline = highlights[0];
    }
    if (highlights[1]) {
      state.subline = highlights[1];
    }
    state.context = buildTaskContext(overview, taskModule);
    return state;
  }

  state.headline = focusSummary.title;
  state.subline = [
    formatTaskStatusLabel(focusSummary.status),
    focusSummary.current_step,
    runtimeSummary.latest_event_type ? `最近事件：${runtimeSummary.latest_event_type}` : `下一步：${focusSummary.next_action}`,
  ].join(" · ");
  state.label = formatTaskStatusLabel(focusSummary.status);
  state.tag = formatTaskTag(focusSummary.status);
  state.progressLabel = runtimeSummary.active_steering_count > 0 ? `待消费要求 ${runtimeSummary.active_steering_count} 条` : focusSummary.next_action;
  state.context = buildTaskContext(overview, taskModule);
  state.navigationTarget = buildTaskNavigationTarget(focusSummary);

  if (focusSummary.status === "confirming_intent") {
    state.anomaly = {
      actionLabel: "确认继续",
      desc: `当前建议动作是：${focusSummary.next_action}。确认后会继续推进这条任务链。`,
      dismissLabel: "稍后处理",
      severity: "info",
      title: "当前任务正在等待你确认",
    };
  } else if (focusSummary.status === "waiting_auth") {
    state.anomaly = {
      actionLabel: "前往授权",
      desc: "当前任务已经进入待授权状态，处理完授权后会继续执行。",
      dismissLabel: "稍后处理",
      severity: "error",
      title: "有一项任务正在等待授权",
    };
  } else if (focusSummary.status === "waiting_input") {
    state.anomaly = {
      actionLabel: "补充信息",
      desc: "这条任务还缺少继续推进所需的输入，补充后可以继续执行。",
      dismissLabel: "稍后处理",
      severity: "warn",
      title: "当前任务需要补充信息",
    };
  } else if (focusSummary.status === "completed") {
    state.anomaly = undefined;
  }

  return state;
}

function mapHomeNotesFromBuckets(noteBuckets: DashboardHomeNoteBuckets) {
  return [
    ...noteBuckets.upcoming,
    ...noteBuckets.later,
    ...noteBuckets.recurring,
    ...noteBuckets.closed,
  ].slice(0, 3).map((item) => ({
    id: item.item.item_id,
    status: item.item.bucket === "closed"
      ? "done"
      : item.item.bucket === "recurring_rule"
        ? "recurring"
        : item.item.status === "normal"
          ? "pending"
          : "processing",
    tag: item.experience.summaryLabel,
    text: item.item.title,
    time: item.experience.timeHint,
  } satisfies DashboardHomeNoteItem));
}

function buildNotesHeadline(noteBuckets: DashboardHomeNoteBuckets | null) {
  if (noteBuckets?.primaryItem) {
    return noteBuckets.primaryItem.item.title;
  }

  return "这里还没有可协作的事项";
}

function buildNotesSubline(noteBuckets: DashboardHomeNoteBuckets | null, exceptions: number) {
  if (noteBuckets?.primaryItem) {
    return describeNotePreview(noteBuckets.primaryItem.item, noteBuckets.primaryItem.experience);
  }

  if (exceptions > 0) {
    return `当前例外项 ${exceptions} 条，建议优先整理最接近执行窗口的事项。`;
  }

  return "等你把想记住的事情交给便签协作后，这里会按近期要做、后续安排、重复事项和已结束四组方式整理出来。";
}

function buildNotesContext(noteItems: DashboardHomeNoteItem[], highlights: string[], noteBuckets: DashboardHomeNoteBuckets | null) {
  if (noteBuckets?.primaryItem) {
    return [
      {
        iconKey: "note",
        text: `${noteBuckets.primaryItem.experience.summaryLabel} · ${describeNotePreview(noteBuckets.primaryItem.item, noteBuckets.primaryItem.experience)}`,
        type: "active" as const,
      },
      {
        iconKey: "calendar",
        text: `今日待处理 ${noteBuckets.summary.dueToday} 条 · 已逾期 ${noteBuckets.summary.overdue} 条`,
        type: "normal" as const,
      },
      {
        iconKey: "repeat",
        text: `今日重复 ${noteBuckets.summary.recurringToday} 条 · 适合转任务 ${noteBuckets.summary.readyForAgent} 条`,
        type: "hint" as const,
      },
    ];
  }

  return [
    {
      iconKey: "note",
      text: "暂无便签",
      type: "normal" as const,
    },
    {
      iconKey: "calendar",
      text: "等你把想记住的事情交给便签协作后，这里会按近期要做、后续安排、重复事项和已结束四组方式整理出来。",
      type: "hint" as const,
    },
  ];
}

function buildNotesState(
  stateKey: DashboardHomeEventStateKey,
  notesModule: AgentDashboardModuleGetResult,
  noteBuckets: DashboardHomeNoteBuckets | null,
) {
  const state = cloneStateData(dashboardHomeStates[stateKey]);
  const highlights = getNotesHighlights(notesModule);
  const noteItems = noteBuckets ? mapHomeNotesFromBuckets(noteBuckets) : [];
  const exceptions = getModuleSummaryNumber(notesModule, "exceptions");

  state.headline = buildNotesHeadline(noteBuckets);
  state.subline = buildNotesSubline(noteBuckets, exceptions);
  state.context = buildNotesContext(noteItems, highlights, noteBuckets);
  state.notes = noteItems.length > 0 ? noteItems : state.notes;
  state.navigationTarget = buildModuleNavigationTarget("notes");

  return state;
}

function buildMemoryInsights(memoryModule: AgentDashboardModuleGetResult): DashboardHomeInsightItem[] {
  const icons = ["brain", "time", "repeat", "chat"] as const;
  const highlights = getMemoryHighlights(memoryModule);

  if (highlights.length === 0) {
    return cloneStateData(dashboardHomeStates.memory_summary).insights ?? [];
  }

  return highlights.slice(0, 4).map((text, index) => ({
    emphasis: index === 0,
    iconKey: icons[index] ?? "brain",
    text,
  }));
}

function createFormalMirrorStateBase(stateKey: DashboardHomeEventStateKey) {
  return cloneStateData(dashboardHomeStates[stateKey]);
}

function buildReferenceMirrorState(
  stateKey: DashboardHomeEventStateKey,
  overview: AgentMirrorOverviewGetResult,
) {
  const latestReference = overview.memory_references[0] ?? null;
  if (!latestReference) {
    return null;
  }

  const state = createFormalMirrorStateBase(stateKey);
  const summary =
    normalizeDashboardHomeCopy(latestReference.summary)
    ?? normalizeDashboardHomeCopy(latestReference.reason)
    ?? "最近有一条长期记忆再次命中当前协作。";
  const reason = normalizeDashboardHomeCopy(latestReference.reason);
  state.headline = "近期被调用记忆";
  state.subline = summary;
  state.context = [];
  appendDistinctContextItem(state.context, {
    iconKey: "link",
    text: "来源：近期长期记忆命中",
    type: "hint",
  });
  appendDistinctContextItem(
    state.context,
    reason && reason !== summary
      ? {
          iconKey: "brain",
          text: reason,
          type: "active",
        }
      : null,
  );
  state.insights = undefined;
  state.navigationTarget = buildMirrorDetailNavigationTarget("memory", "打开镜子页", latestReference.memory_id);
  return state;
}

function buildProfileMirrorState(
  stateKey: DashboardHomeEventStateKey,
  overview: AgentMirrorOverviewGetResult,
) {
  const profile = overview.profile;
  if (!profile) {
    return null;
  }

  const state = createFormalMirrorStateBase(stateKey);
  state.headline = "用户画像";
  state.subline = `工作风格：${profile.work_style}`;
  state.context = [
    {
      iconKey: "chat",
      text: `偏好交付：${profile.preferred_output}`,
      type: "normal",
    },
    {
      iconKey: "time",
      text: `活跃时段：${profile.active_hours}`,
      type: "hint",
    },
  ];
  state.insights = undefined;
  state.navigationTarget = buildMirrorDetailNavigationTarget("profile", "打开镜子页");
  return state;
}

function buildHistoryMirrorState(
  stateKey: DashboardHomeEventStateKey,
  overview: AgentMirrorOverviewGetResult,
) {
  if (!overview.history_summary[0]) {
    return null;
  }

  const state = createFormalMirrorStateBase(stateKey);
  state.headline = "历史概要";
  state.subline = overview.history_summary[0];
  state.context = overview.history_summary.slice(1, 3).map((summary, index) => ({
    iconKey: index === 0 ? "repeat" : "time",
    text: summary,
    type: "hint",
  }));
  state.insights = undefined;
  state.navigationTarget = buildMirrorDetailNavigationTarget("history", "打开镜子页");
  return state;
}

function buildMemoryHeadline(highlights: string[]) {
  if (highlights.length >= 2) {
    return "本周镜子观察";
  }

  return "最近协作镜像";
}

function buildMemorySubline(highlights: string[]) {
  return highlights[0] ?? "镜子会持续整理近期协作节奏和重复出现的模式。";
}

function buildPreferredFormalMirrorState(
  stateKey: DashboardHomeEventStateKey,
  overview: AgentMirrorOverviewGetResult,
) {
  return (
    buildReferenceMirrorState(stateKey, overview) ??
    buildProfileMirrorState(stateKey, overview) ??
    buildHistoryMirrorState(stateKey, overview)
  );
}

function buildFormalMirrorSummons(
  stateKey: DashboardHomeEventStateKey,
  overview: AgentMirrorOverviewGetResult,
): Array<Omit<DashboardHomeSummonEvent, "id">> {
  const templates = [
    buildReferenceMirrorState(stateKey, overview),
    buildProfileMirrorState(stateKey, overview),
    buildHistoryMirrorState(stateKey, overview),
  ].flatMap((state) => {
    if (!state) {
      return [];
    }

    return [{
      duration: 5_600,
      expandedState: state,
      message: state.headline,
      module: "memory",
      nextStep: state.navigationTarget?.label ?? dashboardModuleNextSteps.memory,
      priority: "low",
      reason: state.subline,
      stateKey: state.key,
    } satisfies Omit<DashboardHomeSummonEvent, "id">];
  });

  return dedupeSummonTemplates(templates);
}

function buildMemoryState(
  stateKey: DashboardHomeEventStateKey,
  memoryModule: AgentDashboardModuleGetResult,
  mirrorOverview: AgentMirrorOverviewGetResult | null,
) {
  const state = cloneStateData(dashboardHomeStates[stateKey]);
  if (mirrorOverview) {
    const formalMirrorState = buildPreferredFormalMirrorState(stateKey, mirrorOverview);
    if (formalMirrorState) {
      state.headline = formalMirrorState.headline;
      state.subline = formalMirrorState.subline;
      state.insights = formalMirrorState.insights;
      state.context = formalMirrorState.context;
      state.navigationTarget = formalMirrorState.navigationTarget;
      return state;
    }
  }

  const highlights = getMemoryHighlights(memoryModule);

  state.headline = buildMemoryHeadline(highlights);
  state.subline = buildMemorySubline(highlights);
  state.insights = buildMemoryInsights(memoryModule);
  state.context = highlights.slice(0, 3).map((item, index) => ({
    iconKey: index === 0 ? "brain" : index === 1 ? "repeat" : "time",
    text: item,
    type: index === 0 ? "active" : "hint",
  }));
  state.navigationTarget = buildModuleNavigationTarget("memory");

  return state;
}

function buildSafetyState(stateKey: DashboardHomeEventStateKey, overview: AgentDashboardOverviewGetResult, safetyModule: AgentDashboardModuleGetResult) {
  const state = cloneStateData(dashboardHomeStates[stateKey]);
  const trustSummary = overview.overview.trust_summary;
  const highlights = getModuleHighlights(safetyModule);
  const riskLabel = formatRiskLabel(trustSummary.risk_level);
  state.navigationTarget = buildModuleNavigationTarget(
    "safety",
    trustSummary.pending_authorizations > 0 ? "处理待授权操作" : "查看安全详情",
  );

  state.headline =
    trustSummary.pending_authorizations > 0
      ? `当前有 ${trustSummary.pending_authorizations} 项操作等待授权`
      : `当前整体风险等级为 ${riskLabel}`;
  state.subline =
    highlights[0] ??
    (trustSummary.pending_authorizations > 0
      ? "建议先处理待授权操作，再继续推进其它任务。"
      : `工作区位于 ${trustSummary.workspace_path || "当前默认目录"}。`);
  state.signals = [
    {
      iconKey: "shield",
      label: "风险等级",
      level: getSignalLevel(trustSummary.risk_level),
      translation: trustSummary.risk_level === "green" ? "当前边界稳定" : "建议先确认再继续",
      value: riskLabel,
    },
    {
      iconKey: "lock",
      label: "待授权",
      level: trustSummary.pending_authorizations > 0 ? "critical" : "normal",
      translation: trustSummary.pending_authorizations > 0 ? "存在等待你确认的操作" : "当前没有挂起请求",
      value: String(trustSummary.pending_authorizations),
    },
    {
      iconKey: "history",
      label: "恢复点",
      level: trustSummary.has_restore_point ? "normal" : "warn",
      translation: trustSummary.has_restore_point ? "当前可以回退" : "建议执行高风险动作前补一个恢复点",
      value: trustSummary.has_restore_point ? "可用" : "暂无",
    },
  ];
  state.context = [];
  appendDistinctContextItem(
    state.context,
    highlights[0]
      ? {
          iconKey: trustSummary.pending_authorizations > 0 ? "lock" : "shield",
          text: highlights[0],
          type: trustSummary.pending_authorizations > 0 ? "warn" : "hint",
        }
      : null,
  );
  appendDistinctContextItem(
    state.context,
    trustSummary.workspace_path
      ? {
          iconKey: "file",
          text: `工作区：${trustSummary.workspace_path}`,
          type: "hint",
        }
      : null,
  );

  if (trustSummary.pending_authorizations > 0 || trustSummary.risk_level !== "green") {
    state.anomaly = {
      actionLabel: "查看安全详情",
      desc: trustSummary.pending_authorizations > 0 ? "先处理待授权操作，再继续推进其它任务。" : "建议先确认当前风险边界。",
      dismissLabel: "稍后再看",
      severity: trustSummary.pending_authorizations > 0 ? "error" : "warn",
      title: trustSummary.pending_authorizations > 0 ? "安全链路有待处理项" : "当前需要留意执行边界",
    };
  } else {
    state.anomaly = undefined;
  }

  return state;
}

function getModuleStateKeyMap(
  overview: AgentDashboardOverviewGetResult,
  moduleResults: Record<DashboardHomeModuleKey, AgentDashboardModuleGetResult>,
  recommendations: RecommendationItem[],
) {
  return {
    memory: getMemoryStateKey(moduleResults.memory),
    notes: getNotesStateKey(moduleResults.notes, recommendations),
    safety: getSafetyStateKey(overview),
    tasks: getTaskStateKey(overview, moduleResults.tasks),
  } satisfies Record<DashboardHomeModuleKey, DashboardHomeEventStateKey>;
}

function buildStateGroups(stateKeys: Record<DashboardHomeModuleKey, DashboardHomeEventStateKey>): DashboardHomeStateGroup[] {
  return (Object.keys(stateKeys) as DashboardHomeModuleKey[]).map((module) => ({
    key: module,
    label: dashboardModuleLabels[module],
    states: [stateKeys[module]],
  }));
}

function getSummonPriority(module: DashboardHomeModuleKey, stateKey: DashboardHomeEventStateKey): DashboardHomeSummonEvent["priority"] {
  if (module === "safety" || stateKey === "task_completing" || stateKey === "task_error_permission") {
    return "urgent";
  }

  if (module === "memory") {
    return "low";
  }

  return "normal";
}

function buildOverviewSummons(
  overview: AgentDashboardOverviewGetResult,
  stateKeys: Record<DashboardHomeModuleKey, DashboardHomeEventStateKey>,
  stateMap: Record<DashboardHomeEventStateKey, DashboardHomeStateData>,
): Array<Omit<DashboardHomeSummonEvent, "id">> {
  const quickActions = getOverviewQuickActions(overview);
  const highValueSignals = getOverviewSignals(overview);
  const focusSummary = overview.overview.focus_summary;
  const trustSummary = overview.overview.trust_summary;
  const safetyState = stateMap[stateKeys.safety];
  const taskState = stateMap[stateKeys.tasks];
  const templates: Array<Omit<DashboardHomeSummonEvent, "id">> = [];

  // Keep the first summon anchored to formal overview fields so the home orb
  // surfaces live task/security signals before softer recommendation copy.
  const hasUrgentSafetySignal = trustSummary.pending_authorizations > 0 || trustSummary.risk_level !== "green";

  if (hasUrgentSafetySignal) {
    templates.push({
      duration: 6_200,
      // Safety summons are hard-routed to the safety module. Keep their
      // headline anchored to safety state so task-oriented overview copy does
      // not leak into a different navigation target.
      message: safetyState.headline,
      module: "safety",
      nextStep: getSummonNextStep(safetyState),
      priority: "urgent",
      reason: safetyState.subline,
      stateKey: stateKeys.safety,
    });
  }

  if (focusSummary) {
    const overflowSignal = highValueSignals.find((signal) => signal !== templates[0]?.message);
    templates.push({
      duration: 6_000,
      message: focusSummary.title,
      module: "tasks",
      nextStep: getSummonNextStep(taskState, quickActions),
      priority: getSummonPriority("tasks", stateKeys.tasks),
      reason: [focusSummary.current_step, focusSummary.next_action, overflowSignal].filter(Boolean).join(" · "),
      stateKey: stateKeys.tasks,
    });
  }

  if (!hasUrgentSafetySignal && trustSummary.has_restore_point) {
    templates.push({
      duration: 5_600,
      message: "最近恢复点可用",
      module: "safety",
      nextStep: getSummonNextStep(safetyState),
      priority: "low",
      reason: safetyState.subline,
      stateKey: stateKeys.safety,
    });
  }

  if (templates.length === 0 && highValueSignals[0]) {
    const targetState = trustSummary.pending_authorizations > 0 || trustSummary.risk_level !== "green"
      ? safetyState
      : taskState;

    templates.push({
      duration: 5_800,
      message: highValueSignals[0],
      module: targetState.module,
      nextStep: getSummonNextStep(targetState, quickActions),
      priority: targetState.module === "safety" ? "urgent" : getSummonPriority(targetState.module, targetState.key),
      reason: targetState.subline,
      stateKey: targetState.key,
    });
  }

  return dedupeSummonTemplates(templates);
}

function buildModuleSummarySummons(
  stateKeys: Record<DashboardHomeModuleKey, DashboardHomeEventStateKey>,
  stateMap: Record<DashboardHomeEventStateKey, DashboardHomeStateData>,
): Array<Omit<DashboardHomeSummonEvent, "id">> {
  const modules: DashboardHomeModuleKey[] = ["notes", "memory"];

  return dedupeSummonTemplates(
    modules.flatMap((module) => {
      const state = stateMap[stateKeys[module]];
      if (!state.headline.trim()) {
        return [];
      }

      if (module === "notes" && state.headline === "这里还没有可协作的事项") {
        return [];
      }

      if (module === "memory" && state.headline === "最近协作镜像" && state.subline === "镜子会持续整理近期协作节奏和重复出现的模式。") {
        return [];
      }

      return [{
        duration: 5_600,
        message: state.headline,
        module,
        nextStep: state.navigationTarget?.label ?? dashboardModuleNextSteps[module],
        priority: getSummonPriority(module, state.key),
        reason: state.subline,
        stateKey: state.key,
      } satisfies Omit<DashboardHomeSummonEvent, "id">];
    }),
  );
}

function buildRecommendationSummons(
  recommendations: RecommendationItem[],
  stateKeys: Record<DashboardHomeModuleKey, DashboardHomeEventStateKey>,
  moduleResults: Record<DashboardHomeModuleKey, AgentDashboardModuleGetResult>,
): Array<Omit<DashboardHomeSummonEvent, "id">> {
  const templates = recommendations.slice(0, 4).map((item) => {
    const module = inferModuleFromRecommendation(item);
    const highlights = getModuleHighlights(moduleResults[module]);

    return {
      duration: 6_200,
      message: item.text,
      module,
      nextStep: dashboardModuleNextSteps[module],
      priority: getSummonPriority(module, stateKeys[module]),
      reason: highlights[0] ?? `来自 ${dashboardModuleLabels[module]} 模块的实时建议`,
      recommendationId: item.recommendation_id,
      stateKey: stateKeys[module],
    } satisfies Omit<DashboardHomeSummonEvent, "id">;
  });

  return dedupeSummonTemplates(templates);
}

function buildVoiceSequences(
  recommendations: RecommendationItem[],
  stateKeys: Record<DashboardHomeModuleKey, DashboardHomeEventStateKey>,
  stateMap: Record<DashboardHomeEventStateKey, DashboardHomeStateData>,
) {
  const sequences = recommendations.slice(0, 4).map((item) => {
    const module = inferModuleFromRecommendation(item);
    const state = stateMap[stateKeys[module]];

    return {
      echoPool: [dashboardModuleLabels[module], item.intent.name, "当前建议"].filter(Boolean),
      executingSteps: [...dashboardVoiceExecutionSteps[module]],
      fragments: [item.text, state.subline].filter(Boolean),
      module,
      recommendationId: item.recommendation_id,
      suggestion: item.text,
      summary: `我会先把你带到${dashboardModuleLabels[module]}页，并继续围绕这条建议推进。`,
    } satisfies DashboardVoiceSequence;
  });

  return sequences;
}

function buildFocusLine(
  overview: AgentDashboardOverviewGetResult,
  taskModule: AgentDashboardModuleGetResult,
  summonTemplates: Array<Omit<DashboardHomeSummonEvent, "id">>,
) {
  if (overview.overview.focus_summary) {
    const runtimeSummary = getTaskModuleRuntimeSummary(taskModule, overview.overview.focus_summary.task_id).focusRuntimeSummary;
    const overviewSignals = getOverviewSignals(overview);

    return {
      headline: overview.overview.focus_summary.title,
      reason: [
        overview.overview.focus_summary.current_step,
        overview.overview.focus_summary.next_action,
        runtimeSummary.latest_event_type ?? runtimeSummary.loop_stop_reason ?? overviewSignals[0],
      ]
        .filter(Boolean)
        .join(" · "),
    };
  }

  if (summonTemplates[0]) {
    return {
      headline: summonTemplates[0].message,
      reason: summonTemplates[0].reason,
    };
  }

  return {
    headline: "首页总览已经连接到真实任务轨道。",
    reason: "当有新的焦点任务、授权或推荐出现时，这里会优先展示最值得关注的信号。",
  };
}

function buildDashboardHomeData(input: {
  loadWarnings: string[];
  moduleResults: Record<DashboardHomeModuleKey, AgentDashboardModuleGetResult>;
  mirrorOverview: AgentMirrorOverviewGetResult | null;
  noteBuckets: DashboardHomeNoteBuckets | null;
  overview: AgentDashboardOverviewGetResult;
  recommendations: AgentRecommendationGetResult;
}): DashboardHomeData {
  const stateMap = createBaseStateMap();
  const noteBucketsStateKey = getNotesStateKeyFromBuckets(input.noteBuckets);
  const inferredStateKeys = getModuleStateKeyMap(input.overview, input.moduleResults, input.recommendations.items);
  const stateKeys = {
    ...inferredStateKeys,
    notes: noteBucketsStateKey ?? inferredStateKeys.notes,
  };

  stateMap[stateKeys.tasks] = buildTaskState(stateKeys.tasks, input.overview, input.moduleResults.tasks);
  stateMap[stateKeys.notes] = buildNotesState(stateKeys.notes, input.moduleResults.notes, input.noteBuckets);
  stateMap[stateKeys.memory] = buildMemoryState(stateKeys.memory, input.moduleResults.memory, input.mirrorOverview);
  stateMap[stateKeys.safety] = buildSafetyState(stateKeys.safety, input.overview, input.moduleResults.safety);

  const overviewSummons = buildOverviewSummons(input.overview, stateKeys, stateMap);
  const mirrorSummons = input.mirrorOverview ? buildFormalMirrorSummons(stateKeys.memory, input.mirrorOverview) : [];
  const moduleSummons = buildModuleSummarySummons(stateKeys, stateMap);
  const recommendationSummons = buildRecommendationSummons(input.recommendations.items, stateKeys, input.moduleResults);
  const summonTemplates = dedupeSummonTemplates([...overviewSummons, ...mirrorSummons, ...moduleSummons, ...recommendationSummons]);

  return {
    focusLine: buildFocusLine(input.overview, input.moduleResults.tasks, summonTemplates),
    loadWarnings: [...input.loadWarnings],
    stateGroups: buildStateGroups(stateKeys),
    stateMap,
    summonTemplates,
    voiceSequences: buildVoiceSequences(input.recommendations.items, stateKeys, stateMap),
  };
}

function createEmptyDashboardModuleResult(module: DashboardHomeModuleKey): AgentDashboardModuleGetResult {
  return {
    highlights: [],
    module,
    summary: {},
    tab: dashboardModuleTabs[module],
  };
}

function createEmptyRecommendationResult(): AgentRecommendationGetResult {
  return {
    cooldown_hit: false,
    items: [],
  };
}

function formatDashboardHomeLoadWarning(label: string, error: unknown) {
  const message = error instanceof Error ? error.message : "unknown dashboard read failure";
  return `${label}同步失败：${message}`;
}

export async function loadDashboardHomeData(): Promise<DashboardHomeData> {
  const [overviewResult, tasksResult, notesResult, memoryResult, safetyResult, recommendationsResult, mirrorOverviewResult, noteBucketsResult] = await Promise.allSettled([
    getDashboardOverview({
      focus_mode: false,
      include: ["focus_summary", "trust_summary", "quick_actions", "high_value_signal"],
      request_meta: createRequestMeta("dashboard_overview"),
    }),
    getDashboardModule({
      module: "tasks",
      request_meta: createRequestMeta("dashboard_module_tasks"),
      tab: dashboardModuleTabs.tasks,
    }),
    getDashboardModule({
      module: "notes",
      request_meta: createRequestMeta("dashboard_module_notes"),
      tab: dashboardModuleTabs.notes,
    }),
    getDashboardModule({
      module: "memory",
      request_meta: createRequestMeta("dashboard_module_memory"),
      tab: dashboardModuleTabs.memory,
    }),
    getDashboardModule({
      module: "safety",
      request_meta: createRequestMeta("dashboard_module_safety"),
      tab: dashboardModuleTabs.safety,
    }),
    getRecommendations({
      context: {
        app_name: "CialloClaw Desktop",
        page_title: "Dashboard Orbit",
      },
      request_meta: createRequestMeta("dashboard_recommendations"),
      scene: "idle",
      source: "dashboard",
    }),
    getMirrorOverview({
      include: ["profile", "history_summary", "daily_summary", "memory_references"],
      request_meta: createRequestMeta("dashboard_home_mirror_overview"),
    }),
    loadDashboardHomeNoteBuckets(),
  ]);

  if (overviewResult.status === "rejected") {
    throw overviewResult.reason;
  }

  const loadWarnings: string[] = [];
  const tasksModule = tasksResult.status === "fulfilled"
    ? tasksResult.value
    : (loadWarnings.push(formatDashboardHomeLoadWarning("任务摘要", tasksResult.reason)), createEmptyDashboardModuleResult("tasks"));
  const notesModule = notesResult.status === "fulfilled"
    ? notesResult.value
    : (loadWarnings.push(formatDashboardHomeLoadWarning("便签摘要", notesResult.reason)), createEmptyDashboardModuleResult("notes"));
  const memoryModule = memoryResult.status === "fulfilled"
    ? memoryResult.value
    : (loadWarnings.push(formatDashboardHomeLoadWarning("镜子摘要", memoryResult.reason)), createEmptyDashboardModuleResult("memory"));
  const safetyModule = safetyResult.status === "fulfilled"
    ? safetyResult.value
    : (loadWarnings.push(formatDashboardHomeLoadWarning("安全摘要", safetyResult.reason)), createEmptyDashboardModuleResult("safety"));
  const recommendations = recommendationsResult.status === "fulfilled"
    ? recommendationsResult.value
    : (loadWarnings.push(formatDashboardHomeLoadWarning("建议流", recommendationsResult.reason)), createEmptyRecommendationResult());
  const mirrorOverview = mirrorOverviewResult.status === "fulfilled"
    ? mirrorOverviewResult.value
    : (loadWarnings.push(formatDashboardHomeLoadWarning("镜子概览", mirrorOverviewResult.reason)), null);
  const noteBuckets = noteBucketsResult.status === "fulfilled"
    ? noteBucketsResult.value
    : null;

  if (noteBuckets) {
    loadWarnings.push(...noteBuckets.warnings);
  } else if (noteBucketsResult.status === "rejected") {
    loadWarnings.push(formatDashboardHomeLoadWarning("便签详情", noteBucketsResult.reason));
  }

  return buildDashboardHomeData({
    loadWarnings,
    moduleResults: {
      memory: memoryModule,
      notes: notesModule,
      safety: safetyModule,
      tasks: tasksModule,
    },
    mirrorOverview,
    noteBuckets,
    overview: overviewResult.value,
    recommendations,
  });
}

export async function submitDashboardHomeRecommendationFeedback(recommendationId: string, feedback: RecommendationFeedback) {
  return submitRecommendationFeedback({
    feedback,
    recommendation_id: recommendationId,
    request_meta: createRequestMeta(`dashboard_recommendation_feedback_${recommendationId}`),
  });
}
