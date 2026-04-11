import type {
  AgentSecurityPendingListResult,
  AgentSecuritySummaryGetResult,
  ApprovalDecision,
  AgentSecurityRespondResult,
} from "@cialloclaw/protocol";

export const securitySummaryMock: AgentSecuritySummaryGetResult = {
  summary: {
    security_status: "pending_confirmation",
    pending_authorizations: 5,
    latest_restore_point: {
      recovery_point_id: "rp_mock_001",
      task_id: "task_security_mock_001",
      summary: "进入安全模块前创建的桌面恢复点",
      created_at: "2026-04-09T09:20:00+08:00",
      objects: ["D:/CialloClawWorkspace", "Chrome", "Docker sandbox"],
    },
    token_cost_summary: {
      current_task_tokens: 18420,
      current_task_cost: 1.86,
      today_tokens: 98214,
      today_cost: 8.42,
      single_task_limit: 50000,
      daily_limit: 300000,
      budget_auto_downgrade: true,
    },
  },
};

export const securityPendingMock: AgentSecurityPendingListResult = {
  items: [
    {
      approval_id: "approval_mock_001",
      task_id: "task_security_mock_001",
      operation_name: "批量改写工作区文件",
      risk_level: "red",
      target_object: "D:/CialloClawWorkspace/project-alpha",
      reason: "涉及 18 个文件的覆盖写入，并超出当前工作区建议范围。",
      status: "pending",
      created_at: "2026-04-09T09:22:00+08:00",
    },
    {
      approval_id: "approval_mock_002",
      task_id: "task_security_mock_002",
      operation_name: "连接 Docker 沙盒执行浏览器自动化",
      risk_level: "yellow",
      target_object: "docker://browser-runner",
      reason: "需要在隔离环境中执行 Playwright 测试并回传产物。",
      status: "pending",
      created_at: "2026-04-09T09:26:00+08:00",
    },
  ],
  page: {
    limit: 2,
    offset: 0,
    total: 5,
    has_more: true,
  },
};

export function buildMockRespondResult(
  approvalId: string,
  taskId: string,
  decision: ApprovalDecision,
  rememberRule: boolean,
): AgentSecurityRespondResult {
  return {
    authorization_record: {
      authorization_record_id: `auth_${approvalId}`,
      task_id: taskId,
      approval_id: approvalId,
      decision,
      remember_rule: rememberRule,
      operator: "frontend-mock",
      created_at: new Date().toISOString(),
    },
    task: {
      task_id: taskId,
      title: "安全审批联调任务",
      source_type: "dragged_file",
      status: decision === "allow_once" ? "processing" : "paused",
      intent: null,
      updated_at: new Date().toISOString(),
      current_step: "security_approval",
      risk_level: decision === "allow_once" ? "yellow" : "green",
      started_at: new Date().toISOString(),
      finished_at: null,
    },
    bubble_message: {
      bubble_id: `bubble_${approvalId}`,
      task_id: taskId,
      type: decision === "allow_once" ? "result" : "status",
      text: decision === "allow_once" ? "已在前端联调模式下放行该审批。" : "已在前端联调模式下拒绝该审批。",
      pinned: rememberRule,
      hidden: false,
      created_at: new Date().toISOString(),
    },
    impact_scope: {
      files: [
        "D:/CialloClawWorkspace/project-alpha/src",
        "D:/CialloClawWorkspace/project-alpha/package.json",
      ],
      webpages: ["local://security-review", "https://ci.example.dev/build/451"],
      apps: ["VS Code", "Docker Desktop", "PowerShell"],
      out_of_workspace: true,
      overwrite_or_delete_risk: decision === "allow_once",
    },
  };
}
