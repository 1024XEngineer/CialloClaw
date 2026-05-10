import { getCurrentWindow } from "@tauri-apps/api/window";
import type { NavigateFunction } from "react-router-dom";
import { openOrFocusDesktopWindow } from "@/platform/windowController";
import { resolveDashboardModuleRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";

export const dashboardTaskDeliveryRoutePattern = "delivery/:taskId";
export const dashboardTaskDeliveryNavigationEvent = "desktop-dashboard:task-delivery-open";

const DASHBOARD_TASK_DELIVERY_RETRY_DELAYS_MS = [180, 520] as const;
const dashboardTaskDeliveryHrefPrefix = "./dashboard.html#/tasks/delivery/";

export type DashboardTaskDeliveryOpenRequest = {
  request_id: string;
  task_id: string;
};

function createDashboardTaskDeliveryRequestId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }

  return `dashboard-task-delivery-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

async function emitDashboardTaskDeliveryOpenRequest(
  request: DashboardTaskDeliveryOpenRequest,
  windowHandle = getCurrentWindow(),
) {
  if (windowHandle.label === "dashboard") {
    await windowHandle.emit(dashboardTaskDeliveryNavigationEvent, request);
    return;
  }

  await windowHandle.emitTo("dashboard", dashboardTaskDeliveryNavigationEvent, request);
}

/**
 * Builds the dashboard route that shows the formal delivery page for one task.
 *
 * @param taskId Formal task identifier that owns the delivery result.
 * @returns Absolute dashboard route path for the delivery page.
 */
export function resolveDashboardTaskDeliveryRoutePath(taskId: string) {
  return `${resolveDashboardModuleRoutePath("tasks")}/delivery/${encodeURIComponent(taskId)}`;
}

/**
 * Builds the relative dashboard href used by the formal result-page payload.
 *
 * @param taskId Formal task identifier that owns the delivery result.
 * @returns Relative dashboard href for the delivery page.
 */
export function resolveDashboardTaskDeliveryRouteHref(taskId: string) {
  return `${dashboardTaskDeliveryHrefPrefix}${encodeURIComponent(taskId)}`;
}

/**
 * Checks whether one result-page URL targets the dashboard delivery route.
 *
 * @param url Formal URL payload returned by the backend.
 * @returns True when the URL points at the dashboard delivery page.
 */
export function isDashboardTaskDeliveryHref(url: string) {
  return url.trim().startsWith(dashboardTaskDeliveryHrefPrefix);
}

/**
 * Navigates inside the dashboard to the dedicated delivery route for one task.
 *
 * @param navigate React Router navigate function from the current dashboard view.
 * @param taskId Formal task identifier that should be opened.
 */
export function navigateToDashboardTaskDelivery(navigate: NavigateFunction, taskId: string) {
  navigate(resolveDashboardTaskDeliveryRoutePath(taskId));
}

/**
 * Opens or focuses the dashboard window, then requests the delivery route for
 * one task. Delayed retries cover freshly mounted dashboard windows that have
 * not attached the delivery listener yet.
 *
 * @param taskId Formal task identifier that should be opened.
 */
export async function requestDashboardTaskDeliveryOpen(taskId: string) {
  const request = {
    request_id: createDashboardTaskDeliveryRequestId(),
    task_id: taskId,
  } satisfies DashboardTaskDeliveryOpenRequest;

  await openOrFocusDesktopWindow("dashboard");
  await emitDashboardTaskDeliveryOpenRequest(request);

  if (typeof window === "undefined") {
    return;
  }

  for (const delayMs of DASHBOARD_TASK_DELIVERY_RETRY_DELAYS_MS) {
    window.setTimeout(() => {
      void emitDashboardTaskDeliveryOpenRequest(request).catch((): void => undefined);
    }, delayMs);
  }
}
