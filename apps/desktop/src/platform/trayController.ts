import { invoke } from "@tauri-apps/api/core";

// 该文件封装托盘入口控制能力。
export function openControlPanelFromTray() {
  return invoke<void>("desktop_open_or_focus_control_panel");
}
