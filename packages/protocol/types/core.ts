export const TASK_STATUSES = [
  "confirming_intent",
  "processing",
  "waiting_auth",
  "waiting_input",
  "paused",
  "blocked",
  "failed",
  "completed",
  "cancelled",
  "ended_unfinished",
] as const;

export const TASK_LIST_GROUPS = ["unfinished", "finished"] as const;

export const TODO_BUCKETS = ["upcoming", "later", "recurring_rule", "closed"] as const;

export const TASK_STEP_STATUSES = ["pending", "running", "completed", "failed", "skipped", "cancelled"] as const;

export const STEP_STATUSES = ["pending", "running", "completed", "failed", "skipped", "cancelled"] as const;

export const TOOL_CALL_STATUSES = ["pending", "running", "succeeded", "failed"] as const;

export const RISK_LEVELS = ["green", "yellow", "red"] as const;

export const SECURITY_STATUSES = [
  "normal",
  "pending_confirmation",
  "intercepted",
  "execution_error",
  "recoverable",
  "recovered",
] as const;

export const DELIVERY_TYPES = [
  "bubble",
  "workspace_document",
  "result_page",
  "open_file",
  "reveal_in_folder",
  "task_detail",
] as const;

export const VOICE_SESSION_STATES = ["listening", "locked", "processing", "cancelled", "finished"] as const;

export const REQUEST_SOURCES = ["floating_ball", "dashboard", "tray_panel"] as const;

export const REQUEST_TRIGGERS = [
  "voice_commit",
  "hover_text_input",
  "text_selected_click",
  "file_drop",
  "error_detected",
  "recommendation_click",
] as const;

export const INPUT_TYPES = ["text", "text_selection", "file", "error"] as const;

export const INPUT_MODES = ["voice", "text"] as const;

export const TASK_SOURCE_TYPES = ["voice", "hover_input", "selected_text", "dragged_file", "todo", "error_signal"] as const;

export const BUBBLE_MESSAGE_TYPES = ["status", "intent_confirm", "result"] as const;

export const APPROVAL_DECISIONS = ["allow_once", "deny_once"] as const;

export const APPROVAL_STATUSES = ["pending", "approved", "denied"] as const;

export const SETTINGS_SCOPES = ["all", "general", "floating_ball", "memory", "task_automation", "data_log"] as const;

export const APPLY_MODES = ["immediate", "restart_required", "next_task_effective"] as const;

export const THEME_MODES = ["follow_system", "light", "dark"] as const;

export const POSITION_MODES = ["fixed", "draggable"] as const;

export const TODO_STATUSES = ["normal", "due_today", "overdue", "completed", "cancelled"] as const;

export const RECOMMENDATION_SCENES = ["hover", "selected_text", "idle", "error"] as const;

export const RECOMMENDATION_FEEDBACKS = ["positive", "negative", "ignore"] as const;

export const TASK_CONTROL_ACTIONS = ["pause", "resume", "cancel", "restart"] as const;

export const TIME_UNITS = ["minute", "hour", "day", "week"] as const;

export const RUN_STATUSES = ["processing", "completed"] as const;

export type TaskStatus = (typeof TASK_STATUSES)[number];
export type TaskListGroup = (typeof TASK_LIST_GROUPS)[number];
export type TodoBucket = (typeof TODO_BUCKETS)[number];
export type TaskStepStatus = (typeof TASK_STEP_STATUSES)[number];
export type StepStatus = (typeof STEP_STATUSES)[number];
export type ToolCallStatus = (typeof TOOL_CALL_STATUSES)[number];
export type RiskLevel = (typeof RISK_LEVELS)[number];
export type SecurityStatus = (typeof SECURITY_STATUSES)[number];
export type DeliveryType = (typeof DELIVERY_TYPES)[number];
export type VoiceSessionState = (typeof VOICE_SESSION_STATES)[number];
export type RequestSource = (typeof REQUEST_SOURCES)[number];
export type RequestTrigger = (typeof REQUEST_TRIGGERS)[number];
export type InputType = (typeof INPUT_TYPES)[number];
export type InputMode = (typeof INPUT_MODES)[number];
export type TaskSourceType = (typeof TASK_SOURCE_TYPES)[number];
export type BubbleMessageType = (typeof BUBBLE_MESSAGE_TYPES)[number];
export type ApprovalDecision = (typeof APPROVAL_DECISIONS)[number];
export type ApprovalStatus = (typeof APPROVAL_STATUSES)[number];
export type SettingsScope = (typeof SETTINGS_SCOPES)[number];
export type ApplyMode = (typeof APPLY_MODES)[number];
export type ThemeMode = (typeof THEME_MODES)[number];
export type PositionMode = (typeof POSITION_MODES)[number];
export type TodoStatus = (typeof TODO_STATUSES)[number];
export type RecommendationScene = (typeof RECOMMENDATION_SCENES)[number];
export type RecommendationFeedback = (typeof RECOMMENDATION_FEEDBACKS)[number];
export type TaskControlAction = (typeof TASK_CONTROL_ACTIONS)[number];
export type TimeUnit = (typeof TIME_UNITS)[number];
export type RunStatus = (typeof RUN_STATUSES)[number];

export interface IntentPayload {
  name: string;
  arguments: Record<string, unknown>;
}

export interface TimeInterval {
  unit: TimeUnit;
  value: number;
}

export interface Task {
  task_id: string;
  title: string;
  source_type: TaskSourceType;
  status: TaskStatus;
  intent: IntentPayload | null;
  current_step: string;
  risk_level: RiskLevel;
  started_at: string | null;
  updated_at: string;
  finished_at: string | null;
}

export interface TaskStep {
  step_id: string;
  task_id: string;
  name: string;
  status: TaskStepStatus;
  order_index: number;
  input_summary: string;
  output_summary: string;
}

export interface BubbleMessage {
  bubble_id: string;
  task_id: string;
  type: BubbleMessageType;
  text: string;
  pinned: boolean;
  hidden: boolean;
  created_at: string;
}

export interface DeliveryPayload {
  path: string | null;
  url: string | null;
  task_id: string | null;
}

export interface DeliveryResult {
  type: DeliveryType;
  title: string;
  payload: DeliveryPayload;
  preview_text: string;
}

export interface Artifact {
  artifact_id: string;
  task_id: string;
  artifact_type: string;
  title: string;
  path: string;
  mime_type: string;
}

export interface TodoItem {
  item_id: string;
  title: string;
  bucket: TodoBucket;
  status: TodoStatus;
  type: string;
  due_at: string | null;
  agent_suggestion: string | null;
}

export interface RecurringRule {
  rule_id: string;
  title: string;
  bucket: Extract<TodoBucket, "recurring_rule">;
  repeat_rule: string;
  next_occurrence_at: string;
  enabled: boolean;
}

export interface ApprovalRequest {
  approval_id: string;
  task_id: string;
  operation_name: string;
  risk_level: RiskLevel;
  target_object: string;
  reason: string;
  status: ApprovalStatus;
  created_at: string;
}

export interface AuthorizationRecord {
  authorization_record_id: string;
  task_id: string;
  approval_id: string;
  decision: ApprovalDecision;
  remember_rule: boolean;
  operator: string;
  created_at: string;
}

export interface AuditRecord {
  audit_id: string;
  task_id: string;
  type: string;
  action: string;
  summary: string;
  target: string;
  result: string;
  created_at: string;
}

export interface ImpactScope {
  files: string[];
  webpages: string[];
  apps: string[];
  out_of_workspace: boolean;
  overwrite_or_delete_risk: boolean;
}

export interface RecoveryPoint {
  recovery_point_id: string;
  task_id: string;
  summary: string;
  created_at: string;
  objects: string[];
}

export interface TokenCostSummary {
  current_task_tokens: number;
  current_task_cost: number;
  today_tokens: number;
  today_cost: number;
  single_task_limit: number;
  daily_limit: number;
  budget_auto_downgrade: boolean;
}

export interface MirrorReference {
  memory_id: string;
  reason: string;
  summary: string;
}

export interface SettingsSnapshot {
  settings: {
    general: {
      language: string;
      auto_launch: boolean;
      theme_mode: ThemeMode;
      voice_notification_enabled: boolean;
      voice_type: string;
      download: {
        workspace_path: string;
        ask_before_save_each_file: boolean;
      };
    };
    floating_ball: {
      auto_snap: boolean;
      idle_translucent: boolean;
      position_mode: PositionMode;
      size: string;
    };
    memory: {
      enabled: boolean;
      lifecycle: string;
      work_summary_interval: TimeInterval;
      profile_refresh_interval: TimeInterval;
    };
    task_automation: {
      inspect_on_startup: boolean;
      inspect_on_file_change: boolean;
      inspection_interval: TimeInterval;
      task_sources: string[];
      remind_before_deadline: boolean;
      remind_when_stale: boolean;
    };
    data_log: {
      provider: string;
      budget_auto_downgrade: boolean;
    };
  };
}

export interface SettingItem {
  key: string;
  label: string;
  value: string | boolean | number | null;
  apply_mode: ApplyMode;
  dangerous: boolean;
  need_second_confirm: boolean;
}

export interface AsyncJob {
  job_id: string;
  task_id: string;
  state: string;
  progress: number;
  created_at: string;
}

export interface Session {
  session_id: string;
  title: string;
  status: "active" | "archived";
  created_at: string;
  updated_at: string;
}

export interface Run {
  run_id: string;
  task_id: string;
  session_id: string;
  source_type: Extract<TaskSourceType, "selected_text" | "dragged_file" | "voice" | "hover_input" | "todo" | "error_signal">;
  status: RunStatus;
  started_at: string | null;
  finished_at: string | null;
}

export interface Step {
  step_id: string;
  run_id: string;
  task_id: string;
  name: string;
  status: StepStatus;
  order_index: number;
  input_summary: string;
  output_summary: string;
}

export interface Event {
  event_id: string;
  run_id: string;
  task_id: string;
  step_id: string | null;
  type: string;
  level: "info" | "warn" | "error";
  payload: Record<string, unknown>;
  created_at: string;
}

export interface ToolCall {
  tool_call_id: string;
  run_id: string;
  task_id: string;
  step_id: string | null;
  tool_name: string;
  status: ToolCallStatus;
  input: Record<string, unknown>;
  output: Record<string, unknown>;
  error_code: number | null;
  duration_ms: number;
}

export interface Citation {
  citation_id: string;
  task_id: string;
  run_id: string;
  source_type: "file" | "web" | "context";
  source_ref: string;
  label: string;
}

export interface MemorySummary {
  memory_summary_id: string;
  task_id: string;
  run_id: string;
  summary: string;
  created_at: string;
}

export interface MemoryCandidate {
  memory_candidate_id: string;
  task_id: string;
  run_id: string;
  summary: string;
  source: string;
}

export interface RetrievalHit {
  retrieval_hit_id: string;
  task_id: string;
  run_id: string;
  memory_id: string;
  score: number;
  source: string;
  summary: string;
}

export interface PluginManifest {
  plugin_id: string;
  name: string;
  version: string;
  entry: string;
  capabilities: string[];
  permissions: string[];
}

export interface PluginRuntimeState {
  plugin_id: string;
  healthy: boolean;
  last_heartbeat_at: string | null;
  current_task_id: string | null;
  last_error: string | null;
}

export interface PluginMetricSnapshot {
  plugin_id: string;
  call_count: number;
  error_count: number;
  average_duration_ms: number;
  artifact_count: number;
}

export interface RpcResponseMeta {
  server_time: string;
}

export interface RpcResult<T> {
  data: T;
  meta: RpcResponseMeta;
  warnings: string[];
}

export interface RpcSuccessResponse<T> {
  jsonrpc: "2.0";
  id: string;
  result: RpcResult<T>;
}

export interface RpcErrorResponse {
  jsonrpc: "2.0";
  id: string;
  error: {
    code: number;
    message: string;
    data?: Record<string, unknown>;
  };
}
