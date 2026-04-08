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

// RPC_METHODS_STABLE 定义共享常量。
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

// RPC_METHODS_PLANNED 定义共享常量。
export const RPC_METHODS_PLANNED = {
  AGENT_SECURITY_AUDIT_LIST: "agent.security.audit.list",
  AGENT_SECURITY_RESTORE_POINTS_LIST: "agent.security.restore_points.list",
  AGENT_SECURITY_RESTORE_APPLY: "agent.security.restore.apply",
  AGENT_MIRROR_MEMORY_MANAGE: "agent.mirror.memory.manage",
  AGENT_TASK_ARTIFACT_LIST: "agent.task.artifact.list",
  AGENT_TASK_ARTIFACT_OPEN: "agent.task.artifact.open",
  AGENT_DELIVERY_OPEN: "agent.delivery.open",
} as const;

// RPC_METHODS 定义共享常量。
export const RPC_METHODS = {
  ...RPC_METHODS_STABLE,
  ...RPC_METHODS_PLANNED,
} as const;

// NOTIFICATION_METHODS 定义共享常量。
export const NOTIFICATION_METHODS = {
  TASK_UPDATED: "task.updated",
  DELIVERY_READY: "delivery.ready",
  APPROVAL_PENDING: "approval.pending",
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

// PageContext 定义当前模块的接口约束。
export interface PageContext {
  title: string;
  app_name: string;
  url: string;
}

// InputContext 定义当前模块的接口约束。
export interface InputContext {
  page?: PageContext;
  selection?: {
    text: string;
  };
  files?: string[];
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

// AgentInputSubmitResult 定义当前模块的接口约束。
export interface AgentInputSubmitResult {
  task: Task;
  bubble_message: BubbleMessage | null;
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
  intent?: IntentPayload;
  delivery?: DeliveryPreference;
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
  pending_authorizations: number;
  latest_restore_point: string | RecoveryPoint | null;
}

// AgentTaskDetailGetParams 定义当前模块的接口约束。
export interface AgentTaskDetailGetParams {
  request_meta: RequestMeta;
  task_id: string;
}

// AgentTaskDetailGetResult 定义当前模块的接口约束。
export interface AgentTaskDetailGetResult {
  task: Task;
  timeline: TaskStep[];
  artifacts: Artifact[];
  mirror_references: MirrorReference[];
  security_summary: SecuritySummary;
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

// AgentSecurityRespondParams 定义当前模块的接口约束。
export interface AgentSecurityRespondParams {
  request_meta: RequestMeta;
  task_id: string;
  approval_id: string;
  decision: ApprovalDecision;
  remember_rule?: boolean;
}

// AgentSecurityRespondResult 定义当前模块的接口约束。
export interface AgentSecurityRespondResult {
  authorization_record: AuthorizationRecord;
  task: Task;
  bubble_message: BubbleMessage | null;
  impact_scope?: ImpactScope;
}

// AgentSettingsGetParams 定义当前模块的接口约束。
export interface AgentSettingsGetParams {
  request_meta: RequestMeta;
  scope: "all" | "general" | "floating_ball" | "memory" | "task_automation" | "data_log";
}

// AgentSettingsGetResult 定义当前模块的接口约束。
export interface AgentSettingsGetResult {
  settings: SettingsSnapshot["settings"];
}

// AgentSettingsUpdateParams 定义当前模块的接口约束。
export interface AgentSettingsUpdateParams {
  request_meta: RequestMeta;
  general?: Partial<SettingsSnapshot["settings"]["general"]>;
  floating_ball?: Partial<SettingsSnapshot["settings"]["floating_ball"]>;
  memory?: Partial<SettingsSnapshot["settings"]["memory"]>;
  task_automation?: Partial<SettingsSnapshot["settings"]["task_automation"]>;
  data_log?: Partial<SettingsSnapshot["settings"]["data_log"]>;
}

// AgentSettingsUpdateResult 定义当前模块的接口约束。
export interface AgentSettingsUpdateResult {
  updated_keys: string[];
  effective_settings: Partial<SettingsSnapshot["settings"]>;
  apply_mode: ApplyMode;
  need_restart: boolean;
}

// TaskUpdatedNotification 定义当前模块的接口约束。
export interface TaskUpdatedNotification {
  task_id: string;
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
