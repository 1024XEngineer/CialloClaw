// 该文件定义稳定与规划中的 JSON-RPC 方法及其参数结构。
import type {
  ApprovalDecision,
  ApprovalRequest,
  ApplyMode,
  Artifact,
  AuditRecord,
  BubbleMessage,
  DeliveryResult,
  ImpactScope,
  InputMode,
  InputType,
  IntentPayload,
  MirrorReference,
  RecommendationFeedback,
  RecommendationScene,
  RecoveryPoint,
  RequestSource,
  RequestTrigger,
  RiskLevel,
  SecurityStatus,
  Session,
  SettingsSnapshot,
  Task,
  TaskControlAction,
  TaskListGroup,
  TaskStep,
  TimeInterval,
  TokenCostSummary,
  TodoBucket,
  TodoItem,
  AuthorizationRecord,
} from "../types/index";

export const RPC_METHODS_STABLE = {
  AGENT_INPUT_SUBMIT: "agent.input.submit",
  AGENT_TASK_START: "agent.task.start",
  AGENT_TASK_CONFIRM: "agent.task.confirm",
  AGENT_RECOMMENDATION_GET: "agent.recommendation.get",
  AGENT_RECOMMENDATION_FEEDBACK_SUBMIT: "agent.recommendation.feedback.submit",
  AGENT_TASK_LIST: "agent.task.list",
  AGENT_TASK_DETAIL_GET: "agent.task.detail.get",
  AGENT_TASK_CONTROL: "agent.task.control",
  AGENT_TASK_INSPECTOR_CONFIG_GET: "agent.task_inspector.config.get",
  AGENT_TASK_INSPECTOR_CONFIG_UPDATE: "agent.task_inspector.config.update",
  AGENT_TASK_INSPECTOR_RUN: "agent.task_inspector.run",
  AGENT_NOTEPAD_LIST: "agent.notepad.list",
  AGENT_NOTEPAD_CONVERT_TO_TASK: "agent.notepad.convert_to_task",
  AGENT_DASHBOARD_OVERVIEW_GET: "agent.dashboard.overview.get",
  AGENT_DASHBOARD_MODULE_GET: "agent.dashboard.module.get",
  AGENT_MIRROR_OVERVIEW_GET: "agent.mirror.overview.get",
  AGENT_SECURITY_SUMMARY_GET: "agent.security.summary.get",
  AGENT_SECURITY_PENDING_LIST: "agent.security.pending.list",
  AGENT_SECURITY_RESPOND: "agent.security.respond",
  AGENT_SETTINGS_GET: "agent.settings.get",
  AGENT_SETTINGS_UPDATE: "agent.settings.update",
} as const;

export const RPC_METHODS_PLANNED = {
  AGENT_SECURITY_AUDIT_LIST: "agent.security.audit.list",
  AGENT_SECURITY_RESTORE_POINTS_LIST: "agent.security.restore_points.list",
  AGENT_SECURITY_RESTORE_APPLY: "agent.security.restore.apply",
  AGENT_MIRROR_MEMORY_MANAGE: "agent.mirror.memory.manage",
  AGENT_TASK_ARTIFACT_LIST: "agent.task.artifact.list",
  AGENT_TASK_ARTIFACT_OPEN: "agent.task.artifact.open",
  AGENT_DELIVERY_OPEN: "agent.delivery.open",
} as const;

export const RPC_METHODS = {
  ...RPC_METHODS_STABLE,
  ...RPC_METHODS_PLANNED,
} as const;

export const NOTIFICATION_METHODS = {
  TASK_UPDATED: "task.updated",
  DELIVERY_READY: "delivery.ready",
  APPROVAL_PENDING: "approval.pending",
  PLUGIN_UPDATED: "plugin.updated",
  PLUGIN_METRIC_UPDATED: "plugin.metric.updated",
  PLUGIN_TASK_UPDATED: "plugin.task.updated",
} as const;

export type RpcMethodName = (typeof RPC_METHODS)[keyof typeof RPC_METHODS];

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
  title: string;
  app_name: string;
  url: string;
}

export interface InputContext {
  page?: PageContext;
  selection?: {
    text: string;
  };
  files?: string[];
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
  intent?: IntentPayload;
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
  pending_authorizations: number;
  latest_restore_point: string | RecoveryPoint | null;
}

export interface AgentTaskDetailGetParams {
  request_meta: RequestMeta;
  task_id: string;
}

export interface AgentTaskDetailGetResult {
  task: Task;
  timeline: TaskStep[];
  artifacts: Artifact[];
  mirror_references: MirrorReference[];
  security_summary: SecuritySummary;
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

export interface InspectorConfig {
  task_sources: string[];
  inspection_interval: TimeInterval;
  inspect_on_file_change: boolean;
  inspect_on_startup: boolean;
  remind_before_deadline: boolean;
  remind_when_stale: boolean;
}

export interface AgentTaskInspectorConfigGetParams {
  request_meta: RequestMeta;
}

export interface AgentTaskInspectorConfigGetResult extends InspectorConfig {}

export interface AgentTaskInspectorConfigUpdateParams {
  request_meta: RequestMeta;
  task_sources: string[];
  inspection_interval: TimeInterval;
  inspect_on_file_change: boolean;
  inspect_on_startup: boolean;
  remind_before_deadline: boolean;
  remind_when_stale: boolean;
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

export interface AgentSecurityRespondParams {
  request_meta: RequestMeta;
  task_id: string;
  approval_id: string;
  decision: ApprovalDecision;
  remember_rule?: boolean;
}

export interface AgentSecurityRespondResult {
  authorization_record: AuthorizationRecord;
  task: Task;
  bubble_message: BubbleMessage | null;
  impact_scope?: ImpactScope;
}

export interface AgentSettingsGetParams {
  request_meta: RequestMeta;
  scope: "all" | "general" | "floating_ball" | "memory" | "task_automation" | "data_log";
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
  data_log?: Partial<SettingsSnapshot["settings"]["data_log"]>;
}

export interface AgentSettingsUpdateResult {
  updated_keys: string[];
  effective_settings: Partial<SettingsSnapshot["settings"]>;
  apply_mode: ApplyMode;
  need_restart: boolean;
}

export interface TaskUpdatedNotification {
  task_id: string;
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
