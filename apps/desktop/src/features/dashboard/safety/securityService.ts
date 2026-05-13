import type {
  AgentSecurityAuditListParams,
  AgentSecurityApprovalRespondResult,
  AgentSecurityPendingListParams,
  AgentSecurityPendingListResult,
  AgentSecurityRestoreApplyParams,
  AgentSecurityRestoreApplyResult,
  AgentSecurityRestorePointsListParams,
  AgentSecurityRestoreRespondResult,
  AgentSecurityRespondParams,
  AgentSecurityRespondResult,
  AgentSecuritySummaryGetParams,
  AgentSecuritySummaryGetResult,
  AuditRecord,
  ApprovalDecision,
  ApprovalRequest,
  JsonRpcPage,
  RecoveryPoint,
  RequestMeta,
  AgentTaskDetailGetResult,
} from "@cialloclaw/protocol";
import { isRpcChannelUnavailable, logRpcMockFallback } from "@/rpc/fallback";
import {
  applySecurityRestoreDetailed,
  getSecuritySummaryDetailed,
  listSecurityAuditDetailed,
  listSecurityPendingDetailed,
  listSecurityRestorePointsDetailed,
  respondSecurityDetailed,
} from "@/rpc/methods";
import { loadTaskDetailData } from "../tasks/taskPage.service";

export type SecurityModuleSource = "rpc";

export type SecurityRpcContext = {
  serverTime: string | null;
  warnings: string[];
};

export type SecurityModuleData = {
  summary: AgentSecuritySummaryGetResult["summary"];
  pending: AgentSecurityPendingListResult["items"];
  pendingPage: JsonRpcPage;
  rpcContext: SecurityRpcContext;
  source: SecurityModuleSource;
};

export type SecurityPendingListData = {
  items: ApprovalRequest[];
  page: JsonRpcPage;
  rpcContext: SecurityRpcContext;
  source: SecurityModuleSource;
};

export type SecurityRespondOutcome = {
  response: AgentSecurityRespondResult;
  rpcContext: SecurityRpcContext;
};

export type SecurityRestorePointListData = {
  items: RecoveryPoint[];
  page: JsonRpcPage;
  rpcContext: SecurityRpcContext;
  source: SecurityModuleSource;
};

export type SecurityAuditRecordListData = {
  items: AuditRecord[];
  page: JsonRpcPage;
  rpcContext: SecurityRpcContext;
  source: SecurityModuleSource;
  taskId: string | null;
};

export type SecurityRestoreApplyOutcome = {
  response: AgentSecurityRestoreApplyResult;
  rpcContext: SecurityRpcContext;
};

export async function loadSecurityFocusedTaskDetail(taskId: string, source: SecurityModuleSource): Promise<AgentTaskDetailGetResult | null> {
  const normalizedTaskId = taskId.trim();
  if (!normalizedTaskId) {
    return null;
  }
  const detail = await loadTaskDetailData(normalizedTaskId, source);
  return detail.detail;
}

export function isSecurityApprovalRespondResult(
  response: AgentSecurityRespondResult,
): response is AgentSecurityApprovalRespondResult {
  return "authorization_record" in response;
}

export function isSecurityRestoreRespondResult(
  response: AgentSecurityRespondResult,
): response is AgentSecurityRestoreRespondResult {
  return "recovery_point" in response;
}

function createRequestMeta(): RequestMeta {
  return {
    trace_id: `trace_security_${Date.now()}`,
    client_time: new Date().toISOString(),
  };
}

export async function loadSecurityModuleData(_source: SecurityModuleSource = "rpc"): Promise<SecurityModuleData> {
  return loadSecurityModuleRpcData();
}

export async function loadSecurityModuleRpcData(): Promise<SecurityModuleData> {
  const summaryParams: AgentSecuritySummaryGetParams = {
    request_meta: createRequestMeta(),
  };

  const pendingParams: AgentSecurityPendingListParams = {
    request_meta: createRequestMeta(),
    limit: 20,
    offset: 0,
  };

  const [summaryResult, pendingResult] = await Promise.all([
    getSecuritySummaryDetailed(summaryParams),
    listSecurityPendingDetailed(pendingParams),
  ]);

  const serverTime = pendingResult.meta?.server_time ?? summaryResult.meta?.server_time ?? null;

  return {
    summary: summaryResult.data.summary,
    pending: pendingResult.data.items,
    pendingPage: pendingResult.data.page,
    rpcContext: {
      serverTime,
      warnings: [...summaryResult.warnings, ...pendingResult.warnings],
    },
    source: "rpc",
  };
}

export async function respondToApproval(
  approval: ApprovalRequest,
  decision: ApprovalDecision,
  rememberRule: boolean,
  _source: SecurityModuleSource,
): Promise<SecurityRespondOutcome> {
  const params: AgentSecurityRespondParams = {
    request_meta: createRequestMeta(),
    task_id: approval.task_id,
    approval_id: approval.approval_id,
    decision,
    remember_rule: rememberRule,
  };

  try {
    const response = await respondSecurityDetailed(params);

    return {
      response: response.data,
      rpcContext: {
        serverTime: response.meta?.server_time ?? null,
        warnings: response.warnings,
      },
    };
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      logRpcMockFallback("security approval response blocked", error);
      throw new Error("当前服务不可用，安全审批未提交。请恢复连接后重试。");
    }

    throw error;
  }
}

export async function loadSecurityPendingApprovals(
  _source: SecurityModuleSource,
  options?: {
    limit?: number;
    offset?: number;
  },
): Promise<SecurityPendingListData> {
  const limit = options?.limit ?? 20;
  const offset = options?.offset ?? 0;

  const params: AgentSecurityPendingListParams = {
    request_meta: createRequestMeta(),
    limit,
    offset,
  };
  const response = await listSecurityPendingDetailed(params);

  return {
    items: response.data.items,
    page: response.data.page,
    rpcContext: {
      serverTime: response.meta?.server_time ?? null,
      warnings: response.warnings,
    },
    source: "rpc",
  };
}

export async function loadSecurityRestorePoints(
  _source: SecurityModuleSource,
  options?: {
    limit?: number;
    offset?: number;
    taskId?: string | null;
  },
): Promise<SecurityRestorePointListData> {
  const limit = options?.limit ?? 20;
  const offset = options?.offset ?? 0;
  const taskId = options?.taskId?.trim() || undefined;

  const params: AgentSecurityRestorePointsListParams = {
    request_meta: createRequestMeta(),
    limit,
    offset,
    ...(taskId ? { task_id: taskId } : {}),
  };
  const response = await listSecurityRestorePointsDetailed(params);

  return {
    items: response.data.items,
    page: response.data.page,
    rpcContext: {
      serverTime: response.meta?.server_time ?? null,
      warnings: response.warnings,
    },
    source: "rpc",
  };
}

export async function loadSecurityAuditRecords(
  _source: SecurityModuleSource,
  taskId?: string | null,
  options?: {
    limit?: number;
    offset?: number;
  },
): Promise<SecurityAuditRecordListData> {
  const limit = options?.limit ?? 20;
  const offset = options?.offset ?? 0;
  const normalizedTaskId = taskId?.trim() || null;

  if (!normalizedTaskId) {
    throw new Error("当前没有任务上下文，无法查看安全审计记录。");
  }

  const params: AgentSecurityAuditListParams = {
    request_meta: createRequestMeta(),
    task_id: normalizedTaskId,
    limit,
    offset,
  };
  const response = await listSecurityAuditDetailed(params);

  return {
    items: response.data.items,
    page: response.data.page,
    rpcContext: {
      serverTime: response.meta?.server_time ?? null,
      warnings: response.warnings,
    },
    source: "rpc",
    taskId: normalizedTaskId,
  };
}

export async function applySecurityRestorePoint(
  restorePoint: RecoveryPoint,
  _source: SecurityModuleSource,
): Promise<SecurityRestoreApplyOutcome> {
  const params: AgentSecurityRestoreApplyParams = {
    request_meta: createRequestMeta(),
    task_id: restorePoint.task_id,
    recovery_point_id: restorePoint.recovery_point_id,
  };

  try {
    const response = await applySecurityRestoreDetailed(params);

    return {
      response: response.data,
      rpcContext: {
        serverTime: response.meta?.server_time ?? null,
        warnings: response.warnings,
      },
    };
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      logRpcMockFallback("security restore apply blocked", error);
      throw new Error("当前服务不可用，恢复点申请未提交。请恢复连接后重试。");
    }

    throw error;
  }
}
