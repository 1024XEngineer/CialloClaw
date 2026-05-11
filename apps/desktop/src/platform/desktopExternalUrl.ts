import { invoke } from "@tauri-apps/api/core";

/**
 * Opens one trusted external http/https URL through the desktop host so links
 * leave the embedded WebView and use the system default browser instead.
 *
 * @param url External browser target returned by formal delivery or note resources.
 */
export async function openDesktopExternalUrl(url: string) {
  return invoke<void>("desktop_open_external_url", { url });
}
