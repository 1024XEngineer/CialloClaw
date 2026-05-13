import type {
  AgentDeliveryOpenParams,
  AgentDeliveryOpenResult,
  AgentTaskArtifactListParams,
  AgentTaskArtifactListResult,
  AgentTaskArtifactOpenParams,
  AgentTaskArtifactOpenResult,
  Artifact,
  DeliveryPayload,
  DeliveryResult,
  RequestMeta,
} from "@cialloclaw/protocol";
import { openDesktopExternalUrl } from "@/platform/desktopExternalUrl";
import { openDesktopLocalPath, revealDesktopLocalPath } from "@/platform/desktopLocalPath";
import { listTaskArtifacts, openDelivery, openTaskArtifact } from "@/rpc/methods";
import { isDashboardTaskDeliveryHref, requestDashboardTaskDeliveryOpen } from "./taskDeliveryNavigation";

export type TaskOutputDataMode = "rpc";

export type TaskOpenExecutionPlan = {
  mode: "task_detail" | "open_url" | "open_local_path" | "reveal_local_path" | "copy_path";
  taskId: string | null;
  path: string | null;
  url: string | null;
  feedback: string;
};

export type TaskOpenExecutionOptions = {
  onOpenTaskDetail?: (input: {
    plan: TaskOpenExecutionPlan;
    taskId: string;
  }) => Promise<string | void> | string | void;
  onOpenTaskDelivery?: (input: {
    plan: TaskOpenExecutionPlan;
    taskId: string;
  }) => Promise<string | void> | string | void;
};

const TASK_OUTPUT_RPC_TIMEOUT_MS = 300_000;

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

/**
 * Merges task artifacts from the dedicated artifact query and the task detail
 * payload so both task detail and delivery views expose the same formal output
 * set while the backend catches list queries up with fresh detail payloads.
 */
export function mergeTaskArtifactItems(listedArtifacts: Artifact[], detailArtifacts: Artifact[]): Artifact[] {
  const mergedArtifacts = [...listedArtifacts];
  const artifactKeys = new Set(
    listedArtifacts.map((artifact) => `${artifact.artifact_id}::${artifact.path}`),
  );

  for (const artifact of detailArtifacts) {
    const artifactKey = `${artifact.artifact_id}::${artifact.path}`;
    if (artifactKeys.has(artifactKey)) {
      continue;
    }

    artifactKeys.add(artifactKey);
    mergedArtifacts.push(artifact);
  }

  return mergedArtifacts;
}

/**
 * Resolves the user-facing open label for the formal delivery result shown in
 * task detail output rows.
 */
export function getTaskDeliveryOpenLabel(deliveryResult: DeliveryResult | null | undefined): string {
  switch (deliveryResult?.type) {
    case "result_page":
      return "打开结果页";
    case "workspace_document":
    case "open_file":
      return "打开文件";
    case "reveal_in_folder":
      return "定位文件";
    case "task_detail":
      return "查看任务详情";
    default:
      return "打开结果";
  }
}

/**
 * Returns whether the current formal delivery result exposes a real open
 * target. Inline bubble outputs stay readable in place and should not render a
 * dead-end open action.
 */
export function canOpenTaskDeliveryResult(
  deliveryResult: DeliveryResult | null | undefined,
  fallbackTaskId: string | null = null,
): boolean {
  if (!deliveryResult) {
    return false;
  }

  if (deliveryResult.type === "result_page") {
    return typeof deliveryResult.payload.url === "string" && deliveryResult.payload.url.trim().length > 0;
  }

  if (
    deliveryResult.type === "workspace_document" ||
    deliveryResult.type === "open_file" ||
    deliveryResult.type === "reveal_in_folder"
  ) {
    return typeof deliveryResult.payload.path === "string" && deliveryResult.payload.path.trim().length > 0;
  }

  if (deliveryResult.type === "task_detail") {
    return Boolean(deliveryResult.payload.task_id || fallbackTaskId);
  }

  return false;
}

/**
 * Auto-open should only run for delivery types that already have a stable
 * renderer-to-OS or dashboard handoff. Bubble-like results stay on the current
 * task surface even though they still use the formal delivery contract.
 *
 * @param deliveryResult Formal delivery result returned by task creation or replay.
 * @returns Whether the renderer can safely auto-open the delivery target.
 */
export function shouldAutoOpenTaskDeliveryResult(
  deliveryResult: DeliveryResult | null | undefined,
): deliveryResult is DeliveryResult {
  return canOpenTaskDeliveryResult(deliveryResult);
}

async function withTimeout<T>(promise: Promise<T>, label: string): Promise<T> {
  return Promise.race([
    promise,
    new Promise<T>((_, reject) => {
      window.setTimeout(() => reject(new Error(`${label} request timed out`)), TASK_OUTPUT_RPC_TIMEOUT_MS);
    }),
  ]);
}

function resolveTaskId(
  payload: DeliveryPayload,
  result: AgentTaskArtifactOpenResult | AgentDeliveryOpenResult,
  fallbackTaskId: string | null = null,
) {
  return payload.task_id ?? result.artifact?.task_id ?? fallbackTaskId;
}

/**
 * Normalizes the formal task open payload into one renderer-side execution
 * plan so task detail routing, browser opens, and local desktop opens share the
 * same decision surface.
 *
 * @param result Formal artifact or delivery open payload returned by RPC.
 * @returns The renderer execution plan for the requested output action.
 */
export function resolveTaskOpenExecutionPlan(
  result: AgentTaskArtifactOpenResult | AgentDeliveryOpenResult,
  fallbackTaskId: string | null = null,
): TaskOpenExecutionPlan {
  const payload = result.resolved_payload;
  const taskId = resolveTaskId(payload, result, fallbackTaskId);
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

  if (result.open_action === "reveal_in_folder" && path) {
    return {
      feedback: "已在文件夹中定位结果。",
      mode: "reveal_local_path",
      path,
      taskId,
      url,
    };
  }

  if ((result.open_action === "open_file" || result.open_action === "workspace_document") && path) {
    return {
      feedback: "已打开本地文件。",
      mode: "open_local_path",
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

async function copyPreparedPath(feedback: string, path: string | null) {
  if (!path) {
    return feedback;
  }

  if (globalThis.navigator?.clipboard?.writeText) {
    try {
      await globalThis.navigator.clipboard.writeText(path);
      return `${feedback} 已复制路径。`;
    } catch {
      return `${feedback} 路径：${path}`;
    }
  }

  return `${feedback} 路径：${path}`;
}

function localPathExecutionFailure(message: string, error: unknown) {
  const detail = error instanceof Error ? error.message.trim() : "";
  if (!detail) {
    return message;
  }

  return `${message}（${detail}）`;
}

function externalUrlExecutionFailure(message: string, error: unknown) {
  const detail = error instanceof Error ? error.message.trim() : "";
  if (!detail) {
    return message;
  }

  return `${message}（${detail}）`;
}

/**
 * Executes a renderer-side open plan while keeping task-detail routing and
 * copy-path fallback inside the same formal execution entry.
 *
 * @param plan Renderer-side execution plan derived from the formal open payload.
 * @param options Optional task-detail delegate for callers that need to route into a view.
 * @returns User-facing feedback describing the completed action or fallback.
 */
export async function performTaskOpenExecution(plan: TaskOpenExecutionPlan, options: TaskOpenExecutionOptions = {}): Promise<string> {
  if (plan.mode === "task_detail" && plan.taskId) {
    const detailFeedback = await options.onOpenTaskDetail?.({
      plan,
      taskId: plan.taskId,
    });

    return typeof detailFeedback === "string" && detailFeedback.trim() !== ""
      ? detailFeedback
      : plan.feedback;
  }

  if (plan.mode === "open_url" && plan.url) {
    if (plan.taskId && isDashboardTaskDeliveryHref(plan.url)) {
      // Result-page URLs stay inside the formal dashboard surface instead of
      // leaking into an external browser tab, even when the request starts from
      // shell-ball or another non-dashboard window.
      const deliveryFeedback = await options.onOpenTaskDelivery?.({
        plan,
        taskId: plan.taskId,
      });

      if (typeof deliveryFeedback === "string" && deliveryFeedback.trim() !== "") {
        return deliveryFeedback;
      }

      await requestDashboardTaskDeliveryOpen(plan.taskId);
      return plan.feedback;
    }

    if (!isAllowedTaskOpenUrl(plan.url)) {
      return "已拦截不受支持的结果链接。";
    }

    try {
      await openDesktopExternalUrl(plan.url);
      return plan.feedback;
    } catch (error) {
      return externalUrlExecutionFailure("无法通过系统浏览器打开结果链接", error);
    }
  }

  if (plan.mode === "open_local_path" && plan.path) {
    try {
      await openDesktopLocalPath(plan.path);
      return plan.feedback;
    } catch (error) {
      return copyPreparedPath(localPathExecutionFailure("无法直接打开本地文件，已准备复制路径", error), plan.path);
    }
  }

  if (plan.mode === "reveal_local_path" && plan.path) {
    try {
      await revealDesktopLocalPath(plan.path);
      return plan.feedback;
    } catch (error) {
      return copyPreparedPath(localPathExecutionFailure("无法在文件夹中定位结果，已准备复制路径", error), plan.path);
    }
  }

  if (plan.mode === "copy_path" && plan.path) {
    return copyPreparedPath(plan.feedback, plan.path);
  }

  return plan.feedback;
}

export function describeTaskOpenResultForCurrentTask(plan: TaskOpenExecutionPlan, currentTaskId: string | null): string | null {
  if (plan.mode === "task_detail" && plan.taskId && plan.taskId === currentTaskId) {
    return "当前任务没有独立可打开结果，请先查看成果区。";
  }

  return null;
}

export async function loadTaskArtifactPage(taskId: string, _source: TaskOutputDataMode = "rpc"): Promise<AgentTaskArtifactListResult> {
  const params: AgentTaskArtifactListParams = {
    limit: 50,
    offset: 0,
    request_meta: createRequestMeta(`task_artifacts_${taskId}`),
    task_id: taskId,
  };

  return withTimeout(listTaskArtifacts(params), `task artifacts ${taskId}`);
}

export async function openTaskArtifactForTask(taskId: string, artifactId: string, _source: TaskOutputDataMode = "rpc"): Promise<AgentTaskArtifactOpenResult> {
  const params: AgentTaskArtifactOpenParams = {
    artifact_id: artifactId,
    request_meta: createRequestMeta(`task_artifact_open_${artifactId}`),
    task_id: taskId,
  };

  return withTimeout(openTaskArtifact(params), `task artifact open ${artifactId}`);
}

export async function openTaskDeliveryForTask(taskId: string, artifactId: string | undefined, _source: TaskOutputDataMode = "rpc"): Promise<AgentDeliveryOpenResult> {
  const params: AgentDeliveryOpenParams = {
    ...(artifactId ? { artifact_id: artifactId } : {}),
    request_meta: createRequestMeta(`task_delivery_open_${taskId}`),
    task_id: taskId,
  };

  return withTimeout(openDelivery(params), `task delivery open ${taskId}`);
}
