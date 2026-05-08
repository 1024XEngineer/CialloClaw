import { invoke } from "@tauri-apps/api/core";

/**
 * Opens an external web url through the desktop host so the system default
 * browser, not the embedded WebView, owns the navigation.
 */
export async function openDesktopExternalUrl(url: string) {
  await invoke<void>("desktop_open_external_url", { url });
}
