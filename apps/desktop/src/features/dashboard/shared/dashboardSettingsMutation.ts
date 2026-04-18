import type { AgentSettingsUpdateParams, ApplyMode, RequestMeta } from "@cialloclaw/protocol";
import { isRpcChannelUnavailable, logRpcMockFallback } from "@/rpc/fallback";
import { updateSettings as requestUpdateSettings } from "@/rpc/methods";
import { loadSettings, saveSettings, type DesktopSettings } from "@/services/settingsService";
import { loadDashboardSettingsSnapshot, type DashboardSettingsSnapshotData, type DashboardSettingsSource } from "./dashboardSettingsSnapshot";

export type DashboardSettingsPatch = Pick<
  AgentSettingsUpdateParams,
  "general" | "floating_ball" | "memory" | "task_automation" | "data_log"
>;

export type DashboardSettingsMutationResult = {
  snapshot: DashboardSettingsSnapshotData;
  applyMode: ApplyMode;
  needRestart: boolean;
  updatedKeys: string[];
  source: DashboardSettingsSource;
  persisted: boolean;
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
    data_log: patch.data_log
      ? {
          ...current.data_log,
          ...patch.data_log,
        }
      : current.data_log,
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

function inferUpdatedKeys(patch: DashboardSettingsPatch) {
  return (Object.keys(patch) as Array<keyof DashboardSettingsPatch>).filter((key) => patch[key] !== undefined).map((key) => String(key));
}

// Dashboard modules need the same settings mutation rule as the control panel:
// use JSON-RPC when available, but keep the local snapshot authoritative for
// immediate rendering and mock-mode operation.
export async function updateDashboardSettings(
  patch: DashboardSettingsPatch,
  source: DashboardSettingsSource = "rpc",
): Promise<DashboardSettingsMutationResult> {
  if (source === "mock") {
    persistPatchedSettings(patch);

    return {
      snapshot: await loadDashboardSettingsSnapshot("mock"),
      applyMode: "immediate",
      needRestart: false,
      updatedKeys: inferUpdatedKeys(patch),
      source: "mock",
      persisted: true,
    };
  }

  try {
    const response = await requestUpdateSettings({
      request_meta: createRequestMeta(),
      ...patch,
    });

    persistPatchedSettings(response.effective_settings as DashboardSettingsPatch);
    const snapshot = await loadDashboardSettingsSnapshot("rpc");

    return {
      snapshot,
      applyMode: response.apply_mode,
      needRestart: response.need_restart,
      updatedKeys: response.updated_keys,
      source: snapshot.source,
      persisted: true,
    };
  } catch (error) {
    if (!isRpcChannelUnavailable(error)) {
      throw error;
    }

    logRpcMockFallback("dashboard settings update", error);
    const snapshot = await loadDashboardSettingsSnapshot("mock");

    return {
      snapshot,
      applyMode: "immediate",
      needRestart: false,
      updatedKeys: [],
      source: snapshot.source,
      persisted: false,
    };
  }
}

export function formatDashboardSettingsMutationFeedback(result: DashboardSettingsMutationResult, subject: string) {
  if (!result.persisted) {
    return `${subject}未保存，当前仅显示本地快照。`;
  }

  const suffix = result.source === "mock" ? " 当前使用本地快照。" : "";

  if (result.needRestart || result.applyMode === "restart_required") {
    return `${subject}已保存，重启桌面端后生效。${suffix}`;
  }

  if (result.applyMode === "next_task_effective") {
    return `${subject}已保存，将在下一任务周期生效。${suffix}`;
  }

  return `${subject}已更新。${suffix}`;
}
