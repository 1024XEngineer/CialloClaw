import type { AgentSettingsUpdateParams, ApplyMode, RequestMeta } from "@cialloclaw/protocol";
import { updateSettings as requestUpdateSettings } from "@/rpc/methods";
import { loadSettings, saveSettings, toProtocolSettingsSnapshot, type DesktopSettings } from "@/services/settingsService";
import {
  loadDashboardSettingsSnapshot,
  type DashboardSettingsSnapshotData,
  type DashboardSettingsSnapshotScope,
  type DashboardSettingsSource,
} from "./dashboardSettingsSnapshot";

type DashboardModelPatch = AgentSettingsUpdateParams["models"] | Partial<DesktopSettings["settings"]["models"]>;

export type DashboardSettingsPatch = Pick<
  AgentSettingsUpdateParams,
  "general" | "floating_ball" | "memory" | "task_automation"
> & {
  models?: DashboardModelPatch;
};

function normalizeDashboardModelPatch(modelPatch: DashboardModelPatch | undefined) {
  if (!modelPatch) {
    return undefined;
  }

  const formalCredentials =
    "credentials" in modelPatch && typeof modelPatch.credentials === "object" && modelPatch.credentials !== null
      ? modelPatch.credentials
      : undefined;
  const providerApiKeyConfigured =
    "provider_api_key_configured" in modelPatch ? modelPatch.provider_api_key_configured : undefined;
  const stronghold = "stronghold" in modelPatch ? modelPatch.stronghold : undefined;

  return {
    ...(modelPatch.provider === undefined ? {} : { provider: modelPatch.provider }),
    ...(formalCredentials?.budget_auto_downgrade === undefined && modelPatch.budget_auto_downgrade === undefined
      ? {}
      : {
          budget_auto_downgrade: formalCredentials?.budget_auto_downgrade ?? modelPatch.budget_auto_downgrade,
        }),
    ...(formalCredentials?.provider_api_key_configured === undefined && providerApiKeyConfigured === undefined
      ? {}
      : {
          provider_api_key_configured: formalCredentials?.provider_api_key_configured ?? providerApiKeyConfigured,
        }),
    ...(formalCredentials?.stronghold === undefined && stronghold === undefined
      ? {}
      : { stronghold: formalCredentials?.stronghold ?? stronghold }),
    ...(formalCredentials?.base_url === undefined && modelPatch.base_url === undefined
      ? {}
      : { base_url: formalCredentials?.base_url ?? modelPatch.base_url }),
    ...(formalCredentials?.model === undefined && modelPatch.model === undefined
      ? {}
      : { model: formalCredentials?.model ?? modelPatch.model }),
  } satisfies Partial<DesktopSettings["settings"]["models"]>;
}

function buildRpcModelPatch(modelPatch: DashboardModelPatch | undefined): AgentSettingsUpdateParams["models"] | undefined {
  const normalizedModelPatch = normalizeDashboardModelPatch(modelPatch);

  if (!modelPatch || !normalizedModelPatch) {
    return undefined;
  }

  return {
    ...(normalizedModelPatch.provider === undefined ? {} : { provider: normalizedModelPatch.provider }),
    ...(normalizedModelPatch.budget_auto_downgrade === undefined
      ? {}
      : { budget_auto_downgrade: normalizedModelPatch.budget_auto_downgrade }),
    ...(normalizedModelPatch.base_url === undefined ? {} : { base_url: normalizedModelPatch.base_url }),
    ...(normalizedModelPatch.model === undefined ? {} : { model: normalizedModelPatch.model }),
    ...("api_key" in modelPatch && typeof modelPatch.api_key === "string" ? { api_key: modelPatch.api_key } : {}),
    ...("delete_api_key" in modelPatch && typeof modelPatch.delete_api_key === "boolean"
      ? { delete_api_key: modelPatch.delete_api_key }
      : {}),
  };
}

export type DashboardSettingsMutationResult = {
  snapshot: DashboardSettingsSnapshotData;
  applyMode: ApplyMode;
  needRestart: boolean;
  updatedKeys: string[];
  source: DashboardSettingsSource;
  persisted: boolean;
  readbackWarning: string | null;
};

function createRequestMeta(): RequestMeta {
  return {
    trace_id: `trace_dashboard_settings_update_${Date.now()}`,
    client_time: new Date().toISOString(),
  };
}

function mergeSettingsSnapshot(
  current: DesktopSettings["settings"],
  patch: DashboardSettingsPatch,
): DesktopSettings["settings"] {
  const modelPatch = normalizeDashboardModelPatch(patch.models);

  return {
    ...current,
    general: patch.general
      ? {
          ...current.general,
          ...patch.general,
          download: {
            ...current.general.download,
            ...(patch.general.download ?? {}),
          },
        }
      : current.general,
    floating_ball: patch.floating_ball
      ? {
          ...current.floating_ball,
          ...patch.floating_ball,
        }
      : current.floating_ball,
    memory: patch.memory
      ? {
          ...current.memory,
          ...patch.memory,
          work_summary_interval: {
            ...current.memory.work_summary_interval,
            ...(patch.memory.work_summary_interval ?? {}),
          },
          profile_refresh_interval: {
            ...current.memory.profile_refresh_interval,
            ...(patch.memory.profile_refresh_interval ?? {}),
          },
        }
      : current.memory,
    task_automation: patch.task_automation
      ? {
          ...current.task_automation,
          ...patch.task_automation,
          inspection_interval: {
            ...current.task_automation.inspection_interval,
            ...(patch.task_automation.inspection_interval ?? {}),
          },
        }
      : current.task_automation,
    models: modelPatch
      ? {
          ...current.models,
          ...modelPatch,
        }
      : current.models,
  };
}

function buildRpcSettingsPatch(patch: DashboardSettingsPatch): AgentSettingsUpdateParams {
  return {
    request_meta: createRequestMeta(),
    general: patch.general,
    floating_ball: patch.floating_ball,
    memory: patch.memory,
    task_automation: patch.task_automation,
    models: buildRpcModelPatch(patch.models),
  };
}

function persistPatchedSettings(patch: DashboardSettingsPatch) {
  const current = loadSettings();
  const nextSettings: DesktopSettings = {
    settings: mergeSettingsSnapshot(current.settings, patch),
  };

  saveSettings(nextSettings);
  return nextSettings;
}

/**
 * Builds one settings snapshot from the just-persisted desktop settings when
 * the formal `agent.settings.get` readback fails after a successful write.
 *
 * @param persistedSettings The local desktop settings that already include the
 * just-applied effective settings returned by `agent.settings.update`.
 * @param warning The readback failure message that should stay visible in UI.
 * @returns A snapshot aligned with the saved settings plus a readback warning.
 */
function buildPersistedSettingsSnapshot(
  persistedSettings: DesktopSettings,
  warning: string,
): DashboardSettingsSnapshotData {
  return {
    settings: toProtocolSettingsSnapshot(persistedSettings.settings),
    source: "rpc",
    rpcContext: {
      serverTime: null,
      warnings: [warning],
    },
  };
}

function inferDashboardSettingsRefreshScope(patch: DashboardSettingsPatch): DashboardSettingsSnapshotScope {
  const touchedScopes = new Set<DashboardSettingsSnapshotScope>();

  if (patch.general) {
    touchedScopes.add("general");
  }
  if (patch.floating_ball) {
    touchedScopes.add("floating_ball");
  }
  if (patch.memory) {
    touchedScopes.add("memory");
  }
  if (patch.task_automation) {
    touchedScopes.add("task_automation");
  }
  if (patch.models) {
    touchedScopes.add("models");
  }

  if (touchedScopes.size !== 1) {
    return "all";
  }

  return touchedScopes.values().next().value ?? "all";
}

/**
 * Updates dashboard-facing settings while keeping the local desktop snapshot in sync.
 *
 * @param patch The changed settings groups to persist.
 * @param source The preferred settings transport.
 * @returns The refreshed dashboard settings snapshot and persistence metadata.
 */
export async function updateDashboardSettings(
  patch: DashboardSettingsPatch,
  _source: DashboardSettingsSource = "rpc",
): Promise<DashboardSettingsMutationResult> {
  const response = await requestUpdateSettings(buildRpcSettingsPatch(patch));
  const refreshScope = inferDashboardSettingsRefreshScope(patch);

  const persistedSettings = persistPatchedSettings(response.effective_settings as DashboardSettingsPatch);

  try {
    const snapshot = await loadDashboardSettingsSnapshot("rpc", refreshScope);

    return {
      snapshot,
      applyMode: response.apply_mode,
      needRestart: response.need_restart,
      updatedKeys: response.updated_keys,
      source: snapshot.source,
      persisted: true,
      readbackWarning: null,
    };
  } catch (error) {
    const readbackWarning = error instanceof Error ? error.message : "settings readback unavailable";
    const snapshot = buildPersistedSettingsSnapshot(persistedSettings, readbackWarning);

    return {
      snapshot,
      applyMode: response.apply_mode,
      needRestart: response.need_restart,
      updatedKeys: response.updated_keys,
      source: snapshot.source,
      persisted: true,
      readbackWarning,
    };
  }
}

/**
 * Formats one user-facing feedback string for a completed settings mutation.
 *
 * @param result The mutation result returned by `updateDashboardSettings`.
 * @param subject The UI subject label that changed.
 * @returns A localized feedback string for toast or inline status UI.
 */
export function formatDashboardSettingsMutationFeedback(result: DashboardSettingsMutationResult, subject: string) {
  if (!result.persisted) {
    return `${subject}未保存，当前仅显示本地快照。`;
  }

  const readbackSuffix = result.readbackWarning
    ? ` 设置已写入，但 settings.get 回读失败：${result.readbackWarning}。当前先展示刚保存的本地快照。`
    : "";

  if (result.needRestart || result.applyMode === "restart_required") {
    return `${subject}已保存，重启桌面端后生效。${readbackSuffix}`;
  }

  if (result.applyMode === "next_task_effective") {
    return `${subject}已保存，将在下一任务周期生效。${readbackSuffix}`;
  }

  return `${subject}已更新。${readbackSuffix}`;
}
