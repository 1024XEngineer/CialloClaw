import { invoke } from "@tauri-apps/api/core";

export type DesktopWindowContextPayload = {
  app_name: string;
  process_path: string | null;
  process_id: number | null;
  title: string | null;
  url: string | null;
  browser_kind: "chrome" | "edge" | "other_browser" | "non_browser";
  window_switch_count?: number | null;
  page_switch_count?: number | null;
};

/**
 * Reads the current active desktop window context from the host platform
 * adapter.
 *
 * @returns The foreground window context, or `null` when no active window could
 *          be resolved.
 */
export async function getActiveWindowContext() {
  return invoke<DesktopWindowContextPayload | null>("desktop_get_active_window_context");
}
