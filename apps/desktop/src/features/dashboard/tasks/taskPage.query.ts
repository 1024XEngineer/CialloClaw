export const dashboardTaskBucketQueryPrefix = ["dashboard", "tasks", "bucket"] as const;
export const dashboardTaskDetailQueryPrefix = ["dashboard", "tasks", "detail"] as const;

export function buildDashboardTaskBucketQueryKey(dataMode: "rpc" | "mock", group: "unfinished" | "finished", limit: number) {
  return [...dashboardTaskBucketQueryPrefix, dataMode, group, limit] as const;
}

export function buildDashboardTaskDetailQueryKey(dataMode: "rpc" | "mock", taskId: string) {
  return [...dashboardTaskDetailQueryPrefix, dataMode, taskId] as const;
}

export function getDashboardTaskSecurityRefreshPlan(dataMode: "rpc" | "mock") {
  return {
    invalidatePrefixes: [dashboardTaskBucketQueryPrefix, dashboardTaskDetailQueryPrefix] as const,
    refetchOnMount: dataMode === "rpc",
  };
}

export function shouldEnableDashboardTaskDetailQuery(selectedTaskId: string | null, detailOpen: boolean) {
  return Boolean(selectedTaskId && detailOpen);
}

export function resolveDashboardTaskSafetyOpenPlan(detailSource: "rpc" | "mock" | "fallback") {
  return {
    shouldRefetchDetail: detailSource === "fallback",
  };
}
