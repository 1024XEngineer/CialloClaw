import type { AgentMirrorOverviewGetResult, ApprovalRequest, RecoveryPoint, Task, TokenCostSummary } from "@cialloclaw/protocol";

type MirrorOverviewMockSupportContext = {
  finished_tasks: Task[];
  unfinished_tasks: Task[];
  pending_approvals: ApprovalRequest[];
  latest_restore_point: RecoveryPoint | null;
  latest_restore_point_summary: string | null;
  security_status: string | null;
  token_cost_summary: TokenCostSummary | null;
};

export type MirrorOverviewMockData = {
  overview: AgentMirrorOverviewGetResult;
  supportContext: MirrorOverviewMockSupportContext;
};

// The mirror page keeps this fixture separate so daily reports and user
// profiles can stay predictable in roadshow/mock mode without coupling the
// view model to any one rendering path.
function createMockTask(task: Task): Task {
  return { ...task };
}
function createMockApprovalRequest(approval: ApprovalRequest): ApprovalRequest {
  return { ...approval };
}
function createMockRecoveryPoint(recoveryPoint: RecoveryPoint | null): RecoveryPoint | null {
  return recoveryPoint ? { ...recoveryPoint, objects: [...recoveryPoint.objects] } : null;
}

function createMockTokenCostSummary(summary: TokenCostSummary | null): TokenCostSummary | null {
  return summary ? { ...summary } : null;
}

const MOCK_MIRROR_OVERVIEW: AgentMirrorOverviewGetResult = {
  history_summary: [
    "示例历史记录强调最近结果会同时呈现结构化结论、步骤和风险说明。",
    "示例任务记录强调同一工作区的延续任务会沿用既有上下文线索。",
    "示例安全记录强调在权限或恢复点场景里会先出现确认说明。",
  ],
  daily_summary: {
    date: "2026-04-13",
    completed_tasks: 4,
    generated_outputs: 9,
  },
  profile: {
    work_style: "协议对齐优先，再进入实现。",
    preferred_output: "结果页 + 结构化摘要 + 验证说明",
    active_hours: "09:30 - 19:15",
  },
  memory_references: [
    {
      memory_id: "mem_contract_tailwind_desktop",
      reason: "示例引用记录：桌面前端继续沿用 task-centric 和既有镜子页外壳。",
      summary: "新增内容继续接入现有 MirrorApp 与可拖动卡片布局，不新增独立路由。",
    },
    {
      memory_id: "mem_delivery_preference_structured",
      reason: "示例引用记录：近期结果页通常采用分区清晰的结构化布局。",
      summary: "内容拆分为历史、日报、画像和本地对话记录等区域。",
    },
    {
      memory_id: "mem_guardrail_confirmation_first",
      reason: "示例引用记录：存在权限、恢复点或覆盖风险时，会先显示确认链路。",
      summary: "镜子页会并列显示今日任务结果、等待确认项和记忆引用。",
    },
  ],
};

const MOCK_MIRROR_SUPPORT_CONTEXT: MirrorOverviewMockSupportContext = {
  finished_tasks: [
    createMockTask({
      task_id: "task_mirror_daily_001",
      session_id: null,
      title: "整理镜子日报展示",
      source_type: "todo",
      status: "completed",
      intent: null,
      current_step: "已完成",
      risk_level: "green",
      started_at: "2026-04-13T08:54:00+08:00",
      updated_at: "2026-04-13T09:34:00+08:00",
      finished_at: "2026-04-13T09:34:00+08:00",
    }),
    createMockTask({
      task_id: "task_mirror_daily_002",
      session_id: null,
      title: "补齐用户画像卡片",
      source_type: "todo",
      status: "completed",
      intent: null,
      current_step: "已完成",
      risk_level: "green",
      started_at: "2026-04-13T09:40:00+08:00",
      updated_at: "2026-04-13T10:12:00+08:00",
      finished_at: "2026-04-13T10:12:00+08:00",
    }),
    createMockTask({
      task_id: "task_mirror_daily_003",
      session_id: null,
      title: "导出路演截图",
      source_type: "screen_capture",
      status: "completed",
      intent: null,
      current_step: "已完成",
      risk_level: "green",
      started_at: "2026-04-13T10:18:00+08:00",
      updated_at: "2026-04-13T11:07:00+08:00",
      finished_at: "2026-04-13T11:07:00+08:00",
    }),
    createMockTask({
      task_id: "task_mirror_daily_004",
      session_id: null,
      title: "补齐最终交付文案",
      source_type: "todo",
      status: "completed",
      intent: null,
      current_step: "已完成",
      risk_level: "green",
      started_at: "2026-04-13T11:10:00+08:00",
      updated_at: "2026-04-13T11:35:00+08:00",
      finished_at: "2026-04-13T11:35:00+08:00",
    }),
  ],
  unfinished_tasks: [
    createMockTask({
      task_id: "task_mirror_daily_005",
      session_id: null,
      title: "等待安全确认",
      source_type: "todo",
      status: "waiting_auth",
      intent: null,
      current_step: "等待授权",
      risk_level: "yellow",
      started_at: "2026-04-13T11:42:00+08:00",
      updated_at: "2026-04-13T11:45:00+08:00",
      finished_at: null,
    }),
    createMockTask({
      task_id: "task_mirror_daily_006",
      session_id: null,
      title: "整理最终展示稿",
      source_type: "todo",
      status: "processing",
      intent: null,
      current_step: "生成日报草稿",
      risk_level: "green",
      started_at: "2026-04-13T11:46:00+08:00",
      updated_at: "2026-04-13T11:48:00+08:00",
      finished_at: null,
    }),
  ],
  pending_approvals: [
    createMockApprovalRequest({
      approval_id: "approval_mirror_daily_001",
      task_id: "task_mirror_daily_005",
      operation_name: "写入路演材料",
      risk_level: "yellow",
      target_object: "workspace",
      reason: "需要把 mock 日报落盘到展示稿。",
      status: "pending",
      created_at: "2026-04-13T11:43:00+08:00",
    }),
  ],
  latest_restore_point: createMockRecoveryPoint({
    recovery_point_id: "rp_mirror_daily_001",
    task_id: "task_mirror_daily_005",
    summary: "在写入展示稿前保留恢复点。",
    created_at: "2026-04-13T11:43:30+08:00",
    objects: ["workspace/mirror-demo.md"],
  }),
  latest_restore_point_summary: "在写入展示稿前保留恢复点。",
  security_status: "pending_confirmation",
  token_cost_summary: createMockTokenCostSummary({
    current_task_tokens: 1_200,
    current_task_cost: 0.24,
    today_tokens: 9_200,
    today_cost: 1.62,
    single_task_limit: 50_000,
    daily_limit: 300_000,
    budget_auto_downgrade: true,
  }),
};

export function createMirrorOverviewMockData(): MirrorOverviewMockData {
  return {
    overview: {
      history_summary: [...MOCK_MIRROR_OVERVIEW.history_summary],
      daily_summary: MOCK_MIRROR_OVERVIEW.daily_summary ? { ...MOCK_MIRROR_OVERVIEW.daily_summary } : null,
      profile: MOCK_MIRROR_OVERVIEW.profile ? { ...MOCK_MIRROR_OVERVIEW.profile } : null,
      memory_references: MOCK_MIRROR_OVERVIEW.memory_references.map((reference) => ({ ...reference })),
    },
    supportContext: {
      finished_tasks: MOCK_MIRROR_SUPPORT_CONTEXT.finished_tasks.map(createMockTask),
      unfinished_tasks: MOCK_MIRROR_SUPPORT_CONTEXT.unfinished_tasks.map(createMockTask),
      pending_approvals: MOCK_MIRROR_SUPPORT_CONTEXT.pending_approvals.map(createMockApprovalRequest),
      latest_restore_point: createMockRecoveryPoint(MOCK_MIRROR_SUPPORT_CONTEXT.latest_restore_point),
      latest_restore_point_summary: MOCK_MIRROR_SUPPORT_CONTEXT.latest_restore_point_summary,
      security_status: MOCK_MIRROR_SUPPORT_CONTEXT.security_status,
      token_cost_summary: createMockTokenCostSummary(MOCK_MIRROR_SUPPORT_CONTEXT.token_cost_summary),
    },
  };
}
/**
 * Fills the nullable mirror overview fields from the roadshow fixture while
 * leaving the live history summary and reference list untouched.
 */
export function mergeMirrorOverviewWithMockDefaults(
  overview: AgentMirrorOverviewGetResult,
): AgentMirrorOverviewGetResult {
  return {
    history_summary: [...overview.history_summary],
    daily_summary: overview.daily_summary
      ? { ...overview.daily_summary }
      : MOCK_MIRROR_OVERVIEW.daily_summary
        ? { ...MOCK_MIRROR_OVERVIEW.daily_summary }
        : null,
    profile: overview.profile
      ? { ...overview.profile }
      : MOCK_MIRROR_OVERVIEW.profile
        ? { ...MOCK_MIRROR_OVERVIEW.profile }
        : null,
    memory_references: overview.memory_references.map((reference) => ({ ...reference })),
  };
}
