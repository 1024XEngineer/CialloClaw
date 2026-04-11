import { Window } from "@tauri-apps/api/window";

export type DesktopWindowLabel = "dashboard" | "control-panel";

// 该文件封装桌面窗口控制能力。
export async function openOrFocusDesktopWindow(label: DesktopWindowLabel) {
  const windowHandle = await Window.getByLabel(label);

  if (windowHandle === null) {
    throw new Error(`Desktop window not found: ${label}`);
  }

  await windowHandle.unminimize();
  await windowHandle.show();
  await windowHandle.setFocus();

  return label;
}
