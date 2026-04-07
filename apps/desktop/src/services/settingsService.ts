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
        },
        task_automation: {
          inspect_on_startup: true,
          inspect_on_file_change: true,
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
