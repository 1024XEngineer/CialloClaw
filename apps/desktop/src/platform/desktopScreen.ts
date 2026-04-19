import { invoke } from "@tauri-apps/api/core";

type DesktopScreenCapturePayload = {
  path: string;
  relative_path: string;
  width: number;
  height: number;
  captured_at: string;
};

/**
 * Captures the current desktop screen through the Tauri host and stores the
 * image inside `apps/.temp`.
 *
 * @returns Metadata describing the saved screenshot file.
 */
export async function captureDesktopScreenshot() {
  return invoke<DesktopScreenCapturePayload>("desktop_capture_screenshot");
}
