import type { AgentSettingsGetParams, RequestMeta, SettingsSnapshot, TimeInterval } from "@cialloclaw/protocol";
import { isRpcChannelUnavailable, logRpcMockFallback } from "@/rpc/fallback";
import { getSettingsDetailed } from "@/rpc/methods";
import { hydrateDesktopSettings, loadSettings, toProtocolSettingsSnapshot } from "@/services/settingsService";

export type DashboardSettingsSource = "rpc" | "mock";
export type DashboardSettingsSnapshotScope = AgentSettingsGetParams["scope"];

// Dashboard modules only need a read-only settings view, so this snapshot shape
// normalizes RPC data and local fallback data into one stable contract.
export type DashboardSettingsSnapshotData = {
  settings: SettingsSnapshot["settings"];
  source: DashboardSettingsSource;
  rpcContext: {
    serverTime: string | null;
    warnings: string[];
  };
};

const INTERVAL_UNIT_LABELS: Record<string, string> = {
  minute: "分钟",
  hour: "小时",
  day: "天",
  week: "周",
  month: "个月",
};

const MEMORY_LIFECYCLE_LABELS: Record<string, string> = {
  session: "仅本轮",
  "7d": "保留 7 天",
  "30d": "保留 30 天",
  long_term: "长期保留",
};

function createRequestMeta(): RequestMeta {
  return {
    trace_id: `trace_dashboard_settings_${Date.now()}`,
    client_time: new Date().toISOString(),
  };
}

function mergeDashboardSettingsSnapshot(
  baseSettings: SettingsSnapshot["settings"],
  nextSettings: Partial<SettingsSnapshot["settings"]>,
  scope: DashboardSettingsSnapshotScope,
) {
  if (scope === "all") {
    return toProtocolSettingsSnapshot(hydrateDesktopSettings(nextSettings as SettingsSnapshot["settings"]));
  }

  return toProtocolSettingsSnapshot(
    hydrateDesktopSettings({
      ...baseSettings,
      ...(scope === "general" ? { general: nextSettings.general ?? baseSettings.general } : {}),
      ...(scope === "floating_ball" ? { floating_ball: nextSettings.floating_ball ?? baseSettings.floating_ball } : {}),
      ...(scope === "memory" ? { memory: nextSettings.memory ?? baseSettings.memory } : {}),
      ...(scope === "task_automation"
        ? {
            task_automation: nextSettings.task_automation ?? baseSettings.task_automation,
          }
        : {}),
      ...(scope === "models" ? { models: nextSettings.models ?? baseSettings.models } : {}),
    }),
  );
}

/**
 * Builds the dashboard-safe local settings baseline used before RPC sync.
 */
// Local settings are the safe bootstrap source for dashboard cards. They let the
// UI render immediately and remain usable when the RPC pipe is unavailable.
export function getInitialDashboardSettingsSnapshot(): DashboardSettingsSnapshotData {
  return {
    settings: toProtocolSettingsSnapshot(loadSettings().settings),
    source: "mock",
    rpcContext: {
      serverTime: null,
      warnings: [],
    },
  };
}

/**
 * Loads one dashboard settings snapshot from JSON-RPC when available, then
 * merges any scoped payload back into the full local baseline.
 *
 * @param source The preferred read source for the dashboard.
 * @param scope The formal `agent.settings.get` scope to request.
 */
// Dashboard pages should not each reimplement their own settings fallback logic.
// This helper keeps the "RPC when available, local snapshot otherwise" rule in one place.
export async function loadDashboardSettingsSnapshot(
  source: DashboardSettingsSource = "rpc",
  scope: DashboardSettingsSnapshotScope = "all",
): Promise<DashboardSettingsSnapshotData> {
  if (source === "mock") {
    return getInitialDashboardSettingsSnapshot();
  }

  const baseline = getInitialDashboardSettingsSnapshot();
  const params: AgentSettingsGetParams = {
    request_meta: createRequestMeta(),
    scope,
  };

  try {
    const response = await getSettingsDetailed(params);

    return {
      settings: mergeDashboardSettingsSnapshot(
        baseline.settings,
        response.data.settings as unknown as Partial<SettingsSnapshot["settings"]>,
        scope,
      ),
      source: "rpc",
      rpcContext: {
        serverTime: response.meta?.server_time ?? null,
        warnings: response.warnings,
      },
    };
  } catch (error) {
    if (!isRpcChannelUnavailable(error)) {
      console.warn("Dashboard settings snapshot failed, using local settings fallback.", error);
    } else {
      logRpcMockFallback("dashboard settings snapshot", error);
    }

    return {
      ...baseline,
      rpcContext: {
        serverTime: null,
        warnings: [error instanceof Error ? error.message : "settings snapshot unavailable"],
      },
    };
  }
}

/**
 * Formats one shared time interval for dashboard-facing settings copy.
 */
// Mirror and security cards reuse the same human-readable interval labels, so the
// formatter lives next to the shared snapshot loader.
export function formatDashboardTimeInterval(interval: TimeInterval) {
  const unitLabel = INTERVAL_UNIT_LABELS[interval.unit] ?? interval.unit;
  return `${interval.value} ${unitLabel}`;
}

/**
 * Formats the mirror memory lifecycle label from the shared settings snapshot.
 */
// Memory lifecycle labels are also shared by multiple dashboard modules and should
// stay consistent with the same settings snapshot source.
export function formatDashboardMemoryLifecycle(lifecycle: string) {
  return MEMORY_LIFECYCLE_LABELS[lifecycle] ?? lifecycle;
}
