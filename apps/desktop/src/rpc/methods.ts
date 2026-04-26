import type {
  AgentDashboardModuleGetParams,
  AgentDashboardModuleGetResult,
  AgentDeliveryOpenParams,
  AgentDeliveryOpenResult,
  AgentDashboardOverviewGetParams,
  AgentDashboardOverviewGetResult,
  AgentInputSubmitParams,
  AgentInputSubmitResult,
  AgentPluginDetailGetParams,
  AgentPluginDetailGetResult,
  AgentPluginListParams,
  AgentPluginListResult,
  AgentPluginRuntimeListParams,
  AgentPluginRuntimeListResult,
  AgentNotepadConvertToTaskParams,
  AgentNotepadConvertToTaskResult,
  AgentNotepadListParams,
  AgentNotepadListResult,
  AgentNotepadUpdateParams,
  AgentNotepadUpdateResult,
  AgentRecommendationFeedbackSubmitParams,
  AgentRecommendationFeedbackSubmitResult,
  AgentRecommendationGetParams,
  AgentRecommendationGetResult,
  AgentSettingsGetParams,
  AgentSettingsGetResult,
  AgentSettingsModelValidateParams,
  AgentSettingsModelValidateResult,
  AgentSettingsUpdateParams,
  AgentSettingsUpdateResult,
  AgentTaskInspectorConfigGetParams,
  AgentTaskInspectorConfigGetResult,
  AgentTaskInspectorConfigUpdateParams,
  AgentTaskInspectorConfigUpdateResult,
  AgentTaskInspectorRunParams,
  AgentTaskInspectorRunResult,
  AgentSecurityPendingListParams,
  AgentSecurityPendingListResult,
  AgentSecurityRestoreApplyParams,
  AgentSecurityRestoreApplyResult,
  AgentSecurityRestorePointsListParams,
  AgentSecurityRestorePointsListResult,
  AgentSecurityRespondParams,
  AgentSecurityRespondResult,
  AgentSecuritySummaryGetParams,
  AgentSecuritySummaryGetResult,
  AgentSecurityAuditListParams,
  AgentSecurityAuditListResult,
  AgentMirrorOverviewGetParams,
  AgentMirrorOverviewGetResult,
  AgentTaskConfirmParams,
  AgentTaskConfirmResult,
  AgentTaskArtifactListParams,
  AgentTaskArtifactListResult,
  AgentTaskArtifactOpenParams,
  AgentTaskArtifactOpenResult,
  AgentTaskControlParams,
  AgentTaskControlResult,
  AgentTaskDetailGetParams,
  AgentTaskDetailGetResult,
  AgentTaskEventsListParams,
  AgentTaskEventsListResult,
  AgentTaskToolCallsListParams,
  AgentTaskToolCallsListResult,
  AgentTaskListParams,
  AgentTaskListResult,
  AgentTaskStartParams,
  AgentTaskStartResult,
  AgentTaskSteerParams,
  AgentTaskSteerResult,
} from "@cialloclaw/protocol";
import { rpcClient, type JsonRpcResponsePayload } from "./client";
import { RPC_METHODS } from "./protocolConstants";

export function submitInput(params: AgentInputSubmitParams) {
  return rpcClient.request<AgentInputSubmitResult>(RPC_METHODS.AGENT_INPUT_SUBMIT, params);
}

// startTask creates a formal task from a confirmed desktop input payload.
export function startTask(params: AgentTaskStartParams) {
  return rpcClient.request<AgentTaskStartResult>(RPC_METHODS.AGENT_TASK_START, params);
}

// confirmTask sends the user-reviewed intent back into the task pipeline.
export function confirmTask(params: AgentTaskConfirmParams) {
  return rpcClient.request<AgentTaskConfirmResult>(RPC_METHODS.AGENT_TASK_CONFIRM, params);
}

export function getRecommendations(params: AgentRecommendationGetParams) {
  return rpcClient.request<AgentRecommendationGetResult>(RPC_METHODS.AGENT_RECOMMENDATION_GET, params);
}

export function submitRecommendationFeedback(params: AgentRecommendationFeedbackSubmitParams) {
  return rpcClient.request<AgentRecommendationFeedbackSubmitResult>(RPC_METHODS.AGENT_RECOMMENDATION_FEEDBACK_SUBMIT, params);
}

// listTasks reads the task-centric dashboard list view from the RPC boundary.
export function listTasks(params: AgentTaskListParams) {
  return rpcClient.request<AgentTaskListResult>(RPC_METHODS.AGENT_TASK_LIST, params);
}

// getTaskDetail loads the task detail view without exposing run-centric internals.
export function getTaskDetail(params: AgentTaskDetailGetParams) {
  return rpcClient.request<AgentTaskDetailGetResult>(RPC_METHODS.AGENT_TASK_DETAIL_GET, params);
}

// listTaskEvents loads persisted runtime events for the current task detail view.
export function listTaskEvents(params: AgentTaskEventsListParams) {
  return rpcClient.request<AgentTaskEventsListResult>(RPC_METHODS.AGENT_TASK_EVENTS_LIST, params);
}

// listTaskToolCalls loads persisted tool_call records for task-level debugging.
export function listTaskToolCalls(params: AgentTaskToolCallsListParams) {
  return rpcClient.request<AgentTaskToolCallsListResult>(RPC_METHODS.AGENT_TASK_TOOL_CALLS_LIST, params);
}

// steerTask appends one follow-up instruction to an active task.
export function steerTask(params: AgentTaskSteerParams) {
  return rpcClient.request<AgentTaskSteerResult>(RPC_METHODS.AGENT_TASK_STEER, params);
}

export function listTaskArtifacts(params: AgentTaskArtifactListParams) {
  return rpcClient.request<AgentTaskArtifactListResult>(RPC_METHODS.AGENT_TASK_ARTIFACT_LIST, params);
}

export function openTaskArtifact(params: AgentTaskArtifactOpenParams) {
  return rpcClient.request<AgentTaskArtifactOpenResult>(RPC_METHODS.AGENT_TASK_ARTIFACT_OPEN, params);
}

export function openDelivery(params: AgentDeliveryOpenParams) {
  return rpcClient.request<AgentDeliveryOpenResult>(RPC_METHODS.AGENT_DELIVERY_OPEN, params);
}

export function controlTask(params: AgentTaskControlParams) {
  return rpcClient.request<AgentTaskControlResult>(RPC_METHODS.AGENT_TASK_CONTROL, params);
}

export function listNotepad(params: AgentNotepadListParams) {
  return rpcClient.request<AgentNotepadListResult>(RPC_METHODS.AGENT_NOTEPAD_LIST, params);
}

export function convertNotepadToTask(params: AgentNotepadConvertToTaskParams) {
  return rpcClient.request<AgentNotepadConvertToTaskResult>(RPC_METHODS.AGENT_NOTEPAD_CONVERT_TO_TASK, params);
}

export function updateNotepad(params: AgentNotepadUpdateParams) {
  return rpcClient.request<AgentNotepadUpdateResult>(RPC_METHODS.AGENT_NOTEPAD_UPDATE, params);
}

export function getDashboardOverview(params: AgentDashboardOverviewGetParams) {
  return rpcClient.request<AgentDashboardOverviewGetResult>(RPC_METHODS.AGENT_DASHBOARD_OVERVIEW_GET, params);
}

export function getDashboardModule(params: AgentDashboardModuleGetParams) {
  return rpcClient.request<AgentDashboardModuleGetResult>(RPC_METHODS.AGENT_DASHBOARD_MODULE_GET, params);
}

export function getMirrorOverview(params: AgentMirrorOverviewGetParams) {
  return rpcClient.request<AgentMirrorOverviewGetResult>(RPC_METHODS.AGENT_MIRROR_OVERVIEW_GET, params);
}

export function getMirrorOverviewDetailed(params: AgentMirrorOverviewGetParams): Promise<JsonRpcResponsePayload<AgentMirrorOverviewGetResult>> {
  return rpcClient.requestDetailed<AgentMirrorOverviewGetResult>(RPC_METHODS.AGENT_MIRROR_OVERVIEW_GET, params);
}

export function getSecuritySummary(params: AgentSecuritySummaryGetParams) {
  return rpcClient.request<AgentSecuritySummaryGetResult>(RPC_METHODS.AGENT_SECURITY_SUMMARY_GET, params);
}

export function getSecuritySummaryDetailed(params: AgentSecuritySummaryGetParams): Promise<JsonRpcResponsePayload<AgentSecuritySummaryGetResult>> {
  return rpcClient.requestDetailed<AgentSecuritySummaryGetResult>(RPC_METHODS.AGENT_SECURITY_SUMMARY_GET, params);
}

export function listSecurityPending(params: AgentSecurityPendingListParams) {
  return rpcClient.request<AgentSecurityPendingListResult>(RPC_METHODS.AGENT_SECURITY_PENDING_LIST, params);
}

export function listSecurityPendingDetailed(params: AgentSecurityPendingListParams): Promise<JsonRpcResponsePayload<AgentSecurityPendingListResult>> {
  return rpcClient.requestDetailed<AgentSecurityPendingListResult>(RPC_METHODS.AGENT_SECURITY_PENDING_LIST, params);
}

export function listSecurityRestorePoints(params: AgentSecurityRestorePointsListParams) {
  return rpcClient.request<AgentSecurityRestorePointsListResult>(RPC_METHODS.AGENT_SECURITY_RESTORE_POINTS_LIST, params);
}

export function listSecurityRestorePointsDetailed(params: AgentSecurityRestorePointsListParams): Promise<JsonRpcResponsePayload<AgentSecurityRestorePointsListResult>> {
  return rpcClient.requestDetailed<AgentSecurityRestorePointsListResult>(RPC_METHODS.AGENT_SECURITY_RESTORE_POINTS_LIST, params);
}

export function applySecurityRestore(params: AgentSecurityRestoreApplyParams) {
  return rpcClient.request<AgentSecurityRestoreApplyResult>(RPC_METHODS.AGENT_SECURITY_RESTORE_APPLY, params);
}

export function applySecurityRestoreDetailed(params: AgentSecurityRestoreApplyParams): Promise<JsonRpcResponsePayload<AgentSecurityRestoreApplyResult>> {
  return rpcClient.requestDetailed<AgentSecurityRestoreApplyResult>(RPC_METHODS.AGENT_SECURITY_RESTORE_APPLY, params);
}

export function respondSecurity(params: AgentSecurityRespondParams) {
  return rpcClient.request<AgentSecurityRespondResult>(RPC_METHODS.AGENT_SECURITY_RESPOND, params);
}

export function respondSecurityDetailed(params: AgentSecurityRespondParams): Promise<JsonRpcResponsePayload<AgentSecurityRespondResult>> {
  return rpcClient.requestDetailed<AgentSecurityRespondResult>(RPC_METHODS.AGENT_SECURITY_RESPOND, params);
}

export function listSecurityAudit(params: AgentSecurityAuditListParams) {
  return rpcClient.request<AgentSecurityAuditListResult>(RPC_METHODS.AGENT_SECURITY_AUDIT_LIST, params);
}

export function listSecurityAuditDetailed(params: AgentSecurityAuditListParams): Promise<JsonRpcResponsePayload<AgentSecurityAuditListResult>> {
  return rpcClient.requestDetailed<AgentSecurityAuditListResult>(RPC_METHODS.AGENT_SECURITY_AUDIT_LIST, params);
}

export function getSettings(params: AgentSettingsGetParams) {
  return rpcClient.request<AgentSettingsGetResult>(RPC_METHODS.AGENT_SETTINGS_GET, params);
}

export function getSettingsDetailed(params: AgentSettingsGetParams): Promise<JsonRpcResponsePayload<AgentSettingsGetResult>> {
  return rpcClient.requestDetailed<AgentSettingsGetResult>(RPC_METHODS.AGENT_SETTINGS_GET, params);
}

export function updateSettings(params: AgentSettingsUpdateParams) {
  return rpcClient.request<AgentSettingsUpdateResult>(RPC_METHODS.AGENT_SETTINGS_UPDATE, params);
}

export function validateSettingsModel(params: AgentSettingsModelValidateParams) {
  return rpcClient.request<AgentSettingsModelValidateResult>(RPC_METHODS.AGENT_SETTINGS_MODEL_VALIDATE, params);
}

export function listPluginRuntimes(params: AgentPluginRuntimeListParams) {
  return rpcClient.request<AgentPluginRuntimeListResult>(RPC_METHODS.AGENT_PLUGIN_RUNTIME_LIST, params);
}

export function listPlugins(params: AgentPluginListParams) {
  return rpcClient.request<AgentPluginListResult>(RPC_METHODS.AGENT_PLUGIN_LIST, params);
}

export function getPluginDetail(params: AgentPluginDetailGetParams) {
  return rpcClient.request<AgentPluginDetailGetResult>(RPC_METHODS.AGENT_PLUGIN_DETAIL_GET, params);
}

export function getTaskInspectorConfig(params: AgentTaskInspectorConfigGetParams) {
  return rpcClient.request<AgentTaskInspectorConfigGetResult>(RPC_METHODS.AGENT_TASK_INSPECTOR_CONFIG_GET, params);
}

export function updateTaskInspectorConfig(params: AgentTaskInspectorConfigUpdateParams) {
  return rpcClient.request<AgentTaskInspectorConfigUpdateResult>(RPC_METHODS.AGENT_TASK_INSPECTOR_CONFIG_UPDATE, params);
}

export function runTaskInspector(params: AgentTaskInspectorRunParams) {
  return rpcClient.request<AgentTaskInspectorRunResult>(RPC_METHODS.AGENT_TASK_INSPECTOR_RUN, params);
}
