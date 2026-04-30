import type { AgentSettingsGetParams, RequestMeta, SettingsSnapshot, TimeInterval } from "@cialloclaw/protocol";
import { getSettingsDetailed } from "@/rpc/methods";
import {
  hydrateDesktopRuntimeDefaults,
  hydrateDesktopSettings,
  loadHydratedSettings,
  toProtocolSettingsSnapshot,
} from "@/services/settingsService";

export type DashboardSettingsSource = "rpc";
export type DashboardSettingsSnapshotScope = AgentSettingsGetParams["scope"];

// Dashboard modules only need a read-only settings view, so this snapshot shape
// keeps the formal RPC payload stable while still allowing scoped responses to
// merge onto the current local baseline.
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

async function getDashboardSettingsBaseline() {
  return toProtocolSettingsSnapshot((await loadHydratedSettings()).settings);
}

/**
 * Builds a warning-bearing dashboard settings snapshot from the current local
 * baseline when a caller chooses to keep rendering after a scoped RPC read
 * failed. This keeps the degraded state explicit without reintroducing global
 * transport fallbacks into every dashboard page.
 *
 * @param warning The user-visible warning that explains why the formal read is missing.
 * @returns A snapshot based on the persisted local settings plus the warning.
 */
export async function buildDashboardSettingsWarningSnapshot(warning: string): Promise<DashboardSettingsSnapshotData> {
  return {
    settings: await getDashboardSettingsBaseline(),
    source: "rpc",
    rpcContext: {
      serverTime: null,
      warnings: [warning],
    },
  };
}

/**
 * Loads one dashboard settings snapshot from JSON-RPC, then merges any scoped
 * payload back into the full local baseline.
 *
 * @param scope The formal `agent.settings.get` scope to request.
 */
// Dashboard pages should not each reimplement their own scoped settings merge.
// This helper keeps the protocol-first read path in one place and uses the local
// baseline only to fill fields omitted by scoped settings responses.
export async function loadDashboardSettingsSnapshot(
  _source: DashboardSettingsSource = "rpc",
  scope: DashboardSettingsSnapshotScope = "all",
): Promise<DashboardSettingsSnapshotData> {
  await hydrateDesktopRuntimeDefaults();
  const baseline = await getDashboardSettingsBaseline();
  const params: AgentSettingsGetParams = {
    request_meta: createRequestMeta(),
    scope,
  };

  const response = await getSettingsDetailed(params);

  return {
    settings: mergeDashboardSettingsSnapshot(
      baseline,
      response.data.settings as unknown as Partial<SettingsSnapshot["settings"]>,
      scope,
    ),
    source: "rpc",
    rpcContext: {
      serverTime: response.meta?.server_time ?? null,
      warnings: response.warnings,
    },
  };
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
