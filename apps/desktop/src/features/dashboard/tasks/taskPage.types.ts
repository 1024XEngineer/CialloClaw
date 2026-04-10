import type {
  AgentTaskControlResult,
  AgentTaskDetailGetResult,
  Task,
  TaskControlAction,
} from "@cialloclaw/protocol";

export type TaskPriority = "critical" | "high" | "steady";
export type TaskDataSource = "rpc" | "mock";
export type TaskTabsValue = "details" | "subtasks" | "outputs" | "notes";
export type AssistantCardKey = "agent" | "files" | "context";

export type TaskRelatedFile = {
  id: string;
  kind: string;
  note: string;
  path: string;
  title: string;
};

export type TaskContextSnippet = {
  id: string;
  label: string;
  content: string;
};

export type TaskOutputBlock = {
  id: string;
  label: string;
  content: string;
  tone: "draft" | "result" | "editable";
};

export type TaskExperience = {
  priority: TaskPriority;
  dueAt: string | null;
  goal: string;
  phase: string;
  nextAction: string;
  progressHint: string;
  background: string;
  constraints: string[];
  acceptance: string[];
  noteDraft: string;
  noteEntries: string[];
  relatedFiles: TaskRelatedFile[];
  quickContext: TaskContextSnippet[];
  recentConversation: string[];
  suggestedNext: string;
  assistantState: {
    label: string;
    hint: string;
  };
  outputs: TaskOutputBlock[];
  stepTargets: Record<string, AssistantCardKey>;
  endedSummary?: string;
  waitingReason?: string;
  blockedReason?: string;
};

export type TaskListItem = {
  task: Task;
  experience: TaskExperience;
};

export type TaskBucketsData = {
  unfinished: TaskListItem[];
  finished: TaskListItem[];
  source: TaskDataSource;
};

export type TaskDetailData = {
  task: Task;
  detail: AgentTaskDetailGetResult;
  experience: TaskExperience;
  source: TaskDataSource;
};

export type TaskControlOutcome = {
  result: AgentTaskControlResult;
  source: TaskDataSource;
};

export type TaskActionShortcut = {
  id: string;
  label: string;
  tooltip: string;
};

export type TaskProgressState = {
  completedCount: number;
  currentLabel: string;
  percent: number;
  remainingCount: number;
  total: number;
};

export type TaskStateVoice = {
  title: string;
  body: string;
};

export type TaskPrimaryAction = {
  action: TaskControlAction | "edit" | "open-safety";
  label: string;
  tooltip: string;
};

export type FinishedTaskGroup = {
  key: "recent" | "weekly" | "older";
  title: string;
  description: string;
  items: TaskListItem[];
};
