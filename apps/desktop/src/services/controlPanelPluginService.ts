import type {
  AgentPluginDetailGetResult,
  PluginListItem,
  PluginRuntimeState,
  RequestMeta,
} from "@cialloclaw/protocol";
import { getPluginDetail, listPlugins } from "@/rpc/methods";
import { isRpcChannelUnavailable } from "@/rpc/fallback";
import { loadStoredValue, saveStoredValue } from "@/platform/storage";

const CONTROL_PANEL_PLUGIN_PAGE_LIMIT = 100;
const CONTROL_PANEL_PLUGIN_MOCK_STORAGE_KEY = "cialloclaw.control-panel.plugin-mocks";

type StoredControlPanelPluginMockState = {
  enabled: boolean;
  updated_at: string;
};

type StoredControlPanelPluginMockMap = Record<string, StoredControlPanelPluginMockState>;

export type ControlPanelPluginControlState = {
  baseline_enabled: boolean;
  effective_enabled: boolean;
  source: "live" | "mock";
  updated_at: string | null;
};

export type ControlPanelPluginSummary = {
  plugin: PluginListItem;
  capability_count: number;
  permission_count: number;
  runtime_count: number;
  runtime_health: PluginRuntimeState["health"];
  control: ControlPanelPluginControlState;
};

export type ControlPanelPluginSnapshot = {
  items: ControlPanelPluginSummary[];
  summary: {
    total: number;
    healthy: number;
    degraded: number;
    failed: number;
    stopped: number;
    unavailable: number;
    unknown: number;
    live_enabled: number;
    mock_overrides: number;
  };
};

export type ControlPanelPluginDetail = AgentPluginDetailGetResult & {
  control: ControlPanelPluginControlState;
};

function createRequestMeta(): RequestMeta {
  return {
    trace_id: `trace_control_panel_plugin_${Date.now()}`,
    client_time: new Date().toISOString(),
  };
}

function loadStoredPluginMocks() {
  return loadStoredValue<StoredControlPanelPluginMockMap>(CONTROL_PANEL_PLUGIN_MOCK_STORAGE_KEY) ?? {};
}

function saveStoredPluginMocks(snapshot: StoredControlPanelPluginMockMap) {
  saveStoredValue(CONTROL_PANEL_PLUGIN_MOCK_STORAGE_KEY, snapshot);
}

function resolvePluginControlState(
  pluginId: string,
  baselineEnabled: boolean,
  storedMocks: StoredControlPanelPluginMockMap,
): ControlPanelPluginControlState {
  const stored = storedMocks[pluginId];

  if (!stored || stored.enabled === baselineEnabled) {
    return {
      baseline_enabled: baselineEnabled,
      effective_enabled: baselineEnabled,
      source: "live",
      updated_at: null,
    };
  }

  return {
    baseline_enabled: baselineEnabled,
    effective_enabled: stored.enabled,
    source: "mock",
    updated_at: stored.updated_at,
  };
}

function getRuntimeHealthSeverity(health: PluginRuntimeState["health"]) {
  switch (health) {
    case "failed":
      return 5;
    case "degraded":
      return 4;
    case "unavailable":
      return 3;
    case "stopped":
      return 2;
    case "healthy":
      return 1;
    default:
      return 0;
  }
}

function resolvePluginRuntimeHealth(runtimes: PluginRuntimeState[]): PluginRuntimeState["health"] {
  if (runtimes.length === 0) {
    return "unknown";
  }

  return [...runtimes].sort((left, right) => getRuntimeHealthSeverity(right.health) - getRuntimeHealthSeverity(left.health))[0].health;
}

function buildPluginSummary(
  plugin: PluginListItem,
  storedMocks: StoredControlPanelPluginMockMap,
): ControlPanelPluginSummary {
  return {
    plugin,
    capability_count: plugin.capabilities.length,
    permission_count: plugin.permissions.length,
    runtime_count: plugin.runtimes.length,
    runtime_health: resolvePluginRuntimeHealth(plugin.runtimes),
    control: resolvePluginControlState(plugin.plugin_id, plugin.enabled, storedMocks),
  };
}

function buildPluginSnapshotSummary(items: ControlPanelPluginSummary[]) {
  return items.reduce<ControlPanelPluginSnapshot["summary"]>(
    (summary, item) => {
      summary.total += 1;
      summary.live_enabled += item.plugin.enabled ? 1 : 0;
      summary.mock_overrides += item.control.source === "mock" ? 1 : 0;
      summary[item.runtime_health] += 1;
      return summary;
    },
    {
      total: 0,
      healthy: 0,
      degraded: 0,
      failed: 0,
      stopped: 0,
      unavailable: 0,
      unknown: 0,
      live_enabled: 0,
      mock_overrides: 0,
    },
  );
}

function sortPluginItems(items: PluginListItem[]) {
  return [...items].sort((left, right) => {
    const leftLabel = (left.display_name || left.name).trim().toLowerCase();
    const rightLabel = (right.display_name || right.name).trim().toLowerCase();
    return leftLabel.localeCompare(rightLabel, "en");
  });
}

/**
 * Reads every plugin list page because the control-panel surface has no paging
 * affordance and therefore needs a complete registered-plugin snapshot.
 */
async function listAllPlugins() {
  const items: PluginListItem[] = [];
  let offset = 0;

  for (;;) {
    const result = await listPlugins({
      request_meta: createRequestMeta(),
      page: {
        limit: CONTROL_PANEL_PLUGIN_PAGE_LIMIT,
        offset,
      },
    });

    items.push(...result.items);
    if (!result.page.has_more) {
      return items;
    }

    offset += result.items.length;
    if (result.items.length === 0) {
      return items;
    }
  }
}

/**
 * Reads the formal plugin query endpoints and projects them into a control-panel
 * view model while keeping mock enable/disable state local to the desktop shell.
 */
export async function loadControlPanelPluginSnapshot(): Promise<ControlPanelPluginSnapshot> {
  try {
    const storedMocks = loadStoredPluginMocks();
    const pluginItems = await listAllPlugins();

    const items = sortPluginItems(pluginItems).map((plugin) => buildPluginSummary(plugin, storedMocks));

    return {
      items,
      summary: buildPluginSnapshotSummary(items),
    };
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      throw new Error("插件扩展服务暂时不可用，请稍后重试。");
    }

    throw error;
  }
}

/**
 * Reads one plugin detail from the formal RPC boundary and augments it with the
 * local mock control state used by the current desktop control panel.
 */
export async function loadControlPanelPluginDetail(pluginId: string): Promise<ControlPanelPluginDetail> {
  try {
    const detail = await getPluginDetail({
      request_meta: createRequestMeta(),
      plugin_id: pluginId,
      include_runtime: true,
      include_metrics: true,
      include_events: true,
    });
    const storedMocks = loadStoredPluginMocks();

    return {
      ...detail,
      control: resolvePluginControlState(detail.plugin.plugin_id, detail.plugin.enabled, storedMocks),
    };
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      throw new Error("插件详情服务暂时不可用，请稍后重试。");
    }

    throw error;
  }
}

/**
 * Persists a control-panel-local mock enable/disable toggle without claiming a
 * formal backend write path. When the requested state matches the live source,
 * the local override is removed so the UI falls back to formal data.
 */
export function saveControlPanelPluginMockEnabled(
  pluginId: string,
  nextEnabled: boolean,
  baselineEnabled: boolean,
): ControlPanelPluginControlState {
  const storedMocks = loadStoredPluginMocks();

  if (nextEnabled === baselineEnabled) {
    delete storedMocks[pluginId];
  } else {
    storedMocks[pluginId] = {
      enabled: nextEnabled,
      updated_at: new Date().toISOString(),
    };
  }

  saveStoredPluginMocks(storedMocks);
  return resolvePluginControlState(pluginId, baselineEnabled, storedMocks);
}
