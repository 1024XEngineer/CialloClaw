import type { TodoBucket } from "@cialloclaw/protocol";

export const dashboardNoteBucketQueryPrefix = ["dashboard", "notes", "bucket"] as const;
export const dashboardNoteBucketGroups = ["upcoming", "later", "recurring_rule", "closed"] as const satisfies readonly TodoBucket[];

export function buildDashboardNoteBucketQueryKey(dataMode: "rpc" | "mock", group: TodoBucket) {
  return [...dashboardNoteBucketQueryPrefix, dataMode, group] as const;
}

export function buildDashboardNoteBucketInvalidateKeys(dataMode: "rpc" | "mock", groups: readonly TodoBucket[]) {
  const uniqueGroups = [...new Set(groups)];
  return uniqueGroups.map((group) => buildDashboardNoteBucketQueryKey(dataMode, group));
}

export function getDashboardNoteRefreshPlan(dataMode: "rpc" | "mock") {
  return {
    invalidatePrefixes: [dashboardNoteBucketQueryPrefix] as const,
    refetchOnMount: dataMode === "rpc",
  };
}
