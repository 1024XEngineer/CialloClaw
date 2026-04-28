export type DashboardRouteTarget = "home" | "safety" | "result";
export type DashboardModuleRouteTarget = "tasks" | "notes" | "memory" | "safety";
type DashboardRoutePath = "/" | `/${Exclude<DashboardRouteTarget, "home">}`;
type DashboardModuleRoutePath = `/${DashboardModuleRouteTarget}`;
export const dashboardSafetyRoutePath = "/safety";
export const dashboardResultRoutePath = "/result";

export const dashboardRoutePaths: Record<DashboardRouteTarget, DashboardRoutePath> = {
  home: "/",
  result: dashboardResultRoutePath,
  safety: dashboardSafetyRoutePath,
};

export const dashboardModuleRoutePaths: Record<DashboardModuleRouteTarget, DashboardModuleRoutePath> = {
  tasks: "/tasks",
  notes: "/notes",
  memory: "/memory",
  safety: dashboardSafetyRoutePath,
};

export function resolveDashboardRoutePath(target: DashboardRouteTarget) {
  return dashboardRoutePaths[target];
}

export function resolveDashboardModuleRoutePath<TTarget extends DashboardModuleRouteTarget>(target: TTarget): (typeof dashboardModuleRoutePaths)[TTarget] {
  return dashboardModuleRoutePaths[target];
}

export function resolveDashboardRouteHref(target: DashboardRouteTarget) {
  const routePath = resolveDashboardRoutePath(target);

  if (routePath === "/") {
    return "./dashboard.html";
  }

  return `./dashboard.html#${routePath}`;
}
