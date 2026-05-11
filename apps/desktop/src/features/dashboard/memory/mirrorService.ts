import type {
  AgentMirrorOverviewGetParams,
  AgentMirrorOverviewGetResult,
  ApprovalRequest,
  MirrorReference,
  RecoveryPoint,
  RequestMeta,
  Task,
  TokenCostSummary,
} from "@cialloclaw/protocol";
import { isRpcChannelUnavailable, logRpcMockFallback } from "@/rpc/fallback";
import { getMirrorOverviewDetailed as requestMirrorOverview } from "@/rpc/methods";
import { loadMirrorConversationRecords, type MirrorConversationRecord } from "@/services/mirrorMemoryService";
import { loadSecurityModuleData } from "@/features/dashboard/safety/securityService";
import { loadTaskBuckets } from "@/features/dashboard/tasks/taskPage.service";
import {
  buildDashboardSettingsWarningSnapshot,
  loadDashboardSettingsSnapshot,
  type DashboardSettingsSnapshotScope,
  type DashboardSettingsSnapshotData,
} from "@/features/dashboard/shared/dashboardSettingsSnapshot";
import {
  buildMirrorConversationSummary,
  buildMirrorDailyDigest,
  buildMirrorProfileBaseItems,
  type MirrorConversationSummary,
  type MirrorDailyDigest,
  type MirrorProfileBaseItem,
} from "./mirrorViewModel";
import { createMirrorOverviewMockData, mergeMirrorOverviewWithMockDefaults } from "./mirrorOverview.mock";

export type MirrorOverviewSource = "rpc";

type MirrorOverviewMode = MirrorOverviewSource | "mock";

export type MirrorInsightPreview = {
  badge: string;
  title: string;
  description: string;
  primaryReference: MirrorReference | null;
};

export type MirrorOverviewData = {
  overview: AgentMirrorOverviewGetResult;
  insight: MirrorInsightPreview;
  latestRestorePoint: RecoveryPoint | null;
  rpcContext: {
    serverTime: string | null;
    warnings: string[];
  };
  settingsSnapshot: DashboardSettingsSnapshotData;
  source: MirrorOverviewMode;
  conversations: MirrorConversationRecord[];
  conversationSummary: MirrorConversationSummary;
  dailyDigest: MirrorDailyDigest;
  profileItems: MirrorProfileBaseItem[];
};

type MirrorSupportContext = {
  finishedTasks: Task[];
  unfinishedTasks: Task[];
  latestRestorePoint: RecoveryPoint | null;
  pendingApprovals: ApprovalRequest[];
  latestRestorePointSummary: string | null;
  securityStatus: string | null;
  tokenCostSummary: TokenCostSummary | null;
  warnings: string[];
};

const MIRROR_SETTINGS_SCOPE: DashboardSettingsSnapshotScope = "memory";

function createRequestMeta(): RequestMeta {
  return {
    trace_id: `trace_mirror_overview_${Date.now()}`,
    client_time: new Date().toISOString(),
  };
}

async function loadMirrorSupportContext(): Promise<MirrorSupportContext> {
  const [taskBucketsResult, securityResult] = await Promise.allSettled([
    loadTaskBuckets({ source: "rpc" }),
    loadSecurityModuleData("rpc"),
  ]);
  const warnings: string[] = [];

  const taskBuckets = taskBucketsResult.status === "fulfilled" ? taskBucketsResult.value : null;
  if (taskBucketsResult.status === "rejected") {
    warnings.push(taskBucketsResult.reason instanceof Error ? `task-context: ${taskBucketsResult.reason.message}` : "task-context: load failed");
  }

  const securityModule = securityResult.status === "fulfilled" ? securityResult.value : null;
  if (securityResult.status === "rejected") {
    warnings.push(securityResult.reason instanceof Error ? `security-context: ${securityResult.reason.message}` : "security-context: load failed");
  }

  return {
    finishedTasks: taskBuckets?.finished.items.map((item) => item.task) ?? [],
    unfinishedTasks: taskBuckets?.unfinished.items.map((item) => item.task) ?? [],
    latestRestorePoint:
      securityModule?.summary.latest_restore_point && typeof securityModule.summary.latest_restore_point !== "string"
        ? securityModule.summary.latest_restore_point
        : null,
    pendingApprovals: securityModule?.pending ?? [],
    latestRestorePointSummary:
      securityModule?.summary.latest_restore_point && typeof securityModule.summary.latest_restore_point !== "string"
        ? securityModule.summary.latest_restore_point.summary
        : null,
    securityStatus: securityModule?.summary.security_status ?? null,
    tokenCostSummary: securityModule?.summary.token_cost_summary ?? null,
    warnings,
  };
}

async function loadMirrorMockSettingsSnapshot(): Promise<DashboardSettingsSnapshotData> {
  return loadDashboardSettingsSnapshot("mock", MIRROR_SETTINGS_SCOPE);
}

function loadMirrorMockSupportContext(): MirrorSupportContext {
  const mockData = createMirrorOverviewMockData();

  return {
    finishedTasks: mockData.supportContext.finished_tasks.map((task) => ({ ...task })),
    unfinishedTasks: mockData.supportContext.unfinished_tasks.map((task) => ({ ...task })),
    latestRestorePoint: mockData.supportContext.latest_restore_point
      ? {
          ...mockData.supportContext.latest_restore_point,
          objects: [...mockData.supportContext.latest_restore_point.objects],
        }
      : null,
    pendingApprovals: mockData.supportContext.pending_approvals.map((approval) => ({ ...approval })),
    latestRestorePointSummary: mockData.supportContext.latest_restore_point_summary,
    securityStatus: mockData.supportContext.security_status,
    tokenCostSummary: mockData.supportContext.token_cost_summary
      ? { ...mockData.supportContext.token_cost_summary }
      : null,
    warnings: [],
  };
}

async function loadMirrorOverviewMockData(): Promise<MirrorOverviewData> {
  const mockData = createMirrorOverviewMockData();
  const settingsSnapshot = await loadMirrorMockSettingsSnapshot();

  return buildMirrorOverviewData(
    mockData.overview,
    "mock",
    {
      serverTime: null,
      warnings: [],
    },
    loadMirrorMockSupportContext(),
    settingsSnapshot,
  );
}

async function loadMirrorOverviewRpcData(params: AgentMirrorOverviewGetParams): Promise<MirrorOverviewData> {
  try {
    const response = await requestMirrorOverview(params);
    const [supportContext, settingsSnapshotResult] = await Promise.all([
      loadMirrorSupportContext(),
      loadDashboardSettingsSnapshot("rpc", MIRROR_SETTINGS_SCOPE)
        .then((snapshot) => ({ snapshot, warning: null as string | null }))
        .catch(async (error) => {
          const warning = error instanceof Error ? `settings-context: ${error.message}` : "settings-context: load failed";

          return {
            snapshot: await buildDashboardSettingsWarningSnapshot(warning),
            warning,
          };
        }),
    ]);
    const settingsSnapshot = settingsSnapshotResult.snapshot;
    const settingsWarnings = settingsSnapshotResult.warning ? [settingsSnapshotResult.warning] : [];

    return buildMirrorOverviewData(
      response.data,
      "rpc",
      {
        serverTime: response.meta?.server_time ?? null,
        warnings: [...response.warnings, ...settingsWarnings],
      },
      supportContext,
      settingsSnapshot,
    );
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      logRpcMockFallback("mirror.overview", error);
      return loadMirrorOverviewMockData();
    }

    throw error;
  }
}

export function buildMirrorInsightPreview(
  overview: AgentMirrorOverviewGetResult,
  dailyDigest: MirrorDailyDigest,
  conversationSummary: MirrorConversationSummary,
): MirrorInsightPreview {
  const latestReference = overview.memory_references[0] ?? null;
  const overviewLead = overview.history_summary[0] ?? latestReference?.reason ?? dailyDigest.lede;
  const localConversationCopy =
    conversationSummary.total_records > 0
      ? `本地最近 100 条对话中记录了 ${conversationSummary.total_records} 条可见会话。`
      : "当前没有本地对话统计。";

  return {
    badge: latestReference ? "mirror ready" : "mirror quiet",
    title: dailyDigest.headline,
    description: `${overviewLead} ${localConversationCopy}`,
    primaryReference: latestReference,
  };
}
function buildMirrorOverviewData(
  overview: AgentMirrorOverviewGetResult,
  source: MirrorOverviewMode,
  rpcContext: MirrorOverviewData["rpcContext"],
  supportContext: MirrorSupportContext,
  settingsSnapshot: DashboardSettingsSnapshotData,
): MirrorOverviewData {
  // Mirror detail cards mix protocol-backed overview data with frontend support
  // context so the page can explain related tasks, safety state, and settings policy.
  const resolvedOverview = mergeMirrorOverviewWithMockDefaults(overview);
  const conversations = loadMirrorConversationRecords(source);
  const conversationSummary = buildMirrorConversationSummary(conversations);
  const dailyDigest = buildMirrorDailyDigest({
    overview: resolvedOverview,
    unfinished_tasks: supportContext.unfinishedTasks,
    finished_tasks: supportContext.finishedTasks,
    pending_approvals: supportContext.pendingApprovals,
    security_status: supportContext.securityStatus,
    latest_restore_point_summary: supportContext.latestRestorePointSummary,
    token_cost_summary: supportContext.tokenCostSummary,
    conversations,
  });
  const profileItems = buildMirrorProfileBaseItems({
    profile: resolvedOverview.profile,
    conversations,
  });

  return {
    overview: resolvedOverview,
    insight: buildMirrorInsightPreview(resolvedOverview, dailyDigest, conversationSummary),
    latestRestorePoint: supportContext.latestRestorePoint,
    rpcContext: {
      ...rpcContext,
      warnings: [...rpcContext.warnings, ...supportContext.warnings],
    },
    settingsSnapshot,
    source,
    conversations,
    conversationSummary,
    dailyDigest,
    profileItems,
  };
}

/**
 * Reuses an already refreshed dashboard settings snapshot inside the current
 * mirror overview state so settings writes do not need a second mirror reload.
 */
export function applyMirrorSettingsSnapshot(
  current: MirrorOverviewData,
  settingsSnapshot: DashboardSettingsSnapshotData,
): MirrorOverviewData {
  return {
    ...current,
    settingsSnapshot,
  };
}

export async function loadMirrorOverviewData(_source: MirrorOverviewMode = "rpc"): Promise<MirrorOverviewData> {
  if (_source === "mock") {
    return loadMirrorOverviewMockData();
  }

  const params: AgentMirrorOverviewGetParams = {
    request_meta: createRequestMeta(),
    include: ["history_summary", "daily_summary", "profile", "memory_references"],
  };

  // Support context and settings are independent read paths, so load them in
  // parallel with the main mirror overview request to keep refreshes responsive.
  // Settings are advisory for the mirror settings card, so a transient
  // `agent.settings.get` failure should degrade into a warning instead of
  // blanking the whole mirror overview page.
  return loadMirrorOverviewRpcData(params);
}
