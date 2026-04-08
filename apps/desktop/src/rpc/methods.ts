import type {
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

export function startTask(params: AgentTaskStartParams) {
  return rpcClient.request<AgentTaskStartResult>(RPC_METHODS.AGENT_TASK_START, params);
}

export function confirmTask(params: AgentTaskConfirmParams) {
  return rpcClient.request<AgentTaskConfirmResult>(RPC_METHODS.AGENT_TASK_CONFIRM, params);
}

export function listTasks(params: AgentTaskListParams) {
  return rpcClient.request<AgentTaskListResult>(RPC_METHODS.AGENT_TASK_LIST, params);
}

export function getTaskDetail(params: AgentTaskDetailGetParams) {
  return rpcClient.request<AgentTaskDetailGetResult>(RPC_METHODS.AGENT_TASK_DETAIL_GET, params);
}
