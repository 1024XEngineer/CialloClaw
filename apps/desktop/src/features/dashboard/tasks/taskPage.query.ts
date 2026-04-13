export const dashboardTaskBucketQueryPrefix = ["dashboard", "tasks", "bucket"] as const;
export const dashboardTaskDetailQueryPrefix = ["dashboard", "tasks", "detail"] as const;

export function buildDashboardTaskBucketQueryKey(dataMode: "rpc" | "mock", group: "unfinished" | "finished", limit: number) {
  return [...dashboardTaskBucketQueryPrefix, dataMode, group, limit] as const;
}

export function buildDashboardTaskDetailQueryKey(dataMode: "rpc" | "mock", taskId: string) {
  return [...dashboardTaskDetailQueryPrefix, dataMode, taskId] as const;
}
