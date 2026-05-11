// 该文件定义稳定与规划中的 JSON-RPC 方法及其参数结构。
import type {
  ApprovalDecision,
  ApprovalRequest,
  ApplyMode,
  Artifact,
  AuditRecord,
  BubbleMessage,
  Citation,
  DeliveryPayload,
  DeliveryResult,
  DeliveryType,
  ImpactScope,
  InputMode,
  InputType,
  IntentPayload,
  MirrorReference,
  NotepadAction,
  PluginListItem,
  PluginManifest,
  RecommendationFeedback,
  RecommendationScene,
  RecoveryPoint,
  RequestSource,
  RequestTrigger,
  RiskLevel,
  SecurityStatus,
  Session,
  SettingsSnapshot,
  PluginMetricSnapshot,
  PluginRuntimeEvent,
  PluginRuntimeState,
  PluginToolContract,
  Task,
  TaskControlAction,
  ToolCall,
  TaskListGroup,
  TaskStep,
  TimeInterval,
  TokenCostSummary,
  TodoBucket,
  TodoItem,
  AuthorizationRecord,
} from "../types/index";

// RPC_METHODS_STABLE lists the frozen JSON-RPC methods that are already
// implemented and safe for frontend/backend integration.
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
  AGENT_SETTINGS_MODEL_VALIDATE: "agent.settings.model.validate",
  AGENT_PLUGIN_RUNTIME_LIST: "agent.plugin.runtime.list",
  AGENT_PLUGIN_LIST: "agent.plugin.list",
  AGENT_PLUGIN_DETAIL_GET: "agent.plugin.detail.get",
} as const;

// RPC_METHODS_PLANNED reserves method names that are still documented as
// planned and do not have a frozen implementation contract yet.
export const RPC_METHODS_PLANNED = {
  AGENT_MIRROR_MEMORY_MANAGE: "agent.mirror.memory.manage",
  AGENT_PLUGIN_ENABLE: "agent.plugin.enable",
  AGENT_PLUGIN_DISABLE: "agent.plugin.disable",
} as const;

// RPC_METHODS combines stable and planned method names for typed reuse.
export const RPC_METHODS = {
  ...RPC_METHODS_STABLE,
  ...RPC_METHODS_PLANNED,
} as const;

// NOTIFICATION_METHODS 定义共享常量。
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
  TASK_SESSION_QUEUED: "task.session_queued",
  TASK_SESSION_RESUMED: "task.session_resumed",
  MIRROR_OVERVIEW_UPDATED: "mirror.overview.updated",
  PLUGIN_UPDATED: "plugin.updated",
  PLUGIN_METRIC_UPDATED: "plugin.metric.updated",
  PLUGIN_TASK_UPDATED: "plugin.task.updated",
} as const;

// RpcMethodName 定义当前模块的数据结构。
export type RpcMethodName = (typeof RPC_METHODS)[keyof typeof RPC_METHODS];

// RequestMeta 定义当前模块的接口约束。
export interface RequestMeta {
  trace_id: string;
  client_time: string;
}

// JsonRpcPage 定义当前模块的接口约束。
export interface JsonRpcPage {
  limit: number;
  offset: number;
  total: number;
  has_more: boolean;
}

// PageContext defines the stable page-level metadata that task entrypoints can
// carry into backend context capture.
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

// ScreenContext defines the stable screen-derived signals that can help infer
// controlled visual tasks without introducing a parallel RPC entrypoint.
export interface ScreenContext {
  summary?: string;
  screen_summary?: string;
  visible_text?: string;
  window_title?: string;
  hover_target?: string;
}

// BehaviorContext defines lightweight interaction signals that stay attached to
// formal task entry requests.
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

// InputContext defines the stable request-context envelope shared by
// `agent.input.submit` and `agent.task.start`.
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

// VoiceMeta 定义当前模块的接口约束。
export interface VoiceMeta {
  voice_session_id: string;
  is_locked_session: boolean;
  asr_confidence: number;
  segment_id: string;
}

// DeliveryPreference 定义当前模块的接口约束。
export interface DeliveryPreference {
  preferred: DeliveryResult["type"];
  fallback?: DeliveryResult["type"];
}

// AgentInputSubmitParams 定义当前模块的接口约束。
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

// AgentInputSubmitResult returns either a formal task handoff or a detached
// lightweight chat bubble when the input does not need task creation.
export interface AgentInputSubmitResult {
  task: Task | null;
  bubble_message: BubbleMessage | null;
  delivery_result: DeliveryResult | null;
}

// AgentTaskStartParams 定义当前模块的接口约束。
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
  options?: {
    confirm_required?: boolean;
  };
}

// AgentTaskStartResult 定义当前模块的接口约束。
export interface AgentTaskStartResult {
  task: Task;
  bubble_message: BubbleMessage | null;
  delivery_result: DeliveryResult | null;
}

// AgentTaskConfirmParams 定义当前模块的接口约束。
export interface AgentTaskConfirmParams {
  request_meta: RequestMeta;
  task_id: string;
  confirmed: boolean;
  corrected_intent?: IntentPayload;
}

// AgentTaskConfirmResult 定义当前模块的接口约束。
export interface AgentTaskConfirmResult {
  task: Task;
  bubble_message: BubbleMessage | null;
  delivery_result: DeliveryResult | null;
}

// RecommendationItem 定义当前模块的接口约束。
export interface RecommendationItem {
  recommendation_id: string;
  text: string;
  intent: IntentPayload;
}

// AgentRecommendationGetParams 定义当前模块的接口约束。
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

// AgentRecommendationGetResult 定义当前模块的接口约束。
export interface AgentRecommendationGetResult {
  cooldown_hit: boolean;
  items: RecommendationItem[];
}

// AgentRecommendationFeedbackSubmitParams 定义当前模块的接口约束。
export interface AgentRecommendationFeedbackSubmitParams {
  request_meta: RequestMeta;
  recommendation_id: string;
  feedback: RecommendationFeedback;
}

// AgentRecommendationFeedbackSubmitResult 定义当前模块的接口约束。
export interface AgentRecommendationFeedbackSubmitResult {
  applied: boolean;
}

// AgentTaskListParams 定义当前模块的接口约束。
export interface AgentTaskListParams {
  request_meta: RequestMeta;
  group: TaskListGroup;
  limit: number;
  offset: number;
  sort_by?: "updated_at" | "started_at" | "finished_at";
  sort_order?: "asc" | "desc";
}

// AgentTaskListResult 定义当前模块的接口约束。
export interface AgentTaskListResult {
  items: Task[];
  page: JsonRpcPage;
}

// SecuritySummary 定义当前模块的接口约束。
export interface SecuritySummary {
  security_status: SecurityStatus;
  risk_level: RiskLevel;
  pending_authorizations: 0 | 1;
  latest_restore_point: RecoveryPoint | null;
}

// AgentTaskDetailGetParams 定义当前模块的接口约束。
export interface AgentTaskDetailGetParams {
  request_meta: RequestMeta;
  task_id: string;
}

// AgentTaskDetailGetResult 定义当前模块的接口约束。
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

export interface AgentTaskDetailGetResult {
  task: Task;
  timeline: TaskStep[];
  // delivery_result carries the latest formal conclusion for task detail.
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

// TaskEvent defines one persisted compatibility event exposed through task-centric queries.
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

// AgentTaskEventsListParams defines the parameters for agent.task.events.list.
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

// AgentTaskEventsListResult defines the result for agent.task.events.list.
export interface AgentTaskEventsListResult {
  items: TaskEvent[];
  page: JsonRpcPage;
}

// AgentTaskToolCallsListParams defines the parameters for
// agent.task.tool_calls.list.
export interface AgentTaskToolCallsListParams {
  request_meta: RequestMeta;
  task_id: string;
  run_id?: string;
  limit?: number;
  offset?: number;
}

// AgentTaskToolCallsListResult defines the result for
// agent.task.tool_calls.list.
export interface AgentTaskToolCallsListResult {
  items: ToolCall[];
  page: JsonRpcPage;
}

// AgentTaskSteerParams defines the parameters for agent.task.steer.
export interface AgentTaskSteerParams {
  request_meta: RequestMeta;
  task_id: string;
  message: string;
}

// AgentTaskSteerResult defines the result for agent.task.steer.
export interface AgentTaskSteerResult {
  task: Task;
  bubble_message: BubbleMessage | null;
}

// AgentTaskArtifactListParams defines the parameters for agent.task.artifact.list.
export interface AgentTaskArtifactListParams {
  request_meta: RequestMeta;
  task_id: string;
  limit: number;
  offset: number;
}

// AgentTaskArtifactListResult defines the result for agent.task.artifact.list.
export interface AgentTaskArtifactListResult {
  items: Artifact[];
  page: JsonRpcPage;
}

// AgentTaskArtifactOpenParams defines the parameters for agent.task.artifact.open.
export interface AgentTaskArtifactOpenParams {
  request_meta: RequestMeta;
  task_id: string;
  artifact_id: string;
}

// AgentTaskArtifactOpenResult defines the result for agent.task.artifact.open.
export interface AgentTaskArtifactOpenResult {
  artifact: Artifact;
  delivery_result: DeliveryResult;
  open_action: DeliveryType;
  resolved_payload: DeliveryPayload;
}

// AgentDeliveryOpenParams defines the parameters for agent.delivery.open.
export interface AgentDeliveryOpenParams {
  request_meta: RequestMeta;
  task_id: string;
  artifact_id?: string;
}

// AgentDeliveryOpenResult defines the result for agent.delivery.open.
export interface AgentDeliveryOpenResult {
  artifact?: Artifact;
  delivery_result: DeliveryResult;
  open_action: DeliveryType;
  resolved_payload: DeliveryPayload;
}

// AgentTaskControlParams 定义当前模块的接口约束。
export interface AgentTaskControlParams {
  request_meta: RequestMeta;
  task_id: string;
  action: TaskControlAction;
  arguments?: Record<string, unknown>;
}

// AgentTaskControlResult 定义当前模块的接口约束。
export interface AgentTaskControlResult {
  task: Task;
  bubble_message: BubbleMessage | null;
}

// InspectorConfig 定义当前模块的接口约束。
export interface InspectorConfig {
  task_sources: string[];
  inspection_interval: TimeInterval;
  inspect_on_file_change: boolean;
  inspect_on_startup: boolean;
  remind_before_deadline: boolean;
  remind_when_stale: boolean;
}

// AgentTaskInspectorConfigGetParams 定义当前模块的接口约束。
export interface AgentTaskInspectorConfigGetParams {
  request_meta: RequestMeta;
}

// AgentTaskInspectorConfigGetResult 定义当前模块的接口约束。
export interface AgentTaskInspectorConfigGetResult extends InspectorConfig {}

// AgentTaskInspectorConfigUpdateParams 定义当前模块的接口约束。
export interface AgentTaskInspectorConfigUpdateParams {
  request_meta: RequestMeta;
  task_sources: string[];
  inspection_interval: TimeInterval;
  inspect_on_file_change: boolean;
  inspect_on_startup: boolean;
  remind_before_deadline: boolean;
  remind_when_stale: boolean;
}

// AgentTaskInspectorConfigUpdateResult 定义当前模块的接口约束。
export interface AgentTaskInspectorConfigUpdateResult {
  updated: boolean;
  effective_config: InspectorConfig;
}

// AgentTaskInspectorRunParams 定义当前模块的接口约束。
export interface AgentTaskInspectorRunParams {
  request_meta: RequestMeta;
  reason: string;
  target_sources: string[];
}

// AgentTaskInspectorRunResult 定义当前模块的接口约束。
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

// AgentNotepadListParams 定义当前模块的接口约束。
export interface AgentNotepadListParams {
  request_meta: RequestMeta;
  group: TodoBucket;
  limit: number;
  offset: number;
}

// AgentNotepadListResult 定义当前模块的接口约束。
export interface AgentNotepadListResult {
  items: TodoItem[];
  page: JsonRpcPage;
}

// AgentNotepadConvertToTaskParams 定义当前模块的接口约束。
export interface AgentNotepadConvertToTaskParams {
  request_meta: RequestMeta;
  item_id: string;
  confirmed: boolean;
}

// AgentNotepadConvertToTaskResult 定义当前模块的接口约束。
export interface AgentNotepadConvertToTaskResult {
  task: Task;
  notepad_item: TodoItem;
  refresh_groups: TodoBucket[];
}

// AgentNotepadUpdateParams defines the parameters for `agent.notepad.update`.
export interface AgentNotepadUpdateParams {
  request_meta: RequestMeta;
  item_id: string;
  action: NotepadAction;
}

// AgentNotepadUpdateResult defines the result for `agent.notepad.update`.
export interface AgentNotepadUpdateResult {
  notepad_item: TodoItem | null;
  refresh_groups: TodoBucket[];
  deleted_item_id?: string | null;
}

// AgentDashboardOverviewGetParams 定义当前模块的接口约束。
export interface AgentDashboardOverviewGetParams {
  request_meta: RequestMeta;
  focus_mode?: boolean;
  include?: Array<"focus_summary" | "trust_summary" | "quick_actions" | "global_state" | "high_value_signal">;
}

// AgentDashboardOverviewGetResult 定义当前模块的接口约束。
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

// AgentDashboardModuleGetParams 定义当前模块的接口约束。
export interface AgentDashboardModuleGetParams {
  request_meta: RequestMeta;
  module: string;
  tab: string;
}

// AgentDashboardModuleGetResult 定义当前模块的接口约束。
export interface AgentDashboardModuleGetResult {
  module: string;
  tab: string;
  summary: Record<string, unknown>;
  highlights: string[];
}

// AgentMirrorOverviewGetParams 定义当前模块的接口约束。
export interface AgentMirrorOverviewGetParams {
  request_meta: RequestMeta;
  include?: Array<"history_summary" | "daily_summary" | "profile" | "memory_references">;
}

// AgentMirrorOverviewGetResult 定义当前模块的接口约束。
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

// AgentSecuritySummaryGetParams 定义当前模块的接口约束。
export interface AgentSecuritySummaryGetParams {
  request_meta: RequestMeta;
}

// AgentSecuritySummaryGetResult 定义当前模块的接口约束。
export interface AgentSecuritySummaryGetResult {
  summary: {
    security_status: SecurityStatus;
    pending_authorizations: number;
    latest_restore_point: RecoveryPoint | null;
    token_cost_summary: TokenCostSummary;
  };
}

// AgentSecurityPendingListParams 定义当前模块的接口约束。
export interface AgentSecurityPendingListParams {
  request_meta: RequestMeta;
  limit: number;
  offset: number;
}

// AgentSecurityPendingListResult 定义当前模块的接口约束。
export interface AgentSecurityPendingListResult {
  items: ApprovalRequest[];
  page: JsonRpcPage;
}

// AgentSecurityAuditListParams defines the parameters for
// `agent.security.audit.list`.
export interface AgentSecurityAuditListParams {
  request_meta: RequestMeta;
  task_id: string;
  limit: number;
  offset: number;
}

// AgentSecurityAuditListResult defines the result for
// `agent.security.audit.list`.
export interface AgentSecurityAuditListResult {
  items: AuditRecord[];
  page: JsonRpcPage;
}

// AgentSecurityRestorePointsListParams 定义当前模块的接口约束。
export interface AgentSecurityRestorePointsListParams {
  request_meta: RequestMeta;
  task_id?: string;
  limit: number;
  offset: number;
}

// AgentSecurityRestorePointsListResult 定义当前模块的接口约束。
export interface AgentSecurityRestorePointsListResult {
  items: RecoveryPoint[];
  page: JsonRpcPage;
}

// AgentSecurityRestoreApplyParams 定义当前模块的接口约束。
export interface AgentSecurityRestoreApplyParams {
  request_meta: RequestMeta;
  task_id?: string;
  recovery_point_id: string;
}

// AgentSecurityRestoreApplyResult 定义当前模块的接口约束。
export interface AgentSecurityRestoreApplyResult {
  applied: boolean;
  task: Task;
  recovery_point: RecoveryPoint;
  audit_record: AuditRecord | null;
  bubble_message: BubbleMessage | null;
}

// AgentSecurityRespondParams 定义当前模块的接口约束。
export interface AgentSecurityRespondParams {
  request_meta: RequestMeta;
  task_id: string;
  approval_id: string;
  decision: ApprovalDecision;
  remember_rule?: boolean;
}

// AgentSecurityRespondResult 定义当前模块的接口约束。
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

// AgentSettingsGetParams defines the frozen settings snapshot query params.
export interface AgentSettingsGetParams {
  request_meta: RequestMeta;
  scope: "all" | "general" | "floating_ball" | "memory" | "task_automation" | "models";
}

// AgentSettingsGetResult defines the settings snapshot query result.
export interface AgentSettingsGetResult {
  settings: SettingsSnapshot["settings"];
}

// AgentSettingsUpdateParams defines the writable settings update payload.
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

// AgentSettingsUpdateResult defines the persisted settings update result.
export interface AgentSettingsUpdateResult {
  updated_keys: string[];
  effective_settings: AgentSettingsEffectiveSettings;
  apply_mode: ApplyMode;
  need_restart: boolean;
}

export type AgentSettingsModelValidateStatus =
  | "valid"
  | "missing_provider"
  | "missing_base_url"
  | "missing_model"
  | "missing_api_key"
  | "secret_store_unavailable"
  | "auth_failed"
  | "endpoint_not_found"
  | "request_rejected"
  | "request_timeout"
  | "request_failed"
  | "invalid_response"
  | "tool_calling_unavailable"
  | "unknown_error";

// AgentSettingsModelValidateParams defines the post-save validation payload for
// the effective model route that should power future tasks.
export interface AgentSettingsModelValidateParams {
  request_meta: RequestMeta;
  models?: Partial<SettingsSnapshot["settings"]["models"]> & {
    budget_auto_downgrade?: boolean;
    base_url?: string;
    model?: string;
    api_key?: string;
    delete_api_key?: boolean;
  };
}

// AgentSettingsModelValidateResult defines the structured model validation
// result returned after save-time compatibility probes.
export interface AgentSettingsModelValidateResult {
  ok: boolean;
  status: AgentSettingsModelValidateStatus;
  message: string;
  provider: string;
  canonical_provider: string;
  base_url: string;
  model: string;
  text_generation_ready: boolean;
  tool_calling_ready: boolean;
}

// AgentPluginRuntimeListParams defines the stable plugin runtime query params.
export interface AgentPluginRuntimeListParams {
  request_meta: RequestMeta;
}

// AgentPluginRuntimeListResult defines the plugin runtime query result.
export interface AgentPluginRuntimeListResult {
  items: PluginRuntimeState[];
  metrics: PluginMetricSnapshot[];
  events: PluginRuntimeEvent[];
}

export interface AgentPluginListParams {
  request_meta: RequestMeta;
  page?: {
    limit: number;
    offset: number;
  };
  query?: string;
  kinds?: PluginRuntimeState["kind"][];
  health?: PluginRuntimeState["health"][];
}

export interface AgentPluginListResult {
  items: PluginListItem[];
  page: JsonRpcPage;
}

export interface AgentPluginDetailGetParams {
  request_meta: RequestMeta;
  plugin_id: string;
  include_runtime?: boolean;
  include_metrics?: boolean;
  include_events?: boolean;
}

export interface AgentPluginDetailGetResult {
  plugin: PluginManifest;
  runtimes: PluginRuntimeState[];
  metrics: PluginMetricSnapshot[];
  recent_events: PluginRuntimeEvent[];
  tools: PluginToolContract[];
}

// TaskUpdatedNotification carries the minimal task status delta emitted by the backend.
export interface TaskUpdatedNotification {
  task_id: string;
  session_id: Task["session_id"];
  status: Task["status"];
}

// DeliveryReadyNotification 定义当前模块的接口约束。
export interface DeliveryReadyNotification {
  task_id: string;
  delivery_result: DeliveryResult;
}

// ApprovalPendingNotification 定义当前模块的接口约束。
export interface ApprovalPendingNotification {
  task_id: string;
  approval_request: ApprovalRequest;
}

export interface TaskSessionQueuedNotification {
  task_id: string;
  blocking_task_id: string;
}

export interface TaskSessionResumedNotification {
  task_id: string;
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
