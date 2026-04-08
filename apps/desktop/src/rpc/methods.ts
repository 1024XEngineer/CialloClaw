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
