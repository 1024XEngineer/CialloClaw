import type {
  AgentSecurityAuditListResult,
  AgentSecurityPendingListResult,
  AgentSecurityRestoreApplyResult,
  AgentSecurityRestorePointsListResult,
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

export const securityRestorePointsMock: AgentSecurityRestorePointsListResult = {
  items: [
    securitySummaryMock.summary.latest_restore_point!,
    {
      recovery_point_id: "rp_mock_000",
      task_id: "task_security_mock_001",
      summary: "进入批量改写前的工作区快照",
      created_at: "2026-04-09T08:56:00+08:00",
      objects: ["D:/CialloClawWorkspace/project-alpha/src", "VS Code", "PowerShell"],
    },
    {
      recovery_point_id: "rp_mock_002",
      task_id: "task_security_mock_002",
      summary: "浏览器自动化执行前的沙盒恢复点",
      created_at: "2026-04-09T09:18:00+08:00",
      objects: ["docker://browser-runner", "Chrome", "reports/playwright"],
    },
  ],
  page: {
    limit: 3,
    offset: 0,
    total: 3,
    has_more: false,
  },
};

export const securityAuditMock: AgentSecurityAuditListResult = {
  items: [
    {
      audit_id: "audit_mock_001",
      task_id: "task_security_mock_001",
      type: "file",
      action: "write_file",
      summary: "改写了任务工作区中的入口文件并更新依赖清单。",
      target: "D:/CialloClawWorkspace/project-alpha/src/App.tsx",
      result: "success",
      created_at: "2026-04-09T09:25:00+08:00",
    },
    {
      audit_id: "audit_mock_002",
      task_id: "task_security_mock_001",
      type: "command",
      action: "run_command",
      summary: "执行了构建命令并收集标准输出。",
      target: "pnpm build",
      result: "success",
      created_at: "2026-04-09T09:24:00+08:00",
    },
    {
      audit_id: "audit_mock_003",
      task_id: "task_security_mock_001",
      type: "system",
      action: "create_recovery_point",
      summary: "在高风险改写前创建了恢复点。",
      target: "rp_mock_001",
      result: "success",
      created_at: "2026-04-09T09:20:00+08:00",
    },
  ],
  page: {
    limit: 3,
    offset: 0,
    total: 3,
    has_more: false,
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
      session_id: null,
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

export function buildMockRestoreApplyResult(
  recoveryPointId: string,
  taskId: string,
): AgentSecurityRestoreApplyResult {
  const recoveryPoint =
    securityRestorePointsMock.items.find((item) => item.recovery_point_id === recoveryPointId) ??
    securityRestorePointsMock.items[0];

  return {
    applied: false,
    task: {
      task_id: taskId,
      session_id: null,
      title: "恢复点回滚联调任务",
      source_type: "dragged_file",
      status: "waiting_auth",
      intent: null,
      updated_at: new Date().toISOString(),
      current_step: "restore_point_approval",
      risk_level: "red",
      started_at: new Date().toISOString(),
      finished_at: null,
    },
    recovery_point: {
      ...recoveryPoint,
      task_id: taskId,
    },
    audit_record: null,
    bubble_message: {
      bubble_id: `bubble_restore_${recoveryPointId}`,
      task_id: taskId,
      type: "status",
      text: "恢复操作已进入授权确认，处理完待确认项后会继续执行。",
      pinned: false,
      hidden: false,
      created_at: new Date().toISOString(),
    },
  };
}
