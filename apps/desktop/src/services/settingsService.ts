// settingsService centralizes desktop settings persistence.
import type { SettingsSnapshot } from "@cialloclaw/protocol";
import { loadStoredValue, saveStoredValue } from "../platform/storage";

// SETTINGS_KEY is the single storage key for the desktop snapshot.
const SETTINGS_KEY = "cialloclaw.settings";

type ProtocolSettings = SettingsSnapshot["settings"];
type ProtocolDataLogSettings = ProtocolSettings["data_log"];

export type DesktopModelSettings = ProtocolDataLogSettings & {
  base_url: string;
  model: string;
};

export type DesktopSettingsData = Omit<ProtocolSettings, "data_log"> & {
  data_log: ProtocolDataLogSettings;
  models: DesktopModelSettings;
};

export type DesktopSettings = {
  settings: DesktopSettingsData;
};

type StoredDataLogSettings = Partial<ProtocolDataLogSettings> & {
  base_url?: string;
  model?: string;
};

type StoredDesktopSettings = {
  settings?: Partial<Omit<DesktopSettingsData, "data_log" | "models">> & {
    data_log?: StoredDataLogSettings;
    models?: Partial<DesktopModelSettings>;
  };
};

function createDefaultSettings(): DesktopSettings {
  return {
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
        provider_api_key_configured: false,
        stronghold: {
          backend: "stronghold_sqlite_fallback",
          available: true,
          fallback: true,
          initialized: true,
          formal_store: false,
        },
      },
      models: {
        provider: "openai",
        budget_auto_downgrade: true,
        provider_api_key_configured: false,
        stronghold: {
          backend: "stronghold_sqlite_fallback",
          available: true,
          fallback: true,
          initialized: true,
          formal_store: false,
        },
        base_url: "https://api.openai.com/v1",
        model: "gpt-3.5-turbo",
      },
    },
  };
}

// The desktop control panel carries local-only model routing fields while RPC
// still persists the shared `data_log` subset. Keep both shapes synchronized so
// older snapshots and new UI state read back consistently.
function normalizeSettingsSnapshot(
  snapshot: StoredDesktopSettings | SettingsSnapshot | DesktopSettings | null | undefined,
): DesktopSettings {
  const defaults = createDefaultSettings();
  const settings = snapshot?.settings as StoredDesktopSettings["settings"];
  const storedDataLog = settings?.data_log;
  const storedModels = settings?.models;
  const normalizedDataLog: ProtocolDataLogSettings = {
    ...defaults.settings.data_log,
    ...storedDataLog,
    // Shared RPC fields stay authoritative in `data_log`; the desktop-local
    // `models` view mirrors them instead of overriding backend snapshots.
    provider: storedDataLog?.provider ?? storedModels?.provider ?? defaults.settings.data_log.provider,
    budget_auto_downgrade:
      storedDataLog?.budget_auto_downgrade ??
      storedModels?.budget_auto_downgrade ??
      defaults.settings.data_log.budget_auto_downgrade,
    provider_api_key_configured:
      storedDataLog?.provider_api_key_configured ??
      storedModels?.provider_api_key_configured ??
      defaults.settings.data_log.provider_api_key_configured,
    stronghold: storedDataLog?.stronghold ?? storedModels?.stronghold ?? defaults.settings.data_log.stronghold,
  };
  const normalizedModels: DesktopModelSettings = {
    ...defaults.settings.models,
    ...(storedDataLog?.base_url === undefined ? {} : { base_url: storedDataLog.base_url }),
    ...(storedDataLog?.model === undefined ? {} : { model: storedDataLog.model }),
    ...storedModels,
    provider: normalizedDataLog.provider,
    budget_auto_downgrade: normalizedDataLog.budget_auto_downgrade,
    provider_api_key_configured: normalizedDataLog.provider_api_key_configured,
    stronghold: normalizedDataLog.stronghold,
  };

  return {
    settings: {
      general: {
        ...defaults.settings.general,
        ...settings?.general,
        download: {
          ...defaults.settings.general.download,
          ...settings?.general?.download,
        },
      },
      floating_ball: {
        ...defaults.settings.floating_ball,
        ...settings?.floating_ball,
      },
      memory: {
        ...defaults.settings.memory,
        ...settings?.memory,
        work_summary_interval: {
          ...defaults.settings.memory.work_summary_interval,
          ...settings?.memory?.work_summary_interval,
        },
        profile_refresh_interval: {
          ...defaults.settings.memory.profile_refresh_interval,
          ...settings?.memory?.profile_refresh_interval,
        },
      },
      task_automation: {
        ...defaults.settings.task_automation,
        ...settings?.task_automation,
        inspection_interval: {
          ...defaults.settings.task_automation.inspection_interval,
          ...settings?.task_automation?.inspection_interval,
        },
      },
      data_log: normalizedDataLog,
      models: normalizedModels,
    },
  };
}

/**
 * Hydrates shared RPC settings into the desktop-local settings shape.
 *
 * @param settings Shared or desktop-local settings snapshot.
 * @returns Normalized desktop settings data with synchronized `data_log` and `models`.
 */
export function hydrateDesktopSettings(settings: ProtocolSettings | DesktopSettingsData): DesktopSettingsData {
  return normalizeSettingsSnapshot({ settings: settings as StoredDesktopSettings["settings"] }).settings;
}

/**
 * Loads the persisted desktop settings snapshot.
 *
 * @returns A normalized settings snapshot for desktop UI consumers.
 */
export function loadSettings(): DesktopSettings {
  return normalizeSettingsSnapshot(loadStoredValue<StoredDesktopSettings>(SETTINGS_KEY));
}

/**
 * Persists the latest desktop settings snapshot.
 *
 * @param settings The desktop settings snapshot to store locally.
 */
export function saveSettings(settings: DesktopSettings) {
  saveStoredValue(SETTINGS_KEY, normalizeSettingsSnapshot(settings));
}
