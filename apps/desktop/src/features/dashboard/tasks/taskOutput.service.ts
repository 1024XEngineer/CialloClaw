import type {
  AgentDeliveryOpenParams,
  AgentDeliveryOpenResult,
  AgentTaskArtifactListParams,
  AgentTaskArtifactListResult,
  AgentTaskArtifactOpenParams,
  AgentTaskArtifactOpenResult,
  Artifact,
  DeliveryPayload,
  RequestMeta,
} from "@cialloclaw/protocol";
import { listTaskArtifacts, openDelivery, openTaskArtifact } from "@/rpc/methods";
import { getMockTaskDetail } from "./taskPage.mock";

export type TaskOutputDataMode = "rpc" | "mock";

export type TaskOpenExecutionPlan = {
  mode: "task_detail" | "open_url" | "copy_path";
  taskId: string | null;
  path: string | null;
  url: string | null;
  feedback: string;
};

const TASK_OUTPUT_RPC_TIMEOUT_MS = 2_500;

function createRequestMeta(scope: string): RequestMeta {
  return {
    client_time: new Date().toISOString(),
    trace_id: `trace_${scope}_${Date.now()}`,
  };
}

export function isAllowedTaskOpenUrl(url: string): boolean {
  try {
    const parsed = new URL(url);
    return parsed.protocol === "https:" || parsed.protocol === "http:";
  } catch {
    return false;
  }
}

async function withTimeout<T>(promise: Promise<T>, label: string): Promise<T> {
  return Promise.race([
    promise,
    new Promise<T>((_, reject) => {
      window.setTimeout(() => reject(new Error(`${label} request timed out`)), TASK_OUTPUT_RPC_TIMEOUT_MS);
    }),
  ]);
}

function buildMockArtifactPage(taskId: string): AgentTaskArtifactListResult {
  const detail = getMockTaskDetail(taskId).detail;

  return {
    items: detail.artifacts,
    page: {
      has_more: false,
      limit: detail.artifacts.length,
      offset: 0,
      total: detail.artifacts.length,
    },
  };
}

function buildMockDeliveryPayload(taskId: string, artifact: Artifact | null): DeliveryPayload {
  return {
    path: artifact?.path ?? null,
    task_id: taskId,
    url: null,
  };
}

function inferMockOpenAction(artifact: Artifact | null) {
  if (!artifact) {
    return "task_detail" as const;
  }

  if (artifact.artifact_type === "reveal_in_folder") {
    return "reveal_in_folder" as const;
  }

  return "open_file" as const;
}

function buildMockOpenResult(taskId: string, artifact: Artifact | null): AgentTaskArtifactOpenResult | AgentDeliveryOpenResult {
  const openAction = inferMockOpenAction(artifact);
  const payload = buildMockDeliveryPayload(taskId, artifact);
  const title = artifact?.title ?? "任务结果";

  return {
    ...(artifact ? { artifact } : {}),
    delivery_result: {
      payload,
      preview_text: title,
      title,
      type: openAction,
    },
    open_action: openAction,
    resolved_payload: payload,
  };
}

function resolveTaskId(payload: DeliveryPayload, result: AgentTaskArtifactOpenResult | AgentDeliveryOpenResult) {
  return payload.task_id ?? result.artifact?.task_id ?? null;
}

export function resolveTaskOpenExecutionPlan(result: AgentTaskArtifactOpenResult | AgentDeliveryOpenResult): TaskOpenExecutionPlan {
  const payload = result.resolved_payload;
  const taskId = resolveTaskId(payload, result);
  const path = payload.path;
  const url = payload.url;

  if (result.open_action === "task_detail") {
    return {
      feedback: "已定位到任务详情。",
      mode: "task_detail",
      path,
      taskId,
      url,
    };
  }

  if (url) {
    return {
      feedback: result.open_action === "result_page" ? "已打开结果页。" : "已打开链接。",
      mode: "open_url",
      path,
      taskId,
      url,
    };
  }

  return {
    feedback: path ? "当前环境暂不支持直接打开，已准备复制路径。" : "当前结果已准备好，但缺少可直接打开的地址。",
    mode: "copy_path",
    path,
    taskId,
    url,
  };
}

export async function performTaskOpenExecution(plan: TaskOpenExecutionPlan): Promise<string> {
  if (plan.mode === "open_url" && plan.url) {
    if (!isAllowedTaskOpenUrl(plan.url)) {
      return "已拦截不受支持的结果链接。";
    }

    window.open(plan.url, "_blank", "noopener,noreferrer");
    return plan.feedback;
  }

  if (plan.mode === "copy_path" && plan.path) {
    if (navigator.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(plan.path);
        return `${plan.feedback} 已复制路径。`;
      } catch {
        return `${plan.feedback} 路径：${plan.path}`;
      }
    }

    return `${plan.feedback} 路径：${plan.path}`;
  }

  return plan.feedback;
}

export function describeTaskOpenResultForCurrentTask(plan: TaskOpenExecutionPlan, currentTaskId: string | null): string | null {
  if (plan.mode === "task_detail" && plan.taskId && plan.taskId === currentTaskId) {
    return "当前任务没有独立可打开结果，请先查看成果区。";
  }

  return null;
}

export async function loadTaskArtifactPage(taskId: string, source: TaskOutputDataMode = "rpc"): Promise<AgentTaskArtifactListResult> {
  if (source === "mock") {
    return buildMockArtifactPage(taskId);
  }

  const params: AgentTaskArtifactListParams = {
    limit: 50,
    offset: 0,
    request_meta: createRequestMeta(`task_artifacts_${taskId}`),
    task_id: taskId,
  };

  return withTimeout(listTaskArtifacts(params), `task artifacts ${taskId}`);
}

export async function openTaskArtifactForTask(taskId: string, artifactId: string, source: TaskOutputDataMode = "rpc"): Promise<AgentTaskArtifactOpenResult> {
  if (source === "mock") {
    const artifact = getMockTaskDetail(taskId).detail.artifacts.find((item) => item.artifact_id === artifactId);
    if (!artifact) {
      throw new Error(`mock artifact not found: ${artifactId}`);
    }
    return buildMockOpenResult(taskId, artifact) as AgentTaskArtifactOpenResult;
  }

  const params: AgentTaskArtifactOpenParams = {
    artifact_id: artifactId,
    request_meta: createRequestMeta(`task_artifact_open_${artifactId}`),
    task_id: taskId,
  };

  return withTimeout(openTaskArtifact(params), `task artifact open ${artifactId}`);
}

export async function openTaskDeliveryForTask(taskId: string, artifactId: string | undefined, source: TaskOutputDataMode = "rpc"): Promise<AgentDeliveryOpenResult> {
  if (source === "mock") {
    const artifact = artifactId ? getMockTaskDetail(taskId).detail.artifacts.find((item) => item.artifact_id === artifactId) ?? null : null;
    return buildMockOpenResult(taskId, artifact) as AgentDeliveryOpenResult;
  }

  const params: AgentDeliveryOpenParams = {
    ...(artifactId ? { artifact_id: artifactId } : {}),
    request_meta: createRequestMeta(`task_delivery_open_${taskId}`),
    task_id: taskId,
  };

  return withTimeout(openDelivery(params), `task delivery open ${taskId}`);
}
