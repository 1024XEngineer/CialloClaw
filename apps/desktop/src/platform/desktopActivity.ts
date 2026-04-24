import { invoke } from "@tauri-apps/api/core";

export type DesktopMouseActivitySnapshotPayload = {
  updated_at: string;
};

/**
 * Reads the latest desktop mouse-activity snapshot tracked by the Tauri host.
 *
 * @returns The latest mouse activity metadata, or `null` when the host has not
 *          recorded any recent interaction yet.
 */
export async function getDesktopMouseActivitySnapshot() {
  return invoke<DesktopMouseActivitySnapshotPayload | null>("desktop_get_mouse_activity_snapshot");
}
