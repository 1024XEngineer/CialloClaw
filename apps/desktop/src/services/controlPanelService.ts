import type {
  AgentSecuritySummaryGetResult,
  AgentTaskInspectorConfigGetResult,
  AgentTaskInspectorRunResult,
  ApplyMode,
  RequestMeta,
  SettingsSnapshot,
} from "@cialloclaw/protocol";
import {
  getSecuritySummary,
  getSettings,
  getTaskInspectorConfig,
  runTaskInspector,
  updateSettings,
  updateTaskInspectorConfig,
} from "@/rpc/methods";
import { isRpcChannelUnavailable } from "@/rpc/fallback";
import {
  hydrateDesktopSettings,
  loadSettings,
  saveSettings,
  type DesktopSettingsData,
} from "@/services/settingsService";

export type ControlPanelSource = "rpc";

export type ControlPanelData = {
  settings: DesktopSettingsData;
  inspector: AgentTaskInspectorConfigGetResult;
  securitySummary: AgentSecuritySummaryGetResult["summary"];
  providerApiKeyInput: string;
  source: ControlPanelSource;
  warnings?: string[];
};

export type ControlPanelSaveResult = {
  applyMode: ApplyMode;
  needRestart: boolean;
  updatedKeys: string[];
  warnings: string[];
  effectiveSettings: Partial<DesktopSettingsData>;
  effectiveInspector: AgentTaskInspectorConfigGetResult;
  savedInspector: boolean;
  savedSettings: boolean;
  source: ControlPanelSource;
};

export type ControlPanelSaveOptions = {
  saveInspector?: boolean;
  saveSettings?: boolean;
  timeoutMs?: number;
};

const CONTROL_PANEL_RPC_TIMEOUT_MS = 10_000;

const CONTROL_PANEL_INSPECTOR_UPDATED_KEYS = [
  "task_automation.task_sources",
  "task_automation.inspection_interval",
  "task_automation.inspect_on_file_change",
  "task_automation.inspect_on_startup",
  "task_automation.remind_before_deadline",
  "task_automation.remind_when_stale",
];

const CONTROL_PANEL_SUPPORTED_MODEL_PROVIDERS = new Set(["openai", "openai_responses"]);

/**
 * ControlPanelSaveError reports a save failure while optionally carrying the
 * already-applied subset so the UI can preserve successful groups.
 */
export class ControlPanelSaveError extends Error {
  readonly partialResult: ControlPanelSaveResult | null;

  constructor(message: string, partialResult: ControlPanelSaveResult | null = null) {
    super(message);
    this.name = "ControlPanelSaveError";
    this.partialResult = partialResult;
  }
}

function buildEditableFloatingBallUpdate(
  floatingBall: DesktopSettingsData["floating_ball"],
): Partial<SettingsSnapshot["settings"]["floating_ball"]> {
  return floatingBall;
}

function projectInspectorToTaskAutomation(
  settings: DesktopSettingsData,
  inspector: AgentTaskInspectorConfigGetResult,
): DesktopSettingsData {
  return {
    ...settings,
    task_automation: {
      ...settings.task_automation,
      task_sources: inspector.task_sources,
      inspection_interval: inspector.inspection_interval,
      inspect_on_file_change: inspector.inspect_on_file_change,
      inspect_on_startup: inspector.inspect_on_startup,
      remind_before_deadline: inspector.remind_before_deadline,
      remind_when_stale: inspector.remind_when_stale,
    },
  };
}

function mergeProtocolSettings(
  base: DesktopSettingsData,
  patch: Partial<SettingsSnapshot["settings"]>,
): DesktopSettingsData {
  return {
    ...hydrateDesktopSettings({
      ...base,
      general: patch.general
        ? {
            ...base.general,
            ...patch.general,
            download: {
              ...base.general.download,
              ...(patch.general.download ?? {}),
            },
          }
        : base.general,
      floating_ball: patch.floating_ball
        ? {
            ...base.floating_ball,
            ...patch.floating_ball,
          }
        : base.floating_ball,
      memory: patch.memory
        ? {
            ...base.memory,
            ...patch.memory,
            work_summary_interval: {
              ...base.memory.work_summary_interval,
              ...(patch.memory.work_summary_interval ?? {}),
            },
            profile_refresh_interval: {
              ...base.memory.profile_refresh_interval,
              ...(patch.memory.profile_refresh_interval ?? {}),
            },
          }
        : base.memory,
      task_automation: patch.task_automation
        ? {
            ...base.task_automation,
            ...patch.task_automation,
            inspection_interval: {
              ...base.task_automation.inspection_interval,
              ...(patch.task_automation.inspection_interval ?? {}),
            },
          }
        : base.task_automation,
      data_log: patch.data_log
        ? {
            ...base.data_log,
            ...patch.data_log,
          }
        : base.data_log,
      models: patch.models
        ? {
            ...base.models,
            ...patch.models,
            ...(patch.models.credentials ?? {}),
          }
        : base.models,
    }),
  };
}

function buildModelsUpdatePayload(input: ControlPanelData) {
  const apiKey = input.providerApiKeyInput.trim();

  return {
    provider: input.settings.models.provider,
    budget_auto_downgrade: input.settings.models.budget_auto_downgrade,
    base_url: input.settings.models.base_url,
    model: input.settings.models.model,
    ...(apiKey === "" ? {} : { api_key: apiKey }),
  };
}

function buildSettingsUpdatePayload(input: ControlPanelData) {
  return {
    request_meta: createRequestMeta(),
    general: input.settings.general,
    floating_ball: buildEditableFloatingBallUpdate(input.settings.floating_ball),
    memory: input.settings.memory,
    models: buildModelsUpdatePayload(input),
  };
}

function normalizeControlPanelModelProvider(provider: string): string {
  return provider.trim().toLowerCase();
}

function buildControlPanelSaveWarnings(
  provider: string,
  updatedKeys: string[],
  providerApiKeyInput: string,
): string[] {
  const normalizedProvider = normalizeControlPanelModelProvider(provider);
  if (normalizedProvider === "") {
    return [];
  }

  const providerChanged = updatedKeys.includes("models.provider");
  const apiKeySaved = providerApiKeyInput.trim() !== "";
  if (!providerChanged && !apiKeySaved) {
    return [];
  }

  if (CONTROL_PANEL_SUPPORTED_MODEL_PROVIDERS.has(normalizedProvider)) {
    return [];
  }

  return ["API Key 已保存，但当前 provider 名暂不受支持；请改用 openai / openai_responses。"];
}

function buildInspectorUpdatePayload(input: ControlPanelData) {
  return {
    request_meta: createRequestMeta(),
    task_sources: input.inspector.task_sources,
    inspection_interval: input.inspector.inspection_interval,
    inspect_on_file_change: input.inspector.inspect_on_file_change,
    inspect_on_startup: input.inspector.inspect_on_startup,
    remind_before_deadline: input.inspector.remind_before_deadline,
    remind_when_stale: input.inspector.remind_when_stale,
  };
}

// The desktop bridge currently has no abort channel, so timeout only releases
// the UI from waiting forever and lets the user retry instead of hanging.
function withRpcTimeout<T>(promise: Promise<T>, timeoutMs: number, actionLabel: string): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => {
      reject(new Error(`${actionLabel}请求超时，请重试。`));
    }, timeoutMs);

    void promise.then(
      (value) => {
        clearTimeout(timer);
        resolve(value);
      },
      (error: unknown) => {
        clearTimeout(timer);
        reject(error);
      },
    );
  });
}

function buildControlPanelSaveResult(
  settings: DesktopSettingsData,
  inspector: AgentTaskInspectorConfigGetResult,
  source: ControlPanelSource,
  options: {
    applyMode: ApplyMode;
    needRestart: boolean;
    savedInspector: boolean;
    savedSettings: boolean;
    updatedKeys: string[];
    warnings?: string[];
  },
): ControlPanelSaveResult {
  return {
    applyMode: options.applyMode,
    needRestart: options.needRestart,
    updatedKeys: options.updatedKeys,
    warnings: options.warnings ?? [],
    effectiveSettings: settings,
    effectiveInspector: inspector,
    savedInspector: options.savedInspector,
    savedSettings: options.savedSettings,
    source,
  };
}

function createRequestMeta(): RequestMeta {
  return {
    trace_id: `trace_control_panel_${Date.now()}`,
    client_time: new Date().toISOString(),
  };
}

/**
 * Reads the latest formal control-panel snapshot from the RPC boundary.
 *
 * The local desktop snapshot only provides a compatibility merge baseline for
 * aliased fields; the returned view model is still sourced from formal RPC data.
 *
 * @param timeoutMs Optional timeout guard for the RPC reads.
 * @returns The latest control-panel settings, inspector config, and security summary.
 */
async function loadControlPanelRpcSnapshot(
  timeoutMs: number = CONTROL_PANEL_RPC_TIMEOUT_MS,
): Promise<Pick<ControlPanelData, "inspector" | "securitySummary" | "settings">> {
  const requestMeta = createRequestMeta();
  const localSettings = loadSettings().settings;
  const [settingsResult, inspectorResult, securityResult] = await Promise.all([
    withRpcTimeout(getSettings({ request_meta: requestMeta, scope: "all" }), timeoutMs, "设置读取"),
    withRpcTimeout(getTaskInspectorConfig({ request_meta: createRequestMeta() }), timeoutMs, "巡检设置读取"),
    withRpcTimeout(getSecuritySummary({ request_meta: createRequestMeta() }), timeoutMs, "安全摘要读取"),
  ]);

  const effectiveSettings = projectInspectorToTaskAutomation(
    mergeProtocolSettings(localSettings, settingsResult.settings),
    inspectorResult,
  );

  return {
    settings: effectiveSettings,
    inspector: inspectorResult,
    securitySummary: securityResult.summary,
  };
}

/**
 * loadControlPanelData hydrates the control panel from the formal RPC boundary.
 */
export async function loadControlPanelData(): Promise<ControlPanelData> {
  try {
    const snapshot = await loadControlPanelRpcSnapshot();
    saveSettings({ settings: snapshot.settings });

    return {
      ...snapshot,
      providerApiKeyInput: "",
      source: "rpc",
      warnings: [],
    };
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      throw new Error("控制面板设置服务暂时不可用，请稍后重试。");
    }

    throw error;
  }
}

/**
 * saveControlPanelData persists only the dirty settings groups requested by
 * the caller so unrelated RPC writes do not block the entire settings surface.
 * The save path applies effective update payloads directly; formal readback
 * stays on the open/load path instead of adding extra save-time RPC reads.
 */
export async function saveControlPanelData(
  data: ControlPanelData,
  options: ControlPanelSaveOptions = {},
): Promise<ControlPanelSaveResult> {
  const saveSettingsRequested = options.saveSettings ?? true;
  const saveInspectorRequested = options.saveInspector ?? true;
  const timeoutMs = options.timeoutMs ?? CONTROL_PANEL_RPC_TIMEOUT_MS;

  if (!saveSettingsRequested && !saveInspectorRequested) {
    return buildControlPanelSaveResult(data.settings, data.inspector, data.source, {
      applyMode: "immediate",
      needRestart: false,
      savedInspector: false,
      savedSettings: false,
      updatedKeys: [],
    });
  }

  let applyMode: ApplyMode = "immediate";
  let effectiveInspector = data.inspector;
  let effectiveSettings = data.settings;
  let needRestart = false;
  let savedInspector = false;
  let savedSettings = false;
  const updatedKeys: string[] = [];
  const warnings: string[] = [];

  try {
    if (saveSettingsRequested) {
      const settingsResult = await withRpcTimeout(updateSettings(buildSettingsUpdatePayload(data)), timeoutMs, "设置保存");
      effectiveSettings = mergeProtocolSettings(data.settings, settingsResult.effective_settings as Partial<SettingsSnapshot["settings"]>);
      effectiveSettings = projectInspectorToTaskAutomation(effectiveSettings, effectiveInspector);
      applyMode = settingsResult.apply_mode;
      needRestart = settingsResult.need_restart;
      savedSettings = true;
      updatedKeys.push(...settingsResult.updated_keys);
      saveSettings({ settings: effectiveSettings });
      warnings.push(...buildControlPanelSaveWarnings(data.settings.models.provider, settingsResult.updated_keys, data.providerApiKeyInput));
    }

    if (saveInspectorRequested) {
      try {
        const inspectorResult = await withRpcTimeout(
          updateTaskInspectorConfig(buildInspectorUpdatePayload(data)),
          timeoutMs,
          "巡检设置保存",
        );
        effectiveInspector = inspectorResult.effective_config;
        effectiveSettings = projectInspectorToTaskAutomation(effectiveSettings, effectiveInspector);
        savedInspector = true;
        updatedKeys.push(...CONTROL_PANEL_INSPECTOR_UPDATED_KEYS);
        saveSettings({ settings: effectiveSettings });
      } catch (error) {
        if (savedSettings) {
          throw new ControlPanelSaveError(
            `通用设置已保存，但巡检设置保存失败：${error instanceof Error ? error.message : "请重试。"}`,
            buildControlPanelSaveResult(effectiveSettings, effectiveInspector, "rpc", {
              applyMode,
              needRestart,
              savedInspector: false,
              savedSettings: true,
              updatedKeys,
              warnings,
            }),
          );
        }

        throw error;
      }
    }

    return buildControlPanelSaveResult(effectiveSettings, effectiveInspector, "rpc", {
      applyMode,
      needRestart,
      savedInspector,
      savedSettings,
      updatedKeys,
      warnings,
    });
  } catch (error) {
    if (error instanceof ControlPanelSaveError) {
      throw error;
    }

    if (isRpcChannelUnavailable(error)) {
      throw new ControlPanelSaveError("设置服务暂时不可用，请稍后重试。");
    }

    throw error;
  }
}

/**
 * runControlPanelInspection triggers one manual inspection pass from the
 * current control-panel state.
 */
export async function runControlPanelInspection(data: ControlPanelData): Promise<AgentTaskInspectorRunResult> {
  try {
    return await withRpcTimeout(
      runTaskInspector({
        request_meta: createRequestMeta(),
        reason: "control_panel_manual_run",
        target_sources: data.inspector.task_sources,
      }),
      CONTROL_PANEL_RPC_TIMEOUT_MS,
      "巡检执行",
    );
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      throw new Error("巡检服务暂时不可用，请稍后重试。");
    }

    throw error;
  }
}
