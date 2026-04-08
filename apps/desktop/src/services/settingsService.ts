// 该文件封装前端设置读写服务。 
import type { SettingsSnapshot } from "@cialloclaw/protocol";
import { loadStoredValue, saveStoredValue } from "@/platform/storage";

const SETTINGS_KEY = "cialloclaw.settings";

export type DesktopSettings = SettingsSnapshot;

export function loadSettings(): DesktopSettings {
  return (
    loadStoredValue<DesktopSettings>(SETTINGS_KEY) ?? {
      settings: {
        general: {
          language: "zh-CN",
          auto_launch: true,
          theme_mode: "follow_system",
          voice_notification_enabled: true,
          voice_type: "default_female",
          download: {
            workspace_path: "D:/CialloClawWorkspace",
            ask_before_save_each_file: true,
          },
        },
        floating_ball: {
          auto_snap: true,
          idle_translucent: true,
          position_mode: "draggable",
          size: "medium",
        },
        memory: {
          enabled: true,
          lifecycle: "30d",
          work_summary_interval: {
            unit: "day",
            value: 7,
          },
          profile_refresh_interval: {
            unit: "week",
            value: 2,
          },
        },
        task_automation: {
          inspect_on_startup: true,
          inspect_on_file_change: true,
          inspection_interval: {
            unit: "minute",
            value: 15,
          },
          task_sources: ["D:/workspace/todos"],
          remind_before_deadline: true,
          remind_when_stale: false,
        },
        data_log: {
          provider: "openai",
          budget_auto_downgrade: true,
        },
      },
    }
  );
}

export function saveSettings(settings: DesktopSettings) {
  saveStoredValue(SETTINGS_KEY, settings);
}
