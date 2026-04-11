import type {
  AgentMirrorOverviewGetParams,
  AgentMirrorOverviewGetResult,
  MirrorReference,
  RequestMeta,
} from "@cialloclaw/protocol";
import mirrorOverviewMock from "./mirrorOverview.json";
import { getMirrorOverviewDetailed as requestMirrorOverview } from "@/rpc/methods";

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
  rpcContext: {
    serverTime: string | null;
    warnings: string[];
  };
  source: MirrorOverviewSource;
};

function adaptMirrorReference(reference: MirrorOverviewMock["memory_references"][number]): MirrorReference {
  return {
    memory_id: reference.memory_id,
    reason: reference.reason,
    summary: reference.summary,
  };
}

function buildFallbackOverview(): AgentMirrorOverviewGetResult {
  const overview = {
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

  return overview;
}

function createRequestMeta(): RequestMeta {
  return {
    trace_id: `trace_mirror_overview_${Date.now()}`,
    client_time: new Date().toISOString(),
  };
}

export function buildMirrorInsightPreview(overview: AgentMirrorOverviewGetResult): MirrorInsightPreview {
  const latestReference = overview.memory_references[0] ?? null;
  const completedTasks = overview.daily_summary?.completed_tasks ?? 0;
  const generatedOutputs = overview.daily_summary?.generated_outputs ?? 0;
  const preferredOutput = overview.profile?.preferred_output ?? "结构化摘要";

  return {
    badge: latestReference ? "mirror ready" : "mirror empty",
    title: `今日记录了 ${completedTasks} 个完成任务`,
    description: `当前偏好 ${preferredOutput}，并已累计 ${generatedOutputs} 份可复用输出线索。`,
    primaryReference: latestReference,
  };
}

export function getInitialMirrorOverviewData(): MirrorOverviewData {
  const overview = buildFallbackOverview();

  return {
    overview,
    insight: buildMirrorInsightPreview(overview),
    rpcContext: {
      serverTime: null,
      warnings: [],
    },
    source: "mock",
  };
}

export async function loadMirrorOverviewData(): Promise<MirrorOverviewData> {
  const params: AgentMirrorOverviewGetParams = {
    request_meta: createRequestMeta(),
    include: ["history_summary", "daily_summary", "profile", "memory_references"],
  };

  try {
    const response = await requestMirrorOverview(params);
    const overview = response.data;

    return {
      overview,
      insight: buildMirrorInsightPreview(overview),
      rpcContext: {
        serverTime: response.meta?.server_time ?? null,
        warnings: response.warnings,
      },
      source: "rpc",
    };
  } catch (error) {
    console.warn("Mirror overview RPC unavailable, using local mock fallback.", error);

    return getInitialMirrorOverviewData();
  }
}
