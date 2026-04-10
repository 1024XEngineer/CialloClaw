import type {
  AgentSecuritySummaryGetResult,
  AgentSettingsUpdateResult,
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
import { loadSettings, saveSettings, type DesktopSettings } from "@/services/settingsService";

export type ControlPanelSource = "rpc" | "mock";

export type ControlPanelData = {
  settings: SettingsSnapshot["settings"];
  inspector: AgentTaskInspectorConfigGetResult;
  securitySummary: AgentSecuritySummaryGetResult["summary"];
  source: ControlPanelSource;
};

export type ControlPanelSaveResult = {
  applyMode: ApplyMode;
  needRestart: boolean;
  updatedKeys: string[];
  effectiveSettings: Partial<SettingsSnapshot["settings"]>;
  effectiveInspector: AgentTaskInspectorConfigGetResult;
  source: ControlPanelSource;
};

function projectInspectorToTaskAutomation(
  settings: SettingsSnapshot["settings"],
  inspector: AgentTaskInspectorConfigGetResult,
): SettingsSnapshot["settings"] {
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
  return {
    settings: projectInspectorToTaskAutomation(settings.settings, inspector),
    inspector,
    securitySummary: buildMockSecuritySummary(),
    source: "mock",
  };
}

export async function loadControlPanelData(): Promise<ControlPanelData> {
  try {
    const requestMeta = createRequestMeta();
    const [settingsResult, inspectorResult, securityResult] = await Promise.all([
      getSettings({ request_meta: requestMeta, scope: "all" }),
      getTaskInspectorConfig({ request_meta: createRequestMeta() }),
      getSecuritySummary({ request_meta: createRequestMeta() }),
    ]);

    return {
      settings: projectInspectorToTaskAutomation(settingsResult.settings, inspectorResult),
      inspector: inspectorResult,
      securitySummary: securityResult.summary,
      source: "rpc",
    };
  } catch (error) {
    console.warn("Control panel RPC unavailable, using local settings fallback.", error);
    return getInitialControlPanelData();
  }
}

export async function saveControlPanelData(data: ControlPanelData): Promise<ControlPanelSaveResult> {
  if (data.source === "mock") {
    const nextSettings: DesktopSettings = {
      settings: projectInspectorToTaskAutomation(data.settings, data.inspector),
    };
    saveSettings(nextSettings);
    return {
      applyMode: "immediate",
      needRestart: false,
      updatedKeys: ["general", "floating_ball", "memory", "task_automation", "data_log"],
      effectiveSettings: nextSettings.settings,
      effectiveInspector: data.inspector,
      source: "mock",
    };
  }

  const [settingsResult, inspectorResult] = await Promise.all([
    updateSettings({
      request_meta: createRequestMeta(),
      general: data.settings.general,
      floating_ball: data.settings.floating_ball,
      memory: data.settings.memory,
      data_log: data.settings.data_log,
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

  return {
    applyMode: settingsResult.apply_mode,
    needRestart: settingsResult.need_restart,
    updatedKeys: settingsResult.updated_keys,
    effectiveSettings: projectInspectorToTaskAutomation(settingsResult.effective_settings as SettingsSnapshot["settings"], inspectorResult.effective_config),
    effectiveInspector: inspectorResult.effective_config,
    source: "rpc",
  };
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

  return runTaskInspector({
    request_meta: createRequestMeta(),
    reason: "control_panel_manual_run",
    target_sources: data.inspector.task_sources,
  });
}
