import type {
  AgentSecuritySummaryGetResult,
  AgentSettingsModelValidateResult,
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
  validateSettingsModel,
  updateSettings,
  updateTaskInspectorConfig,
} from "@/rpc/methods";
import { isRpcChannelUnavailable } from "@/rpc/fallback";
import {
  buildDefaultDesktopSettingsSnapshot,
  hydrateDesktopSettings,
  loadDesktopRuntimeDefaultsSnapshot,
  loadSettings,
  saveSettings,
  type DesktopSettingsData,
} from "@/services/settingsService";

export type ControlPanelSource = "rpc";

export type ControlPanelData = {
  settings: DesktopSettingsData;
  inspector: AgentTaskInspectorConfigGetResult;
  securitySummary: AgentSecuritySummaryGetResult["summary"];
  runtimeWorkspacePath: string | null;
  providerApiKeyInput: string;
  source: ControlPanelSource;
  warnings?: string[];
};

export type ControlPanelSaveResult = {
  applyMode: ApplyMode;
  needRestart: boolean;
  updatedKeys: string[];
  warnings: string[];
  modelValidation: AgentSettingsModelValidateResult | null;
  effectiveSettings: Partial<DesktopSettingsData>;
  effectiveInspector: AgentTaskInspectorConfigGetResult;
  savedInspector: boolean;
  savedSettings: boolean;
  source: ControlPanelSource;
};

export type ControlPanelSaveOptions = {
  confirmedInspector?: AgentTaskInspectorConfigGetResult;
  saveInspector?: boolean;
  saveSettings?: boolean;
  validateModel?: boolean;
  timeoutMs?: number;
};

export type ControlPanelModelValidationOptions = {
  timeoutMs?: number;
};

type ControlPanelSaveErrorKind = "general" | "model_validation_failed";

const CONTROL_PANEL_RPC_TIMEOUT_MS = 10_000;

const CONTROL_PANEL_INSPECTOR_UPDATED_KEYS = [
  "task_automation.task_sources",
  "task_automation.inspection_interval",
  "task_automation.inspect_on_file_change",
  "task_automation.inspect_on_startup",
  "task_automation.remind_before_deadline",
  "task_automation.remind_when_stale",
];

/**
 * ControlPanelSaveError reports a save failure while optionally carrying the
 * already-applied subset so the UI can preserve successful groups.
 */
export class ControlPanelSaveError extends Error {
  readonly partialResult: ControlPanelSaveResult | null;
  readonly kind: ControlPanelSaveErrorKind;

  constructor(message: string, partialResult: ControlPanelSaveResult | null = null, kind: ControlPanelSaveErrorKind = "general") {
    super(message);
    this.name = "ControlPanelSaveError";
    this.partialResult = partialResult;
    this.kind = kind;
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

/**
 * Builds a restore-defaults draft while preserving workspace-bound task
 * sources, the active model route, and any already-saved provider secret state
 * that lives outside the ordinary settings snapshot.
 *
 * @param currentDraft The current control-panel draft.
 * @param persisted The last persisted control-panel snapshot.
 * @returns A draft aligned with desktop defaults and persisted boundary fields.
 */
export function buildControlPanelRestoreDefaultsData(currentDraft: ControlPanelData, persisted: ControlPanelData): ControlPanelData {
  const defaultSettings = buildDefaultDesktopSettingsSnapshot().settings;
  const preservedTaskSources = persisted.inspector.task_sources;
  const preservedModels = persisted.settings.models;

  return {
    ...currentDraft,
    inspector: {
      task_sources: preservedTaskSources,
      inspection_interval: defaultSettings.task_automation.inspection_interval,
      inspect_on_file_change: defaultSettings.task_automation.inspect_on_file_change,
      inspect_on_startup: defaultSettings.task_automation.inspect_on_startup,
      remind_before_deadline: defaultSettings.task_automation.remind_before_deadline,
      remind_when_stale: defaultSettings.task_automation.remind_when_stale,
    },
    providerApiKeyInput: "",
    settings: {
      ...defaultSettings,
      general: {
        ...defaultSettings.general,
        download: {
          ...defaultSettings.general.download,
          workspace_path: persisted.settings.general.download.workspace_path,
        },
      },
      task_automation: {
        ...defaultSettings.task_automation,
        task_sources: preservedTaskSources,
      },
      models: {
        ...defaultSettings.models,
        provider: preservedModels.provider,
        provider_api_key_configured: preservedModels.provider_api_key_configured,
        stronghold: preservedModels.stronghold,
        base_url: preservedModels.base_url,
        model: preservedModels.model,
      },
    },
    warnings: [],
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

  return [];
}

function shouldValidateSavedModelRoute(updatedKeys: string[], providerApiKeyInput: string) {
  return updatedKeys.some((key) => key.startsWith("models.")) || providerApiKeyInput.trim() !== "";
}

function buildModelValidatePayload(input: ControlPanelData) {
  return {
    request_meta: createRequestMeta(),
    models: buildModelsUpdatePayload(input),
  };
}

async function runControlPanelModelValidation(
  data: ControlPanelData,
  timeoutMs: number,
): Promise<AgentSettingsModelValidateResult> {
  return withRpcTimeout(
    validateSettingsModel(buildModelValidatePayload(data)),
    timeoutMs,
    "模型配置校验",
  );
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
    modelValidation?: AgentSettingsModelValidateResult | null;
  },
): ControlPanelSaveResult {
  return {
    applyMode: options.applyMode,
    needRestart: options.needRestart,
    updatedKeys: options.updatedKeys,
    warnings: options.warnings ?? [],
    modelValidation: options.modelValidation ?? null,
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
): Promise<Pick<ControlPanelData, "inspector" | "runtimeWorkspacePath" | "securitySummary" | "settings">> {
  const runtimeDefaults = await loadDesktopRuntimeDefaultsSnapshot();
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
    runtimeWorkspacePath: runtimeDefaults?.workspace_path ?? null,
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
 * Model-route changes run through a read-only validation probe before the
 * formal settings write so invalid provider/base-url/model/key drafts never
 * reach the persisted snapshot.
 */
export async function saveControlPanelData(
  data: ControlPanelData,
  options: ControlPanelSaveOptions = {},
): Promise<ControlPanelSaveResult> {
  const confirmedInspector = options.confirmedInspector ?? data.inspector;
  const saveSettingsRequested = options.saveSettings ?? true;
  const saveInspectorRequested = options.saveInspector ?? true;
  const validateModelRequested = options.validateModel ?? saveSettingsRequested;
  const timeoutMs = options.timeoutMs ?? CONTROL_PANEL_RPC_TIMEOUT_MS;

  if (!saveSettingsRequested && !saveInspectorRequested) {
    return buildControlPanelSaveResult(data.settings, data.inspector, data.source, {
      applyMode: "immediate",
      needRestart: false,
      savedInspector: false,
      savedSettings: false,
      updatedKeys: [],
      modelValidation: null,
    });
  }

  let applyMode: ApplyMode = "immediate";
  let effectiveInspector = confirmedInspector;
  let effectiveSettings = data.settings;
  let needRestart = false;
  let savedInspector = false;
  let savedSettings = false;
  const updatedKeys: string[] = [];
  const warnings: string[] = [];
  let modelValidation: AgentSettingsModelValidateResult | null = null;

  try {
    if (saveSettingsRequested && validateModelRequested) {
      modelValidation = await validateControlPanelModel(data, { timeoutMs });
      if (!modelValidation.ok) {
        throw new ControlPanelSaveError(`${modelValidation.message} 当前设置未保存。`, null, "model_validation_failed");
      }
    }

    if (saveSettingsRequested) {
      const settingsResult = await withRpcTimeout(updateSettings(buildSettingsUpdatePayload(data)), timeoutMs, "设置保存");
      effectiveSettings = mergeProtocolSettings(data.settings, settingsResult.effective_settings as Partial<SettingsSnapshot["settings"]>);
      effectiveSettings = projectInspectorToTaskAutomation(effectiveSettings, effectiveInspector);
      applyMode = settingsResult.apply_mode;
      needRestart = settingsResult.need_restart;
      savedSettings = true;
      updatedKeys.push(...settingsResult.updated_keys);
      warnings.push(...buildControlPanelSaveWarnings(data.settings.models.provider, settingsResult.updated_keys, data.providerApiKeyInput));
      if (!modelValidation && shouldValidateSavedModelRoute(settingsResult.updated_keys, data.providerApiKeyInput)) {
        modelValidation = await validateControlPanelModel(data, { timeoutMs });
      }
      if (!saveInspectorRequested) {
        saveSettings({ settings: effectiveSettings });
      }
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
          saveSettings({ settings: effectiveSettings });
          throw new ControlPanelSaveError(
            `通用设置已保存，但巡检设置保存失败：${error instanceof Error ? error.message : "请重试。"}`,
            buildControlPanelSaveResult(effectiveSettings, effectiveInspector, "rpc", {
              applyMode,
              needRestart,
              savedInspector: false,
              savedSettings: true,
              updatedKeys,
              warnings,
              modelValidation,
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
      modelValidation,
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
 * validateControlPanelModel runs the formal model-route probe without mutating
 * the saved settings snapshot, so the UI can distinguish saved-vs-usable state.
 */
export async function validateControlPanelModel(
  data: ControlPanelData,
  options: ControlPanelModelValidationOptions = {},
): Promise<AgentSettingsModelValidateResult> {
  const timeoutMs = options.timeoutMs ?? CONTROL_PANEL_RPC_TIMEOUT_MS;

  try {
    return await runControlPanelModelValidation(data, timeoutMs);
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      throw new Error("模型配置校验服务暂时不可用，请稍后重试。");
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
