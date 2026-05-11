import type { TodoBucket, TodoItem, TodoStatus } from "@cialloclaw/protocol";
import type { NoteDetailExperience, NoteListItem, NotePreviewGroupKey, NoteSummary } from "./notePage.types";

export function getNoteBucketLabel(bucket: TodoBucket) {
  const labels: Record<TodoBucket, string> = {
    closed: "已结束",
    later: "后续安排",
    recurring_rule: "重复事项",
    upcoming: "近期要做",
  };

  return labels[bucket];
}

export function getNoteStatusBadgeClass(status: TodoStatus) {
  const classes: Record<TodoStatus, string> = {
    normal: "bg-[#EAD7C2]/70 text-[#875D39] ring-[#E2C4A6]/70",
    due_today: "bg-[#F4D2B0]/72 text-[#8A5A22] ring-[#E9B883]/72",
    overdue: "bg-[#F0C7BE]/76 text-[#9D4A46] ring-[#E6AAA3]/72",
    completed: "bg-[#D7E4D7]/72 text-[#52745C] ring-[#BED1BF]/72",
    cancelled: "bg-[#DDD4CF]/72 text-[#706661] ring-[#CBC0BA]/72",
  };

  return classes[status];
}

export function sortNotesByUrgency(items: NoteListItem[]) {
  return [...items].sort((left, right) => {
    const leftTime = left.item.due_at ? new Date(left.item.due_at).getTime() : Number.MAX_SAFE_INTEGER;
    const rightTime = right.item.due_at ? new Date(right.item.due_at).getTime() : Number.MAX_SAFE_INTEGER;
    return leftTime - rightTime;
  });
}

export function sortClosedNotes(items: NoteListItem[]) {
  return [...items].sort((left, right) => {
    const leftTime = left.experience.endedAt ? new Date(left.experience.endedAt).getTime() : Date.now();
    const rightTime = right.experience.endedAt ? new Date(right.experience.endedAt).getTime() : Date.now();
    return rightTime - leftTime;
  });
}

export function describeNotePreview(item: TodoItem, experience: NoteDetailExperience) {
  if (item.bucket === "upcoming") {
    return `${experience.previewStatus} · ${experience.timeHint}`;
  }

  if (item.bucket === "later") {
    return `未到时间 · ${experience.timeHint}`;
  }

  if (item.bucket === "recurring_rule") {
    if (experience.isRecurringEnabled === false) {
      return "重复规则 · 已暂停";
    }

    return `${experience.repeatRule ?? "重复规则"} · 下次 ${experience.timeHint}`;
  }

  return `${experience.previewStatus} · ${experience.timeHint}`;
}

export function formatNoteBoardTimeHint(item: TodoItem, experience: NoteDetailExperience) {
  if (item.bucket === "recurring_rule") {
    if (experience.isRecurringEnabled === false) {
      return "重复已暂停";
    }

    return `下次执行 ${experience.timeHint}`;
  }

  if (item.bucket === "closed") {
    return `结束时间 ${experience.timeHint}`;
  }

  return `开始时间 ${experience.timeHint}`;
}

/**
 * Strips Windows extended-path prefixes from note UI copy while keeping the
 * underlying filesystem path untouched for open/reveal actions.
 *
 * @param value Raw path-like text from note metadata or related resources.
 * @returns User-facing path text without the `\\?\` prefix.
 */
export function formatNoteDisplayPath(value: string | null | undefined) {
  if (!value) {
    return "";
  }

  if (value.startsWith("\\\\?\\UNC\\")) {
    return `\\\\${value.slice("\\\\?\\UNC\\".length)}`;
  }

  if (value.startsWith("\\\\?\\")) {
    return value.slice("\\\\?\\".length);
  }

  return value;
}

export function buildNoteSummary(groups: Pick<Record<NotePreviewGroupKey, NoteListItem[]>, "upcoming" | "recurring_rule">) {
  const dueToday = groups.upcoming.filter((item) => item.item.status === "due_today").length;
  const overdue = groups.upcoming.filter((item) => item.item.status === "overdue").length;
  const recurringToday = groups.recurring_rule.filter((item) => {
    if (!item.experience.isRecurringEnabled) {
      return false;
    }
    const occurrence = item.experience.nextOccurrenceAt ?? item.experience.plannedAt ?? item.item.due_at;
    if (!occurrence) {
      return false;
    }

    const date = new Date(occurrence);
    const now = new Date();

    return (
      date.getFullYear() === now.getFullYear() &&
      date.getMonth() === now.getMonth() &&
      date.getDate() === now.getDate()
    );
  }).length;
  const readyForAgent = groups.upcoming.filter((item) => item.experience.canConvertToTask).length;

  return {
    dueToday,
    overdue,
    readyForAgent,
    recurringToday,
  } satisfies NoteSummary;
}

export function groupClosedNotes(items: NoteListItem[], expanded: boolean) {
  const now = Date.now();
  const recent: NoteListItem[] = [];
  const weekly: NoteListItem[] = [];
  const older: NoteListItem[] = [];

  items.forEach((item) => {
    const endedAt = item.experience.endedAt ? new Date(item.experience.endedAt).getTime() : now;
    const diffDays = (now - endedAt) / (1000 * 60 * 60 * 24);

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

  const groups: Array<{ key: "recent" | "weekly" | "older"; title: string; description: string; items: NoteListItem[] }> = [
    { key: "recent" as const, title: "近 3 天", description: "最近完成或取消的事项。", items: recent },
    { key: "weekly" as const, title: "近 7 天", description: "一周内结束的记录。", items: weekly },
  ];

  if (expanded && older.length > 0) {
    groups.push({ key: "older" as const, title: "更多", description: "更早结束的事项。", items: older });
  }

  return groups.filter((group) => group.items.length > 0);
}

export function getNoteActionLabel(action: NotePreviewGroupKey, canConvertToTask: boolean) {
  if (action === "upcoming") {
    return canConvertToTask ? "转交给 Agent" : "继续安排";
  }

  if (action === "later") {
    return canConvertToTask ? "提前并转任务" : "提前到近期";
  }

  if (action === "recurring_rule") {
    return "查看规则";
  }

  return "查看记录";
}
