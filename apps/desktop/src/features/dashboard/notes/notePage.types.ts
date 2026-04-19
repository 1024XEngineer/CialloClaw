import type { AgentNotepadConvertToTaskResult, AgentNotepadUpdateResult, TodoBucket, TodoItem } from "@cialloclaw/protocol";

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
  | "convert-to-task"
  | "view-task";

export type NoteType = "reminder" | "follow-up" | "template" | "recurring" | "archive";

export type NoteResource = {
  id: string;
  label: string;
  path: string | null;
  type: string;
  openAction?: "task_detail" | "open_url" | "open_file" | "reveal_in_folder" | null;
  taskId?: string | null;
  url?: string | null;
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

export type NoteUpdateOutcome = {
  result: AgentNotepadUpdateResult;
  source: NoteDataSource;
};

export type NoteActionShortcut = {
  id: string;
  label: string;
  tooltip: string;
};

export type NoteCanvasCardLayout = {
  itemId: string;
  sourceBucket: TodoBucket;
  x: number;
  y: number;
};

export type NoteCanvasLayoutSnapshot = Record<string, NoteCanvasCardLayout>;

export type NoteDrawerPreferenceSnapshot = {
  drawerOpen: boolean;
  expandedBucket: TodoBucket;
};

export type NoteSelectionState = {
  bucket: TodoBucket | null;
  itemId: string | null;
};

export type NoteDetailLayerState = {
  activeBucket: TodoBucket | null;
  openItemIds: string[];
};
