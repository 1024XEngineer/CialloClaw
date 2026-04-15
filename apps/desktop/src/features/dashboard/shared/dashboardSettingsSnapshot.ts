import type { AgentSettingsGetParams, RequestMeta, SettingsSnapshot, TimeInterval } from "@cialloclaw/protocol";
import { isRpcChannelUnavailable, logRpcMockFallback } from "@/rpc/fallback";
import { getSettingsDetailed } from "@/rpc/methods";
import { loadSettings } from "@/services/settingsService";

export type DashboardSettingsSource = "rpc" | "mock";

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

// Local settings are the safe bootstrap source for dashboard cards. They let the
// UI render immediately and remain usable when the RPC pipe is unavailable.
export function getInitialDashboardSettingsSnapshot(): DashboardSettingsSnapshotData {
  return {
    settings: loadSettings().settings,
    source: "mock",
    rpcContext: {
      serverTime: null,
      warnings: [],
    },
  };
}

// Dashboard pages should not each reimplement their own settings fallback logic.
// This helper keeps the "RPC when available, local snapshot otherwise" rule in one place.
export async function loadDashboardSettingsSnapshot(source: DashboardSettingsSource = "rpc"): Promise<DashboardSettingsSnapshotData> {
  if (source === "mock") {
    return getInitialDashboardSettingsSnapshot();
  }

  const params: AgentSettingsGetParams = {
    request_meta: createRequestMeta(),
    scope: "all",
  };

  try {
    const response = await getSettingsDetailed(params);

    return {
      settings: response.data.settings,
      source: "rpc",
      rpcContext: {
        serverTime: response.meta?.server_time ?? null,
        warnings: response.warnings,
      },
    };
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      logRpcMockFallback("dashboard settings snapshot", error);
    } else {
      console.warn("Dashboard settings snapshot failed, using local settings fallback.", error);
    }

    return {
      ...getInitialDashboardSettingsSnapshot(),
      rpcContext: {
        serverTime: null,
        warnings: [error instanceof Error ? error.message : "settings snapshot unavailable"],
      },
    };
  }
}

// Mirror and security cards reuse the same human-readable interval labels, so the
// formatter lives next to the shared snapshot loader.
export function formatDashboardTimeInterval(interval: TimeInterval) {
  const unitLabel = INTERVAL_UNIT_LABELS[interval.unit] ?? interval.unit;
  return `${interval.value} ${unitLabel}`;
}

// Memory lifecycle labels are also shared by multiple dashboard modules and should
// stay consistent with the same settings snapshot source.
export function formatDashboardMemoryLifecycle(lifecycle: string) {
  return MEMORY_LIFECYCLE_LABELS[lifecycle] ?? lifecycle;
}
