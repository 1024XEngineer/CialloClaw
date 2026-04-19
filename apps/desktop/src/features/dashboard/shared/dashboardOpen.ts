import { loadSettings } from "@/services/settingsService";

export type DashboardDesktopOpenAction = "open_file" | "reveal_in_folder";
export type DashboardOpenMode = "task_detail" | "open_url" | DashboardDesktopOpenAction;

export type DashboardOutsideWorkspaceConfirmationSession = {
  approvedForWindow: boolean;
};

export type DashboardOpenPlan = {
  mode: DashboardOpenMode;
  label: string;
  feedback: string;
  confirmMessage: string;
  missingTargetMessage: string;
  path: string | null;
  resolvedPath: string | null;
  taskId: string | null;
  url: string | null;
  workspacePath: string | null;
  requiresWorkspaceConfirmation: boolean;
};

export type DashboardOpenExecutionResult =
  | { type: "task_detail"; message: string; plan: DashboardOpenPlan; taskId: string }
  | { type: "opened"; message: string; plan: DashboardOpenPlan }
  | { type: "confirm_required"; message: string; plan: DashboardOpenPlan }
  | { type: "error"; message: string; plan: DashboardOpenPlan };

type DashboardOpenPlanInput = {
  mode: DashboardOpenMode;
  label: string;
  feedback: string;
  confirmMessage?: string;
  missingTargetMessage: string;
  path?: string | null;
  taskId?: string | null;
  url?: string | null;
  workspacePath?: string | null;
};

type DashboardOpenAttemptOptions = {
  approveOutsideWorkspace?: boolean;
};

const WORKSPACE_ALIAS_PREFIX = "workspace/";
const WINDOWS_ABSOLUTE_PATH_PATTERN = /^(?:[A-Za-z]:[\\/]|\\\\)/;

let outsideWorkspaceApprovedForWindow = false;

/**
 * Reads the current per-window outside-workspace approval state.
 *
 * @returns The current approval state for this dashboard window.
 */
export function readDashboardOutsideWorkspaceConfirmationSession(): DashboardOutsideWorkspaceConfirmationSession {
  return {
    approvedForWindow: outsideWorkspaceApprovedForWindow,
  };
}

/**
 * Clears the current per-window outside-workspace approval state.
 */
export function resetDashboardOutsideWorkspaceConfirmationSession() {
  outsideWorkspaceApprovedForWindow = false;
}

/**
 * Accepts only browser-openable URLs for dashboard deep links.
 *
 * @param url Candidate URL.
 * @returns Whether the URL uses a supported web protocol.
 */
export function isAllowedDashboardOpenUrl(url: string): boolean {
  try {
    const parsed = new URL(url);
    return parsed.protocol === "https:" || parsed.protocol === "http:";
  } catch {
    return false;
  }
}

/**
 * Normalizes a dashboard resource path into a slash-stable comparable string.
 *
 * @param path Raw resource path.
 * @returns A trimmed normalized path, or `null` when empty.
 */
export function normalizeDashboardPath(path: string | null | undefined): string | null {
  if (!path) {
    return null;
  }

  const trimmedPath = path.trim();
  if (trimmedPath.length === 0) {
    return null;
  }

  return trimmedPath.replace(/\\/g, "/");
}

/**
 * Resolves a dashboard resource path to an absolute path when the client can do
 * so locally.
 *
 * Relative workspace aliases are expanded under the configured download
 * workspace. Other relative paths are left unresolved and treated as
 * outside-workspace until the desktop host opens them.
 *
 * @param path Raw resource path.
 * @param workspacePath Trusted download workspace root.
 * @returns The absolute path when resolvable on the client, otherwise `null`.
 */
export function resolveDashboardAbsolutePath(path: string | null | undefined, workspacePath: string | null | undefined): string | null {
  const normalizedPath = normalizeDashboardPath(path);
  if (!normalizedPath) {
    return null;
  }

  if (WINDOWS_ABSOLUTE_PATH_PATTERN.test(normalizedPath) || normalizedPath.startsWith("/")) {
    return normalizedPath;
  }

  const normalizedWorkspacePath = normalizeDashboardWorkspacePath(workspacePath);
  if (normalizedWorkspacePath && normalizedPath.toLowerCase().startsWith(WORKSPACE_ALIAS_PREFIX)) {
    return joinDashboardPath(normalizedWorkspacePath, normalizedPath.slice(WORKSPACE_ALIAS_PREFIX.length));
  }

  return null;
}

/**
 * Checks whether a path stays inside the configured trusted download workspace.
 *
 * @param absolutePath Absolute candidate path.
 * @param workspacePath Trusted workspace root.
 * @returns Whether the candidate is inside the workspace root.
 */
export function isDashboardPathWithinTrustedWorkspace(absolutePath: string | null | undefined, workspacePath: string | null | undefined): boolean {
  const normalizedAbsolutePath = normalizeComparablePath(absolutePath);
  const normalizedWorkspacePath = normalizeComparablePath(workspacePath);

  if (!normalizedAbsolutePath || !normalizedWorkspacePath) {
    return false;
  }

  return (
    normalizedAbsolutePath === normalizedWorkspacePath ||
    normalizedAbsolutePath.startsWith(`${normalizedWorkspacePath}/`)
  );
}

/**
 * Builds a normalized dashboard open plan that can be shared by notes and task
 * delivery entry points.
 *
 * @param input Open-plan source data.
 * @returns A normalized plan ready for UI confirmation and execution.
 */
export function createDashboardOpenPlan(input: DashboardOpenPlanInput): DashboardOpenPlan {
  const workspacePath = normalizeDashboardWorkspacePath(input.workspacePath ?? readDashboardWorkspacePath());
  const path = normalizeDashboardPath(input.path);
  const resolvedPath = resolveDashboardAbsolutePath(path, workspacePath);
  const requiresWorkspaceConfirmation =
    (input.mode === "open_file" || input.mode === "reveal_in_folder") &&
    Boolean(path) &&
    !isDashboardPathWithinTrustedWorkspace(resolvedPath, workspacePath);

  return {
    confirmMessage:
      input.confirmMessage ??
      `“${input.label}” 位于下载工作区之外。确认后，本次仪表盘窗口里的后续工作区外路径将直接打开。`,
    feedback: input.feedback,
    label: input.label,
    missingTargetMessage: input.missingTargetMessage,
    mode: input.mode,
    path,
    resolvedPath,
    requiresWorkspaceConfirmation,
    taskId: input.taskId ?? null,
    url: input.url ?? null,
    workspacePath,
  };
}

/**
 * Executes a normalized dashboard open plan and reports whether the current UI
 * should navigate, confirm, or surface a local failure message.
 *
 * @param plan Shared dashboard open plan.
 * @param options Optional execution overrides.
 * @returns The execution outcome for the current module view.
 */
export async function performDashboardOpenPlan(
  plan: DashboardOpenPlan,
  options: DashboardOpenAttemptOptions = {},
): Promise<DashboardOpenExecutionResult> {
  if (plan.mode === "task_detail") {
    if (!plan.taskId) {
      return {
        type: "error",
        message: plan.missingTargetMessage,
        plan,
      };
    }

    return {
      type: "task_detail",
      message: plan.feedback,
      plan,
      taskId: plan.taskId,
    };
  }

  if (plan.mode === "open_url") {
    if (!plan.url) {
      return {
        type: "error",
        message: plan.missingTargetMessage,
        plan,
      };
    }

    if (!isAllowedDashboardOpenUrl(plan.url)) {
      return {
        type: "error",
        message: `已拦截不受支持的资源链接：${plan.url}`,
        plan,
      };
    }

    try {
      window.open(plan.url, "_blank", "noopener,noreferrer");
      return {
        type: "opened",
        message: plan.feedback,
        plan,
      };
    } catch (error) {
      return {
        type: "error",
        message: buildDashboardOpenErrorMessage(plan.label, error),
        plan,
      };
    }
  }

  if (!plan.path) {
    return {
      type: "error",
      message: plan.missingTargetMessage,
      plan,
    };
  }

  const shouldRequireConfirmation =
    plan.requiresWorkspaceConfirmation &&
    !outsideWorkspaceApprovedForWindow &&
    options.approveOutsideWorkspace !== true;

  if (shouldRequireConfirmation) {
    return {
      type: "confirm_required",
      message: plan.confirmMessage,
      plan,
    };
  }

  try {
    await invokeDashboardDesktopOpenResource(plan.mode, plan.resolvedPath ?? plan.path);

    if (plan.requiresWorkspaceConfirmation && options.approveOutsideWorkspace) {
      outsideWorkspaceApprovedForWindow = true;
    }

    return {
      type: "opened",
      message: plan.feedback,
      plan,
    };
  } catch (error) {
    return {
      type: "error",
      message: buildDashboardOpenErrorMessage(plan.label, error),
      plan,
    };
  }
}

function buildDashboardOpenErrorMessage(label: string, error: unknown) {
  const errorMessage = error instanceof Error ? error.message : "desktop host is unavailable";
  return `打开“${label}”失败：${errorMessage}`;
}

function joinDashboardPath(basePath: string, relativePath: string) {
  const normalizedBasePath = basePath.replace(/[\\/]+$/g, "");
  const normalizedRelativePath = relativePath.replace(/^[\\/]+/g, "");
  return `${normalizedBasePath}/${normalizedRelativePath}`.replace(/\\/g, "/");
}

function normalizeComparablePath(path: string | null | undefined) {
  const normalizedPath = normalizeDashboardPath(path);
  if (!normalizedPath) {
    return null;
  }

  const withoutTrailingSlash = normalizedPath.replace(/\/+$/g, "");
  return WINDOWS_ABSOLUTE_PATH_PATTERN.test(withoutTrailingSlash)
    ? withoutTrailingSlash.toLowerCase()
    : withoutTrailingSlash;
}

function normalizeDashboardWorkspacePath(path: string | null | undefined) {
  const normalizedPath = normalizeDashboardPath(path);
  if (!normalizedPath) {
    return null;
  }

  return normalizedPath.replace(/\/+$/g, "");
}

function readDashboardWorkspacePath() {
  if (typeof window === "undefined" || !("localStorage" in window)) {
    return null;
  }

  try {
    return loadSettings().settings.general.download.workspace_path ?? null;
  } catch {
    return null;
  }
}

async function invokeDashboardDesktopOpenResource(action: DashboardDesktopOpenAction, path: string) {
  const tauriInvoker =
    typeof window !== "undefined" &&
    "__TAURI_INTERNALS__" in window &&
    typeof window.__TAURI_INTERNALS__?.invoke === "function"
      ? window.__TAURI_INTERNALS__.invoke
      : null;

  if (tauriInvoker) {
    await tauriInvoker("desktop_open_resource", { action, path });
    return;
  }

  const tauriCore = await import("@tauri-apps/api/core");
  await tauriCore.invoke("desktop_open_resource", { action, path });
}

declare global {
  interface Window {
    __TAURI_INTERNALS__?: {
      invoke?: (command: string, args?: Record<string, unknown>) => Promise<unknown>;
    };
  }
}
