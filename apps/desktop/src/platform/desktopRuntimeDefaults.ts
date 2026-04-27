import { invoke } from "@tauri-apps/api/core";

export type DesktopRuntimeDefaults = {
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
