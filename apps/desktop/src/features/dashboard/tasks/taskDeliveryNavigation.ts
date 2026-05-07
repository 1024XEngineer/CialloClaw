import { resolveDashboardModuleRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";

export const dashboardTaskDeliveryRoutePattern = "delivery/:taskId";

/**
 * Builds the dashboard route that shows the formal delivery page for one task.
 *
 * @param taskId Formal task identifier that owns the delivery result.
 * @returns Absolute dashboard route path for the delivery page.
 */
export function resolveDashboardTaskDeliveryRoutePath(taskId: string) {
  return `${resolveDashboardModuleRoutePath("tasks")}/delivery/${encodeURIComponent(taskId)}`;
}
