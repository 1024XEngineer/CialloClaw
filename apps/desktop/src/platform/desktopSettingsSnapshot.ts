import { invoke } from "@tauri-apps/api/core";
import type { SettingsSnapshot } from "@cialloclaw/protocol";

/**
 * Reports whether the current renderer can sync settings into the desktop host
 * snapshot cache.
 */
export function canUseDesktopSettingsSnapshot() {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;
}

/**
 * Pushes the latest formal settings snapshot into the desktop host cache so
 * platform bridges can reuse it without re-requesting RPC settings.
 *
 * @param settings Formal settings payload from the shared RPC snapshot.
 */
export async function syncDesktopSettingsSnapshot(settings: SettingsSnapshot["settings"]) {
  if (!canUseDesktopSettingsSnapshot()) {
    return;
  }

  await invoke("desktop_sync_settings_snapshot", { settings });
}
