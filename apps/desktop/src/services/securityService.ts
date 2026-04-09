import type {
  AgentSecurityPendingListParams,
  AgentSecurityPendingListResult,
  AgentSecurityRespondParams,
  AgentSecurityRespondResult,
  AgentSecuritySummaryGetParams,
  AgentSecuritySummaryGetResult,
  ApprovalDecision,
  ApprovalRequest,
  RequestMeta,
} from "@cialloclaw/protocol";
import { listSecurityPending, respondSecurity, getSecuritySummary } from "@/rpc/methods";
import { buildMockRespondResult, securityPendingMock, securitySummaryMock } from "@/mocks/securityModuleMock";

export type SecurityModuleSource = "rpc" | "mock";

export type SecurityModuleData = {
  summary: AgentSecuritySummaryGetResult["summary"];
  pending: AgentSecurityPendingListResult["items"];
  source: SecurityModuleSource;
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
    getSecuritySummary(summaryParams),
    listSecurityPending(pendingParams),
  ]);

  return {
    summary: summaryResult.summary,
    pending: pendingResult.items,
    source: "rpc",
  };
}

export async function respondToApproval(
  approval: ApprovalRequest,
  decision: ApprovalDecision,
  source: SecurityModuleSource,
): Promise<AgentSecurityRespondResult> {
  const params: AgentSecurityRespondParams = {
    request_meta: createRequestMeta(),
    task_id: approval.task_id,
    approval_id: approval.approval_id,
    decision,
  };

  if (source === "mock") {
    return buildMockRespondResult(approval.approval_id, approval.task_id, decision);
  }

  return respondSecurity(params);
}
