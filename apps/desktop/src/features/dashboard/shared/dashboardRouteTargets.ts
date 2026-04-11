export type DashboardRouteTarget = "home" | "safety";
export type DashboardModuleRouteTarget = "tasks" | "notes" | "memory" | "safety";

export const dashboardRoutePaths: Record<DashboardRouteTarget, string> = {
  home: "/",
  safety: "/safety",
};

export const dashboardModuleRoutePaths: Record<DashboardModuleRouteTarget, string> = {
  tasks: "/tasks",
  notes: "/notes",
  memory: "/memory",
  safety: "/safety",
};

export function resolveDashboardRoutePath(target: DashboardRouteTarget) {
  return dashboardRoutePaths[target];
}

export function resolveDashboardModuleRoutePath(target: DashboardModuleRouteTarget) {
  return dashboardModuleRoutePaths[target];
}

export function resolveDashboardRouteHref(target: DashboardRouteTarget) {
  const routePath = resolveDashboardRoutePath(target);

  if (routePath === "/") {
    return "./dashboard.html";
  }

  return `./dashboard.html#${routePath}`;
}
