let dashboardEscapeCloseSuppressionVersion = 0;

/**
 * Marks the current dashboard Escape press as consumed by route-local UI so the
 * desktop window fallback does not close the shell in the same key cycle.
 */
export function suppressDashboardEscapeClose() {
  dashboardEscapeCloseSuppressionVersion += 1;
  return dashboardEscapeCloseSuppressionVersion;
}

/**
 * Returns whether a later route-level handler consumed Escape after the caller
 * captured its own snapshot of the suppression version.
 */
export function wasDashboardEscapeCloseSuppressed(snapshotVersion: number) {
  return dashboardEscapeCloseSuppressionVersion !== snapshotVersion;
}

/**
 * Captures the current suppression version for one native keydown dispatch.
 */
export function readDashboardEscapeCloseSuppressionVersion() {
  return dashboardEscapeCloseSuppressionVersion;
}
