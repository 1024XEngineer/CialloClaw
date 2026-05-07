import { invoke } from "@tauri-apps/api/core";

export type DesktopClipboardActivitySnapshotPayload = {
  copy_count: number;
};

/**
 * Reads the recent desktop copy-activity snapshot tracked by the Tauri host.
 *
 * @returns The recent copy-count metadata, or `null` when the host has not
 *          recorded any recent clipboard-copy interaction yet.
 */
export async function getDesktopClipboardActivitySnapshot() {
  return invoke<DesktopClipboardActivitySnapshotPayload | null>("desktop_get_clipboard_activity_snapshot");
}
