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
import {
  createDashboardOpenPlan,
  isAllowedDashboardOpenUrl,
  performDashboardOpenPlan,
  type DashboardOpenExecutionResult,
  type DashboardOpenPlan,
} from "@/features/dashboard/shared/dashboardOpen";
import { listTaskArtifacts, openDelivery, openTaskArtifact } from "@/rpc/methods";
import { getMockTaskDetail } from "./taskPage.mock";

export type TaskOutputDataMode = "rpc" | "mock";
export type TaskOpenExecutionPlan = DashboardOpenPlan;
export type TaskOpenExecutionResult = DashboardOpenExecutionResult;
export type TaskOpenExecutionOptions = {
  approveOutsideWorkspace?: boolean;
};

const TASK_OUTPUT_RPC_TIMEOUT_MS = 2_500;

function createRequestMeta(scope: string): RequestMeta {
  return {
    client_time: new Date().toISOString(),
    trace_id: `trace_${scope}_${Date.now()}`,
  };
}

export function isAllowedTaskOpenUrl(url: string): boolean {
  return isAllowedDashboardOpenUrl(url);
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

/**
 * Converts backend task output open metadata into the shared dashboard open
 * plan shape.
 *
 * @param result Task artifact or delivery open result.
 * @returns A normalized dashboard open plan.
 */
export function resolveTaskOpenExecutionPlan(result: AgentTaskArtifactOpenResult | AgentDeliveryOpenResult): TaskOpenExecutionPlan {
  const payload = result.resolved_payload;
  const taskId = resolveTaskId(payload, result);
  const path = payload.path;
  const url = payload.url;

  if (result.open_action === "task_detail") {
    return createDashboardOpenPlan({
      feedback: "已定位到任务详情。",
      label: result.delivery_result.title || "任务详情",
      missingTargetMessage: "当前结果缺少可定位的任务。",
      mode: "task_detail",
      path,
      taskId,
      url,
    });
  }

  if (url) {
    return createDashboardOpenPlan({
      feedback: result.open_action === "result_page" ? "已打开结果页。" : "已打开链接。",
      label: result.delivery_result.title || "结果链接",
      missingTargetMessage: "当前结果缺少可打开的链接。",
      mode: "open_url",
      path,
      taskId,
      url,
    });
  }

  if (result.open_action === "reveal_in_folder") {
    return createDashboardOpenPlan({
      feedback: "已在文件夹中定位结果。",
      label: result.delivery_result.title || "结果文件",
      missingTargetMessage: "当前结果缺少可定位的路径。",
      mode: "reveal_in_folder",
      path,
      taskId,
      url,
    });
  }

  return createDashboardOpenPlan({
    feedback: "已打开结果文件。",
    label: result.delivery_result.title || "结果文件",
    missingTargetMessage: "当前结果缺少可打开的路径。",
    mode: "open_file",
    path,
    taskId,
    url,
  });
}

/**
 * Executes a normalized task output open plan.
 *
 * @param plan Shared dashboard open plan.
 * @param options Optional execution overrides.
 * @returns The normalized execution result for the task page.
 */
export async function performTaskOpenExecution(plan: TaskOpenExecutionPlan, options: TaskOpenExecutionOptions = {}): Promise<TaskOpenExecutionResult> {
  return performDashboardOpenPlan(plan, options);
}

export function describeTaskOpenResultForCurrentTask(plan: TaskOpenExecutionPlan, currentTaskId: string | null): string | null {
  if (plan.mode === "task_detail" && plan.taskId && plan.taskId === currentTaskId) {
    return "当前任务没有独立可打开结果，请先查看成果区。";
  }

  return null;
}

/**
 * Loads the artifact list for a task detail panel.
 *
 * @param taskId Stable task id.
 * @param source Data source mode for the page.
 * @returns The artifact page payload.
 */
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

/**
 * Resolves the open contract for one task artifact.
 *
 * @param taskId Stable task id.
 * @param artifactId Stable artifact id.
 * @param source Data source mode for the page.
 * @returns The backend open result for that artifact.
 */
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

/**
 * Resolves the latest delivery open contract for a task.
 *
 * @param taskId Stable task id.
 * @param artifactId Optional artifact id when targeting a specific delivery.
 * @param source Data source mode for the page.
 * @returns The backend delivery open result.
 */
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
