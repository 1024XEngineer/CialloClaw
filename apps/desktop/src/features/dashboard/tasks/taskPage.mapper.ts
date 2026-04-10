import type { Task, TaskStep } from "@cialloclaw/protocol";
import { formatStatusLabel, formatTimestamp } from "@/utils/formatters";
import type {
  FinishedTaskGroup,
  TaskExperience,
  TaskListItem,
  TaskPriority,
  TaskProgressState,
  TaskPrimaryAction,
  TaskStateVoice,
} from "./taskPage.types";

export function formatTaskSourceLabel(sourceType: Task["source_type"]) {
  const labels: Record<Task["source_type"], string> = {
    dragged_file: "拖入文件",
    error_signal: "错误信号",
    hover_input: "悬浮输入",
    selected_text: "选中文本",
    todo: "待办触发",
    voice: "语音提交",
  };

  return labels[sourceType] ?? sourceType;
}

export function getTaskStatusBadgeClass(status: Task["status"]) {
  const classes: Record<Task["status"], string> = {
    confirming_intent: "bg-sky-100 text-sky-700 ring-sky-200/80",
    processing: "bg-blue-100 text-blue-700 ring-blue-200/80",
    waiting_auth: "bg-amber-100 text-amber-700 ring-amber-200/80",
    waiting_input: "bg-violet-100 text-violet-700 ring-violet-200/80",
    paused: "bg-slate-200 text-slate-700 ring-slate-300/80",
    blocked: "bg-orange-100 text-orange-700 ring-orange-200/80",
    failed: "bg-rose-100 text-rose-700 ring-rose-200/80",
    completed: "bg-emerald-100 text-emerald-700 ring-emerald-200/80",
    cancelled: "bg-zinc-200 text-zinc-700 ring-zinc-300/80",
    ended_unfinished: "bg-stone-200 text-stone-700 ring-stone-300/80",
  };

  return classes[status];
}

export function getPriorityBadgeClass(priority: TaskPriority) {
  const classes: Record<TaskPriority, string> = {
    critical: "bg-orange-100 text-orange-700 ring-orange-200/80",
    high: "bg-slate-100 text-slate-700 ring-slate-200/80",
    steady: "bg-blue-50 text-blue-700 ring-blue-100/80",
  };

  return classes[priority];
}

export function getTaskProgress(timeline: TaskStep[]): TaskProgressState {
  const total = timeline.length || 1;
  const completedCount = timeline.filter((step) => step.status === "completed").length;
  const current = timeline.find((step) => step.status === "running") ?? timeline.find((step) => step.status === "pending") ?? null;
  const remainingCount = Math.max(total - completedCount, current?.status === "running" ? 0 : 0);

  return {
    completedCount,
    currentLabel: current?.name ?? "全部步骤已走完",
    percent: Math.round((completedCount / total) * 100),
    remainingCount,
    total,
  };
}

export function getTaskStateVoice(task: Task, experience: TaskExperience, timeline: TaskStep[]): TaskStateVoice {
  const progress = getTaskProgress(timeline);

  if (task.status === "processing") {
    return {
      title: progress.percent >= 70 ? "接近完成" : "正在推进",
      body: `${progress.currentLabel}，还剩 ${Math.max(progress.total - progress.completedCount, 1)} 个动作待收口。`,
    };
  }

  if (task.status === "waiting_auth" || task.status === "waiting_input" || task.status === "paused") {
    return {
      title: "暂时等待中",
      body: experience.waitingReason ?? "当前任务暂时停在等待节点，需要额外确认后继续。",
    };
  }

  if (task.status === "failed" || task.status === "blocked") {
    return {
      title: "异常中",
      body: experience.blockedReason ?? "当前任务遇到阻塞，需要先补齐条件或恢复后再继续。",
    };
  }

  return {
    title: "已结束",
    body: experience.endedSummary ?? "当前任务已经收束，可查看产出摘要或重新启动。",
  };
}

export function getFinishedTaskGroups(items: TaskListItem[], expanded: boolean): FinishedTaskGroup[] {
  const now = Date.now();
  const recent: TaskListItem[] = [];
  const weekly: TaskListItem[] = [];
  const older: TaskListItem[] = [];

  items.forEach((item) => {
    const finishedAt = item.task.finished_at ? new Date(item.task.finished_at).getTime() : now;
    const diffDays = (now - finishedAt) / (1000 * 60 * 60 * 24);

    if (diffDays <= 3) {
      recent.push(item);
      return;
    }

    if (diffDays <= 7) {
      weekly.push(item);
      return;
    }

    older.push(item);
  });

  const groups: FinishedTaskGroup[] = [
    { key: "recent", title: "近 3 天", description: "刚刚结束的任务与取消记录。", items: recent },
    { key: "weekly", title: "近 7 天", description: "一周内已沉淀的任务舱记录。", items: weekly },
  ];

  if (expanded && older.length > 0) {
    groups.push({ key: "older", title: "更早", description: "继续向前追溯的历史任务。", items: older });
  }

  return groups.filter((group) => group.items.length > 0);
}

export function getTaskPrimaryActions(task: Task): TaskPrimaryAction[] {
  if (task.status === "processing") {
    return [
      { action: "pause", label: "暂停", tooltip: "先把当前执行暂停在这里。" },
      { action: "cancel", label: "取消", tooltip: "结束当前任务，并保留已有轨迹。" },
      { action: "open-safety", label: "安全详情", tooltip: "查看当前任务的风险与授权摘要。" },
      { action: "edit", label: "修改任务", tooltip: "任务修改能力将在后续开放。" },
    ];
  }

  if (task.status === "waiting_auth" || task.status === "waiting_input" || task.status === "paused" || task.status === "blocked") {
    return [
      { action: "resume", label: "继续", tooltip: "恢复当前任务并继续推进。" },
      { action: "cancel", label: "取消", tooltip: "结束当前任务，并保留已有轨迹。" },
      { action: "open-safety", label: "安全详情", tooltip: "查看当前任务的风险与授权摘要。" },
      { action: "edit", label: "修改任务", tooltip: "任务修改能力将在后续开放。" },
    ];
  }

  if (task.status === "failed") {
    return [
      { action: "restart", label: "重新启动", tooltip: "从当前任务生成一条新的执行路径。" },
      { action: "open-safety", label: "安全详情", tooltip: "查看失败前后的风险和恢复点摘要。" },
      { action: "edit", label: "修改任务", tooltip: "任务修改能力将在后续开放。" },
    ];
  }

  return [
    { action: "restart", label: "重启任务", tooltip: "以当前任务为蓝本重新开始一轮。" },
    { action: "open-safety", label: "安全详情", tooltip: "查看任务收束后的风险摘要与恢复点。" },
  ];
}

export function sortTasksByLatest(items: TaskListItem[]) {
  return [...items].sort((left, right) => {
    const leftTime = new Date(left.task.finished_at ?? left.task.updated_at).getTime();
    const rightTime = new Date(right.task.finished_at ?? right.task.updated_at).getTime();
    return rightTime - leftTime;
  });
}

export function describeTaskPreview(task: Task, currentStep: string) {
  if (task.status === "completed" || task.status === "cancelled" || task.status === "ended_unfinished") {
    return `${formatStatusLabel(task.status)} · ${formatTimestamp(task.finished_at)}`;
  }

  return `${formatStatusLabel(task.status)} · 当前执行到 ${currentStep}`;
}
