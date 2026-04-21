import { invoke } from "@tauri-apps/api/core";

/**
 * Opens a local file-system target through the desktop host so the operating
 * system can choose the default app or folder handler.
 *
 * @param path Local file or directory path returned by the formal delivery flow.
 */
export async function openDesktopLocalPath(path: string) {
  return invoke<void>("desktop_open_local_path", { path });
}

/**
 * Reveals a local file-system target through the desktop host. File targets are
 * revealed in the platform file manager, while directories are opened directly.
 *
 * @param path Local file or directory path returned by the formal delivery flow.
 */
export async function revealDesktopLocalPath(path: string) {
  return invoke<void>("desktop_reveal_local_path", { path });
}
