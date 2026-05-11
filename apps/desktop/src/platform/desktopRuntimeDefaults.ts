import { invoke } from "@tauri-apps/api/core";

export type DesktopRuntimeDefaults = {
  data_path: string;
  workspace_path: string;
  task_sources: string[];
};

/**
 * Reports whether the current renderer can request runtime-default directories
 * from the trusted Tauri host.
 */
export function canUseDesktopRuntimeDefaults() {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;
}

/**
 * Reads the trusted runtime-default workspace paths from the desktop host.
 */
export async function readDesktopRuntimeDefaults() {
  if (!canUseDesktopRuntimeDefaults()) {
    return null;
  }

  return invoke<DesktopRuntimeDefaults>("desktop_get_runtime_defaults");
}

/**
 * Opens the trusted runtime data directory through the desktop host.
 */
export async function openDesktopRuntimeDataDirectory() {
  if (!canUseDesktopRuntimeDefaults()) {
    throw new Error("desktop runtime defaults are unavailable");
  }

  return invoke<void>("desktop_open_runtime_data_path");
}

/**
 * Opens the trusted runtime workspace directory through the desktop host.
 */
export async function openDesktopRuntimeWorkspaceDirectory() {
  if (!canUseDesktopRuntimeDefaults()) {
    throw new Error("desktop runtime defaults are unavailable");
  }

  return invoke<void>("desktop_open_runtime_workspace_path");
}
