import type { TodoBucket } from "@cialloclaw/protocol";

export const dashboardNoteBucketQueryPrefix = ["dashboard", "notes", "bucket"] as const;
export const dashboardNoteBucketGroups = ["upcoming", "later", "recurring_rule", "closed"] as const satisfies readonly TodoBucket[];

export function buildDashboardNoteBucketQueryKey(dataMode: "rpc", group: TodoBucket) {
  return [...dashboardNoteBucketQueryPrefix, dataMode, group] as const;
}

export function buildDashboardNoteBucketInvalidateKeys(dataMode: "rpc", groups: readonly TodoBucket[]) {
  const uniqueGroups = [...new Set(groups)];
  return uniqueGroups.map((group) => buildDashboardNoteBucketQueryKey(dataMode, group));
}

export function getDashboardNoteRefreshPlan(dataMode: "rpc") {
  return {
    invalidatePrefixes: [dashboardNoteBucketQueryPrefix] as const,
    refetchOnMount: dataMode === "rpc",
  };
}
