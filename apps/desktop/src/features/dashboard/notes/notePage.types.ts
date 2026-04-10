import type { AgentNotepadConvertToTaskResult, TodoBucket, TodoItem, TodoStatus } from "@cialloclaw/protocol";

export type NoteDataSource = "rpc" | "mock";
export type NotePreviewGroupKey = "upcoming" | "later" | "recurring_rule" | "closed";
export type NoteDetailAction =
  | "complete"
  | "cancel"
  | "skip-once"
  | "edit"
  | "open-resource"
  | "move-upcoming"
  | "toggle-recurring"
  | "cancel-recurring"
  | "restore"
  | "delete"
  | "convert-to-task";

export type NoteType = "reminder" | "follow-up" | "template" | "recurring" | "archive";

export type NoteResource = {
  id: string;
  label: string;
  path: string;
  type: string;
};

export type NoteAgentSuggestion = {
  label: string;
  detail: string;
};

export type NoteDetailExperience = {
  title: string;
  previewStatus: string;
  timeHint: string;
  detailStatus: string;
  detailStatusTone: "normal" | "warn" | "overdue" | "done";
  typeLabel: string;
  noteType: NoteType;
  noteText: string;
  prerequisite: string | null;
  relatedResources: NoteResource[];
  agentSuggestion: NoteAgentSuggestion;
  nextOccurrenceAt: string | null;
  repeatRule: string | null;
  recentInstanceStatus: string | null;
  effectiveScope: string | null;
  plannedAt: string | null;
  endedAt: string | null;
  isRecurringEnabled: boolean;
  canConvertToTask: boolean;
  summaryLabel: string;
};

export type NoteListItem = {
  item: TodoItem;
  experience: NoteDetailExperience;
};

export type NoteBucketsData = {
  closed: NoteListItem[];
  later: NoteListItem[];
  recurring_rule: NoteListItem[];
  source: NoteDataSource;
  upcoming: NoteListItem[];
};

export type NoteSummary = {
  dueToday: number;
  overdue: number;
  readyForAgent: number;
  recurringToday: number;
};

export type NoteConvertOutcome = {
  result: AgentNotepadConvertToTaskResult;
  source: NoteDataSource;
};

export type NoteActionShortcut = {
  id: string;
  label: string;
  tooltip: string;
};
