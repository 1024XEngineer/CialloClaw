import type {
  AgentDashboardModuleGetResult,
  AgentDashboardOverviewGetResult,
  AgentDeliveryOpenResult,
  AgentInputSubmitResult,
  AgentMirrorOverviewGetResult,
  AgentRecommendationFeedbackSubmitResult,
  AgentRecommendationGetResult,
  AgentSettingsUpdateResult,
  AgentSecurityAuditListResult,
  AgentSecurityPendingListResult,
  AgentSecurityRespondResult,
  AgentSecurityRestoreApplyResult,
  AgentSecurityRestorePointsListResult,
  AgentSecuritySummaryGetResult,
  AgentTaskArtifactListResult,
  AgentTaskArtifactOpenResult,
  AgentTaskEventsListResult,
  AgentTaskToolCallsListResult,
  AgentTaskStartResult,
  AgentTaskSteerResult,
  AuditRecord,
  DeliveryPayload,
  DeliveryResult,
  RecoveryPoint,
  Task,
  TokenCostSummary,
} from "@cialloclaw/protocol";
import { loadSettings } from "@/services/settingsService";

function createDetailedResponse<T>(data: T): {
  data: T;
  meta: {
    server_time: string;
  };
  warnings: string[];
} {
  return {
    data,
    meta: {
      server_time: new Date().toISOString(),
    },
    warnings: [],
  };
}

function createTask(status: Task["status"], currentStep: string): Task {
  const now = new Date().toISOString();

  return {
    task_id: "task_stub",
    title: "stub task",
    source_type: "hover_input",
    status,
    intent: null,
    current_step: currentStep,
    risk_level: status === "waiting_auth" ? "red" : "yellow",
    started_at: now,
    updated_at: now,
    finished_at: status === "completed" ? now : null,
  };
}

function createTokenCostSummary(): TokenCostSummary {
  return {
    current_task_tokens: 1200,
    current_task_cost: 0.24,
    today_tokens: 9200,
    today_cost: 1.62,
    single_task_limit: 50000,
    daily_limit: 300000,
    budget_auto_downgrade: true,
  };
}

function createRecoveryPoint(): RecoveryPoint {
  return {
    recovery_point_id: "rp_stub",
    task_id: "task_stub",
    summary: "write_file_before_change",
    created_at: new Date().toISOString(),
    objects: ["workspace/stub.txt"],
  };
}

function createRecoveryPointForTask(taskId: string): RecoveryPoint {
  return {
    ...createRecoveryPoint(),
    recovery_point_id: `rp_${taskId}`,
    task_id: taskId,
  };
}

function createAuditRecord(result: AuditRecord["result"] = "success"): AuditRecord {
  return {
    audit_id: `audit_stub_${result}`,
    task_id: "task_stub",
    type: "recovery",
    action: "restore_apply",
    summary: result === "success" ? "Recovered one workspace object from the latest restore point." : "Restore execution failed.",
    target: "workspace/stub.txt",
    result,
    created_at: new Date().toISOString(),
  };
}

function createAuditRecordForTask(taskId: string, result: AuditRecord["result"] = "success"): AuditRecord {
  return {
    ...createAuditRecord(result),
    audit_id: `audit_${taskId}_${result}`,
    task_id: taskId,
    target: `workspace/${taskId}.txt`,
  };
}

function createResolvedPayload(): DeliveryPayload {
  return {
    path: null,
    task_id: "task_stub",
    url: null,
  };
}

function createTaskDeliveryResult(): DeliveryResult {
  return {
    type: "task_detail",
    title: "Task detail",
    preview_text: "Open the task detail view.",
    payload: createResolvedPayload(),
  };
}

function createSecuritySummary(): AgentSecuritySummaryGetResult {
  return {
    summary: {
      security_status: "normal",
      pending_authorizations: 0,
      latest_restore_point: createRecoveryPoint(),
      token_cost_summary: createTokenCostSummary(),
    },
  };
}

function createSecurityPendingList(): AgentSecurityPendingListResult {
  return {
    items: [],
    page: { limit: 20, offset: 0, total: 0, has_more: false },
  };
}

function readStringParam(params: unknown, key: "approval_id" | "task_id"): string | null {
  if (!params || typeof params !== "object") {
    return null;
  }

  const value = (params as Record<string, unknown>)[key];
  return typeof value === "string" ? value : null;
}

function createSecurityApprovalRespondResult(taskId = "task_stub", approvalId = "approval_stub"): AgentSecurityRespondResult {
  return {
    authorization_record: {
      authorization_record_id: "auth_stub",
      task_id: taskId,
      approval_id: approvalId,
      decision: "allow_once",
      remember_rule: false,
      operator: "test-stub",
      created_at: new Date().toISOString(),
    },
    bubble_message: {
      bubble_id: "bubble_stub",
      task_id: taskId,
      type: "status",
      text: "The approval was accepted for this run.",
      pinned: false,
      hidden: false,
      created_at: new Date().toISOString(),
    },
    impact_scope: {
      apps: [],
      files: [],
      webpages: [],
      out_of_workspace: false,
      overwrite_or_delete_risk: false,
    },
    task: {
      ...createTask("processing", "security"),
      task_id: taskId,
    },
  };
}

function createSecurityRestoreRespondResult(taskId = "task_stub"): AgentSecurityRespondResult {
  return {
    applied: true,
    task: {
      ...createTask("completed", "restore_apply"),
      task_id: taskId,
    },
    recovery_point: createRecoveryPointForTask(taskId),
    audit_record: createAuditRecordForTask(taskId),
    bubble_message: {
      bubble_id: "bubble_restore_stub",
      task_id: taskId,
      type: "status",
      text: `Restored the workspace state for ${taskId}.`,
      pinned: false,
      hidden: false,
      created_at: new Date().toISOString(),
    },
  };
}

function createSecurityRestorePoints(): AgentSecurityRestorePointsListResult {
  return {
    items: [createRecoveryPoint()],
    page: {
      limit: 20,
      offset: 0,
      total: 1,
      has_more: false,
    },
  };
}

function createSecurityAuditList(): AgentSecurityAuditListResult {
  return {
    items: [createAuditRecord()],
    page: {
      limit: 20,
      offset: 0,
      total: 1,
      has_more: false,
    },
  };
}

function createSecurityRestoreApplyResult(): AgentSecurityRestoreApplyResult {
  return {
    applied: false,
    task: createTask("waiting_auth", "restore_point_approval"),
    recovery_point: createRecoveryPoint(),
    audit_record: null,
    bubble_message: {
      bubble_id: "bubble_restore_stub",
      task_id: "task_stub",
      type: "status",
      text: "Restore requires approval before execution.",
      pinned: false,
      hidden: false,
      created_at: new Date().toISOString(),
    },
  };
}

function createMirrorOverview(): AgentMirrorOverviewGetResult {
  return {
    history_summary: [
      "Recent deliveries favor structured summaries.",
      "Risky actions still route through approval before execution.",
    ],
    daily_summary: {
      date: "2026-04-17",
      completed_tasks: 2,
      generated_outputs: 4,
    },
    profile: {
      work_style: "task-centric",
      preferred_output: "3-point summary",
      active_hours: "09:30-19:00",
    },
    memory_references: [
      {
        memory_id: "mem_stub_001",
        reason: "Continue the existing summary structure.",
        summary: "Keep a structured conclusion and explicit risk note.",
      },
    ],
  };
}

export async function listSecurityPending(_params?: unknown): Promise<AgentSecurityPendingListResult> {
  return createSecurityPendingList();
}

export async function respondSecurity(params?: unknown): Promise<AgentSecurityRespondResult> {
  const approvalId = readStringParam(params, "approval_id");
  const taskId = readStringParam(params, "task_id") ?? "task_stub";

  if (approvalId?.includes("restore")) {
    return createSecurityRestoreRespondResult(taskId);
  }

  return createSecurityApprovalRespondResult(taskId, approvalId ?? "approval_stub");
}

export async function getSecuritySummary(_params?: unknown): Promise<AgentSecuritySummaryGetResult> {
  return createSecuritySummary();
}

export async function getSecuritySummaryDetailed(_params?: unknown) {
  return createDetailedResponse(await getSecuritySummary());
}

export async function listSecurityPendingDetailed(_params?: unknown) {
  return createDetailedResponse(await listSecurityPending());
}

export async function respondSecurityDetailed(params?: unknown) {
  return createDetailedResponse(await respondSecurity(params));
}

export async function listSecurityRestorePoints(_params?: unknown): Promise<AgentSecurityRestorePointsListResult> {
  return createSecurityRestorePoints();
}

export async function listSecurityRestorePointsDetailed(_params?: unknown) {
  return createDetailedResponse(await listSecurityRestorePoints());
}

export async function applySecurityRestore(_params?: unknown): Promise<AgentSecurityRestoreApplyResult> {
  return createSecurityRestoreApplyResult();
}

export async function applySecurityRestoreDetailed(_params?: unknown) {
  return createDetailedResponse(await applySecurityRestore());
}

export async function listSecurityAudit(_params?: unknown): Promise<AgentSecurityAuditListResult> {
  return createSecurityAuditList();
}

export async function listSecurityAuditDetailed(_params?: unknown) {
  return createDetailedResponse(await listSecurityAudit());
}

export async function getMirrorOverview(_params?: unknown): Promise<AgentMirrorOverviewGetResult> {
  return createMirrorOverview();
}

export async function getMirrorOverviewDetailed(_params?: unknown) {
  return createDetailedResponse(await getMirrorOverview());
}

export async function submitInput(_params?: unknown): Promise<AgentInputSubmitResult> {
  return {
    task: createTask("processing", "submit_input"),
    bubble_message: null,
    delivery_result: null,
  };
}

export async function startTask(_params?: unknown): Promise<AgentTaskStartResult> {
  return {
    task: createTask("processing", "task_start"),
    bubble_message: null,
    delivery_result: null,
  };
}

export async function getDashboardOverview(_params?: unknown): Promise<AgentDashboardOverviewGetResult> {
  return {
    overview: {
      focus_summary: {
        task_id: "task_stub",
        title: "stub task",
        status: "processing",
        current_step: "overview",
        next_action: "Open the active task.",
        updated_at: new Date().toISOString(),
      },
      trust_summary: {
        risk_level: "yellow",
        pending_authorizations: 0,
        has_restore_point: true,
        workspace_path: "workspace",
      },
      quick_actions: ["Open dashboard"],
      global_state: {},
      high_value_signal: ["Continue the current task."],
    },
  };
}

export async function getDashboardModule(_params?: unknown): Promise<AgentDashboardModuleGetResult> {
  return {
    module: "home",
    tab: "summary",
    summary: {},
    highlights: ["Stub dashboard module"],
  };
}

export async function getRecommendations(_params?: unknown): Promise<AgentRecommendationGetResult> {
  return {
    cooldown_hit: false,
    items: [],
  };
}

export async function submitRecommendationFeedback(_params?: unknown): Promise<AgentRecommendationFeedbackSubmitResult> {
  return {
    applied: true,
  };
}

export async function listTaskEvents(_params?: unknown): Promise<AgentTaskEventsListResult> {
  return {
    items: [
      {
        event_id: "evt_stub_001",
        run_id: "run_stub",
        task_id: "task_stub",
        step_id: "step_stub",
        type: "loop.completed",
        level: "info",
        payload_json: JSON.stringify({ stop_reason: "completed" }),
        created_at: new Date().toISOString(),
      },
    ],
    page: {
      has_more: false,
      limit: 20,
      offset: 0,
      total: 1,
    },
  };
}

export async function listTaskToolCalls(_params?: unknown): Promise<AgentTaskToolCallsListResult> {
  return {
    items: [
      {
        tool_call_id: "tool_call_stub_001",
        run_id: "run_stub",
        task_id: "task_stub",
        step_id: null,
        tool_name: "read_file",
        status: "succeeded",
        input: {
          path: "notes/source.txt",
        },
        output: {
          path: "notes/source.txt",
          summary_output: {
            excerpt: "stub content",
          },
        },
        error_code: null,
        duration_ms: 12,
      },
    ],
    page: {
      has_more: false,
      limit: 20,
      offset: 0,
      total: 1,
    },
  };
}

export async function steerTask(_params?: unknown): Promise<AgentTaskSteerResult> {
  return {
    task: createTask("waiting_auth", "agent_loop"),
    bubble_message: {
      bubble_id: "bubble_steer_stub",
      task_id: "task_stub",
      type: "status",
      text: "The follow-up instruction was recorded for the active task.",
      pinned: false,
      hidden: false,
      created_at: new Date().toISOString(),
    },
  };
}

export async function listTaskArtifacts(_params?: unknown): Promise<AgentTaskArtifactListResult> {
  return {
    items: [],
    page: {
      has_more: false,
      limit: 50,
      offset: 0,
      total: 0,
    },
  };
}

export async function openTaskArtifact(_params?: unknown): Promise<AgentTaskArtifactOpenResult> {
  return {
    artifact: {
      artifact_id: "artifact_stub",
      artifact_type: "workspace_document",
      mime_type: "text/plain",
      path: "workspace/stub.txt",
      task_id: "task_stub",
      title: "stub.txt",
    },
    delivery_result: createTaskDeliveryResult(),
    open_action: "task_detail",
    resolved_payload: createResolvedPayload(),
  };
}

export async function openDelivery(_params?: unknown): Promise<AgentDeliveryOpenResult> {
  return {
    delivery_result: createTaskDeliveryResult(),
    open_action: "task_detail",
    resolved_payload: createResolvedPayload(),
  };
}

export async function getSettingsDetailed(_params?: unknown) {
  return createDetailedResponse(loadSettings());
}

export async function updateSettings(_params?: unknown): Promise<AgentSettingsUpdateResult> {
  const current = loadSettings();

  return {
    apply_mode: "immediate",
    effective_settings: {
      general: current.settings.general,
      floating_ball: current.settings.floating_ball,
      memory: current.settings.memory,
      task_automation: current.settings.task_automation,
      models: {
        provider: current.settings.models.provider,
        budget_auto_downgrade: current.settings.models.budget_auto_downgrade,
        provider_api_key_configured: current.settings.models.provider_api_key_configured,
        base_url: current.settings.models.base_url,
        model: current.settings.models.model,
        stronghold: current.settings.models.stronghold,
      },
    },
    need_restart: false,
    updated_keys: [],
  };
}
