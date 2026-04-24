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
import mirrorOverviewMock from "./mirrorOverview.json";
import { isRpcChannelUnavailable, logRpcMockFallback } from "@/rpc/fallback";
import { getMirrorOverviewDetailed as requestMirrorOverview } from "@/rpc/methods";
import { loadMirrorConversationRecords, type MirrorConversationRecord } from "@/services/mirrorMemoryService";
import { loadSecurityModuleData } from "@/features/dashboard/safety/securityService";
import { loadTaskBuckets } from "@/features/dashboard/tasks/taskPage.service";
import {
  getInitialDashboardSettingsSnapshot,
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

type MirrorOverviewMock = typeof mirrorOverviewMock;
export type MirrorOverviewSource = "rpc" | "mock";

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
  source: MirrorOverviewSource;
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

function adaptMirrorReference(reference: MirrorOverviewMock["memory_references"][number]): MirrorReference {
  return {
    memory_id: reference.memory_id,
    reason: reference.reason,
    summary: reference.summary ?? reference.reason,
  };
}

function buildFallbackOverview(): AgentMirrorOverviewGetResult {
  return {
    history_summary: mirrorOverviewMock.history_summary.map((item) => item),
    daily_summary: mirrorOverviewMock.daily_summary
      ? {
          date: mirrorOverviewMock.daily_summary.date,
          completed_tasks: mirrorOverviewMock.daily_summary.completed_tasks,
          generated_outputs: mirrorOverviewMock.daily_summary.generated_outputs,
        }
      : null,
    profile: mirrorOverviewMock.profile
      ? {
          work_style: mirrorOverviewMock.profile.work_style,
          preferred_output: mirrorOverviewMock.profile.preferred_output,
          active_hours: mirrorOverviewMock.profile.active_hours,
        }
      : null,
    memory_references: mirrorOverviewMock.memory_references.map(adaptMirrorReference),
  } satisfies AgentMirrorOverviewGetResult;
}

function createRequestMeta(): RequestMeta {
  return {
    trace_id: `trace_mirror_overview_${Date.now()}`,
    client_time: new Date().toISOString(),
  };
}

function getEmptyMirrorSupportContext(): MirrorSupportContext {
  return {
    finishedTasks: [],
    unfinishedTasks: [],
    latestRestorePoint: null,
    pendingApprovals: [],
    latestRestorePointSummary: null,
    securityStatus: null,
    tokenCostSummary: null,
    warnings: [],
  };
}

async function loadMirrorSupportContext(source: MirrorOverviewSource): Promise<MirrorSupportContext> {
  const [taskBucketsResult, securityResult] = await Promise.allSettled([
    loadTaskBuckets({ source }),
    loadSecurityModuleData(source),
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
  source: MirrorOverviewSource,
  rpcContext: MirrorOverviewData["rpcContext"],
  supportContext: MirrorSupportContext,
  settingsSnapshot: DashboardSettingsSnapshotData,
): MirrorOverviewData {
  // Mirror detail cards mix protocol-backed overview data with frontend support
  // context so the page can explain related tasks, safety state, and settings policy.
  const conversations = loadMirrorConversationRecords(source);
  const conversationSummary = buildMirrorConversationSummary(conversations);
  const dailyDigest = buildMirrorDailyDigest({
    overview,
    unfinished_tasks: supportContext.unfinishedTasks,
    finished_tasks: supportContext.finishedTasks,
    pending_approvals: supportContext.pendingApprovals,
    security_status: supportContext.securityStatus,
    latest_restore_point_summary: supportContext.latestRestorePointSummary,
    token_cost_summary: supportContext.tokenCostSummary,
    conversations,
  });
  const profileItems = buildMirrorProfileBaseItems({
    profile: overview.profile,
    conversations,
  });

  return {
    overview,
    insight: buildMirrorInsightPreview(overview, dailyDigest, conversationSummary),
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

export function getInitialMirrorOverviewData(): MirrorOverviewData {
  const overview = buildFallbackOverview();

  return buildMirrorOverviewData(
    overview,
    "mock",
    {
      serverTime: null,
      warnings: [],
    },
    getEmptyMirrorSupportContext(),
    getInitialDashboardSettingsSnapshot(),
  );
}

export async function loadMirrorOverviewData(source: MirrorOverviewSource = "rpc"): Promise<MirrorOverviewData> {
  if (source === "mock") {
    const overview = buildFallbackOverview();
    const [supportContext, settingsSnapshot] = await Promise.all([
      loadMirrorSupportContext("mock"),
      loadDashboardSettingsSnapshot("mock", MIRROR_SETTINGS_SCOPE),
    ]);

    return buildMirrorOverviewData(
      overview,
      "mock",
      {
        serverTime: null,
        warnings: [],
      },
      supportContext,
      settingsSnapshot,
    );
  }

  try {
    const params: AgentMirrorOverviewGetParams = {
      request_meta: createRequestMeta(),
      include: ["history_summary", "daily_summary", "profile", "memory_references"],
    };

    // Support context and settings are independent read paths, so load them in
    // parallel with the main mirror overview request to keep refreshes responsive.
    const [response, supportContext, settingsSnapshot] = await Promise.all([
      requestMirrorOverview(params),
      loadMirrorSupportContext("rpc"),
      loadDashboardSettingsSnapshot("rpc", MIRROR_SETTINGS_SCOPE),
    ]);
    const overview = response.data;

    return buildMirrorOverviewData(
      overview,
      "rpc",
      {
        serverTime: response.meta?.server_time ?? null,
        warnings: response.warnings,
      },
      supportContext,
      settingsSnapshot,
    );
  } catch (error) {
    if (isRpcChannelUnavailable(error)) {
      logRpcMockFallback("mirror overview", error);
      const overview = buildFallbackOverview();
      const [supportContext, settingsSnapshot] = await Promise.all([
        loadMirrorSupportContext("mock"),
        loadDashboardSettingsSnapshot("mock", MIRROR_SETTINGS_SCOPE),
      ]);

      return buildMirrorOverviewData(
        overview,
        "mock",
        {
          serverTime: null,
          warnings: [],
        },
        supportContext,
        settingsSnapshot,
      );
    }

    throw error;
  }
}
