import { loadStoredValue, saveStoredValue } from "@/platform/storage";

export type DashboardDataMode = "rpc" | "mock";
export type DashboardDataModePage = "tasks" | "notes" | "memory" | "safety";

export function isDashboardMockModeEnabled() {
  if (typeof window === "undefined") {
    return false;
  }

  const explicitFlag = window.localStorage.getItem("dashboard:enable-mock-toggle");
  if (explicitFlag === "true") {
    return true;
  }

  const hostname = window.location.hostname;
  return hostname === "localhost" || hostname === "127.0.0.1";
}

function getDashboardDataModeStorageKey(page: DashboardDataModePage) {
  return `dashboard:data-mode:${page}`;
}

export function loadDashboardDataMode(page: DashboardDataModePage): DashboardDataMode {
  if (!isDashboardMockModeEnabled()) {
    return "rpc";
  }

  try {
    const storedValue = loadStoredValue<DashboardDataMode>(getDashboardDataModeStorageKey(page));
    return storedValue === "mock" ? "mock" : "rpc";
  } catch {
    return "rpc";
  }
}

export function saveDashboardDataMode(page: DashboardDataModePage, mode: DashboardDataMode) {
  const nextMode = isDashboardMockModeEnabled() ? mode : "rpc";

  try {
    saveStoredValue(getDashboardDataModeStorageKey(page), nextMode);
  } catch {
    return;
  }
}
