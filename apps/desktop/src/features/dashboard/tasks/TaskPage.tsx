import { useEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { Link, NavLink, useLocation, useNavigate } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, ArrowLeft, CircleDashed, LayoutList, RefreshCcw } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { subscribeDeliveryReady, subscribeTask } from "@/rpc/subscriptions";
import { loadDashboardDataMode, saveDashboardDataMode } from "@/features/dashboard/shared/dashboardDataMode";
import { DashboardMockToggle } from "@/features/dashboard/shared/DashboardMockToggle";
import { buildDashboardSafetyNavigationState } from "@/features/dashboard/shared/dashboardSafetyNavigation";
import { resolveDashboardRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import { dashboardModules } from "@/features/dashboard/shared/dashboardRoutes";
import { cn } from "@/utils/cn";
import { describeCurrentStep, getFinishedTaskGroups, isTaskEnded, sortTasksByLatest } from "./taskPage.mapper";
import {
  buildDashboardTaskBucketQueryKey,
  buildDashboardTaskDetailQueryKey,
  getDashboardTaskSecurityRefreshPlan,
  resolveDashboardTaskSafetyOpenPlan,
  shouldEnableDashboardTaskDetailQuery,
} from "./taskPage.query";
import { buildFallbackTaskDetailData, controlTaskByAction, loadTaskBucketPage, loadTaskDetailData, type TaskPageDataMode } from "./taskPage.service";
import { TaskDetailPanel } from "./components/TaskDetailPanel";
import { TaskPreviewCard } from "./components/TaskPreviewCard";
import "./taskPage.css";

export function TaskPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const INITIAL_UNFINISHED_LIMIT = 12;
  const INITIAL_FINISHED_LIMIT = 24;
  const LOAD_MORE_UNFINISHED_STEP = 12;
  const LOAD_MORE_FINISHED_STEP = 24;
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [showMoreFinished, setShowMoreFinished] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [dataMode, setDataMode] = useState<TaskPageDataMode>(() => loadDashboardDataMode("tasks") as TaskPageDataMode);
  const [unfinishedLimit, setUnfinishedLimit] = useState(INITIAL_UNFINISHED_LIMIT);
  const [finishedLimit, setFinishedLimit] = useState(INITIAL_FINISHED_LIMIT);
  const feedbackTimeoutRef = useRef<number | null>(null);
  const securityRefreshPlan = useMemo(() => getDashboardTaskSecurityRefreshPlan(dataMode), [dataMode]);

  const unfinishedQuery = useQuery({
    queryKey: buildDashboardTaskBucketQueryKey(dataMode, "unfinished", unfinishedLimit),
    queryFn: () => loadTaskBucketPage("unfinished", { limit: unfinishedLimit, source: dataMode }),
    placeholderData: (previousData) => previousData,
    refetchOnMount: securityRefreshPlan.refetchOnMount,
    refetchOnReconnect: false,
    refetchOnWindowFocus: false,
    retry: false,
  });

  const finishedQuery = useQuery({
    queryKey: buildDashboardTaskBucketQueryKey(dataMode, "finished", finishedLimit),
    queryFn: () => loadTaskBucketPage("finished", { limit: finishedLimit, source: dataMode }),
    placeholderData: (previousData) => previousData,
    refetchOnMount: securityRefreshPlan.refetchOnMount,
    refetchOnReconnect: false,
    refetchOnWindowFocus: false,
    retry: false,
  });

  const unfinishedTasks = sortTasksByLatest(unfinishedQuery.data?.items ?? []);
  const finishedTasks = sortTasksByLatest(finishedQuery.data?.items ?? []);
  const finishedGroups = useMemo(() => getFinishedTaskGroups(finishedTasks, showMoreFinished), [finishedTasks, showMoreFinished]);
  const unfinishedPage = unfinishedQuery.data?.page;
  const finishedPage = finishedQuery.data?.page;
  const allTasks = useMemo(() => [...unfinishedTasks, ...finishedTasks], [finishedTasks, unfinishedTasks]);
  const selectedTaskItem = useMemo(() => allTasks.find((item) => item.task.task_id === selectedTaskId) ?? null, [allTasks, selectedTaskId]);
  const pageStyle = {
    "--task-accent": "#9FB7D8",
    "--task-accent-glow": "rgba(159, 183, 216, 0.18)",
    "--task-accent-soft": "rgba(229, 236, 245, 0.64)",
    "--task-accent-surface": "rgba(241, 245, 250, 0.62)",
    "--task-accent-border": "rgba(159, 183, 216, 0.24)",
    "--task-accent-shadow": "rgba(120, 132, 148, 0.12)",
    "--task-paper": "rgba(255, 250, 244, 0.78)",
    "--task-paper-strong": "rgba(255, 247, 238, 0.9)",
    "--task-line": "rgba(156, 133, 113, 0.16)",
    "--task-ink": "#5f544b",
    "--task-copy": "rgba(95, 84, 75, 0.68)",
  } as CSSProperties;

  useEffect(() => {
    if (allTasks.length === 0) {
      return;
    }

    const focusTaskId = (location.state as { focusTaskId?: string; openDetail?: boolean } | null)?.focusTaskId;
    if (focusTaskId && allTasks.some((item) => item.task.task_id === focusTaskId)) {
      setSelectedTaskId(focusTaskId);
      if ((location.state as { openDetail?: boolean } | null)?.openDetail) {
        setDetailOpen(true);
      }
      navigate(location.pathname, { replace: true, state: null });
      return;
    }

    const selectedExists = selectedTaskId ? allTasks.some((item) => item.task.task_id === selectedTaskId) : false;
    if (selectedExists) {
      return;
    }

    const nextTask = unfinishedTasks.find((item) => item.task.status === "processing") ?? unfinishedTasks[0] ?? finishedTasks[0];
    if (nextTask) {
      setSelectedTaskId(nextTask.task.task_id);
    }
  }, [allTasks, finishedTasks, location.pathname, location.state, navigate, selectedTaskId, unfinishedTasks]);

  const taskDetailQuery = useQuery({
    enabled: shouldEnableDashboardTaskDetailQuery(selectedTaskId, detailOpen),
    queryKey: buildDashboardTaskDetailQueryKey(dataMode, selectedTaskId ?? ""),
    queryFn: () => loadTaskDetailData(selectedTaskId!, dataMode),
    refetchOnMount: securityRefreshPlan.refetchOnMount,
    refetchOnReconnect: false,
    refetchOnWindowFocus: false,
    retry: false,
  });

  const detailData = taskDetailQuery.data ?? (selectedTaskItem ? buildFallbackTaskDetailData(selectedTaskItem) : null);
  const detailErrorMessage = taskDetailQuery.isError ? (taskDetailQuery.error instanceof Error ? taskDetailQuery.error.message : "任务详情请求失败") : null;
  const detailState = taskDetailQuery.isError ? "error" : taskDetailQuery.isPending ? "loading" : "ready";
  const bucketErrors = [
    { error: unfinishedQuery.error, label: "未完成任务" },
    { error: finishedQuery.error, label: "已结束任务" },
  ].filter((item) => item.error);
  const heroTask = detailData?.task ?? selectedTaskItem?.task ?? null;

  const pageNotice =
    feedback ??
    (selectedTaskItem
      ? describeCurrentStep(selectedTaskItem.task, selectedTaskItem.experience)
      : "先浏览任务，再点开查看完整进展与上下文。");

  useEffect(() => {
    saveDashboardDataMode("tasks", dataMode);
  }, [dataMode]);

  useEffect(() => {
    if (dataMode === "mock") {
      return;
    }

    function invalidateTaskQueries() {
      for (const queryKey of securityRefreshPlan.invalidatePrefixes) {
        void queryClient.invalidateQueries({ queryKey });
      }
    }

    const clearDeliverySubscription = subscribeDeliveryReady(() => {
      invalidateTaskQueries();
    });

    const clearTaskSubscription = selectedTaskId
      ? subscribeTask(selectedTaskId, () => {
          invalidateTaskQueries();
        })
      : () => {};

    return () => {
      clearDeliverySubscription();
      clearTaskSubscription();
    };
  }, [dataMode, queryClient, securityRefreshPlan, selectedTaskId]);

  useEffect(() => {
    return () => {
      if (feedbackTimeoutRef.current) {
        window.clearTimeout(feedbackTimeoutRef.current);
      }
    };
  }, []);

  function showFeedback(message: string) {
    setFeedback(message);
    if (feedbackTimeoutRef.current) {
      window.clearTimeout(feedbackTimeoutRef.current);
    }
    feedbackTimeoutRef.current = window.setTimeout(() => setFeedback(null), 2400);
  }

  const taskControlMutation = useMutation({
    mutationFn: ({ action, taskId }: { action: "pause" | "resume" | "cancel" | "restart"; taskId: string }) => controlTaskByAction(taskId, action, dataMode),
    onSuccess: (outcome) => {
      showFeedback(outcome.result.bubble_message?.text ?? "任务操作已执行。");
      for (const queryKey of securityRefreshPlan.invalidatePrefixes) {
        void queryClient.invalidateQueries({ queryKey });
      }
    },
    onError: () => {
      showFeedback("任务操作暂时没有成功返回，请稍后再试。");
    },
  });

  async function handleOpenSafety() {
    if (!detailData) {
      return;
    }

    let resolvedDetailData = detailData;
    const safetyOpenPlan = resolveDashboardTaskSafetyOpenPlan(detailData.source);

    if (safetyOpenPlan.shouldRefetchDetail) {
      const refetchResult = await taskDetailQuery.refetch();

      if (!refetchResult.data || refetchResult.isError) {
        showFeedback("任务详情还在同步，先打开安全总览。");
        navigate(resolveDashboardRoutePath("safety"), {
          state: {
            source: "task-detail",
            taskId: detailData.task.task_id,
          },
        });
        return;
      }

      resolvedDetailData = refetchResult.data;
    }

    navigate(resolveDashboardRoutePath("safety"), { state: buildDashboardSafetyNavigationState(resolvedDetailData.detail) });
  }

  function handlePrimaryAction(action: "pause" | "resume" | "cancel" | "restart" | "open-safety") {
    if (!detailData) {
      return;
    }

    if (action === "open-safety") {
      void handleOpenSafety();
      return;
    }

    taskControlMutation.mutate({ action, taskId: detailData.task.task_id });
  }

  function handleLoadMore(group: "unfinished" | "finished") {
    const page = group === "unfinished" ? unfinishedPage : finishedPage;
    if (!page?.has_more) {
      return;
    }

    if (group === "unfinished") {
      setUnfinishedLimit((current) => current + LOAD_MORE_UNFINISHED_STEP);
      return;
    }

    setFinishedLimit((current) => current + LOAD_MORE_FINISHED_STEP);
  }

  return (
    <main className="dashboard-page task-preview-page" style={pageStyle}>
      <header className="dashboard-page__topbar">
        <Link className="dashboard-page__home-link" to={resolveDashboardRoutePath("home")}>
          <ArrowLeft className="h-4 w-4" />
          返回首页
        </Link>

        <nav aria-label="Dashboard modules" className="dashboard-page__module-nav">
          {dashboardModules.map((item) => (
            <NavLink key={item.route} className={({ isActive }) => cn("dashboard-page__module-link", isActive && "is-active")} to={item.path}>
              {item.title}
            </NavLink>
          ))}
        </nav>
      </header>

      <section className="dashboard-page__hero">
        <div className="dashboard-page__hero-copy">
          <p className="dashboard-page__eyebrow">Task Preview</p>
          <div className="dashboard-page__title-row">
            <LayoutList className="dashboard-page__title-icon" />
            <h1>任务</h1>
          </div>
          <p className="dashboard-page__description">先浏览任务，再点开看详细信息。未完成任务强调状态和当前执行步骤，已结束任务只保留最终状态。</p>
        </div>

          <div className="dashboard-card dashboard-card--status task-preview-page__hero-status">
            <p className="dashboard-card__kicker">当前聚焦</p>
            {pageNotice ? <p className="task-preview-page__hero-notice">{pageNotice}</p> : null}
            {heroTask ? (
              <>
                <p className="task-preview-page__hero-title">{heroTask.title}</p>
                <div className="dashboard-card__status-row">
                  <CircleDashed className="h-4 w-4" />
                  <span>{isTaskEnded(heroTask) ? "这条任务已经结束，可点开回看结果。" : "当前正在推进，点开可查看完整进展与上下文。"}</span>
                </div>
              </>
            ) : (
              <div className="dashboard-card__status-row">
                <CircleDashed className="h-4 w-4" />
                <span>当前还没有可展示的任务，列表成功返回后会显示在左侧。</span>
              </div>
            )}
          </div>
      </section>

      <section className="dashboard-page__grid task-preview-page__grid">
        <article className="dashboard-card task-preview-card-shell">
          <div className="task-preview-card-shell__header">
            <div>
              <p className="dashboard-card__kicker">未完成任务</p>
              <p className="task-preview-card-shell__description">任务名称、任务状态，以及当前执行到哪一步。</p>
            </div>
            <span className="task-preview-card-shell__count">{unfinishedQuery.isPending && !unfinishedQuery.data ? "..." : unfinishedTasks.length}</span>
          </div>
          <div className="task-preview-card-shell__list">
            {unfinishedQuery.isError && unfinishedTasks.length === 0 ? (
              <div className="task-preview-card-shell__empty task-preview-card-shell__empty--error">
                {unfinishedQuery.error instanceof Error ? unfinishedQuery.error.message : "请求失败"}
              </div>
            ) : unfinishedTasks.length > 0 ? (
              unfinishedTasks.map((item) => (
                <TaskPreviewCard
                  key={item.task.task_id}
                  isActive={item.task.task_id === selectedTaskId}
                  item={item}
                  onSelect={(taskId) => {
                    setSelectedTaskId(taskId);
                    setDetailOpen(true);
                  }}
                />
              ))
            ) : (
              <div className="task-preview-card-shell__empty">{unfinishedQuery.isPending && !unfinishedQuery.data ? "加载中" : "无"}</div>
            )}
          </div>
          {unfinishedPage?.has_more ? (
            <div className="task-preview-card-shell__footer">
              <button className="task-preview-card-shell__toggle" disabled={unfinishedQuery.isFetching} onClick={() => handleLoadMore("unfinished")} type="button">
                {unfinishedQuery.isFetching ? "加载中..." : "加载更多"}
              </button>
            </div>
          ) : null}
        </article>

        <article className="dashboard-card task-preview-card-shell">
          <div className="task-preview-card-shell__header">
            <div>
              <p className="dashboard-card__kicker">已结束任务</p>
              <p className="task-preview-card-shell__description">默认展示近 3 天，可展开到近 7 天和更早。</p>
            </div>
                <button className="task-preview-card-shell__toggle" onClick={() => setShowMoreFinished((current) => !current)} type="button">
                  {showMoreFinished ? "收起" : "更多"}
                </button>
              </div>

          <div className="task-preview-finished-groups">
            {finishedQuery.isError && finishedTasks.length === 0 ? (
              <div className="task-preview-card-shell__empty task-preview-card-shell__empty--error">
                {finishedQuery.error instanceof Error ? finishedQuery.error.message : "请求失败"}
              </div>
            ) : finishedGroups.length > 0 ? (
              finishedGroups.map((group) => (
                <section key={group.key} className="task-preview-finished-group">
                  <div>
                    <p className="task-preview-finished-group__title">{group.title}</p>
                    <p className="task-preview-finished-group__description">{group.description}</p>
                  </div>

                  <div className="task-preview-card-shell__list">
                    {group.items.map((item) => (
                      <TaskPreviewCard
                        key={item.task.task_id}
                        isActive={item.task.task_id === selectedTaskId}
                        item={item}
                        onSelect={(taskId) => {
                          setSelectedTaskId(taskId);
                          setDetailOpen(true);
                        }}
                      />
                    ))}
                  </div>
                </section>
              ))
            ) : (
              <div className="task-preview-card-shell__empty">{finishedQuery.isPending && !finishedQuery.data ? "加载中" : "无"}</div>
            )}
          </div>
          {finishedPage?.has_more ? (
            <div className="task-preview-card-shell__footer">
              <button className="task-preview-card-shell__toggle" disabled={finishedQuery.isFetching} onClick={() => handleLoadMore("finished")} type="button">
                {finishedQuery.isFetching ? "加载中..." : "加载更多历史"}
              </button>
            </div>
          ) : null}
        </article>

        <article className="dashboard-card task-preview-card-shell task-preview-card-shell--hint">
          <p className="dashboard-card__kicker">说明</p>
          <ul className="dashboard-card__list">
            <li>未完成任务会直接展示当前执行步骤，方便判断是否需要立即介入。</li>
            <li>等待授权、暂停、失败等任务会保留停住原因，避免只有一个“失败”状态。</li>
            <li>更多的上下文、成果区与操作按钮，都放在点击任务后的详情弹窗中。</li>
            <li>如需修改或补充当前任务，请到悬浮球继续处理。</li>
          </ul>
        </article>
      </section>

      <AnimatePresence>
        {detailOpen && detailData ? (
          <>
            <motion.button
              animate={{ opacity: 1 }}
              className="task-detail-modal__backdrop"
              exit={{ opacity: 0 }}
              initial={{ opacity: 0 }}
              onClick={() => setDetailOpen(false)}
              type="button"
            />
            <motion.div
              animate={{ opacity: 1, scale: 1, y: 0 }}
              className="task-detail-modal"
              exit={{ opacity: 0, scale: 0.98, y: 20 }}
              initial={{ opacity: 0, scale: 0.98, y: 16 }}
              transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
            >
              <TaskDetailPanel
                detailData={detailData}
                detailErrorMessage={detailErrorMessage}
                detailState={detailState}
                feedback={feedback}
                onAction={handlePrimaryAction}
                onClose={() => setDetailOpen(false)}
                onRetryDetail={taskDetailQuery.isError ? () => void taskDetailQuery.refetch() : null}
              />
            </motion.div>
          </>
        ) : null}
      </AnimatePresence>

      <AnimatePresence>
        {feedback || bucketErrors.length > 0 ? (
          <motion.aside
            animate={{ opacity: 1, y: 0 }}
            className="task-preview-page__floating-card"
            exit={{ opacity: 0, y: 12 }}
            initial={{ opacity: 0, y: 16 }}
            transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}
          >
            <div className="task-preview-page__floating-card-icon">
              <AlertTriangle className="h-4 w-4" />
            </div>
            <div className="task-preview-page__floating-card-copy">
              <p className="task-preview-page__floating-card-title">{feedback ? "操作提示" : "任务同步失败"}</p>
              <p className="task-preview-page__floating-card-text">
                {feedback ??
                  (bucketErrors.length === 1
                    ? `${bucketErrors[0].label}：${bucketErrors[0].error instanceof Error ? bucketErrors[0].error.message : "请求失败"}`
                    : `${bucketErrors.length} 个分区加载失败：${bucketErrors
                        .map((item) => `${item.label}${item.error instanceof Error ? `(${item.error.message})` : ""}`)
                        .join("、")}`)}
              </p>
            </div>
            {!feedback ? (
              <button
                className="task-preview-page__floating-card-action"
                onClick={() => {
                  void unfinishedQuery.refetch();
                  void finishedQuery.refetch();
                }}
                type="button"
              >
                <RefreshCcw className="h-4 w-4" />
                重试
              </button>
            ) : null}
          </motion.aside>
        ) : null}
      </AnimatePresence>

      <DashboardMockToggle
        enabled={dataMode === "mock"}
        onToggle={() => {
          setFeedback(null);
          setDataMode((current) => (current === "rpc" ? "mock" : "rpc"));
        }}
      />
    </main>
  );
}
