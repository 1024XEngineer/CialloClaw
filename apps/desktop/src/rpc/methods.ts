import type {
  AgentSettingsGetParams,
  AgentSettingsGetResult,
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
  AgentSecurityRespondParams,
  AgentSecurityRespondResult,
  AgentSecuritySummaryGetParams,
  AgentSecuritySummaryGetResult,
  AgentMirrorOverviewGetParams,
  AgentMirrorOverviewGetResult,
  AgentTaskConfirmParams,
  AgentTaskConfirmResult,
  AgentTaskDetailGetParams,
  AgentTaskDetailGetResult,
  AgentTaskListParams,
  AgentTaskListResult,
  AgentTaskStartParams,
  AgentTaskStartResult,
} from "@cialloclaw/protocol";
import { RPC_METHODS } from "@cialloclaw/protocol";
import { rpcClient } from "./client";

// startTask 处理当前模块的相关逻辑。
export function startTask(params: AgentTaskStartParams) {
  return rpcClient.request<AgentTaskStartResult>(RPC_METHODS.AGENT_TASK_START, params);
}

// confirmTask 处理当前模块的相关逻辑。
export function confirmTask(params: AgentTaskConfirmParams) {
  return rpcClient.request<AgentTaskConfirmResult>(RPC_METHODS.AGENT_TASK_CONFIRM, params);
}

// listTasks 处理当前模块的相关逻辑。
export function listTasks(params: AgentTaskListParams) {
  return rpcClient.request<AgentTaskListResult>(RPC_METHODS.AGENT_TASK_LIST, params);
}

// getTaskDetail 处理当前模块的相关逻辑。
export function getTaskDetail(params: AgentTaskDetailGetParams) {
  return rpcClient.request<AgentTaskDetailGetResult>(RPC_METHODS.AGENT_TASK_DETAIL_GET, params);
}

export function getMirrorOverview(params: AgentMirrorOverviewGetParams) {
  return rpcClient.request<AgentMirrorOverviewGetResult>(RPC_METHODS.AGENT_MIRROR_OVERVIEW_GET, params);
}

export function getSecuritySummary(params: AgentSecuritySummaryGetParams) {
  return rpcClient.request<AgentSecuritySummaryGetResult>(RPC_METHODS.AGENT_SECURITY_SUMMARY_GET, params);
}

export function listSecurityPending(params: AgentSecurityPendingListParams) {
  return rpcClient.request<AgentSecurityPendingListResult>(RPC_METHODS.AGENT_SECURITY_PENDING_LIST, params);
}

export function respondSecurity(params: AgentSecurityRespondParams) {
  return rpcClient.request<AgentSecurityRespondResult>(RPC_METHODS.AGENT_SECURITY_RESPOND, params);
}

export function getSettings(params: AgentSettingsGetParams) {
  return rpcClient.request<AgentSettingsGetResult>(RPC_METHODS.AGENT_SETTINGS_GET, params);
}

export function updateSettings(params: AgentSettingsUpdateParams) {
  return rpcClient.request<AgentSettingsUpdateResult>(RPC_METHODS.AGENT_SETTINGS_UPDATE, params);
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
