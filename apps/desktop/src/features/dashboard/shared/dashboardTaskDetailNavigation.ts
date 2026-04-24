import { getCurrentWindow } from "@tauri-apps/api/window";
import type { NavigateFunction } from "react-router-dom";
import { openOrFocusDesktopWindow } from "@/platform/windowController";
import { resolveDashboardModuleRoutePath } from "./dashboardRouteTargets";

const DASHBOARD_TASK_DETAIL_RETRY_DELAYS_MS = [180, 520] as const;

export const dashboardTaskDetailNavigationEvent = "desktop-dashboard:task-detail-open";

export type DashboardTaskDetailRouteState = {
  focusTaskId: string;
  openDetail: true;
};

export type DashboardTaskDetailOpenRequest = {
  request_id: string;
  task_id: string;
};

function createDashboardTaskDetailRequestId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }

  return `dashboard-task-detail-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

async function emitDashboardTaskDetailOpenRequest(
  request: DashboardTaskDetailOpenRequest,
  windowHandle = getCurrentWindow(),
) {
  if (windowHandle.label === "dashboard") {
    await windowHandle.emit(dashboardTaskDetailNavigationEvent, request);
    return;
  }

  await windowHandle.emitTo("dashboard", dashboardTaskDetailNavigationEvent, request);
}

/**
 * Builds the task-detail router state used by dashboard modules to focus and
 * expand a specific task detail panel.
 *
 * @param taskId Formal task identifier that should be focused in the dashboard.
 * @returns Router state understood by the task page.
 */
export function buildDashboardTaskDetailRouteState(taskId: string): DashboardTaskDetailRouteState {
  return {
    focusTaskId: taskId,
    openDetail: true,
  };
}

/**
 * Reads task-detail router state from an unknown location payload so the task
 * page can ignore unrelated route state safely.
 *
 * @param value Unknown router state supplied by another dashboard module.
 * @returns Normalized task-detail route state or null when the payload does not match.
 */
export function readDashboardTaskDetailRouteState(value: unknown): DashboardTaskDetailRouteState | null {
  if (!value || typeof value !== "object") {
    return null;
  }

  const focusTaskId = typeof (value as { focusTaskId?: unknown }).focusTaskId === "string"
    ? (value as { focusTaskId: string }).focusTaskId
    : null;
  const openDetail = (value as { openDetail?: unknown }).openDetail === true;

  if (!focusTaskId || !openDetail) {
    return null;
  }

  return {
    focusTaskId,
    openDetail: true,
  };
}

/**
 * Navigates inside the dashboard to the task module and opens the requested
 * task detail panel in one step.
 *
 * @param navigate React Router navigate function from the current dashboard view.
 * @param taskId Formal task identifier that should be focused.
 */
export function navigateToDashboardTaskDetail(navigate: NavigateFunction, taskId: string) {
  navigate(resolveDashboardModuleRoutePath("tasks"), {
    state: buildDashboardTaskDetailRouteState(taskId),
  });
}

/**
 * Opens or focuses the dashboard window, then requests the task module to
 * focus a specific task detail. Delayed retries cover freshly mounted windows
 * that have not attached the dashboard listener yet.
 *
 * @param taskId Formal task identifier that should be focused.
 */
export async function requestDashboardTaskDetailOpen(taskId: string) {
  const request = {
    request_id: createDashboardTaskDetailRequestId(),
    task_id: taskId,
  } satisfies DashboardTaskDetailOpenRequest;

  await openOrFocusDesktopWindow("dashboard");
  await emitDashboardTaskDetailOpenRequest(request);

  if (typeof window === "undefined") {
    return;
  }

  for (const delayMs of DASHBOARD_TASK_DETAIL_RETRY_DELAYS_MS) {
    window.setTimeout(() => {
      void emitDashboardTaskDetailOpenRequest(request).catch((): void => undefined);
    }, delayMs);
  }
}
