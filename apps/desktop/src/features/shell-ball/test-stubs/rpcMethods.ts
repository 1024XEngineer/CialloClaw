import type {
  AgentDashboardModuleGetResult,
  AgentDashboardOverviewGetResult,
  AgentDeliveryOpenResult,
  AgentInputSubmitResult,
  AgentMirrorOverviewGetResult,
  AgentRecommendationFeedbackSubmitResult,
  AgentRecommendationGetResult,
  AgentTaskConfirmResult,
  AgentTaskControlResult,
  AgentTaskDetailGetResult,
  AgentTaskListResult,
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
  Citation,
  DeliveryPayload,
  DeliveryResult,
  RecoveryPoint,
  Task,
  TaskRuntimeSummary,
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
    session_id: null,
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

function createRuntimeSummary(): TaskRuntimeSummary {
  return {
    active_steering_count: 1,
    events_count: 4,
    latest_event_type: "loop.completed",
    latest_failure_code: null,
    latest_failure_category: null,
    latest_failure_summary: null,
    loop_stop_reason: "completed",
    observation_signals: ["screen_capture"],
  };
}

function createCitations(): Citation[] {
  return [
    {
      citation_id: "citation_stub_001",
      task_id: "task_stub",
      run_id: "run_stub",
      source_type: "file",
      source_ref: "workspace/stub.txt",
      label: "Stub workspace file",
      artifact_id: "artifact_stub",
      artifact_type: "workspace_document",
      evidence_role: "supporting_material",
      excerpt_text: "stub content",
      screen_session_id: null,
    },
  ];
}

function createTaskListResult(): AgentTaskListResult {
  return {
    items: [createTask("processing", "collect_input")],
    page: {
      has_more: false,
      limit: 20,
      offset: 0,
      total: 1,
    },
  };
}

function createTaskDetailResult(): AgentTaskDetailGetResult {
  return {
    approval_request: null,
    audit_record: createAuditRecord(),
    artifacts: [
      {
        artifact_id: "artifact_stub",
        artifact_type: "workspace_document",
        mime_type: "text/plain",
        path: "workspace/stub.txt",
        task_id: "task_stub",
        title: "stub.txt",
      },
    ],
    authorization_record: {
      authorization_record_id: "auth_stub",
      task_id: "task_stub",
      approval_id: "approval_stub",
      decision: "allow_once",
      remember_rule: false,
      operator: "test-stub",
      created_at: new Date().toISOString(),
    },
    citations: createCitations(),
    delivery_result: createTaskDeliveryResult(),
    mirror_references: [
      {
        memory_id: "mem_stub_001",
        reason: "Continue the structured summary.",
        summary: "Keep a concise wrap-up with one risk note.",
      },
    ],
    runtime_summary: createRuntimeSummary(),
    security_summary: {
      latest_restore_point: createRecoveryPoint(),
      pending_authorizations: 0,
      risk_level: "yellow",
      security_status: "normal",
    },
    task: createTask("processing", "collect_input"),
    timeline: [],
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

function readStringParam(params: unknown, key: "action" | "approval_id" | "task_id"): string | null {
  if (!params || typeof params !== "object") {
    return null;
  }

  const value = (params as Record<string, unknown>)[key];
  return typeof value === "string" ? value : null;
}

function createStubCorrectionIntent(correctionText: string): Task["intent"] {
  return {
    name: "agent_loop",
    arguments: {
      goal: correctionText,
      correction_text: correctionText,
    },
  };
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

export async function confirmTask(params?: unknown): Promise<AgentTaskConfirmResult> {
  const taskId = readStringParam(params, "task_id") ?? "task_stub";
  const confirmed = !!(params && typeof params === "object" && (params as { confirmed?: unknown }).confirmed === true);
  const correctedIntent = params && typeof params === "object"
    ? (params as { corrected_intent?: unknown }).corrected_intent
    : undefined;
  const correctionText = params && typeof params === "object"
    ? (params as { correction_text?: unknown }).correction_text
    : undefined;
  const hasCorrectedIntent = !!correctedIntent && typeof correctedIntent === "object" && !Array.isArray(correctedIntent);
  const correctedIntentName = hasCorrectedIntent
    ? ((correctedIntent as { name?: unknown }).name)
    : undefined;
  const normalizedCorrectedIntentName = typeof correctedIntentName === "string" ? correctedIntentName : null;
  const hasCorrectionText = typeof correctionText === "string" && correctionText.trim() !== "";
  const normalizedCorrectionText = hasCorrectionText ? correctionText.trim() : "";

  if (confirmed && (hasCorrectedIntent || hasCorrectionText)) {
    throw new Error("confirmed=true cannot include corrected_intent or correction_text");
  }

  if (hasCorrectedIntent && hasCorrectionText) {
    throw new Error("corrected_intent and correction_text are mutually exclusive");
  }

  if (hasCorrectionText || (!confirmed && !hasCorrectedIntent)) {
    return {
      task: {
        ...createTask("confirming_intent", "intent_confirmation"),
        task_id: taskId,
        intent: hasCorrectionText ? createStubCorrectionIntent(normalizedCorrectionText) : null,
      },
      bubble_message: {
        bubble_id: "bubble_confirm_stub",
        task_id: taskId,
        type: hasCorrectionText ? "intent_confirm" : "status",
        text: hasCorrectionText ? "请确认你希望我如何处理当前内容。" : "This was not the right action. Please clarify the goal or provide a corrected intent.",
        pinned: false,
        hidden: false,
        created_at: new Date().toISOString(),
      },
      delivery_result: null,
    };
  }

  if (!confirmed && hasCorrectedIntent) {
    if (normalizedCorrectedIntentName === null || normalizedCorrectedIntentName.trim() === "") {
      return {
        task: {
          ...createTask("confirming_intent", "intent_confirmation"),
          task_id: taskId,
          intent: null,
        },
        bubble_message: {
          bubble_id: "bubble_confirm_stub",
          task_id: taskId,
          type: "status",
          text: "This correction did not include a valid intent name. Please clarify the goal or provide a corrected intent.",
          pinned: false,
          hidden: false,
          created_at: new Date().toISOString(),
        },
        delivery_result: null,
      };
    }

    const requiresAuthorization = normalizedCorrectedIntentName === "write_file"
      || normalizedCorrectedIntentName === "exec_command"
      || normalizedCorrectedIntentName === "page_interact"
      || normalizedCorrectedIntentName === "browser_navigate"
      || normalizedCorrectedIntentName === "browser_tab_focus"
      || normalizedCorrectedIntentName === "browser_interact";

    return {
      task: {
        ...createTask(requiresAuthorization ? "waiting_auth" : "completed", requiresAuthorization ? "waiting_authorization" : "completed"),
        task_id: taskId,
        intent: correctedIntent as Task["intent"],
      },
      bubble_message: {
        bubble_id: "bubble_confirm_stub",
        task_id: taskId,
        type: "status",
        text: requiresAuthorization
          ? "Authorization is required before continuing the corrected task."
          : "The corrected task was accepted and executed.",
        pinned: false,
        hidden: false,
        created_at: new Date().toISOString(),
      },
      delivery_result: requiresAuthorization ? null : createTaskDeliveryResult(),
    };
  }

  return {
    task: {
      ...createTask(confirmed ? "completed" : "cancelled", confirmed ? "completed" : "cancelled"),
      task_id: taskId,
    },
    bubble_message: {
      bubble_id: "bubble_confirm_stub",
      task_id: taskId,
      type: confirmed ? "result" : "status",
      text: confirmed ? "The task confirmation was accepted." : "The task was cancelled.",
      pinned: false,
      hidden: false,
      created_at: new Date().toISOString(),
    },
    delivery_result: confirmed ? createTaskDeliveryResult() : null,
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

export async function listTasks(_params?: unknown): Promise<AgentTaskListResult> {
  return createTaskListResult();
}

export async function getTaskDetail(_params?: unknown): Promise<AgentTaskDetailGetResult> {
  return createTaskDetailResult();
}

export async function controlTask(params?: unknown): Promise<AgentTaskControlResult> {
  const taskId = readStringParam(params, "task_id") ?? "task_stub";
  const action = readStringParam(params, "action") ?? "pause";

  if (action === "cancel") {
    return {
      task: {
        ...createTask("cancelled", "cancelled"),
        task_id: taskId,
      },
      bubble_message: {
        bubble_id: "bubble_control_stub",
        task_id: taskId,
        type: "status",
        text: "The task was cancelled.",
        pinned: false,
        hidden: false,
        created_at: new Date().toISOString(),
      },
    };
  }

  return {
    task: {
      ...createTask("processing", "collect_input"),
      task_id: taskId,
    },
    bubble_message: {
      bubble_id: "bubble_control_stub",
      task_id: taskId,
      type: "status",
      text: "The requested task control action was accepted.",
      pinned: false,
      hidden: false,
      created_at: new Date().toISOString(),
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
        created_at: new Date().toISOString(),
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
    apply_mode: "next_task_effective",
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
