import { useEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, ArrowLeft, ArrowUpRight, FolderOutput, Link2, RefreshCcw } from "lucide-react";
import { Link, NavLink, Navigate, useNavigate, useParams } from "react-router-dom";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { dashboardModules } from "@/features/dashboard/shared/dashboardRoutes";
import { buildDashboardTaskDetailRouteState } from "@/features/dashboard/shared/dashboardTaskDetailNavigation";
import { resolveDashboardModuleRoutePath, resolveDashboardRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import { subscribeDeliveryReady, subscribeTaskRuntime, subscribeTaskUpdated } from "@/rpc/subscriptions";
import { cn } from "@/utils/cn";
import { formatTimestamp } from "@/utils/formatters";
import { navigateToDashboardTaskDelivery } from "./taskDeliveryNavigation";
import { getTaskPreviewStatusLabel, getTaskStatusBadgeClass } from "./taskPage.mapper";
import { buildDashboardTaskArtifactQueryKey, buildDashboardTaskDetailQueryKey } from "./taskPage.query";
import { loadTaskDetailData, type TaskPageDataMode } from "./taskPage.service";
import {
  canOpenTaskDeliveryResult,
  getTaskDeliveryOpenLabel,
  isAllowedTaskOpenUrl,
  loadTaskArtifactPage,
  mergeTaskArtifactItems,
  openTaskArtifactForTask,
  openTaskDeliveryForTask,
  performTaskOpenExecution,
  resolveTaskOpenExecutionPlan,
} from "./taskOutput.service";
import "./taskDeliveryPage.css";

type TaskDeliveryOpenResult = Awaited<ReturnType<typeof openTaskArtifactForTask>> | Awaited<ReturnType<typeof openTaskDeliveryForTask>>;
const TASK_DELIVERY_DETAIL_REFRESH_DEBOUNCE_MS = 280;

/**
 * Renders the dedicated task delivery page so formal task output can be read
 * inside the dashboard before the user decides whether to open files, folders,
 * links, or jump back to task detail.
 */
export function TaskDeliveryPage() {
  const { taskId: encodedTaskId } = useParams<{ taskId: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const taskId = useMemo(() => {
    if (!encodedTaskId) {
      return "";
    }

    try {
      return decodeURIComponent(encodedTaskId);
    } catch {
      return encodedTaskId;
    }
  }, [encodedTaskId]);
  const dataMode: TaskPageDataMode = "rpc";
  const [feedback, setFeedback] = useState<string | null>(null);
  const feedbackTimeoutRef = useRef<number | null>(null);

  const pageStyle = {
    "--task-accent": "#92abc1",
    "--task-accent-strong": "#5c7894",
    "--task-accent-soft": "#90a98e",
    "--task-alert": "#d1ad78",
    "--task-danger": "#c9877b",
    "--task-success": "#86a889",
    "--task-ink": "#32444c",
    "--task-copy": "rgba(68, 81, 88, 0.74)",
    "--task-line": "rgba(146, 171, 193, 0.18)",
    "--task-panel": "rgba(252, 248, 242, 0.9)",
    "--task-panel-strong": "rgba(255, 253, 249, 0.97)",
    "--task-panel-soft": "rgba(255, 255, 255, 0.58)",
  } as CSSProperties;

  const taskDetailQuery = useQuery({
    enabled: taskId.length > 0,
    queryKey: buildDashboardTaskDetailQueryKey(dataMode, taskId),
    queryFn: () => loadTaskDetailData(taskId, dataMode),
    refetchOnMount: dataMode === "rpc",
    refetchOnReconnect: false,
    refetchOnWindowFocus: false,
    retry: false,
  });

  const artifactListQuery = useQuery({
    enabled: dataMode === "rpc" && taskId.length > 0,
    queryKey: buildDashboardTaskArtifactQueryKey(dataMode, taskId),
    queryFn: () => loadTaskArtifactPage(taskId, dataMode),
    refetchOnMount: dataMode === "rpc",
    refetchOnReconnect: false,
    refetchOnWindowFocus: false,
    retry: false,
  });

  const detailData = taskDetailQuery.data ?? null;
  const detailState = taskDetailQuery.isError ? "error" : taskDetailQuery.isPending ? "loading" : "ready";
  const detailErrorMessage = taskDetailQuery.isError ? (taskDetailQuery.error instanceof Error ? taskDetailQuery.error.message : "交付详情请求失败") : null;
  const taskDetailArtifacts = useMemo(() => detailData?.detail.artifacts ?? [], [detailData?.detail.artifacts]);
  const artifactItems = useMemo(
    () => mergeTaskArtifactItems(artifactListQuery.data?.items ?? [], taskDetailArtifacts),
    [artifactListQuery.data?.items, taskDetailArtifacts],
  );
  const formalDeliveryResult = detailData?.detail.delivery_result ?? null;
  const formalDeliveryUrl = formalDeliveryResult?.payload.url ?? null;
  const formalDeliveryUrlIsAllowed = formalDeliveryUrl !== null && isAllowedTaskOpenUrl(formalDeliveryUrl);
  const citations = useMemo(() => detailData?.detail.citations ?? [], [detailData?.detail.citations]);
  const evidenceArtifactRefs = useMemo(
    () =>
      new Set(
        citations.map((citation) => {
          const sourceRef = citation.source_ref.trim();
          return sourceRef.length > 0 ? sourceRef : citation.citation_id;
        }),
      ),
    [citations],
  );
  const evidenceArtifacts = artifactItems.filter((artifact) => evidenceArtifactRefs.has(artifact.artifact_id) || evidenceArtifactRefs.has(artifact.path));
  const outputArtifacts = artifactItems.filter((artifact) => !evidenceArtifactRefs.has(artifact.artifact_id) && !evidenceArtifactRefs.has(artifact.path));
  const canOpenFormalDelivery = canOpenTaskDeliveryResult(formalDeliveryResult, taskId);

  useEffect(() => {
    return () => {
      if (feedbackTimeoutRef.current !== null) {
        window.clearTimeout(feedbackTimeoutRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (taskId.length === 0) {
      return;
    }

    let detailRefreshTimeoutId: number | null = null;

    function invalidateCurrentTaskDetail() {
      void queryClient.invalidateQueries({ queryKey: buildDashboardTaskDetailQueryKey(dataMode, taskId) });
    }

    function invalidateCurrentTaskArtifacts() {
      void queryClient.invalidateQueries({ queryKey: buildDashboardTaskArtifactQueryKey(dataMode, taskId) });
    }

    function flushScheduledTaskDetailRefresh() {
      detailRefreshTimeoutId = null;
      invalidateCurrentTaskDetail();
    }

    /**
     * Task delivery only needs task detail progress while the run is active.
     * Debounce task.updated so progress ticks do not refetch both delivery
     * queries on every runtime heartbeat.
     */
    function scheduleTaskDetailRefresh() {
      if (detailRefreshTimeoutId !== null) {
        return;
      }

      detailRefreshTimeoutId = window.setTimeout(() => {
        flushScheduledTaskDetailRefresh();
      }, TASK_DELIVERY_DETAIL_REFRESH_DEBOUNCE_MS);
    }

    function invalidateCurrentTaskDelivery() {
      if (detailRefreshTimeoutId !== null) {
        window.clearTimeout(detailRefreshTimeoutId);
        detailRefreshTimeoutId = null;
      }

      invalidateCurrentTaskDetail();
      invalidateCurrentTaskArtifacts();
    }

    const clearTaskUpdatedSubscription = subscribeTaskUpdated((payload) => {
      if (payload.task_id === taskId) {
        scheduleTaskDetailRefresh();
      }
    });

    const clearDeliveryReadySubscription = subscribeDeliveryReady((payload) => {
      if (payload.task_id === taskId) {
        invalidateCurrentTaskDelivery();
      }
    });

    const clearRuntimeSubscription = subscribeTaskRuntime(taskId, () => {
      scheduleTaskDetailRefresh();
    });

    return () => {
      if (detailRefreshTimeoutId !== null) {
        window.clearTimeout(detailRefreshTimeoutId);
      }
      clearTaskUpdatedSubscription();
      clearDeliveryReadySubscription();
      clearRuntimeSubscription();
    };
  }, [dataMode, queryClient, taskId]);

  function clearFeedbackTimeout() {
    if (feedbackTimeoutRef.current !== null) {
      window.clearTimeout(feedbackTimeoutRef.current);
      feedbackTimeoutRef.current = null;
    }
  }

  function showFeedback(message: string, autoHide = true) {
    setFeedback(message);
    clearFeedbackTimeout();

    if (!autoHide) {
      return;
    }

    feedbackTimeoutRef.current = window.setTimeout(() => {
      setFeedback(null);
      feedbackTimeoutRef.current = null;
    }, 2600);
  }

  function openTaskDetail() {
    navigate(resolveDashboardModuleRoutePath("tasks"), {
      state: buildDashboardTaskDetailRouteState(taskId),
    });
  }

  async function handleResolvedOpen(result: TaskDeliveryOpenResult) {
    const plan = resolveTaskOpenExecutionPlan(result, taskId);

    if (plan.mode === "task_detail" && plan.taskId === taskId) {
      showFeedback("当前结果已经在交付页中展示。");
      return;
    }

    showFeedback(
      await performTaskOpenExecution(plan, {
        onOpenTaskDetail: ({ taskId: resolvedTaskId }) => {
          navigate(resolveDashboardModuleRoutePath("tasks"), {
            state: buildDashboardTaskDetailRouteState(resolvedTaskId),
          });
          return plan.feedback;
        },
        onOpenTaskDelivery: ({ taskId: resolvedTaskId }) => {
          navigateToDashboardTaskDelivery(navigate, resolvedTaskId);
          return plan.feedback;
        },
      }),
    );
  }

  async function handleOpenFormalDeliveryUrl() {
    if (!formalDeliveryResult || !formalDeliveryUrl) {
      return;
    }

    if (formalDeliveryResult.type === "result_page") {
      const deliveryOpenResult: TaskDeliveryOpenResult = {
        delivery_result: formalDeliveryResult,
        open_action: "result_page",
        resolved_payload: {
          path: formalDeliveryResult.payload.path ?? null,
          task_id: formalDeliveryResult.payload.task_id ?? taskId,
          url: formalDeliveryUrl,
        },
      };

      await handleResolvedOpen(deliveryOpenResult);
      return;
    }

    showFeedback(await performTaskOpenExecution({
      feedback: "已打开链接。",
      mode: "open_url",
      path: formalDeliveryResult.payload.path ?? null,
      taskId: formalDeliveryResult.payload.task_id ?? taskId,
      url: formalDeliveryUrl,
    }));
  }

  const artifactOpenMutation = useMutation({
    mutationFn: (artifactId: string) => openTaskArtifactForTask(taskId, artifactId, dataMode),
    onSuccess: async (result) => {
      await handleResolvedOpen(result);
    },
    onError: (error) => {
      showFeedback(error instanceof Error ? `打开交付产物失败：${error.message}` : "打开交付产物失败，请稍后再试。");
    },
  });

  const deliveryOpenMutation = useMutation({
    mutationFn: () => openTaskDeliveryForTask(taskId, undefined, dataMode),
    onSuccess: async (result) => {
      await handleResolvedOpen(result);
    },
    onError: (error) => {
      showFeedback(error instanceof Error ? `执行打开动作失败：${error.message}` : "执行打开动作失败，请稍后再试。");
    },
  });

  if (!taskId) {
    return <Navigate replace to={resolveDashboardModuleRoutePath("tasks")} />;
  }

  return (
    <main className="dashboard-page task-delivery-page" style={pageStyle}>
      <header className="dashboard-page__topbar">
        <div className="task-delivery-page__topbar-actions">
          <Link className="dashboard-page__home-link" to={resolveDashboardRoutePath("home")}>
            <ArrowLeft className="h-4 w-4" />
            返回首页
          </Link>
          <Button className="task-delivery-page__detail-link" onClick={openTaskDetail} type="button" variant="ghost">
            返回任务详情
          </Button>
        </div>

        <nav aria-label="Dashboard modules" className="dashboard-page__module-nav">
          {dashboardModules.map((item) => (
            <NavLink key={item.route} className={({ isActive }) => cn("dashboard-page__module-link", isActive && "is-active")} to={item.path}>
              {item.title}
            </NavLink>
          ))}
        </nav>
      </header>

      <section className="task-delivery-page__hero">
        <div className="task-delivery-page__hero-copy">
          <p className="task-delivery-page__eyebrow">任务交付</p>
          <h1>{formalDeliveryResult?.title ?? detailData?.task.title ?? "正在准备任务交付视图"}</h1>
          <p className="task-delivery-page__hero-text">
            {formalDeliveryResult?.preview_text ??
              (detailData
                ? "当前任务还没有独立的交付结果，你仍然可以在下方查看产物、引用与交付出口。"
                : "正在从本地服务读取交付、产物与引用信息。")}
          </p>
        </div>

        <div className="task-delivery-page__hero-actions">
          {detailData ? (
            <>
              <Badge className={cn("border-0 px-3 py-1 text-[0.74rem] ring-1", getTaskStatusBadgeClass(detailData.task.status))}>
                {getTaskPreviewStatusLabel(detailData.task.status)}
              </Badge>
              {formalDeliveryResult ? <Badge variant="outline">{formalDeliveryResult.type}</Badge> : null}
            </>
          ) : null}

          {canOpenFormalDelivery ? (
            <Button disabled={deliveryOpenMutation.isPending} onClick={() => deliveryOpenMutation.mutate()} type="button">
              <ArrowUpRight className="h-4 w-4" />
              {deliveryOpenMutation.isPending ? "执行中..." : getTaskDeliveryOpenLabel(formalDeliveryResult)}
            </Button>
          ) : null}
        </div>
      </section>

      {feedback ? (
        <aside className="task-delivery-page__feedback">
          <AlertTriangle className="h-4 w-4" />
          <p>{feedback}</p>
        </aside>
      ) : null}

      <section className="task-delivery-page__grid">
        {detailState !== "ready" ? (
          <section className="task-delivery-page__card task-delivery-page__card--notice">
            <div className="task-delivery-page__card-head">
              <div>
                <p className="task-delivery-page__card-eyebrow">同步状态</p>
                <h2>{detailState === "loading" ? "正在同步交付详情" : "交付详情同步失败"}</h2>
              </div>
              {detailState === "error" ? (
                <Button onClick={() => void taskDetailQuery.refetch()} size="sm" type="button" variant="outline">
                  <RefreshCcw className="h-4 w-4" />
                  重试
                </Button>
              ) : null}
            </div>
            <p>
              {detailState === "loading"
                ? "当前先展示已有的任务交付承接，交付结果、产物与引用会在同步完成后补齐。"
                : `${detailErrorMessage ?? "任务交付详情请求失败"}。当前可先返回任务详情，稍后再试。`}
            </p>
          </section>
        ) : null}

        <section className="task-delivery-page__card">
          <div className="task-delivery-page__card-head">
            <div>
              <p className="task-delivery-page__card-eyebrow">交付概览</p>
              <h2>交付摘要</h2>
            </div>
          </div>
          {detailData ? (
            <dl className="task-delivery-page__meta-list">
              <div>
                <dt>任务</dt>
                <dd>{detailData.task.title}</dd>
              </div>
              <div>
                <dt>任务状态</dt>
                <dd>{getTaskPreviewStatusLabel(detailData.task.status)}</dd>
              </div>
              <div>
                <dt>最近更新</dt>
                <dd>{formatTimestamp(detailData.task.updated_at)}</dd>
              </div>
              <div>
                <dt>类型</dt>
                <dd>{formalDeliveryResult?.type ?? "尚未生成"}</dd>
              </div>
              <div>
                <dt>预览说明</dt>
                <dd>{formalDeliveryResult?.preview_text ?? "当前还没有交付结果。"}</dd>
              </div>
            </dl>
          ) : (
            <p className="task-delivery-page__empty">正在读取任务交付摘要。</p>
          )}
        </section>

        <section className="task-delivery-page__card">
          <div className="task-delivery-page__card-head">
            <div>
              <p className="task-delivery-page__card-eyebrow">打开内容</p>
              <h2>打开出口</h2>
            </div>
          </div>
          {formalDeliveryResult ? (
            <dl className="task-delivery-page__payload-list">
              <div>
                <dt>任务编号</dt>
                <dd>{formalDeliveryResult.payload.task_id ?? taskId}</dd>
              </div>
              <div>
                <dt>路径</dt>
                <dd>{formalDeliveryResult.payload.path ?? "无"}</dd>
              </div>
              <div>
                <dt>链接</dt>
                <dd>
                  {formalDeliveryUrl ? (
                    formalDeliveryUrlIsAllowed ? (
                      <button className="task-delivery-page__inline-link" onClick={() => void handleOpenFormalDeliveryUrl()} type="button">
                        {formalDeliveryUrl}
                      </button>
                    ) : (
                      <span>{formalDeliveryUrl}</span>
                    )
                  ) : (
                    "无"
                  )}
                </dd>
              </div>
            </dl>
          ) : (
            <p className="task-delivery-page__empty">当前没有可展示的内容。</p>
          )}
        </section>

        <section className="task-delivery-page__card task-delivery-page__card--wide">
          <div className="task-delivery-page__card-head">
            <div>
              <p className="task-delivery-page__card-eyebrow">产物</p>
              <h2>交付产物</h2>
            </div>
          </div>
          {artifactListQuery.isError ? <p className="task-delivery-page__hint">{artifactListQuery.error instanceof Error ? artifactListQuery.error.message : "产物列表请求失败"}</p> : null}
          <div className="task-delivery-page__item-list">
            {artifactListQuery.isPending && outputArtifacts.length === 0 ? <p className="task-delivery-page__empty">正在同步交付产物...</p> : null}
            {outputArtifacts.map((artifact) => (
              <article key={artifact.artifact_id} className="task-delivery-page__item">
                <div className="task-delivery-page__item-copy">
                  <FolderOutput className="h-4 w-4" />
                  <div>
                    <p>{artifact.title}</p>
                    <span>{artifact.path}</span>
                  </div>
                </div>
                <Button
                  disabled={artifactOpenMutation.isPending && artifactOpenMutation.variables === artifact.artifact_id}
                  onClick={() => artifactOpenMutation.mutate(artifact.artifact_id)}
                  size="sm"
                  type="button"
                  variant="outline"
                >
                  <ArrowUpRight className="h-4 w-4" />
                  {artifactOpenMutation.isPending && artifactOpenMutation.variables === artifact.artifact_id ? "打开中..." : "打开"}
                </Button>
              </article>
            ))}
            {!artifactListQuery.isPending && outputArtifacts.length === 0 ? <p className="task-delivery-page__empty">当前没有独立的交付产物。</p> : null}
          </div>
        </section>

        <section className="task-delivery-page__card task-delivery-page__card--wide">
          <div className="task-delivery-page__card-head">
            <div>
              <p className="task-delivery-page__card-eyebrow">引用与证据</p>
              <h2>引用与证据</h2>
            </div>
          </div>
          <div className="task-delivery-page__item-list">
            {citations.map((citation) => (
              <article key={citation.citation_id} className="task-delivery-page__item">
                <div className="task-delivery-page__item-copy">
                  <Link2 className="h-4 w-4" />
                  <div>
                    <p>{citation.label}</p>
                    <span>{citation.excerpt_text || citation.source_ref}</span>
                  </div>
                </div>
                <div className="task-delivery-page__item-badges">
                  {citation.evidence_role ? <Badge variant="outline">{citation.evidence_role}</Badge> : null}
                  {citation.artifact_type ? <Badge variant="secondary">{citation.artifact_type}</Badge> : null}
                </div>
              </article>
            ))}
            {evidenceArtifacts.map((artifact) => (
              <article key={`evidence_${artifact.artifact_id}`} className="task-delivery-page__item">
                <div className="task-delivery-page__item-copy">
                  <FolderOutput className="h-4 w-4" />
                  <div>
                    <p>{artifact.title}</p>
                    <span>{artifact.path}</span>
                  </div>
                </div>
                <Button
                  disabled={artifactOpenMutation.isPending && artifactOpenMutation.variables === artifact.artifact_id}
                  onClick={() => artifactOpenMutation.mutate(artifact.artifact_id)}
                  size="sm"
                  type="button"
                  variant="outline"
                >
                  <ArrowUpRight className="h-4 w-4" />
                  {artifactOpenMutation.isPending && artifactOpenMutation.variables === artifact.artifact_id ? "打开中..." : "打开证据"}
                </Button>
              </article>
            ))}
            {citations.length === 0 && evidenceArtifacts.length === 0 ? <p className="task-delivery-page__empty">当前没有引用或证据需要展示。</p> : null}
          </div>
        </section>
      </section>
    </main>
  );
}
