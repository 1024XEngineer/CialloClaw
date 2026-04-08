// 该文件封装前端镜子记忆服务调用。 
import type { AgentMirrorOverviewGetResult, RequestMeta } from "@cialloclaw/protocol";
import { RPC_METHODS } from "@cialloclaw/protocol";
import { rpcClient } from "@/rpc/client";

export function getMirrorOverview(taskId: string) {
  const requestMeta: RequestMeta = {
    trace_id: `trace_mirror_${taskId}`,
    client_time: new Date().toISOString(),
  };

  return rpcClient.request<AgentMirrorOverviewGetResult>(RPC_METHODS.AGENT_MIRROR_OVERVIEW_GET, {
    request_meta: requestMeta,
    include: ["history_summary", "daily_summary", "profile", "memory_references"],
  });
}
