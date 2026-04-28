import type {
  AgentSecurityAuditListParams,
  AgentSecurityAuditListResult,
  AgentSecurityApprovalRespondResult,
  AgentSecurityPendingListParams,
  AgentSecurityPendingListResult,
  AgentSecurityRestoreApplyParams,
  AgentSecurityRestoreApplyResult,
  AgentSecurityRestorePointsListParams,
  AgentSecurityRestoreRespondResult,
  AgentSecurityRestorePointsListResult,
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
import {
  buildMockRespondResult,
  buildMockRestoreApplyResult,
  securityAuditMock,
  securityPendingMock,
  securityRestorePointsMock,
  securitySummaryMock,
} from "./securityModuleMock";

export type SecurityModuleSource = "rpc" | "mock";

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
  const detail = await loadTaskDetailData(normalizedTaskId, source === "mock" ? "mock" : "rpc");
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

export function getInitialSecurityModuleData(): SecurityModuleData {
  return {
    summary: securitySummaryMock.summary,
    pending: securityPendingMock.items,
    pendingPage: securityPendingMock.page,
    rpcContext: {
      serverTime: null,
      warnings: [],
    },
    source: "mock",
  };
}

export async function loadSecurityModuleData(source: SecurityModuleSource = "rpc"): Promise<SecurityModuleData> {
  if (source === "mock") {
    return getInitialSecurityModuleData();
  }

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
  source: SecurityModuleSource,
): Promise<SecurityRespondOutcome> {
  const params: AgentSecurityRespondParams = {
    request_meta: createRequestMeta(),
    task_id: approval.task_id,
    approval_id: approval.approval_id,
    decision,
    remember_rule: rememberRule,
  };

  if (source === "mock") {
    return {
      response: buildMockRespondResult(approval.approval_id, approval.task_id, decision, rememberRule),
      rpcContext: {
        serverTime: null,
        warnings: [],
      },
    };
  }

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
      throw new Error("JSON-RPC 当前不可用，安全审批未提交。请恢复连接后重试。");
    }

    throw error;
  }
}

export async function loadSecurityPendingApprovals(
  source: SecurityModuleSource,
  options?: {
    limit?: number;
    offset?: number;
  },
): Promise<SecurityPendingListData> {
  const limit = options?.limit ?? 20;
  const offset = options?.offset ?? 0;

  if (source === "mock") {
    const pagedItems = securityPendingMock.items.slice(offset, offset + limit);

    return {
      items: pagedItems,
      page: {
        has_more: offset + pagedItems.length < securityPendingMock.items.length,
        limit,
        offset,
        total: securityPendingMock.items.length,
      },
      rpcContext: {
        serverTime: null,
        warnings: [],
      },
      source: "mock",
    };
  }

  try {
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
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      logRpcMockFallback("security pending list", error);
      const pagedItems = securityPendingMock.items.slice(offset, offset + limit);

      return {
        items: pagedItems,
        page: {
          has_more: offset + pagedItems.length < securityPendingMock.items.length,
          limit,
          offset,
          total: securityPendingMock.items.length,
        },
        rpcContext: {
          serverTime: null,
          warnings: [],
        },
        source: "mock",
      };
    }

    throw error;
  }
}

export async function loadSecurityRestorePoints(
  source: SecurityModuleSource,
  options?: {
    limit?: number;
    offset?: number;
    taskId?: string | null;
  },
): Promise<SecurityRestorePointListData> {
  const limit = options?.limit ?? 20;
  const offset = options?.offset ?? 0;
  const taskId = options?.taskId?.trim() || undefined;

  if (source === "mock") {
    const filteredItems = securityRestorePointsMock.items.filter((item) => !taskId || item.task_id === taskId);
    const pagedItems = filteredItems.slice(offset, offset + limit);

    return {
      items: pagedItems,
      page: {
        has_more: offset + pagedItems.length < filteredItems.length,
        limit,
        offset,
        total: filteredItems.length,
      },
      rpcContext: {
        serverTime: null,
        warnings: [],
      },
      source: "mock",
    };
  }

  try {
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
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      logRpcMockFallback("security restore points", error);
      const filteredItems = securityRestorePointsMock.items.filter((item) => !taskId || item.task_id === taskId);
      const pagedItems = filteredItems.slice(offset, offset + limit);

      return {
        items: pagedItems,
        page: {
          has_more: offset + pagedItems.length < filteredItems.length,
          limit,
          offset,
          total: filteredItems.length,
        },
        rpcContext: {
          serverTime: null,
          warnings: [],
        },
        source: "mock",
      };
    }

    throw error;
  }
}

export async function loadSecurityAuditRecords(
  source: SecurityModuleSource,
  taskId?: string | null,
  options?: {
    limit?: number;
    offset?: number;
  },
): Promise<SecurityAuditRecordListData> {
  const limit = options?.limit ?? 20;
  const offset = options?.offset ?? 0;
  const normalizedTaskId = taskId?.trim() || null;

  if (source === "mock") {
    const filteredItems = normalizedTaskId
      ? securityAuditMock.items.filter((item) => item.task_id === normalizedTaskId)
      : securityAuditMock.items;
    const pagedItems = filteredItems.slice(offset, offset + limit);

    return {
      items: pagedItems,
      page: {
        has_more: offset + pagedItems.length < filteredItems.length,
        limit,
        offset,
        total: filteredItems.length,
      },
      rpcContext: {
        serverTime: null,
        warnings: [],
      },
      source: "mock",
      taskId: normalizedTaskId,
    };
  }

  try {
    if (!normalizedTaskId) {
      throw new Error("Security audit list requires task context in RPC mode.");
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
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      logRpcMockFallback("security audit list", error);
      const filteredItems = normalizedTaskId
        ? securityAuditMock.items.filter((item) => item.task_id === normalizedTaskId)
        : securityAuditMock.items;
      const pagedItems = filteredItems.slice(offset, offset + limit);

      return {
        items: pagedItems,
        page: {
          has_more: offset + pagedItems.length < filteredItems.length,
          limit,
          offset,
          total: filteredItems.length,
        },
        rpcContext: {
          serverTime: null,
          warnings: [],
        },
        source: "mock",
        taskId: normalizedTaskId,
      };
    }

    throw error;
  }
}

export async function applySecurityRestorePoint(
  restorePoint: RecoveryPoint,
  source: SecurityModuleSource,
): Promise<SecurityRestoreApplyOutcome> {
  const params: AgentSecurityRestoreApplyParams = {
    request_meta: createRequestMeta(),
    task_id: restorePoint.task_id,
    recovery_point_id: restorePoint.recovery_point_id,
  };

  if (source === "mock") {
    return {
      response: buildMockRestoreApplyResult(restorePoint.recovery_point_id, restorePoint.task_id),
      rpcContext: {
        serverTime: null,
        warnings: [],
      },
    };
  }

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
      throw new Error("JSON-RPC 当前不可用，恢复点申请未提交。请恢复连接后重试。");
    }

    throw error;
  }
}
