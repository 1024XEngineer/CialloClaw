import type {
  AgentSecurityPendingListParams,
  AgentSecurityPendingListResult,
  AgentSecurityRespondParams,
  AgentSecurityRespondResult,
  AgentSecuritySummaryGetParams,
  AgentSecuritySummaryGetResult,
  ApprovalDecision,
  ApprovalRequest,
  JsonRpcPage,
  RequestMeta,
} from "@cialloclaw/protocol";
import { getSecuritySummaryDetailed, listSecurityPendingDetailed, respondSecurityDetailed } from "@/rpc/methods";
import { buildMockRespondResult, securityPendingMock, securitySummaryMock } from "./securityModuleMock";

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

export type SecurityRespondOutcome = {
  response: AgentSecurityRespondResult;
  rpcContext: SecurityRpcContext;
};

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

export async function loadSecurityModuleData(): Promise<SecurityModuleData> {
  try {
    return await loadSecurityModuleRpcData();
  } catch (error) {
    console.warn("Security module RPC unavailable, using local mock fallback.", error);
    return getInitialSecurityModuleData();
  }
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

  const response = await respondSecurityDetailed(params);

  return {
    response: response.data,
    rpcContext: {
      serverTime: response.meta?.server_time ?? null,
      warnings: response.warnings,
    },
  };
}
