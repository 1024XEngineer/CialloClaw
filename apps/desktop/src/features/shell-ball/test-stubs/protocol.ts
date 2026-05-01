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
export const TASK_SOURCE_TYPES = ["voice", "hover_input", "selected_text", "dragged_file", "todo", "error_signal", "screen_capture"] as const;
export const BUBBLE_MESSAGE_TYPES = ["status", "intent_confirm", "result"] as const;
export const APPROVAL_DECISIONS = ["allow_once", "deny_once"] as const;
export const APPROVAL_STATUSES = ["pending", "approved", "denied"] as const;
export const APPLY_MODES = ["immediate", "restart_required", "next_task_effective"] as const;
export const THEME_MODES = ["follow_system", "light", "dark"] as const;
export const POSITION_MODES = ["fixed", "draggable"] as const;
export const TODO_STATUSES = ["normal", "due_today", "overdue", "completed", "cancelled"] as const;
export const NOTEPAD_ACTIONS = ["complete", "cancel", "move_upcoming", "toggle_recurring", "cancel_recurring", "restore", "delete"] as const;
export const RECOMMENDATION_SCENES = ["hover", "selected_text", "idle", "error"] as const;
export const RECOMMENDATION_FEEDBACKS = ["positive", "negative", "ignore"] as const;
export const TASK_CONTROL_ACTIONS = ["pause", "resume", "cancel", "restart"] as const;
export const TIME_UNITS = ["minute", "hour", "day", "week"] as const;

export type TaskStatus = (typeof TASK_STATUSES)[number];
export type TaskListGroup = (typeof TASK_LIST_GROUPS)[number];
export type TodoBucket = (typeof TODO_BUCKETS)[number];
export type TaskStepStatus = (typeof TASK_STEP_STATUSES)[number];
export type ToolCallStatus = (typeof TOOL_CALL_STATUSES)[number];
export type RiskLevel = (typeof RISK_LEVELS)[number];
export type SecurityStatus = (typeof SECURITY_STATUSES)[number];
export type DeliveryType = (typeof DELIVERY_TYPES)[number];
export type RequestSource = (typeof REQUEST_SOURCES)[number];
export type RequestTrigger = (typeof REQUEST_TRIGGERS)[number];
export type InputType = (typeof INPUT_TYPES)[number];
export type InputMode = (typeof INPUT_MODES)[number];
export type TaskSourceType = (typeof TASK_SOURCE_TYPES)[number];
export type BubbleMessageType = (typeof BUBBLE_MESSAGE_TYPES)[number];
export type ApprovalDecision = (typeof APPROVAL_DECISIONS)[number];
export type ApprovalStatus = (typeof APPROVAL_STATUSES)[number];
export type ApplyMode = (typeof APPLY_MODES)[number];
export type ThemeMode = (typeof THEME_MODES)[number];
export type PositionMode = (typeof POSITION_MODES)[number];
export type TodoStatus = (typeof TODO_STATUSES)[number];
export type NotepadAction = (typeof NOTEPAD_ACTIONS)[number];
export type RecommendationScene = (typeof RECOMMENDATION_SCENES)[number];
export type RecommendationFeedback = (typeof RECOMMENDATION_FEEDBACKS)[number];
export type TaskControlAction = (typeof TASK_CONTROL_ACTIONS)[number];
export type TimeUnit = (typeof TIME_UNITS)[number];

export interface IntentPayload {
  name: string;
  arguments: Record<string, unknown>;
}

export interface TimeInterval {
  unit: TimeUnit;
  value: number;
}

export interface StrongholdStatus {
  backend: string;
  available: boolean;
  fallback: boolean;
  initialized: boolean;
  formal_store: boolean;
}

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

export interface MirrorReference {
  memory_id: string;
  reason: string;
  summary: string;
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

export interface RecoveryPoint {
  recovery_point_id: string;
  task_id: string;
  summary: string;
  created_at: string;
  objects: string[];
}

export interface ImpactScope {
  files: string[];
  webpages: string[];
  apps: string[];
  out_of_workspace: boolean;
  overwrite_or_delete_risk: boolean;
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
	  };
}

export interface Citation {
  citation_id: string;
  task_id: string;
  run_id: string;
  source_type: "file" | "web" | "context";
  source_ref: string;
  label: string;
  artifact_id?: string | null;
  artifact_type?: string | null;
  evidence_role?: string | null;
  excerpt_text?: string | null;
  screen_session_id?: string | null;
}

export const RPC_METHODS_STABLE = {
  AGENT_INPUT_SUBMIT: "agent.input.submit",
  AGENT_TASK_START: "agent.task.start",
  AGENT_TASK_CONFIRM: "agent.task.confirm",
  AGENT_RECOMMENDATION_GET: "agent.recommendation.get",
  AGENT_RECOMMENDATION_FEEDBACK_SUBMIT: "agent.recommendation.feedback.submit",
  AGENT_TASK_LIST: "agent.task.list",
  AGENT_TASK_DETAIL_GET: "agent.task.detail.get",
  AGENT_TASK_EVENTS_LIST: "agent.task.events.list",
  AGENT_TASK_TOOL_CALLS_LIST: "agent.task.tool_calls.list",
  AGENT_TASK_STEER: "agent.task.steer",
  AGENT_TASK_ARTIFACT_LIST: "agent.task.artifact.list",
  AGENT_TASK_ARTIFACT_OPEN: "agent.task.artifact.open",
  AGENT_TASK_CONTROL: "agent.task.control",
  AGENT_TASK_INSPECTOR_CONFIG_GET: "agent.task_inspector.config.get",
  AGENT_TASK_INSPECTOR_CONFIG_UPDATE: "agent.task_inspector.config.update",
  AGENT_TASK_INSPECTOR_RUN: "agent.task_inspector.run",
  AGENT_NOTEPAD_LIST: "agent.notepad.list",
  AGENT_NOTEPAD_CONVERT_TO_TASK: "agent.notepad.convert_to_task",
  AGENT_NOTEPAD_UPDATE: "agent.notepad.update",
  AGENT_DASHBOARD_OVERVIEW_GET: "agent.dashboard.overview.get",
  AGENT_DASHBOARD_MODULE_GET: "agent.dashboard.module.get",
  AGENT_MIRROR_OVERVIEW_GET: "agent.mirror.overview.get",
  AGENT_SECURITY_SUMMARY_GET: "agent.security.summary.get",
  AGENT_SECURITY_AUDIT_LIST: "agent.security.audit.list",
  AGENT_SECURITY_RESTORE_POINTS_LIST: "agent.security.restore_points.list",
  AGENT_SECURITY_RESTORE_APPLY: "agent.security.restore.apply",
  AGENT_SECURITY_PENDING_LIST: "agent.security.pending.list",
  AGENT_SECURITY_RESPOND: "agent.security.respond",
  AGENT_DELIVERY_OPEN: "agent.delivery.open",
  AGENT_SETTINGS_GET: "agent.settings.get",
  AGENT_SETTINGS_UPDATE: "agent.settings.update",
} as const;

export const RPC_METHODS_PLANNED = {
  AGENT_MIRROR_MEMORY_MANAGE: "agent.mirror.memory.manage",
} as const;

export const RPC_METHODS = {
  ...RPC_METHODS_STABLE,
  ...RPC_METHODS_PLANNED,
} as const;

export const NOTIFICATION_METHODS = {
  TASK_UPDATED: "task.updated",
  DELIVERY_READY: "delivery.ready",
  APPROVAL_PENDING: "approval.pending",
  TASK_STEERED: "task.steered",
  LOOP_STARTED: "loop.started",
  LOOP_ROUND_STARTED: "loop.round.started",
  LOOP_RETRYING: "loop.retrying",
  LOOP_COMPACTED: "loop.compacted",
  LOOP_ROUND_COMPLETED: "loop.round.completed",
  LOOP_COMPLETED: "loop.completed",
  LOOP_FAILED: "loop.failed",
  MIRROR_OVERVIEW_UPDATED: "mirror.overview.updated",
  PLUGIN_UPDATED: "plugin.updated",
  PLUGIN_METRIC_UPDATED: "plugin.metric.updated",
  PLUGIN_TASK_UPDATED: "plugin.task.updated",
} as const;

export interface RequestMeta {
  trace_id: string;
  client_time: string;
}

export interface JsonRpcPage {
  limit: number;
  offset: number;
  total: number;
  has_more: boolean;
}

export interface PageContext {
  title?: string;
  app_name?: string;
  url?: string;
  browser_kind?: "chrome" | "edge" | "other_browser" | "non_browser";
  process_path?: string;
  process_id?: number;
  window_title?: string;
  visible_text?: string;
  hover_target?: string;
}

export interface ScreenContext {
  summary?: string;
  screen_summary?: string;
  visible_text?: string;
  window_title?: string;
  hover_target?: string;
}

export interface BehaviorContext {
  last_action?: string;
  dwell_millis?: number;
  copy_count?: number;
  window_switch_count?: number;
  page_switch_count?: number;
}

export interface ErrorContext {
  message?: string;
}

export interface ClipboardContext {
  text?: string;
}

export interface InputContext {
  page?: PageContext;
  screen?: ScreenContext;
  behavior?: BehaviorContext;
  selection?: {
    text: string;
  };
  error?: ErrorContext;
  clipboard?: ClipboardContext;
  text?: string;
  selection_text?: string;
  files?: string[];
  file_paths?: string[];
  screen_summary?: string;
  clipboard_text?: string;
  hover_target?: string;
  last_action?: string;
  dwell_millis?: number;
  copy_count?: number;
  window_switch_count?: number;
  page_switch_count?: number;
}

export interface VoiceMeta {
  voice_session_id: string;
  is_locked_session: boolean;
  asr_confidence: number;
  segment_id: string;
}

export interface DeliveryPreference {
  preferred: DeliveryResult["type"];
  fallback?: DeliveryResult["type"];
}

export interface AgentInputSubmitParams {
  request_meta: RequestMeta;
  session_id?: string;
  source: RequestSource;
  trigger: Extract<RequestTrigger, "voice_commit" | "hover_text_input">;
  input: {
    type: Extract<InputType, "text">;
    text: string;
    input_mode: InputMode;
  };
  context: InputContext;
  voice_meta?: VoiceMeta;
  options?: {
    confirm_required?: boolean;
    preferred_delivery?: DeliveryResult["type"];
  };
}

export interface AgentInputSubmitResult {
  task: Task;
  bubble_message: BubbleMessage | null;
  delivery_result: DeliveryResult | null;
}

export interface AgentTaskStartParams {
  request_meta: RequestMeta;
  session_id?: string;
  source: RequestSource;
  trigger: RequestTrigger;
  input: {
    type: InputType;
    text?: string;
    files?: string[];
    page_context?: PageContext;
    error_message?: string;
  };
  context?: InputContext;
  delivery?: DeliveryPreference;
}

export interface AgentTaskStartResult {
  task: Task;
  bubble_message: BubbleMessage | null;
  delivery_result: DeliveryResult | null;
}

export interface AgentTaskConfirmParams {
  request_meta: RequestMeta;
  task_id: string;
  confirmed: boolean;
  corrected_intent?: IntentPayload;
}

export interface AgentTaskConfirmResult {
  task: Task;
  bubble_message: BubbleMessage | null;
  delivery_result: DeliveryResult | null;
}

export interface RecommendationItem {
  recommendation_id: string;
  text: string;
  intent: IntentPayload;
}

export interface AgentRecommendationGetParams {
  request_meta: RequestMeta;
  source: RequestSource;
  scene: RecommendationScene;
  context: {
    page_title: string;
    app_name: string;
    selection_text?: string;
  };
}

export interface AgentRecommendationGetResult {
  cooldown_hit: boolean;
  items: RecommendationItem[];
}

export interface AgentRecommendationFeedbackSubmitParams {
  request_meta: RequestMeta;
  recommendation_id: string;
  feedback: RecommendationFeedback;
}

export interface AgentRecommendationFeedbackSubmitResult {
  applied: boolean;
}

export interface AgentTaskListParams {
  request_meta: RequestMeta;
  group: TaskListGroup;
  limit: number;
  offset: number;
  sort_by?: "updated_at" | "started_at" | "finished_at";
  sort_order?: "asc" | "desc";
}

export interface AgentTaskListResult {
  items: Task[];
  page: JsonRpcPage;
}

export interface SecuritySummary {
  security_status: SecurityStatus;
  risk_level: RiskLevel;
  pending_authorizations: 0 | 1;
  latest_restore_point: RecoveryPoint | null;
}

export interface AgentTaskDetailGetParams {
  request_meta: RequestMeta;
  task_id: string;
}

export interface AgentTaskDetailGetResult {
  task: Task;
  timeline: TaskStep[];
  delivery_result: DeliveryResult | null;
  artifacts: Artifact[];
  citations: Citation[];
  mirror_references: MirrorReference[];
  approval_request: ApprovalRequest | null;
  authorization_record: AuthorizationRecord | null;
  audit_record: AuditRecord | null;
  security_summary: SecuritySummary;
  runtime_summary: TaskRuntimeSummary;
}

export interface TaskRuntimeSummary {
  loop_stop_reason?: string | null;
  events_count: number;
  latest_event_type?: string | null;
  active_steering_count: number;
  latest_failure_code?: string | null;
  latest_failure_category?: string | null;
  latest_failure_summary?: string | null;
  observation_signals: string[];
}

export interface TaskEvent {
	 event_id: string;
	 run_id: string;
	 task_id: string;
	 step_id?: string;
	 type: string;
	 level: string;
	 payload_json: string;
	 created_at: string;
}

export interface AgentTaskEventsListParams {
	 request_meta: RequestMeta;
	 task_id: string;
	 run_id?: string;
	 type?: string;
	 created_at_from?: string;
	 created_at_to?: string;
	 limit?: number;
	 offset?: number;
}

export interface AgentTaskEventsListResult {
	 items: TaskEvent[];
	 page: JsonRpcPage;
}

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

export interface AgentTaskToolCallsListParams {
	 request_meta: RequestMeta;
	 task_id: string;
	 run_id?: string;
	 limit?: number;
	 offset?: number;
}

export interface AgentTaskToolCallsListResult {
	 items: ToolCall[];
	 page: JsonRpcPage;
}

export interface AgentTaskArtifactListParams {
  request_meta: RequestMeta;
  task_id: string;
  limit: number;
  offset: number;
}

export interface AgentTaskArtifactListResult {
  items: Artifact[];
  page: JsonRpcPage;
}

export interface AgentTaskArtifactOpenParams {
  request_meta: RequestMeta;
  task_id: string;
  artifact_id: string;
}

export interface AgentTaskArtifactOpenResult {
  artifact: Artifact;
  delivery_result: DeliveryResult;
  open_action: DeliveryType;
  resolved_payload: DeliveryPayload;
}

export interface AgentDeliveryOpenParams {
  request_meta: RequestMeta;
  task_id: string;
  artifact_id?: string;
}

export interface AgentDeliveryOpenResult {
  artifact?: Artifact;
  delivery_result: DeliveryResult;
  open_action: DeliveryType;
  resolved_payload: DeliveryPayload;
}

export interface AgentTaskControlParams {
  request_meta: RequestMeta;
  task_id: string;
  action: TaskControlAction;
  arguments?: Record<string, unknown>;
}

export interface AgentTaskControlResult {
  task: Task;
  bubble_message: BubbleMessage | null;
}

export interface AgentTaskSteerParams {
	 request_meta: RequestMeta;
	 task_id: string;
	 message: string;
}

export interface AgentTaskSteerResult {
	 task: Task;
	 bubble_message: BubbleMessage | null;
}

export interface AgentTaskInspectorConfigGetParams {
  request_meta: RequestMeta;
}

export interface InspectorConfig {
  task_sources: string[];
  inspection_interval: TimeInterval;
  inspect_on_file_change: boolean;
  inspect_on_startup: boolean;
  remind_before_deadline: boolean;
  remind_when_stale: boolean;
}

export type AgentTaskInspectorConfigGetResult = InspectorConfig;

export interface AgentTaskInspectorConfigUpdateParams extends InspectorConfig {
  request_meta: RequestMeta;
}

export interface AgentTaskInspectorConfigUpdateResult {
  updated: boolean;
  effective_config: InspectorConfig;
}

export interface AgentTaskInspectorRunParams {
  request_meta: RequestMeta;
  reason: string;
  target_sources: string[];
}

export interface AgentTaskInspectorRunResult {
  inspection_id: string;
  summary: {
    parsed_files: number;
    identified_items: number;
    due_today: number;
    overdue: number;
    stale: number;
  };
  suggestions: string[];
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

export interface AgentNotepadListParams {
  request_meta: RequestMeta;
  group: TodoBucket;
  limit: number;
  offset: number;
}

export interface AgentNotepadListResult {
  items: TodoItem[];
  page: JsonRpcPage;
}

export interface AgentNotepadConvertToTaskParams {
  request_meta: RequestMeta;
  item_id: string;
  confirmed: boolean;
}

export interface AgentNotepadConvertToTaskResult {
  task: Task;
}

export interface AgentNotepadUpdateParams {
  request_meta: RequestMeta;
  item_id: string;
  action: NotepadAction;
}

export interface AgentNotepadUpdateResult {
  notepad_item: TodoItem | null;
  refresh_groups: TodoBucket[];
  deleted_item_id?: string | null;
}

export interface AgentDashboardOverviewGetParams {
  request_meta: RequestMeta;
  focus_mode?: boolean;
  include?: Array<"focus_summary" | "trust_summary" | "quick_actions" | "global_state" | "high_value_signal">;
}

export interface AgentDashboardOverviewGetResult {
  overview: {
    focus_summary: {
      task_id: string;
      title: string;
      status: Task["status"];
      current_step: string;
      next_action: string;
      updated_at: string;
    } | null;
    trust_summary: {
      risk_level: RiskLevel;
      pending_authorizations: number;
      has_restore_point: boolean;
      workspace_path: string;
    };
    quick_actions?: string[];
    global_state?: Record<string, unknown>;
    high_value_signal?: string[];
  };
}

export interface AgentDashboardModuleGetParams {
  request_meta: RequestMeta;
  module: string;
  tab: string;
}

export interface AgentDashboardModuleGetResult {
  module: string;
  tab: string;
  summary: Record<string, unknown>;
  highlights: string[];
}

export interface AgentMirrorOverviewGetParams {
  request_meta: RequestMeta;
  include?: Array<"history_summary" | "daily_summary" | "profile" | "memory_references">;
}

export interface AgentMirrorOverviewGetResult {
  history_summary: string[];
  daily_summary: {
    date: string;
    completed_tasks: number;
    generated_outputs: number;
  } | null;
  profile: {
    work_style: string;
    preferred_output: string;
    active_hours: string;
  } | null;
  memory_references: MirrorReference[];
}

export interface AgentSecuritySummaryGetParams {
  request_meta: RequestMeta;
}

export interface AgentSecuritySummaryGetResult {
  summary: {
    security_status: SecurityStatus;
    pending_authorizations: number;
    latest_restore_point: RecoveryPoint | null;
    token_cost_summary: TokenCostSummary;
  };
}

export interface AgentSecurityPendingListParams {
  request_meta: RequestMeta;
  limit: number;
  offset: number;
}

export interface AgentSecurityPendingListResult {
  items: ApprovalRequest[];
  page: JsonRpcPage;
}

export interface AgentSecurityAuditListParams {
  request_meta: RequestMeta;
  task_id: string;
  limit: number;
  offset: number;
}

export interface AgentSecurityAuditListResult {
  items: AuditRecord[];
  page: JsonRpcPage;
}

export interface AgentSecurityRestorePointsListParams {
  request_meta: RequestMeta;
  task_id?: string;
  limit: number;
  offset: number;
}

export interface AgentSecurityRestorePointsListResult {
  items: RecoveryPoint[];
  page: JsonRpcPage;
}

export interface AgentSecurityRestoreApplyParams {
  request_meta: RequestMeta;
  task_id?: string;
  recovery_point_id: string;
}

export interface AgentSecurityRestoreApplyResult {
  applied: boolean;
  task: Task;
  recovery_point: RecoveryPoint;
  audit_record: AuditRecord | null;
  bubble_message: BubbleMessage | null;
}

export interface AgentSecurityRespondParams {
  request_meta: RequestMeta;
  task_id: string;
  approval_id: string;
  decision: ApprovalDecision;
  remember_rule?: boolean;
}

export interface AgentSecurityApprovalRespondResult {
  authorization_record: AuthorizationRecord;
  task: Task;
  bubble_message: BubbleMessage | null;
  impact_scope?: ImpactScope;
}

export interface AgentSecurityRestoreRespondResult {
  applied: boolean;
  task: Task;
  recovery_point: RecoveryPoint;
  audit_record: AuditRecord | null;
  bubble_message: BubbleMessage | null;
}

export type AgentSecurityRespondResult =
  | AgentSecurityApprovalRespondResult
  | AgentSecurityRestoreRespondResult;

export interface AgentSettingsGetParams {
  request_meta: RequestMeta;
  scope: "all" | "general" | "floating_ball" | "memory" | "task_automation" | "models";
}

export interface AgentSettingsGetResult {
  settings: SettingsSnapshot["settings"];
}

export interface AgentSettingsUpdateParams {
  request_meta: RequestMeta;
  general?: Partial<SettingsSnapshot["settings"]["general"]>;
  floating_ball?: Partial<SettingsSnapshot["settings"]["floating_ball"]>;
  memory?: Partial<SettingsSnapshot["settings"]["memory"]>;
  task_automation?: Partial<SettingsSnapshot["settings"]["task_automation"]>;
  models?: Partial<SettingsSnapshot["settings"]["models"]> & {
    budget_auto_downgrade?: boolean;
    base_url?: string;
    model?: string;
    api_key?: string;
    delete_api_key?: boolean;
  };
}

export interface AgentSettingsEffectiveSettings {
  general?: Partial<SettingsSnapshot["settings"]["general"]>;
  floating_ball?: Partial<SettingsSnapshot["settings"]["floating_ball"]>;
  memory?: Partial<SettingsSnapshot["settings"]["memory"]>;
  task_automation?: Partial<SettingsSnapshot["settings"]["task_automation"]>;
  models?: {
    provider?: string;
    budget_auto_downgrade?: boolean;
    provider_api_key_configured?: boolean;
    base_url?: string;
    model?: string;
    stronghold?: SettingsSnapshot["settings"]["models"]["credentials"]["stronghold"];
  };
}

export interface AgentSettingsUpdateResult {
  updated_keys: string[];
  effective_settings: AgentSettingsEffectiveSettings;
  apply_mode: ApplyMode;
  need_restart: boolean;
}

export interface TaskUpdatedNotification {
  task_id: string;
  session_id: Task["session_id"];
  status: Task["status"];
}

export interface DeliveryReadyNotification {
  task_id: string;
  delivery_result: DeliveryResult;
}

export interface ApprovalPendingNotification {
  task_id: string;
  approval_request: ApprovalRequest;
}

export interface MirrorOverviewUpdatedNotification {
  revision: number;
  source?: string;
}

export interface TaskSteeredNotification {
  task_id: string;
  message: string;
}

export interface TaskRuntimeNotification {
  task_id: string;
  event: TaskEvent;
  stop_reason?: string;
}
