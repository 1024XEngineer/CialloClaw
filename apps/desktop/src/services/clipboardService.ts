import { readText } from "@tauri-apps/plugin-clipboard-manager";

/**
 * Reads the current system clipboard text through the official Tauri clipboard
 * plugin.
 *
 * @returns The current clipboard text. Returns an empty string when the plugin
 *          yields no text.
 */
export async function readClipboardText() {
  const text = await readText();
  return typeof text === "string" ? text : "";
}
