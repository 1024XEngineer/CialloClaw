export type DashboardHomeModuleKey = "tasks" | "notes" | "memory" | "safety";

export type DashboardHomeEventStateKey =
  | "task_working"
  | "task_highlight"
  | "task_completing"
  | "task_done"
  | "task_error_permission"
  | "task_error_blocked"
  | "task_error_missing_info"
  | "notes_processing"
  | "notes_reminder"
  | "notes_scheduled"
  | "memory_summary"
  | "memory_habit"
  | "safety_alert"
  | "safety_guard";

export type DashboardHomeTagTone = "normal" | "active" | "highlight" | "warn" | "error" | "done" | "mirror" | "safety";

export type DashboardHomeContextItem = {
  iconKey: string;
  text: string;
  time?: string;
  type?: "normal" | "warn" | "error" | "hint" | "active";
};

export type DashboardHomeProgressStep = {
  label: string;
  status: "done" | "active" | "pending" | "error";
};

export type DashboardHomeNoteItem = {
  id: string;
  text: string;
  status: "pending" | "processing" | "done" | "recurring";
  time?: string;
  tag?: string;
};

export type DashboardHomeInsightItem = {
  iconKey: string;
  text: string;
  emphasis?: boolean;
};

export type DashboardHomeSignalItem = {
  iconKey: string;
  label: string;
  value: string;
  level: "normal" | "warn" | "critical";
  translation?: string;
};

export type DashboardHomeAnomaly = {
  title: string;
  desc: string;
  actionLabel: string;
  dismissLabel: string;
  severity: "warn" | "error" | "info";
};

export type DashboardHomeStateData = {
  key: DashboardHomeEventStateKey;
  module: DashboardHomeModuleKey;
  label: string;
  orbColor: string;
  orbGlow: string;
  accentColor: string;
  tag: string;
  tagTone: DashboardHomeTagTone;
  headline: string;
  subline: string;
  progress?: number;
  progressLabel?: string;
  progressSteps?: DashboardHomeProgressStep[];
  context: DashboardHomeContextItem[];
  notes?: DashboardHomeNoteItem[];
  insights?: DashboardHomeInsightItem[];
  signals?: DashboardHomeSignalItem[];
  anomaly?: DashboardHomeAnomaly;
  breathSpeed: number;
};

export type DashboardOrbitNodeConfig = {
  key: string;
  module: DashboardHomeModuleKey;
  label: string;
  color: string;
  glow: string;
  orbitRadius: number;
  orbitSpeed: number;
  orbitOffset: number;
  size: number;
};

export type DashboardEntranceOrbConfig = DashboardOrbitNodeConfig & {
  route: `/${DashboardHomeModuleKey}`;
};

export type DashboardDecorOrbConfig = DashboardOrbitNodeConfig;

export type DashboardHomeSummonEvent = {
  copyDuration?: number;
  id: string;
  hideCopy?: boolean;
  stateKey: DashboardHomeEventStateKey;
  message: string;
  reason: string;
  nextStep?: string;
  priority: "urgent" | "normal" | "low";
  duration?: number;
};

export type DashboardVoiceStage = "ready" | "listening" | "understanding" | "confirming" | "executing";

export type DashboardVoiceSequence = {
  suggestion: string;
  echoPool: string[];
  fragments: string[];
  summary: string;
  executingSteps: string[];
  module: DashboardHomeModuleKey;
};
