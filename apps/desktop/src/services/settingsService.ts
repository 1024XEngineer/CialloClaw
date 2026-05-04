// settingsService centralizes desktop settings persistence.
import type { SettingsSnapshot } from "@cialloclaw/protocol";
import {
  readDesktopRuntimeDefaults,
  type DesktopRuntimeDefaults,
} from "../platform/desktopRuntimeDefaults";
import { syncDesktopSettingsSnapshot } from "../platform/desktopSettingsSnapshot";
import { loadStoredValue, saveStoredValue } from "../platform/storage";

// SETTINGS_KEY is the single storage key for the desktop snapshot.
const SETTINGS_KEY = "cialloclaw.settings";
const RUNTIME_DEFAULTS_KEY = "cialloclaw.runtime-defaults";
const DEFAULT_WORKSPACE_PLACEHOLDER = "workspace";
const DEFAULT_TASK_SOURCE_PLACEHOLDER = ["workspace/todos"];
// These legacy defaults remain as migration sentinels only so packaged startup
// can recognize and rewrite historical snapshots created before runtime-managed
// host defaults were available.
const LEGACY_DEFAULT_WORKSPACE_PATH = "D:/CialloClawWorkspace";
const LEGACY_DEFAULT_TASK_SOURCES = ["D:/workspace/todos"];

type ProtocolSettings = SettingsSnapshot["settings"];
type ProtocolModelSettings = ProtocolSettings["models"];
type ProtocolModelCredentials = ProtocolModelSettings["credentials"];

export type DesktopModelSettings = {
  provider: string;
  budget_auto_downgrade: boolean;
  provider_api_key_configured: boolean;
  stronghold: ProtocolModelCredentials["stronghold"];
  base_url: string;
  model: string;
};

export type DesktopSettingsData = Omit<ProtocolSettings, "models"> & {
  models: DesktopModelSettings;
};

export type DesktopSettings = {
  settings: DesktopSettingsData;
};

type StoredDesktopModelSettings = Partial<DesktopModelSettings> &
  Partial<Omit<ProtocolModelSettings, "credentials">> & {
    credentials?: Partial<ProtocolModelCredentials>;
  };

type StoredDesktopSettings = {
  settings?: Partial<Omit<DesktopSettingsData, "models">> & {
    models?: StoredDesktopModelSettings;
  };
};

function loadRuntimeDefaults(): DesktopRuntimeDefaults | null {
  if (typeof window === "undefined") {
    return null;
  }

  const stored = loadStoredValue<DesktopRuntimeDefaults>(RUNTIME_DEFAULTS_KEY);
  if (!stored || typeof stored.workspace_path !== "string") {
    return null;
  }

  const dataPath = typeof stored.data_path === "string" ? stored.data_path.trim() : "";
  const workspacePath = stored.workspace_path.trim();
  const taskSources = Array.isArray(stored.task_sources)
    ? stored.task_sources
        .filter((source): source is string => typeof source === "string")
        .map((source) => source.trim())
        .filter((source) => source.length > 0)
    : [];

  if (workspacePath.length === 0) {
    return null;
  }

  return {
    data_path: dataPath,
    workspace_path: workspacePath,
    task_sources: taskSources,
  };
}

function createDefaultSettings(): DesktopSettings {
  const runtimeDefaults = loadRuntimeDefaults();

  return {
    settings: {
      general: {
        language: "zh-CN",
        auto_launch: true,
        theme_mode: "follow_system",
        voice_notification_enabled: true,
        voice_type: "default_female",
        download: {
          workspace_path: runtimeDefaults?.workspace_path ?? DEFAULT_WORKSPACE_PLACEHOLDER,
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
        task_sources: runtimeDefaults?.task_sources.length ? runtimeDefaults.task_sources : DEFAULT_TASK_SOURCE_PLACEHOLDER,
        remind_before_deadline: true,
        remind_when_stale: false,
      },
      models: {
        provider: "openai",
        budget_auto_downgrade: true,
        provider_api_key_configured: false,
        stronghold: {
          backend: "none",
          available: false,
          fallback: false,
          initialized: false,
          formal_store: false,
        },
        base_url: "https://api.openai.com/v1",
        model: "gpt-3.5-turbo",
      },
    },
  };
}

/**
 * Builds the default desktop settings snapshot using the latest cached runtime
 * defaults so local reset flows stay aligned with the packaged host baseline.
 *
 * @returns The default desktop settings snapshot.
 */
export function buildDefaultDesktopSettingsSnapshot(): DesktopSettings {
  return createDefaultSettings();
}

// The desktop control panel keeps a flat local `models` view while the formal
// RPC snapshot exposes `models.credentials`. Normalize both shapes into one
// storage-friendly desktop snapshot.
function normalizeSettingsSnapshot(
  snapshot: StoredDesktopSettings | SettingsSnapshot | DesktopSettings | null | undefined,
): DesktopSettings {
  const defaults = createDefaultSettings();
  const settings = snapshot?.settings as StoredDesktopSettings["settings"];
  const storedModels = settings?.models;
  const storedModelCredentials =
    storedModels && "credentials" in storedModels && typeof storedModels.credentials === "object" && storedModels.credentials !== null
      ? storedModels.credentials
      : undefined;
  const normalizedModels: DesktopModelSettings = {
    ...defaults.settings.models,
    ...(storedModels?.provider === undefined ? {} : { provider: storedModels.provider }),
    budget_auto_downgrade:
      storedModelCredentials?.budget_auto_downgrade ??
      storedModels?.budget_auto_downgrade ??
      defaults.settings.models.budget_auto_downgrade,
    provider_api_key_configured:
      storedModelCredentials?.provider_api_key_configured ??
      storedModels?.provider_api_key_configured ??
      defaults.settings.models.provider_api_key_configured,
    stronghold:
      storedModelCredentials?.stronghold ?? storedModels?.stronghold ?? defaults.settings.models.stronghold,
    base_url: storedModelCredentials?.base_url ?? storedModels?.base_url ?? defaults.settings.models.base_url,
    model: storedModelCredentials?.model ?? storedModels?.model ?? defaults.settings.models.model,
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
      models: normalizedModels,
    },
  };
}

function usesLegacyWorkspaceDefault(workspacePath: string) {
  const normalized = workspacePath.trim().replaceAll("\\", "/");
  return (
    normalized.length === 0 ||
    normalized === "workspace" ||
    normalized === LEGACY_DEFAULT_WORKSPACE_PATH
  );
}

function usesLegacyTaskSourceDefaults(taskSources: string[]) {
  if (taskSources.length !== 1) {
    return false;
  }

  // Only the historical single-root placeholder should be replaced during
  // runtime-default hydration. User-owned workspace-relative source layouts
  // must stay intact even when they happen to live under `workspace/...`.
  const normalized = taskSources[0]?.trim().replaceAll("\\", "/").toLowerCase() ?? "";
  return normalized === LEGACY_DEFAULT_TASK_SOURCES[0].toLowerCase() || normalized === DEFAULT_TASK_SOURCE_PLACEHOLDER[0];
}

/**
 * Hydrates runtime-default workspace paths from the trusted desktop host and
 * rewrites legacy local placeholders to the canonical packaged defaults.
 */
export async function hydrateDesktopRuntimeDefaults() {
  let runtimeDefaults: DesktopRuntimeDefaults | null = null;
  try {
    runtimeDefaults = await readDesktopRuntimeDefaults();
  } catch (error) {
    console.warn("Failed to hydrate desktop runtime defaults from the Tauri host.", error);
    return null;
  }
  if (!runtimeDefaults || runtimeDefaults.workspace_path.trim().length === 0) {
    return null;
  }

  const normalizedRuntimeDefaults: DesktopRuntimeDefaults = {
    data_path: typeof runtimeDefaults.data_path === "string" ? runtimeDefaults.data_path.trim() : "",
    workspace_path: runtimeDefaults.workspace_path.trim(),
    task_sources: runtimeDefaults.task_sources
      .filter((source): source is string => typeof source === "string")
      .map((source) => source.trim())
      .filter((source) => source.length > 0),
  };
  saveStoredValue(RUNTIME_DEFAULTS_KEY, normalizedRuntimeDefaults);

  const current = loadSettings();
  const shouldReplaceWorkspace = usesLegacyWorkspaceDefault(current.settings.general.download.workspace_path);
  const shouldReplaceTaskSources = usesLegacyTaskSourceDefaults(current.settings.task_automation.task_sources);
  if (!shouldReplaceWorkspace && !shouldReplaceTaskSources) {
    return normalizedRuntimeDefaults;
  }

  saveSettings({
    settings: {
      ...current.settings,
      general: {
        ...current.settings.general,
        download: {
          ...current.settings.general.download,
          workspace_path: shouldReplaceWorkspace
            ? normalizedRuntimeDefaults.workspace_path
            : current.settings.general.download.workspace_path,
        },
      },
      task_automation: {
        ...current.settings.task_automation,
        task_sources:
          shouldReplaceTaskSources && normalizedRuntimeDefaults.task_sources.length > 0
            ? normalizedRuntimeDefaults.task_sources
            : current.settings.task_automation.task_sources,
      },
    },
  });

  return normalizedRuntimeDefaults;
}

/**
 * Loads the persisted desktop settings after best-effort runtime-default
 * hydration so fallback UI surfaces prefer trusted host paths over legacy
 * packaged placeholders when the RPC channel is temporarily unavailable.
 */
export async function loadHydratedSettings(): Promise<DesktopSettings> {
  await hydrateDesktopRuntimeDefaults();
  return loadSettings();
}

/**
 * Loads the trusted runtime-default directory snapshot cached for desktop-only
 * control-panel affordances.
 *
 * @returns The hydrated runtime-default directory snapshot, when available.
 */
export async function loadDesktopRuntimeDefaultsSnapshot(): Promise<DesktopRuntimeDefaults | null> {
  await hydrateDesktopRuntimeDefaults();
  return loadRuntimeDefaults();
}

/**
 * Hydrates shared RPC settings into the desktop-local settings shape.
 *
 * @param settings Shared or desktop-local settings snapshot.
 * @returns Normalized desktop settings data with flattened model credentials.
 */
export function hydrateDesktopSettings(settings: ProtocolSettings | DesktopSettingsData): DesktopSettingsData {
  return normalizeSettingsSnapshot({ settings: settings as StoredDesktopSettings["settings"] }).settings;
}

/**
 * Projects the desktop-local settings shape back into the formal protocol
 * snapshot shape used by RPC-facing consumers.
 *
 * @param settings The desktop-local settings snapshot.
 * @returns The formal protocol settings snapshot.
 */
export function toProtocolSettingsSnapshot(settings: DesktopSettingsData): ProtocolSettings {
  return {
    general: settings.general,
    floating_ball: settings.floating_ball,
    memory: settings.memory,
    task_automation: settings.task_automation,
    models: {
      provider: settings.models.provider,
      credentials: {
        budget_auto_downgrade: settings.models.budget_auto_downgrade,
        provider_api_key_configured: settings.models.provider_api_key_configured,
        base_url: settings.models.base_url,
        model: settings.models.model,
        stronghold: settings.models.stronghold,
      },
    },
  };
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
 * Best-effort host snapshot sync keeps the Tauri-side cache close to the local
 * desktop snapshot without letting transient bridge failures surface as
 * unhandled promise rejections after the local save already succeeded.
 *
 * @param settings The formal protocol snapshot that the desktop host should cache.
 */
function syncDesktopSettingsSnapshotSafely(settings: ProtocolSettings) {
  void syncDesktopSettingsSnapshot(settings).catch((error) => {
    console.warn("Failed to sync desktop settings snapshot to the Tauri host cache.", error);
  });
}

/**
 * Persists the latest desktop settings snapshot.
 *
 * @param settings The desktop settings snapshot to store locally.
 */
export function saveSettings(settings: DesktopSettings) {
  const normalizedSettings = normalizeSettingsSnapshot(settings);
  saveStoredValue(SETTINGS_KEY, normalizedSettings);
  syncDesktopSettingsSnapshotSafely(toProtocolSettingsSnapshot(normalizedSettings.settings));
}
