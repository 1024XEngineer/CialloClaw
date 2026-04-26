import type { AgentNotepadConvertToTaskResult, AgentNotepadUpdateResult, TodoItem } from "@cialloclaw/protocol";

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
  openAction?: "task_detail" | "open_url" | "open_file" | "reveal_in_folder" | "copy_path" | null;
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
  sourceNote?: {
    localOnly: boolean;
    path: string;
    sourceLine?: number | null;
    title?: string | null;
  } | null;
};

export type SourceNoteMetadataEntry = {
  key: string;
  value: string;
};

export type SourceNoteEditorDraft = {
  agentSuggestion: string;
  bucket: NotePreviewGroupKey;
  checked: boolean;
  createdAt: string;
  dueAt: string;
  effectiveScope: string;
  endedAt: string;
  extraMetadata: SourceNoteMetadataEntry[];
  nextOccurrenceAt: string;
  noteText: string;
  prerequisite: string;
  recentInstanceStatus: string;
  repeatRule: string;
  sourceLine: number | null;
  sourcePath: string | null;
  title: string;
  updatedAt: string;
};

export type SourceNoteEditorBlock = SourceNoteEditorDraft & {
  endLine: number;
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

export type SourceNoteDocument = {
  content: string;
  fileName: string;
  modifiedAtMs: number | null;
  path: string;
  sourceRoot: string;
  title: string;
};

export type SourceNoteIndexEntry = {
  fileName: string;
  modifiedAtMs: number | null;
  path: string;
  sizeBytes: number;
  sourceRoot: string;
};

export type SourceNoteIndexSnapshot = {
  defaultSourceRoot: string | null;
  notes: SourceNoteIndexEntry[];
  sourceRoots: string[];
};

export type SourceNoteSnapshot = {
  defaultSourceRoot: string | null;
  notes: SourceNoteDocument[];
  sourceRoots: string[];
};
