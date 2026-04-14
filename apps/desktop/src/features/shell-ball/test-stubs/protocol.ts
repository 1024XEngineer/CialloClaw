export type ApprovalDecision = "allow_once" | "deny_once";
export type RiskLevel = "green" | "yellow" | "red";
export type SecurityStatus =
  | "normal"
  | "pending_confirmation"
  | "intercepted"
  | "execution_error"
  | "recoverable"
  | "recovered";

export type ApprovalRequest = {
  approval_id: string;
  task_id: string;
  operation_name: string;
  risk_level: RiskLevel;
  target_object: string;
  reason: string;
  status: string;
  [key: string]: any;
};

export type ApprovalPendingNotification = {
  approval_request: ApprovalRequest;
  text?: string;
  [key: string]: any;
};
export type RequestMeta = {
  trace_id: string;
  client_time: string;
};

export type RequestSource = "floating_ball" | "dashboard" | "tray_panel";

export type InputMode = "voice" | "text";

export type AgentInputSubmitParams = {
  request_meta: RequestMeta;
  session_id?: string;
  source: RequestSource;
  trigger: "voice_commit" | "hover_text_input";
  input: {
    type: "text";
    text: string;
    input_mode: InputMode;
  };
  context: {
    files: string[];
  };
  options?: {
    confirm_required?: boolean;
    preferred_delivery?: "bubble" | "workspace_document" | "result_page" | "open_file" | "reveal_in_folder" | "task_detail";
  };
};

export type AgentInputSubmitResult = {
  task: {
    task_id: string;
    status: string;
  };
  bubble_message: BubbleMessage | null;
  delivery_result?: {
    type?: string;
    preview_text?: string | null;
    payload?: {
      task_id?: string | null;
    } | null;
  } | null;
};

export type AgentSecuritySummaryGetParams = { request_meta: RequestMeta };
export type AgentSecurityPendingListParams = { request_meta: RequestMeta; page?: number; limit?: number; offset?: number };
export type AgentSecurityRespondParams = {
  approval_id: string;
  decision: ApprovalDecision;
  request_meta: RequestMeta;
  remember_rule?: boolean;
  task_id?: string;
};

export type JsonRpcPage = {
  limit: number;
  offset: number;
  total: number;
  has_more: boolean;
};

export type AgentSecuritySummaryGetResult = {
  summary: {
    security_status: SecurityStatus;
    pending_authorizations: number;
    latest_restore_point?: {
      recovery_point_id: string;
      task_id: string;
      summary: string;
      created_at: string;
      objects: string[];
    };
    token_cost_summary?: {
      current_task_tokens: number;
      current_task_cost: number;
      today_tokens: number;
      today_cost: number;
      single_task_limit: number;
      daily_limit: number;
      budget_auto_downgrade: boolean;
    };
    [key: string]: any;
  };
};

export type AgentSecurityPendingListResult = {
  items: ApprovalRequest[];
  page?: any;
};

export type AgentSecurityRespondResult = {
  approval_request?: ApprovalRequest;
  [key: string]: any;
};

export type Task = any;

export type BubbleMessageType = "status" | "intent_confirm" | "result";

export type BubbleMessage = {
  bubble_id: string;
  task_id: string;
  type: BubbleMessageType;
  text: string;
  pinned: boolean;
  hidden: boolean;
  created_at: string;
};
