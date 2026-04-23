// 该文件定义 task-centric 主模型、枚举和协议基础结构。
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

// TASK_LIST_GROUPS 定义共享常量。
export const TASK_LIST_GROUPS = ["unfinished", "finished"] as const;

// TODO_BUCKETS 定义共享常量。
export const TODO_BUCKETS = ["upcoming", "later", "recurring_rule", "closed"] as const;

// TASK_STEP_STATUSES 定义共享常量。
export const TASK_STEP_STATUSES = ["pending", "running", "completed", "failed", "skipped", "cancelled"] as const;

// STEP_STATUSES 定义共享常量。
export const STEP_STATUSES = ["pending", "running", "completed", "failed", "skipped", "cancelled"] as const;

// TOOL_CALL_STATUSES 定义共享常量。
export const TOOL_CALL_STATUSES = ["pending", "running", "succeeded", "failed"] as const;

// RISK_LEVELS 定义共享常量。
export const RISK_LEVELS = ["green", "yellow", "red"] as const;

// SECURITY_STATUSES 定义共享常量。
export const SECURITY_STATUSES = [
  "normal",
  "pending_confirmation",
  "intercepted",
  "execution_error",
  "recoverable",
  "recovered",
] as const;

// DELIVERY_TYPES 定义共享常量。
export const DELIVERY_TYPES = [
  "bubble",
  "workspace_document",
  "result_page",
  "open_file",
  "reveal_in_folder",
  "task_detail",
] as const;

// VOICE_SESSION_STATES 定义共享常量。
export const VOICE_SESSION_STATES = ["listening", "locked", "processing", "cancelled", "finished"] as const;

// REQUEST_SOURCES 定义共享常量。
export const REQUEST_SOURCES = ["floating_ball", "dashboard", "tray_panel"] as const;

// REQUEST_TRIGGERS 定义共享常量。
export const REQUEST_TRIGGERS = [
  "voice_commit",
  "hover_text_input",
  "text_selected_click",
  "file_drop",
  "error_detected",
  "recommendation_click",
] as const;

// INPUT_TYPES 定义共享常量。
export const INPUT_TYPES = ["text", "text_selection", "file", "error"] as const;

// INPUT_MODES 定义共享常量。
export const INPUT_MODES = ["voice", "text"] as const;

// TASK_SOURCE_TYPES 定义共享常量。
export const TASK_SOURCE_TYPES = ["voice", "hover_input", "selected_text", "dragged_file", "todo", "error_signal", "screen_capture"] as const;

// BUBBLE_MESSAGE_TYPES 定义共享常量。
export const BUBBLE_MESSAGE_TYPES = ["status", "intent_confirm", "result"] as const;

// APPROVAL_DECISIONS 定义共享常量。
export const APPROVAL_DECISIONS = ["allow_once", "deny_once"] as const;

// APPROVAL_STATUSES 定义共享常量。
export const APPROVAL_STATUSES = ["pending", "approved", "denied"] as const;

// SETTINGS_SCOPES 定义共享常量。
export const SETTINGS_SCOPES = ["all", "general", "floating_ball", "memory", "task_automation", "models"] as const;

// APPLY_MODES 定义共享常量。
export const APPLY_MODES = ["immediate", "restart_required", "next_task_effective"] as const;

// THEME_MODES 定义共享常量。
export const THEME_MODES = ["follow_system", "light", "dark"] as const;

// POSITION_MODES 定义共享常量。
export const POSITION_MODES = ["fixed", "draggable"] as const;

// TODO_STATUSES 定义共享常量。
export const TODO_STATUSES = ["normal", "due_today", "overdue", "completed", "cancelled"] as const;

// NOTEPAD_ACTIONS defines the stable note-state mutation actions supported by
// the RPC boundary.
export const NOTEPAD_ACTIONS = ["complete", "cancel", "move_upcoming", "toggle_recurring", "cancel_recurring", "restore", "delete"] as const;

// RECOMMENDATION_SCENES 定义共享常量。
export const RECOMMENDATION_SCENES = ["hover", "selected_text", "idle", "error"] as const;

// RECOMMENDATION_FEEDBACKS 定义共享常量。
export const RECOMMENDATION_FEEDBACKS = ["positive", "negative", "ignore"] as const;

// TASK_CONTROL_ACTIONS 定义共享常量。
export const TASK_CONTROL_ACTIONS = ["pause", "resume", "cancel", "restart"] as const;

// TIME_UNITS 定义共享常量。
export const TIME_UNITS = ["minute", "hour", "day", "week"] as const;

// RUN_STATUSES 定义共享常量。
export const RUN_STATUSES = ["processing", "completed"] as const;

// TaskStatus 定义当前模块的数据结构。
export type TaskStatus = (typeof TASK_STATUSES)[number];
// TaskListGroup 定义当前模块的数据结构。
export type TaskListGroup = (typeof TASK_LIST_GROUPS)[number];
// TodoBucket 定义当前模块的数据结构。
export type TodoBucket = (typeof TODO_BUCKETS)[number];
// TaskStepStatus 定义当前模块的数据结构。
export type TaskStepStatus = (typeof TASK_STEP_STATUSES)[number];
// StepStatus 定义当前模块的数据结构。
export type StepStatus = (typeof STEP_STATUSES)[number];
// ToolCallStatus 定义当前模块的数据结构。
export type ToolCallStatus = (typeof TOOL_CALL_STATUSES)[number];
// RiskLevel 定义当前模块的数据结构。
export type RiskLevel = (typeof RISK_LEVELS)[number];
// SecurityStatus 定义当前模块的数据结构。
export type SecurityStatus = (typeof SECURITY_STATUSES)[number];
// DeliveryType 定义当前模块的数据结构。
export type DeliveryType = (typeof DELIVERY_TYPES)[number];
// VoiceSessionState 描述当前模块状态。
export type VoiceSessionState = (typeof VOICE_SESSION_STATES)[number];
// RequestSource 定义当前模块的数据结构。
export type RequestSource = (typeof REQUEST_SOURCES)[number];
// RequestTrigger 定义当前模块的数据结构。
export type RequestTrigger = (typeof REQUEST_TRIGGERS)[number];
// InputType 定义当前模块的数据结构。
export type InputType = (typeof INPUT_TYPES)[number];
// InputMode 定义当前模块的数据结构。
export type InputMode = (typeof INPUT_MODES)[number];
// TaskSourceType 定义当前模块的数据结构。
export type TaskSourceType = (typeof TASK_SOURCE_TYPES)[number];
// BubbleMessageType 定义当前模块的数据结构。
export type BubbleMessageType = (typeof BUBBLE_MESSAGE_TYPES)[number];
// ApprovalDecision 定义当前模块的数据结构。
export type ApprovalDecision = (typeof APPROVAL_DECISIONS)[number];
// ApprovalStatus 定义当前模块的数据结构。
export type ApprovalStatus = (typeof APPROVAL_STATUSES)[number];
// SettingsScope 定义当前模块的数据结构。
export type SettingsScope = (typeof SETTINGS_SCOPES)[number];
// ApplyMode 定义当前模块的数据结构。
export type ApplyMode = (typeof APPLY_MODES)[number];
// ThemeMode 定义当前模块的数据结构。
export type ThemeMode = (typeof THEME_MODES)[number];
// PositionMode 定义当前模块的数据结构。
export type PositionMode = (typeof POSITION_MODES)[number];
// TodoStatus 定义当前模块的数据结构。
export type TodoStatus = (typeof TODO_STATUSES)[number];
// NotepadAction 定义当前模块的数据结构。
export type NotepadAction = (typeof NOTEPAD_ACTIONS)[number];
// RecommendationScene 定义当前模块的数据结构。
export type RecommendationScene = (typeof RECOMMENDATION_SCENES)[number];
// RecommendationFeedback 定义当前模块的数据结构。
export type RecommendationFeedback = (typeof RECOMMENDATION_FEEDBACKS)[number];
// TaskControlAction 定义当前模块的数据结构。
export type TaskControlAction = (typeof TASK_CONTROL_ACTIONS)[number];
// TimeUnit 定义当前模块的数据结构。
export type TimeUnit = (typeof TIME_UNITS)[number];
// RunStatus 定义当前模块的数据结构。
export type RunStatus = (typeof RUN_STATUSES)[number];

// IntentPayload 定义当前模块的接口约束。
export interface IntentPayload {
  name: string;
  arguments: Record<string, unknown>;
}

// TimeInterval 定义当前模块的接口约束。
export interface TimeInterval {
  unit: TimeUnit;
  value: number;
}

// StrongholdStatus captures the formal secret-backend lifecycle metadata that
// can be surfaced in settings responses without exposing secret values.
export interface StrongholdStatus {
  backend: string;
  available: boolean;
  fallback: boolean;
  initialized: boolean;
  formal_store: boolean;
}

// Task 定义当前模块的接口约束。
export interface Task {
  task_id: string;
  session_id: string | null;
  title: string;
  source_type: TaskSourceType;
  status: TaskStatus;
  intent: IntentPayload | null;
  current_step: string;
  risk_level: RiskLevel;
  loop_stop_reason?: string | null;
  started_at: string | null;
  updated_at: string;
  finished_at: string | null;
}

// TaskStep 定义当前模块的接口约束。
export interface TaskStep {
  step_id: string;
  task_id: string;
  name: string;
  status: TaskStepStatus;
  order_index: number;
  input_summary: string;
  output_summary: string;
}

// BubbleMessage 定义当前模块的接口约束。
export interface BubbleMessage {
  bubble_id: string;
  task_id: string;
  type: BubbleMessageType;
  text: string;
  pinned: boolean;
  hidden: boolean;
  created_at: string;
}

// DeliveryPayload 定义当前模块的接口约束。
export interface DeliveryPayload {
  path: string | null;
  url: string | null;
  task_id: string | null;
}

// DeliveryResult 定义当前模块的接口约束。
export interface DeliveryResult {
  type: DeliveryType;
  title: string;
  payload: DeliveryPayload;
  preview_text: string;
}

// Artifact 定义当前模块的接口约束。
export interface Artifact {
  artifact_id: string;
  task_id: string;
  artifact_type: string;
  title: string;
  path: string;
  mime_type: string;
}

// TodoResource 定义 notes 详情里相关资料的最小稳定结构。
export interface TodoResource {
  resource_id: string;
  label: string;
  path: string;
  resource_type: string;
  open_action?: DeliveryType | null;
  open_payload?: DeliveryPayload | null;
}

// TodoItem 定义当前模块的接口约束。
export interface TodoItem {
  item_id: string;
  title: string;
  bucket: TodoBucket;
  status: TodoStatus;
  type: string;
  due_at: string | null;
  agent_suggestion: string | null;
  recurring_enabled?: boolean;
  note_text?: string | null;
  prerequisite?: string | null;
  repeat_rule?: string | null;
  next_occurrence_at?: string | null;
  recent_instance_status?: string | null;
  effective_scope?: string | null;
  ended_at?: string | null;
  related_resources?: TodoResource[];
  linked_task_id?: string | null;
}

// RecurringRule 定义当前模块的接口约束。
export interface RecurringRule {
  rule_id: string;
  title: string;
  bucket: Extract<TodoBucket, "recurring_rule">;
  repeat_rule: string;
  next_occurrence_at: string;
  enabled: boolean;
}

// ApprovalRequest 定义当前模块的接口约束。
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

// AuthorizationRecord 定义当前模块的接口约束。
export interface AuthorizationRecord {
  authorization_record_id: string;
  task_id: string;
  approval_id: string;
  decision: ApprovalDecision;
  remember_rule: boolean;
  operator: string;
  created_at: string;
}

// AuditRecord 定义当前模块的接口约束。
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

// ImpactScope 定义当前模块的接口约束。
export interface ImpactScope {
  files: string[];
  webpages: string[];
  apps: string[];
  out_of_workspace: boolean;
  overwrite_or_delete_risk: boolean;
}

// RecoveryPoint 定义当前模块的接口约束。
export interface RecoveryPoint {
  recovery_point_id: string;
  task_id: string;
  summary: string;
  created_at: string;
  objects: string[];
}

// TokenCostSummary 定义当前模块的接口约束。
export interface TokenCostSummary {
  current_task_tokens: number;
  current_task_cost: number;
  today_tokens: number;
  today_cost: number;
  single_task_limit: number;
  daily_limit: number;
  budget_auto_downgrade: boolean;
}

// ModelTokenUsage 定义模型调用的最小 token 计量结构。
export interface ModelTokenUsage {
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
}

// ModelGenerateTextRequest 定义模型最小文本生成请求结构。
export interface ModelGenerateTextRequest {
  task_id: string;
  run_id: string;
  input: string;
}

// ModelGenerateTextResponse 定义模型最小文本生成返回结构。
export interface ModelGenerateTextResponse {
  task_id: string;
  run_id: string;
  request_id: string;
  provider: string;
  model_id: string;
  output_text: string;
  usage: ModelTokenUsage;
  latency_ms: number;
}

// ModelInvocationRecord 定义模型调用记录的最小结构。
export interface ModelInvocationRecord {
  task_id: string;
  run_id: string;
  request_id: string;
  provider: string;
  model_id: string;
  usage: ModelTokenUsage;
  latency_ms: number;
}

// MirrorReference 定义当前模块的接口约束。
export interface MirrorReference {
  memory_id: string;
  reason: string;
  summary: string;
}

// SettingsSnapshot 定义当前模块的接口约束。
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
    models: {
      provider: string;
      credentials: {
        budget_auto_downgrade: boolean;
        provider_api_key_configured: boolean;
        base_url: string;
        model: string;
        stronghold: StrongholdStatus;
      };
    };
    data_log?: {
      provider: string;
      budget_auto_downgrade: boolean;
      provider_api_key_configured: boolean;
      stronghold: StrongholdStatus;
      base_url?: string;
      model?: string;
    };
  };
}

// SettingItem 定义当前模块的接口约束。
export interface SettingItem {
  key: string;
  label: string;
  value: string | boolean | number | null;
  apply_mode: ApplyMode;
  dangerous: boolean;
  need_second_confirm: boolean;
}

// AsyncJob 定义当前模块的接口约束。
export interface AsyncJob {
  job_id: string;
  task_id: string;
  state: string;
  progress: number;
  created_at: string;
}

// Session 定义当前模块的接口约束。
export interface Session {
  session_id: string;
  title: string;
  status: "active" | "idle" | "archived";
  created_at: string;
  updated_at: string;
}

// Run 定义当前模块的接口约束。
export interface Run {
  run_id: string;
  task_id: string;
  session_id: string;
  source_type: Extract<TaskSourceType, "selected_text" | "dragged_file" | "voice" | "hover_input" | "todo" | "error_signal" | "screen_capture">;
  status: RunStatus;
  started_at: string | null;
  finished_at: string | null;
}

// Step 定义当前模块的接口约束。
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

// Event 定义当前模块的接口约束。
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

// ToolCall 定义当前模块的接口约束。
export interface ToolCall {
  tool_call_id: string;
  run_id: string;
  task_id: string;
  step_id: string | null;
  created_at: string | null;
  tool_name: string;
  status: ToolCallStatus;
  input: Record<string, unknown>;
  output: Record<string, unknown>;
  error_code: number | null;
  duration_ms: number;
}

// Citation 定义当前模块的接口约束。
export interface Citation {
  citation_id: string;
  task_id: string;
  run_id: string;
  source_type: "file" | "web" | "context";
  source_ref: string;
  label: string;
  // Optional structured evidence metadata helps task detail distinguish
  // screenshots, OCR excerpts, and other formal references.
  artifact_id?: string | null;
  artifact_type?: string | null;
  evidence_role?: string | null;
  excerpt_text?: string | null;
  screen_session_id?: string | null;
}

// MemorySummary 定义当前模块的接口约束。
export interface MemorySummary {
  memory_summary_id: string;
  task_id: string;
  run_id: string;
  summary: string;
  created_at: string;
}

// MemoryCandidate 定义当前模块的接口约束。
export interface MemoryCandidate {
  memory_candidate_id: string;
  task_id: string;
  run_id: string;
  summary: string;
  source: string;
}

// RetrievalHit 定义当前模块的接口约束。
export interface RetrievalHit {
  retrieval_hit_id: string;
  task_id: string;
  run_id: string;
  memory_id: string;
  score: number;
  source: string;
  summary: string;
}

// PluginManifest 定义当前模块的接口约束。
export interface PluginManifest {
  plugin_id: string;
  name: string;
  display_name: string;
  summary: string;
  version: string;
  source: "builtin" | "local_dir" | "github" | "marketplace";
  entry: string;
  enabled: boolean;
  permissions: string[];
  capabilities: PluginCapabilitySummary[];
}

export interface PluginCapabilitySummary {
  tool_name: string;
  display_name: string;
  description: string;
  source: "builtin" | "worker" | "sidecar";
  risk_hint: RiskLevel;
}

// PluginRuntimeState 定义当前模块的接口约束。
export interface PluginRuntimeState {
  name: string;
  kind: "worker" | "sidecar";
  status: "declared" | "starting" | "running" | "stopped" | "unavailable" | "failed";
  transport: string;
  health: "unknown" | "healthy" | "degraded" | "failed" | "stopped" | "unavailable";
  last_seen_at: string;
  last_error: string | null;
  capabilities: string[];
  manifest?: PluginManifest;
}

// PluginMetricSnapshot 定义当前模块的接口约束。
export interface PluginMetricSnapshot {
  name: string;
  kind: "worker" | "sidecar";
  start_count: number;
  success_count: number;
  failure_count: number;
  last_started_at: string;
  last_failed_at: string;
  last_seen_at: string;
}

export interface PluginRuntimeEvent {
  name: string;
  kind: PluginRuntimeState["kind"];
  event_type: string;
  payload: Record<string, unknown>;
  created_at: string;
}

export interface PluginContractField {
  name: string;
  type: string;
  required: boolean;
  description: string;
  example?: string | null;
}

export interface PluginDataContract {
  schema_ref: string;
  schema_json: Record<string, unknown> | null;
  fields: PluginContractField[];
}

export interface PluginDeliveryMapping {
  emits_tool_call: boolean;
  artifact_types: string[];
  delivery_types: DeliveryType[];
  citation_source_types: Citation["source_type"][];
}

export interface PluginToolContract {
  tool_name: string;
  display_name: string;
  description: string;
  source: "builtin" | "worker" | "sidecar";
  risk_hint: RiskLevel;
  timeout_sec: number;
  supports_dry_run: boolean;
  input_contract: PluginDataContract;
  output_contract: PluginDataContract;
  delivery_mapping: PluginDeliveryMapping;
}

export interface PluginListItem extends PluginManifest {
  runtimes: PluginRuntimeState[];
}

// RpcResponseMeta 定义当前模块的接口约束。
export interface RpcResponseMeta {
  server_time: string;
}

// RpcResult 定义当前模块的接口约束。
export interface RpcResult<T> {
  data: T;
  meta: RpcResponseMeta;
  warnings: string[];
}

// RpcSuccessResponse 定义当前模块的接口约束。
export interface RpcSuccessResponse<T> {
  jsonrpc: "2.0";
  id: string;
  result: RpcResult<T>;
}

// RpcErrorResponse 定义当前模块的接口约束。
export interface RpcErrorResponse {
  jsonrpc: "2.0";
  id: string;
  error: {
    code: number;
    message: string;
    data?: Record<string, unknown>;
  };
}
