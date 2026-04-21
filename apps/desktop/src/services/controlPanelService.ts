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
import { isRpcChannelUnavailable, logRpcMockFallback } from "@/rpc/fallback";
import {
  hydrateDesktopSettings,
  loadSettings,
  saveSettings,
  type DesktopSettings,
  type DesktopSettingsData,
} from "@/services/settingsService";

export type ControlPanelSource = "rpc" | "mock";

export type ControlPanelData = {
  settings: DesktopSettingsData;
  inspector: AgentTaskInspectorConfigGetResult;
  securitySummary: AgentSecuritySummaryGetResult["summary"];
  providerApiKeyInput: string;
  source: ControlPanelSource;
};

export type ControlPanelSaveResult = {
  applyMode: ApplyMode;
  needRestart: boolean;
  updatedKeys: string[];
  effectiveSettings: Partial<DesktopSettingsData>;
  effectiveInspector: AgentTaskInspectorConfigGetResult;
  source: ControlPanelSource;
};

function buildEditableFloatingBallUpdate(
  floatingBall: DesktopSettingsData["floating_ball"],
): Partial<SettingsSnapshot["settings"]["floating_ball"]> {
  return {
    idle_translucent: floatingBall.idle_translucent,
    position_mode: floatingBall.position_mode,
  };
}

/**
 * Floating-ball size and edge snapping are still owned by the shell-ball work.
 * Preserve the last persisted values so control-panel saves only update the
 * fields that are already safe to edit from this window.
 *
 * @param nextSettings Draft settings produced by the control panel flow.
 * @param persistedSettings Last persisted desktop snapshot.
 * @returns Settings with detached floating-ball fields restored.
 */
function preserveDetachedFloatingBallFields(
  nextSettings: DesktopSettingsData,
  persistedSettings: DesktopSettingsData,
): DesktopSettingsData {
  return hydrateDesktopSettings({
    ...nextSettings,
    floating_ball: {
      ...nextSettings.floating_ball,
      auto_snap: persistedSettings.floating_ball.auto_snap,
      size: persistedSettings.floating_ball.size,
    },
  });
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
    }),
  };
}

function buildSettingsWithProviderApiKeyConfigured(
  settings: DesktopSettingsData,
  providerApiKeyConfigured: boolean,
): DesktopSettingsData {
  return hydrateDesktopSettings({
    ...settings,
    data_log: {
      ...settings.data_log,
      provider: settings.models.provider,
      budget_auto_downgrade: settings.models.budget_auto_downgrade,
      provider_api_key_configured: providerApiKeyConfigured,
    },
    models: {
      ...settings.models,
      provider_api_key_configured: providerApiKeyConfigured,
    },
  });
}

function buildDataLogUpdatePayload(input: ControlPanelData) {
  const apiKey = input.providerApiKeyInput.trim();

  return {
    provider: input.settings.models.provider,
    budget_auto_downgrade: input.settings.models.budget_auto_downgrade,
    ...(apiKey === "" ? {} : { api_key: apiKey }),
  };
}

function createRequestMeta(): RequestMeta {
  return {
    trace_id: `trace_control_panel_${Date.now()}`,
    client_time: new Date().toISOString(),
  };
}

function buildMockSecuritySummary(): AgentSecuritySummaryGetResult["summary"] {
  return {
    security_status: "pending_confirmation",
    pending_authorizations: 2,
    latest_restore_point: {
      recovery_point_id: "cp_restore_mock_001",
      task_id: "task_control_panel_mock_001",
      summary: "控制面板变更前的恢复点快照",
      created_at: "2026-04-09T13:20:00+08:00",
      objects: ["D:/CialloClawWorkspace", "VS Code", "Docker sandbox"],
    },
    token_cost_summary: {
      current_task_tokens: 18210,
      current_task_cost: 1.34,
      today_tokens: 91230,
      today_cost: 7.48,
      single_task_limit: 50000,
      daily_limit: 300000,
      budget_auto_downgrade: true,
    },
  };
}

function buildMockInspector(settings: DesktopSettings): AgentTaskInspectorConfigGetResult {
  return {
    task_sources: settings.settings.task_automation.task_sources,
    inspection_interval: settings.settings.task_automation.inspection_interval,
    inspect_on_file_change: settings.settings.task_automation.inspect_on_file_change,
    inspect_on_startup: settings.settings.task_automation.inspect_on_startup,
    remind_before_deadline: settings.settings.task_automation.remind_before_deadline,
    remind_when_stale: settings.settings.task_automation.remind_when_stale,
  };
}

function getInitialControlPanelData(): ControlPanelData {
  const settings = loadSettings();
  const inspector = buildMockInspector(settings);
  const normalizedSettings = projectInspectorToTaskAutomation(settings.settings, inspector);
  return {
    settings: normalizedSettings,
    inspector,
    providerApiKeyInput: "",
    securitySummary: buildMockSecuritySummary(),
    source: "mock",
  };
}

export async function loadControlPanelData(): Promise<ControlPanelData> {
  try {
    const requestMeta = createRequestMeta();
    const localSettings = loadSettings().settings;
    const [settingsResult, inspectorResult, securityResult] = await Promise.all([
      getSettings({ request_meta: requestMeta, scope: "all" }),
      getTaskInspectorConfig({ request_meta: createRequestMeta() }),
      getSecuritySummary({ request_meta: createRequestMeta() }),
    ]);

    const effectiveSettings = projectInspectorToTaskAutomation(
      mergeProtocolSettings(localSettings, settingsResult.settings),
      inspectorResult,
    );
    saveSettings({ settings: effectiveSettings });

    return {
      settings: effectiveSettings,
      inspector: inspectorResult,
      providerApiKeyInput: "",
      securitySummary: securityResult.summary,
      source: "rpc",
    };
  } catch (error) {
    console.warn("Control panel RPC unavailable, using local settings fallback.", error);
    return getInitialControlPanelData();
  }
}

export async function saveControlPanelData(data: ControlPanelData): Promise<ControlPanelSaveResult> {
  const persistedSettings = loadSettings().settings;

  if (data.source === "mock") {
    const nextSettingsSnapshot = preserveDetachedFloatingBallFields(
      buildSettingsWithProviderApiKeyConfigured(
        projectInspectorToTaskAutomation(data.settings, data.inspector),
        data.settings.models.provider_api_key_configured,
      ),
      persistedSettings,
    );
    const nextDesktopSettings: DesktopSettings = {
      settings: nextSettingsSnapshot,
    };
    saveSettings(nextDesktopSettings);
    return {
      applyMode: "immediate",
      needRestart: false,
      updatedKeys: ["general", "floating_ball", "memory", "task_automation", "models"],
      effectiveSettings: nextDesktopSettings.settings,
      effectiveInspector: data.inspector,
      source: "mock",
    };
  }

  try {
    const [settingsResult, inspectorResult] = await Promise.all([
      updateSettings({
        request_meta: createRequestMeta(),
        general: data.settings.general,
        floating_ball: buildEditableFloatingBallUpdate(data.settings.floating_ball),
        memory: data.settings.memory,
        data_log: buildDataLogUpdatePayload(data),
      }),
      updateTaskInspectorConfig({
        request_meta: createRequestMeta(),
        task_sources: data.inspector.task_sources,
        inspection_interval: data.inspector.inspection_interval,
        inspect_on_file_change: data.inspector.inspect_on_file_change,
        inspect_on_startup: data.inspector.inspect_on_startup,
        remind_before_deadline: data.inspector.remind_before_deadline,
        remind_when_stale: data.inspector.remind_when_stale,
      }),
    ]);

    const effectiveSettings = preserveDetachedFloatingBallFields(
      projectInspectorToTaskAutomation(
        mergeProtocolSettings(data.settings, settingsResult.effective_settings as Partial<SettingsSnapshot["settings"]>),
        inspectorResult.effective_config,
      ),
      persistedSettings,
    );
    saveSettings({ settings: effectiveSettings });

    return {
      applyMode: settingsResult.apply_mode,
      needRestart: settingsResult.need_restart,
      updatedKeys: settingsResult.updated_keys,
      effectiveSettings,
      effectiveInspector: inspectorResult.effective_config,
      source: "rpc",
    };
  } catch (error) {
    if (!isRpcChannelUnavailable(error)) {
      throw error;
    }

    logRpcMockFallback("control panel save", error);
    const nextSettingsSnapshot = preserveDetachedFloatingBallFields(
      buildSettingsWithProviderApiKeyConfigured(
        projectInspectorToTaskAutomation(data.settings, data.inspector),
        data.settings.models.provider_api_key_configured,
      ),
      persistedSettings,
    );
    const nextDesktopSettings: DesktopSettings = {
      settings: nextSettingsSnapshot,
    };
    saveSettings(nextDesktopSettings);
    return {
      applyMode: "immediate",
      needRestart: false,
      updatedKeys: ["general", "floating_ball", "memory", "task_automation", "models"],
      effectiveSettings: nextDesktopSettings.settings,
      effectiveInspector: data.inspector,
      source: "mock",
    };
  }
}

export async function runControlPanelInspection(data: ControlPanelData): Promise<AgentTaskInspectorRunResult> {
  if (data.source === "mock") {
    return {
      inspection_id: `inspection_mock_${Date.now()}`,
      summary: {
        parsed_files: 16,
        identified_items: 9,
        due_today: 2,
        overdue: 1,
        stale: 3,
      },
      suggestions: [
        "将 overdue 任务提升到今日工作流卡片。",
        "把高频 task source 固定到前两位，减少巡检噪音。",
      ],
    };
  }

  try {
    return await runTaskInspector({
      request_meta: createRequestMeta(),
      reason: "control_panel_manual_run",
      target_sources: data.inspector.task_sources,
    });
  } catch (error) {
    if (!isRpcChannelUnavailable(error)) {
      throw error;
    }

    logRpcMockFallback("control panel inspection", error);
    return {
      inspection_id: `inspection_mock_${Date.now()}`,
      summary: {
        parsed_files: 16,
        identified_items: 9,
        due_today: 2,
        overdue: 1,
        stale: 3,
      },
      suggestions: [
        "将 overdue 任务提升到今日工作流卡片。",
        "把高频 task source 固定到前两位，减少巡检噪音。",
      ],
    };
  }
}
