import type {
  ApprovalDecision,
  ApprovalRequest,
  Artifact,
  AuditRecord,
  BubbleMessage,
  DeliveryResult,
  InputMode,
  InputType,
  MirrorReference,
  RequestSource,
  RequestTrigger,
  RecoveryPoint,
  RetrievalHit,
  SecurityStatus,
  Session,
  SettingsScope,
  SettingsSnapshot,
  SettingItem,
  Task,
  TaskListGroup,
  TaskSourceType,
  TaskStep,
  TokenCostSummary,
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
  TASK_STEP_UPDATED: "task.step.updated",
  BUBBLE_UPDATED: "bubble.updated",
  ARTIFACT_CREATED: "artifact.created",
  APPROVAL_REQUEST_UPDATED: "approval_request.updated",
  RECOVERY_POINT_CREATED: "recovery_point.created",
} as const;

export type RpcMethodName = (typeof RPC_METHODS)[keyof typeof RPC_METHODS];

export interface RequestMeta {
  trace_id: string;
  client_time: string;
}

export interface AgentInputSubmitParams {
  request_meta: RequestMeta;
  session_id?: string;
  task_id?: string;
  request_source: RequestSource;
  request_trigger: Extract<RequestTrigger, "voice_commit" | "hover_text_input">;
  input_mode: InputMode;
  text: string;
}

export interface AgentInputSubmitResult {
  session: Session;
  task: Task;
  bubble_message: BubbleMessage | null;
}

export interface AgentTaskStartParams {
  request_meta: RequestMeta;
  session_id?: string;
  request_source: RequestSource;
  request_trigger: RequestTrigger;
  input_type: InputType;
  source_type: TaskSourceType;
  title: string;
  payload: {
    text?: string;
    file_paths?: string[];
    error_message?: string;
  };
}

export interface AgentTaskStartResult {
  session: Session;
  task: Task;
  bubble_message: BubbleMessage | null;
}

export interface AgentTaskConfirmParams {
  request_meta: RequestMeta;
  task_id: string;
  confirmed: boolean;
  intent_name: string;
  arguments?: Record<string, unknown>;
}

export interface AgentTaskConfirmResult {
  task: Task;
  bubble_message: BubbleMessage | null;
  delivery_result: DeliveryResult | null;
}

export interface AgentTaskListParams {
  request_meta: RequestMeta;
  group?: TaskListGroup;
}

export interface AgentTaskListResult {
  tasks: Task[];
}

export interface AgentTaskDetailGetParams {
  request_meta: RequestMeta;
  task_id: string;
}

export interface AgentTaskDetailGetResult {
  task: Task;
  task_steps: TaskStep[];
  artifacts: Artifact[];
  approval_requests: ApprovalRequest[];
  audit_records: AuditRecord[];
  recovery_points: RecoveryPoint[];
  mirror_references: MirrorReference[];
}

export interface AgentTaskControlParams {
  request_meta: RequestMeta;
  task_id: string;
  action: "pause" | "resume" | "cancel" | "restart";
}

export interface AgentTaskControlResult {
  task: Task;
}

export interface AgentDashboardOverviewGetParams {
  request_meta: RequestMeta;
}

export interface AgentDashboardOverviewGetResult {
  unfinished_tasks: Task[];
  finished_tasks: Task[];
  pending_approvals: ApprovalRequest[];
  token_cost_summary: TokenCostSummary;
}

export interface AgentMirrorOverviewGetParams {
  request_meta: RequestMeta;
  task_id?: string;
}

export interface AgentMirrorOverviewGetResult {
  retrieval_hits: RetrievalHit[];
  mirror_references: MirrorReference[];
}

export interface AgentSecuritySummaryGetParams {
  request_meta: RequestMeta;
}

export interface AgentSecuritySummaryGetResult {
  security_status: SecurityStatus;
  pending_count: number;
  recent_audit_records: AuditRecord[];
  recovery_points: RecoveryPoint[];
}

export interface AgentSecurityPendingListParams {
  request_meta: RequestMeta;
}

export interface AgentSecurityPendingListResult {
  approval_requests: ApprovalRequest[];
}

export interface AgentSecurityRespondParams {
  request_meta: RequestMeta;
  task_id: string;
  approval_id: string;
  decision: ApprovalDecision;
}

export interface AgentSecurityRespondResult {
  task: Task;
}

export interface AgentSettingsGetParams {
  request_meta: RequestMeta;
  scope?: SettingsScope;
}

export interface AgentSettingsGetResult {
  settings: SettingsSnapshot;
  items: SettingItem[];
}

export interface AgentSettingsUpdateParams {
  request_meta: RequestMeta;
  key: string;
  value: string | boolean | number | null;
}

export interface AgentSettingsUpdateResult {
  settings: SettingsSnapshot;
}

export interface TaskUpdatedNotification {
  task_id: string;
  status: Task["status"];
}
