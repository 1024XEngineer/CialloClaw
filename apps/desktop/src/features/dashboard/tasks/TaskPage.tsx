import { useEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { Link, NavLink, useLocation, useNavigate } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  Archive,
  ArrowLeft,
  ChevronRight,
  Clock3,
  PlaneTakeoff,
  Radar,
  RefreshCcw,
  ShieldAlert,
  Sparkles,
} from "lucide-react";
import { AnimatePresence, LayoutGroup, motion } from "motion/react";
import { subscribeDeliveryReady, subscribeTask, subscribeTaskRuntime } from "@/rpc/subscriptions";
import { loadDashboardDataMode, saveDashboardDataMode } from "@/features/dashboard/shared/dashboardDataMode";
import { DashboardMockToggle } from "@/features/dashboard/shared/DashboardMockToggle";
import { buildDashboardSafetyNavigationState } from "@/features/dashboard/shared/dashboardSafetyNavigation";
import { resolveDashboardRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import { dashboardModules } from "@/features/dashboard/shared/dashboardRoutes";
import { cn } from "@/utils/cn";
import {
  buildTaskTowerCode,
  formatTaskSourceLabel,
  getFinishedTaskGroups,
  getTaskPreviewStatusLabel,
  getTaskPriorityLabel,
  getTaskProgress,
  getTaskStateVoice,
  getTaskStatusBadgeClass,
  sortTasksByLatest,
} from "./taskPage.mapper";
import {
  buildDashboardTaskArtifactQueryKey,
  buildDashboardTaskBucketQueryKey,
  buildDashboardTaskDetailQueryKey,
  buildDashboardTaskEventQueryKey,
  getDashboardTaskSecurityRefreshPlan,
  resolveDashboardTaskSafetyOpenPlan,
  shouldEnableDashboardTaskDetailQuery,
} from "./taskPage.query";
import {
  buildFallbackTaskDetailData,
  controlTaskByAction,
  DEFAULT_TASK_EVENT_FILTERS,
  loadTaskBucketPage,
  loadTaskDetailData,
  loadTaskEventPage,
  steerTaskByMessage,
  type TaskPageDataMode,
} from "./taskPage.service";
import {
  describeTaskOpenResultForCurrentTask,
  loadTaskArtifactPage,
  openTaskArtifactForTask,
  openTaskDeliveryForTask,
  performTaskOpenExecution,
  resolveTaskOpenExecutionPlan,
  type TaskOpenExecutionPlan,
} from "./taskOutput.service";
import { TaskDetailPanel } from "./components/TaskDetailPanel";
import { TaskPreviewCard } from "./components/TaskPreviewCard";
import type { TaskEventFilters, TaskListItem } from "./taskPage.types";
import "./taskPage.css";

type TaskClusterKey = "archive" | "departure" | "holding" | "irregular";

type TaskClusterSection = {
  code: string;
  emptyCopy: string;
  items: TaskListItem[];
  key: TaskClusterKey;
  subtitle: string;
  title: string;
};

const CLUSTER_PREVIEW_LIMIT = 3;
const INITIAL_UNFINISHED_LIMIT = 12;
const INITIAL_FINISHED_LIMIT = 24;
const LOAD_MORE_UNFINISHED_STEP = 12;
const LOAD_MORE_FINISHED_STEP = 24;

const clusterIcons = {
  archive: Archive,
  departure: PlaneTakeoff,
  holding: Clock3,
  irregular: AlertTriangle,
} as const;

/**
 * Presents the task dashboard as a center-stage scene while keeping task
 * detail, artifact, delivery, and safety flows wired to the stable contracts.
 */
export function TaskPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const [requestedTaskId, setRequestedTaskId] = useState<string | null>(null);
  const [detailOpen, setDetailOpen] = useState(true);
  const [showMoreFinished, setShowMoreFinished] = useState(false);
  const [expandedClusterKey, setExpandedClusterKey] = useState<TaskClusterKey | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [pendingConfirmation, setPendingConfirmation] = useState<TaskOpenExecutionPlan | null>(null);
  const [dataMode, setDataMode] = useState<TaskPageDataMode>(() => loadDashboardDataMode("tasks") as TaskPageDataMode);
  const [unfinishedLimit, setUnfinishedLimit] = useState(INITIAL_UNFINISHED_LIMIT);
  const [finishedLimit, setFinishedLimit] = useState(INITIAL_FINISHED_LIMIT);
  const [taskEventFilters, setTaskEventFilters] = useState<TaskEventFilters>(DEFAULT_TASK_EVENT_FILTERS);
  const feedbackTimeoutRef = useRef<number | null>(null);
  const securityRefreshPlan = useMemo(() => getDashboardTaskSecurityRefreshPlan(dataMode), [dataMode]);
  const routeFocusState = location.state as { focusTaskId?: string; openDetail?: boolean } | null;
  const routeFocusTaskId = typeof routeFocusState?.focusTaskId === "string" && routeFocusState.focusTaskId.trim().length > 0 ? routeFocusState.focusTaskId : null;

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

  const unfinishedTasks = useMemo(() => sortTasksByLatest(unfinishedQuery.data?.items ?? []), [unfinishedQuery.data?.items]);
  const finishedTasks = useMemo(() => sortTasksByLatest(finishedQuery.data?.items ?? []), [finishedQuery.data?.items]);
  const unfinishedPage = unfinishedQuery.data?.page;
  const finishedPage = finishedQuery.data?.page;
  const allTasks = useMemo(() => [...unfinishedTasks, ...finishedTasks], [finishedTasks, unfinishedTasks]);
  const selectedTaskItem = useMemo(() => allTasks.find((item) => item.task.task_id === selectedTaskId) ?? null, [allTasks, selectedTaskId]);
  const departureTasks = useMemo(
    () => unfinishedTasks.filter((item) => item.task.status === "confirming_intent" || item.task.status === "processing"),
    [unfinishedTasks],
  );
  const holdingTasks = useMemo(
    () => unfinishedTasks.filter((item) => item.task.status === "waiting_auth" || item.task.status === "waiting_input" || item.task.status === "paused"),
    [unfinishedTasks],
  );
  const irregularTasks = useMemo(() => allTasks.filter((item) => item.task.status === "blocked" || item.task.status === "failed"), [allTasks]);
  const archiveTasks = useMemo(
    () => finishedTasks.filter((item) => item.task.status === "completed" || item.task.status === "cancelled" || item.task.status === "ended_unfinished"),
    [finishedTasks],
  );
  const archiveGroups = useMemo(() => getFinishedTaskGroups(archiveTasks, showMoreFinished), [archiveTasks, showMoreFinished]);
  const clusterSections = useMemo<TaskClusterSection[]>(
    () => [
      {
        code: "Flow",
        emptyCopy: "当前还没有正在顺滑推进的任务。",
        items: departureTasks,
        key: "departure",
        subtitle: "优先关注已经进入推进节奏的任务。",
        title: "推进",
      },
      {
        code: "Pause",
        emptyCopy: "当前没有等待补充或授权的任务。",
        items: holdingTasks,
        key: "holding",
        subtitle: "这里聚合等待补充、授权或继续指令的事项。",
        title: "待命",
      },
      {
        code: "Edge",
        emptyCopy: "当前没有需要排障解释的异常任务。",
        items: irregularTasks,
        key: "irregular",
        subtitle: "把阻塞和失败集中收拢，方便快速看清原因。",
        title: "异常",
      },
      {
        code: "Shelf",
        emptyCopy: "当前还没有可回看的已结束任务。",
        items: archiveTasks,
        key: "archive",
        subtitle: "把最近几天已经收束的任务留在底部书架里。",
        title: "归档",
      },
    ],
    [archiveTasks, departureTasks, holdingTasks, irregularTasks],
  );

  const departureSection = clusterSections[0];
  const holdingSection = clusterSections[1];
  const irregularSection = clusterSections[2];
  const archiveSection = clusterSections[3];

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

  useEffect(() => {
    setTaskEventFilters(DEFAULT_TASK_EVENT_FILTERS);
  }, [selectedTaskId]);

  const taskDetailQuery = useQuery({
    enabled: shouldEnableDashboardTaskDetailQuery(selectedTaskId, detailOpen),
    queryKey: buildDashboardTaskDetailQueryKey(dataMode, selectedTaskId ?? ""),
    queryFn: () => loadTaskDetailData(selectedTaskId!, dataMode),
    refetchOnMount: securityRefreshPlan.refetchOnMount,
    refetchOnReconnect: false,
    refetchOnWindowFocus: false,
    retry: false,
  });

  const selectedDetailTaskId = taskDetailQuery.data?.task.task_id ?? null;
  const detailData = taskDetailQuery.data ?? (selectedTaskItem ? buildFallbackTaskDetailData(selectedTaskItem) : null);
  const detailErrorMessage = taskDetailQuery.isError ? (taskDetailQuery.error instanceof Error ? taskDetailQuery.error.message : "任务详情请求失败") : null;
  const detailState = taskDetailQuery.isError ? "error" : taskDetailQuery.isPending ? "loading" : "ready";
  const artifactListQuery = useQuery({
    enabled: detailOpen && dataMode === "rpc" && Boolean(selectedTaskId),
    queryKey: buildDashboardTaskArtifactQueryKey(dataMode, selectedTaskId ?? ""),
    queryFn: () => loadTaskArtifactPage(selectedTaskId!, dataMode),
    refetchOnMount: securityRefreshPlan.refetchOnMount,
    refetchOnReconnect: false,
    refetchOnWindowFocus: false,
    retry: false,
  });
  const taskEventsQuery = useQuery({
    enabled: detailOpen && Boolean(selectedTaskId),
    queryKey: buildDashboardTaskEventQueryKey(dataMode, selectedTaskId ?? "", taskEventFilters),
    queryFn: () => loadTaskEventPage(selectedTaskId!, dataMode, taskEventFilters),
    refetchOnMount: securityRefreshPlan.refetchOnMount,
    refetchOnReconnect: false,
    refetchOnWindowFocus: false,
    retry: false,
  });

  const bucketErrors = [
    { error: unfinishedQuery.error, label: "在场任务" },
    { error: finishedQuery.error, label: "归档任务" },
  ].filter((item) => item.error);

  const selectedProgress = detailData ? getTaskProgress(detailData.detail.timeline) : null;
  const selectedStateVoice = detailData ? getTaskStateVoice(detailData.task, detailData.experience, detailData.detail.timeline) : null;
  const selectedArtifactCount = detailData ? artifactListQuery.data?.items.length ?? detailData.detail.artifacts.length : 0;
  const selectedUpdateLabel = detailData ? new Date(detailData.task.updated_at).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" }) : "—";

  useEffect(() => {
    if (!routeFocusTaskId) {
      return;
    }

    setRequestedTaskId(routeFocusTaskId);
    setSelectedTaskId(routeFocusTaskId);
    setDetailOpen(routeFocusState?.openDetail ?? true);
    navigate(location.pathname, { replace: true, state: null });
  }, [location.pathname, navigate, routeFocusState?.openDetail, routeFocusTaskId]);

  useEffect(() => {
    if (!requestedTaskId) {
      return;
    }

    const taskBecameVisible = allTasks.some((item) => item.task.task_id === requestedTaskId);
    if (!taskBecameVisible && selectedDetailTaskId !== requestedTaskId) {
      return;
    }

    setRequestedTaskId(null);
  }, [allTasks, requestedTaskId, selectedDetailTaskId]);

  useEffect(() => {
    if (routeFocusTaskId) {
      return;
    }

    if (!selectedTaskId) {
      if (allTasks.length === 0) {
        return;
      }

      const nextTask = departureTasks[0] ?? holdingTasks[0] ?? irregularTasks[0] ?? archiveTasks[0] ?? allTasks[0] ?? null;
      if (nextTask) {
        setSelectedTaskId(nextTask.task.task_id);
        setDetailOpen(true);
      }
      return;
    }

    const selectedExists = allTasks.some((item) => item.task.task_id === selectedTaskId);
    if (selectedExists || requestedTaskId === selectedTaskId || selectedDetailTaskId === selectedTaskId) {
      return;
    }

    const nextTask = departureTasks[0] ?? holdingTasks[0] ?? irregularTasks[0] ?? archiveTasks[0] ?? allTasks[0] ?? null;
    if (nextTask) {
      setSelectedTaskId(nextTask.task.task_id);
      setDetailOpen(true);
      return;
    }

    setSelectedTaskId(null);
    setDetailOpen(false);
  }, [allTasks, archiveTasks, departureTasks, holdingTasks, irregularTasks, requestedTaskId, routeFocusTaskId, selectedDetailTaskId, selectedTaskId]);

  useEffect(() => {
    saveDashboardDataMode("tasks", dataMode);
  }, [dataMode]);

  useEffect(() => {
    if (dataMode === "mock") {
      return;
    }

    function invalidateSelectedTaskDetail(taskId: string) {
      void queryClient.invalidateQueries({ queryKey: buildDashboardTaskDetailQueryKey(dataMode, taskId) });
      void queryClient.invalidateQueries({ queryKey: buildDashboardTaskArtifactQueryKey(dataMode, taskId) });
      void queryClient.invalidateQueries({ queryKey: buildDashboardTaskEventQueryKey(dataMode, taskId, taskEventFilters) });
    }

    function invalidateTaskQueries(deliveryTaskId?: string) {
      for (const queryKey of securityRefreshPlan.invalidatePrefixes) {
        void queryClient.invalidateQueries({ queryKey });
      }
      if (selectedTaskId && (!deliveryTaskId || deliveryTaskId === selectedTaskId)) {
        invalidateSelectedTaskDetail(selectedTaskId);
      }
    }

    const clearDeliverySubscription = subscribeDeliveryReady((payload) => {
      invalidateTaskQueries(payload.task_id);
    });

    const clearTaskSubscription = selectedTaskId
      ? subscribeTask(selectedTaskId, () => {
          invalidateTaskQueries(selectedTaskId);
        })
      : () => {};

    const clearRuntimeSubscription = selectedTaskId
      ? subscribeTaskRuntime(selectedTaskId, () => {
          invalidateSelectedTaskDetail(selectedTaskId);
        })
      : () => {};

    return () => {
      clearDeliverySubscription();
      clearTaskSubscription();
      clearRuntimeSubscription();
    };
  }, [dataMode, queryClient, securityRefreshPlan, selectedTaskId, taskEventFilters]);

  useEffect(() => {
    return () => {
      if (feedbackTimeoutRef.current) {
        window.clearTimeout(feedbackTimeoutRef.current);
      }
    };
  }, []);

  function clearFeedbackTimeout() {
    if (feedbackTimeoutRef.current) {
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

    feedbackTimeoutRef.current = window.setTimeout(() => setFeedback(null), 2400);
  }

  const taskControlMutation = useMutation({
    mutationFn: ({ action, taskId }: { action: "pause" | "resume" | "cancel" | "restart"; taskId: string }) => controlTaskByAction(taskId, action, dataMode),
    onSuccess: (outcome, variables) => {
      showFeedback(outcome.result.bubble_message?.text ?? "任务操作已执行。");
      for (const queryKey of securityRefreshPlan.invalidatePrefixes) {
        void queryClient.invalidateQueries({ queryKey });
      }
      void queryClient.invalidateQueries({ queryKey: buildDashboardTaskArtifactQueryKey(dataMode, variables.taskId) });
    },
    onError: () => {
      showFeedback("任务操作暂时没有成功返回，请稍后再试。");
    },
  });

  async function handleOpenPlan(plan: TaskOpenExecutionPlan, approveOutsideWorkspace = false) {
    const sameTaskMessage = describeTaskOpenResultForCurrentTask(plan, selectedTaskId);
    if (sameTaskMessage) {
      setPendingConfirmation(null);
      setDetailOpen(true);
      showFeedback(sameTaskMessage);
      return;
    }

    const result = await performTaskOpenExecution(plan, { approveOutsideWorkspace });

    if (result.type === "confirm_required") {
      setPendingConfirmation(result.plan);
      showFeedback(result.message, false);
      return;
    }

    setPendingConfirmation(null);

    if (result.type === "task_detail") {
      setSelectedTaskId(result.taskId);
      setDetailOpen(true);
      showFeedback(result.message);
      return;
    }

    showFeedback(result.message);
  }

  async function handleResolvedOpen(result: Awaited<ReturnType<typeof openTaskArtifactForTask>> | Awaited<ReturnType<typeof openTaskDeliveryForTask>>) {
    await handleOpenPlan(resolveTaskOpenExecutionPlan(result));
  }

  const artifactOpenMutation = useMutation({
    mutationFn: ({ artifactId, taskId }: { artifactId: string; taskId: string }) => openTaskArtifactForTask(taskId, artifactId, dataMode),
    onSuccess: async (result) => {
      await handleResolvedOpen(result);
    },
    onError: (error) => {
      showFeedback(error instanceof Error ? `打开成果失败：${error.message}` : "打开成果失败，请稍后再试。");
    },
  });

  const deliveryOpenMutation = useMutation({
    mutationFn: ({ artifactId, taskId }: { artifactId?: string; taskId: string }) => openTaskDeliveryForTask(taskId, artifactId, dataMode),
    onSuccess: async (result) => {
      await handleResolvedOpen(result);
    },
    onError: (error) => {
      showFeedback(error instanceof Error ? `打开结果失败：${error.message}` : "打开结果失败，请稍后再试。");
    },
  });

  const taskSteerMutation = useMutation({
    mutationFn: ({ message, taskId }: { message: string; taskId: string }) => steerTaskByMessage(taskId, message, dataMode),
    onSuccess: (result, variables) => {
      showFeedback(result.bubble_message?.text ?? "已记录新的补充要求。");
      for (const queryKey of securityRefreshPlan.invalidatePrefixes) {
        void queryClient.invalidateQueries({ queryKey });
      }
      void queryClient.invalidateQueries({ queryKey: buildDashboardTaskDetailQueryKey(dataMode, variables.taskId) });
      void queryClient.invalidateQueries({ queryKey: buildDashboardTaskEventQueryKey(dataMode, variables.taskId, taskEventFilters) });
    },
    onError: (error) => {
      showFeedback(error instanceof Error ? `补充要求提交失败：${error.message}` : "补充要求提交失败，请稍后再试。");
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

  function handleOpenArtifact(artifactId: string) {
    if (!detailData) {
      return;
    }

    artifactOpenMutation.mutate({ artifactId, taskId: detailData.task.task_id });
  }

  function handleOpenLatestDelivery() {
    if (!detailData) {
      return;
    }

    deliveryOpenMutation.mutate({ taskId: detailData.task.task_id });
  }

  function handleSteerTask(message: string) {
    if (!detailData) {
      return;
    }

    taskSteerMutation.mutate({ message, taskId: detailData.task.task_id });
  }

  function handleApplyTaskEventFilters(nextFilters: TaskEventFilters) {
    setTaskEventFilters(nextFilters);
  }

  function handleResetTaskEventFilters() {
    setTaskEventFilters(DEFAULT_TASK_EVENT_FILTERS);
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

  function handleSelectTask(taskId: string) {
    setRequestedTaskId(null);
    setSelectedTaskId(taskId);
    setDetailOpen(true);
  }

  function toggleCluster(key: TaskClusterKey) {
    setExpandedClusterKey((current) => (current === key ? null : key));
  }

  function renderClusterCards(section: TaskClusterSection) {
    if (section.items.length === 0) {
      return <div className="task-runway__empty">{section.emptyCopy}</div>;
    }

    if (section.key === "archive") {
      return (
        <div className="task-runway__archive-groups">
          {archiveGroups.map((group) => (
            <section key={group.key} className="task-runway__archive-group">
              <div className="task-runway__archive-head">
                <p className="task-runway__archive-title">{group.title}</p>
                <p className="task-runway__archive-copy">{group.description}</p>
              </div>
              <div className="task-runway__manifest task-runway__manifest--archive">
                {group.items.map((item) => (
                  <TaskPreviewCard
                    key={item.task.task_id}
                    isActive={item.task.task_id === selectedTaskId}
                    item={item}
                    onSelect={handleSelectTask}
                    runwayLabel={section.code}
                  />
                ))}
              </div>
            </section>
          ))}
        </div>
      );
    }

    const isExpanded = expandedClusterKey === section.key;
    const visibleItems = isExpanded ? section.items : section.items.slice(0, CLUSTER_PREVIEW_LIMIT);

    return (
      <div className={cn("task-runway__manifest", !isExpanded && "is-condensed")}>
        {visibleItems.map((item, index) => (
          <TaskPreviewCard
            key={item.task.task_id}
            isActive={item.task.task_id === selectedTaskId}
            isPeeked={!isExpanded && index > 0}
            item={item}
            onSelect={handleSelectTask}
            runwayLabel={section.code}
          />
        ))}
      </div>
    );
  }

  function renderClusterSection(section: TaskClusterSection) {
    const Icon = clusterIcons[section.key];
    const isExpanded = expandedClusterKey === section.key;
    const shouldShowToggle = section.key === "archive" ? section.items.length > 0 : section.items.length > CLUSTER_PREVIEW_LIMIT;
    const isFocused = selectedTaskId ? section.items.some((item) => item.task.task_id === selectedTaskId) : false;

    return (
      <article key={section.key} className={cn("task-runway", "task-cluster", `is-${section.key}`, isFocused && "is-focused")}>
        <header className="task-runway__header">
          <div className="task-cluster__header-copy">
            <span className="task-cluster__icon">
              <Icon className="h-4 w-4" />
            </span>
            <div>
              <span className="task-runway__code">{section.code}</span>
              <h2>{section.title}</h2>
              <p>{section.subtitle}</p>
            </div>
          </div>

          <div className="task-runway__header-actions">
            <span className="task-runway__count">{section.items.length}</span>
            {shouldShowToggle ? (
              <button className="task-runway__toggle" onClick={() => (section.key === "archive" ? setShowMoreFinished((current) => !current) : toggleCluster(section.key))} type="button">
                {section.key === "archive" ? (showMoreFinished ? "收起" : "更多") : isExpanded ? "收起" : "更多"}
              </button>
            ) : null}
          </div>
        </header>

        <div className="task-runway__content">{renderClusterCards(section)}</div>
      </article>
    );
  }

  return (
    <main className="dashboard-page task-tower-page task-cloud-page" style={pageStyle}>
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

      <section className="task-tower task-cloud">
        <LayoutGroup id="task-cloud-layout">
          <div className="task-cloud__scene">
            {renderClusterSection(departureSection)}

            <aside className="task-tower__deck task-cloud__stage">
              {detailData && selectedProgress && selectedStateVoice ? (
                <motion.div className="task-cloud__stage-shell" layout>
                  <header className="task-cloud__stage-header">
                    <div className="task-cloud__stage-lockup">
                      <motion.span className="task-cloud__stage-signal" layoutId={`task-cloud-signal-${detailData.task.task_id}`} />
                      <motion.span className="task-cloud__stage-code" layoutId={`task-cloud-code-${detailData.task.task_id}`}>
                        {buildTaskTowerCode(detailData.task.task_id)}
                      </motion.span>
                      <span className={cn("task-cloud__stage-status", getTaskStatusBadgeClass(detailData.task.status))}>{getTaskPreviewStatusLabel(detailData.task.status)}</span>
                    </div>

                    <button className="task-runway__toggle task-cloud__stage-toggle" onClick={() => setDetailOpen((current) => !current)} type="button">
                      {detailOpen ? "收起详情" : "展开详情"}
                    </button>
                  </header>

                  <section className="task-cloud__stage-summary">
                    <div className="task-cloud__stage-title-row">
                      <div>
                        <p className="task-cloud__stage-kicker">
                          {formatTaskSourceLabel(detailData.task.source_type)} · {getTaskPriorityLabel(detailData.experience.priority)} · 中央舞台
                        </p>
                        <h2>{detailData.task.title}</h2>
                        <p className="task-cloud__stage-copy">{selectedStateVoice.body}</p>
                      </div>

                      <div className="task-cloud__stage-progress-pill">{selectedProgress.percent}%</div>
                    </div>

                    <div className="task-cloud__stage-progress-track" aria-hidden="true">
                      <motion.span
                        className="task-cloud__stage-progress-fill"
                        layout
                        style={{ width: `${Math.max(selectedProgress.percent, 8)}%` }}
                        transition={{ bounce: 0.2, damping: 26, stiffness: 260, type: "spring" }}
                      />
                    </div>

                    <div className="task-cloud__stage-grid">
                      <article className="task-cloud__stage-card">
                        <PlaneTakeoff className="h-4 w-4" />
                        <div>
                          <span>当前步骤</span>
                          <strong>{selectedProgress.currentLabel}</strong>
                        </div>
                      </article>
                      <article className="task-cloud__stage-card is-highlight">
                        <Sparkles className="h-4 w-4" />
                        <div>
                          <span>下一步</span>
                          <strong>{detailData.experience.nextAction}</strong>
                        </div>
                      </article>
                      <article className="task-cloud__stage-card">
                        <Clock3 className="h-4 w-4" />
                        <div>
                          <span>最近更新</span>
                          <strong>{selectedUpdateLabel}</strong>
                        </div>
                      </article>
                      <article className="task-cloud__stage-card">
                        <ShieldAlert className="h-4 w-4" />
                        <div>
                          <span>风险状态</span>
                          <strong>{detailData.detail.security_summary.risk_level}</strong>
                        </div>
                      </article>
                      <article className="task-cloud__stage-card">
                        <Archive className="h-4 w-4" />
                        <div>
                          <span>产物数量</span>
                          <strong>{selectedArtifactCount}</strong>
                        </div>
                      </article>
                    </div>
                  </section>

                  <div className="task-cloud__stage-body">
                    <AnimatePresence mode="wait">
                      {detailOpen ? (
                        <motion.div
                          key="detail"
                          animate={{ opacity: 1, y: 0 }}
                          className="task-cloud__stage-detail"
                          exit={{ opacity: 0, y: 12 }}
                          initial={{ opacity: 0, y: 18 }}
                          transition={{ bounce: 0.18, damping: 24, stiffness: 250, type: "spring" }}
                        >
                          <TaskDetailPanel
                            artifactActionPendingId={artifactOpenMutation.isPending ? artifactOpenMutation.variables?.artifactId ?? null : null}
                            artifactErrorMessage={artifactListQuery.isError ? (artifactListQuery.error instanceof Error ? artifactListQuery.error.message : "成果列表请求失败") : null}
                            artifactItems={artifactListQuery.data?.items ?? detailData.detail.artifacts ?? []}
                            artifactLoading={artifactListQuery.isPending}
                            detailData={detailData}
                            detailWarningMessage={detailData.detailWarningMessage ?? null}
                            detailErrorMessage={detailErrorMessage}
                            eventErrorMessage={taskEventsQuery.isError ? (taskEventsQuery.error instanceof Error ? taskEventsQuery.error.message : "运行时事件请求失败") : null}
                            eventFilters={taskEventFilters}
                            eventItems={taskEventsQuery.data?.items ?? []}
                            eventLoading={taskEventsQuery.isPending}
                            detailState={detailState}
                            deliveryActionPending={deliveryOpenMutation.isPending}
                            feedback={feedback}
                            onAction={handlePrimaryAction}
                            onClose={() => setDetailOpen(false)}
                            onOpenArtifact={handleOpenArtifact}
                            onOpenLatestDelivery={handleOpenLatestDelivery}
                            onApplyEventFilters={handleApplyTaskEventFilters}
                            onResetEventFilters={handleResetTaskEventFilters}
                            onRetryDetail={taskDetailQuery.isError ? () => void taskDetailQuery.refetch() : null}
                            onSteerTask={handleSteerTask}
                            steeringPending={taskSteerMutation.isPending}
                          />
                        </motion.div>
                      ) : (
                        <motion.div
                          key="standby"
                          animate={{ opacity: 1, y: 0 }}
                          className="task-cloud__standby"
                          exit={{ opacity: 0, y: 12 }}
                          initial={{ opacity: 0, y: 18 }}
                          transition={{ bounce: 0.18, damping: 24, stiffness: 250, type: "spring" }}
                        >
                          <section className="task-cloud__standby-card">
                            <h3>{selectedStateVoice.title}</h3>
                            <p>{selectedStateVoice.body}</p>
                          </section>

                          <section className="task-cloud__standby-card is-highlight">
                            <h3>{detailData.experience.nextAction}</h3>
                            <p>{detailData.experience.suggestedNext}</p>
                          </section>

                          <section className="task-cloud__standby-card">
                            <p>如需修改或补充当前任务，请到悬浮球继续处理。</p>
                            <button className="task-tower__standby-action" onClick={() => setDetailOpen(true)} type="button">
                              展开舞台详情
                              <ChevronRight className="h-4 w-4" />
                            </button>
                          </section>
                        </motion.div>
                      )}
                    </AnimatePresence>
                  </div>
                </motion.div>
              ) : (
                <div className="task-cloud__stage-empty">
                  <Radar className="h-6 w-6" />
                  <h2>舞台正在等下一条任务亮起</h2>
                  <p>当前还没有可锁定的任务。新的任务回到这里时，会先在中央舞台落下，再把细节慢慢展开。</p>
                </div>
              )}
            </aside>

            {renderClusterSection(holdingSection)}
            {renderClusterSection(irregularSection)}
          </div>
        </LayoutGroup>

        {renderClusterSection(archiveSection)}

        {unfinishedPage?.has_more || finishedPage?.has_more ? (
          <div className="task-tower__load-more">
            {unfinishedPage?.has_more ? (
              <button className="task-runway__toggle" disabled={unfinishedQuery.isFetching} onClick={() => handleLoadMore("unfinished")} type="button">
                {unfinishedQuery.isFetching ? "加载中..." : "更多任务"}
              </button>
            ) : null}
            {finishedPage?.has_more ? (
              <button className="task-runway__toggle" disabled={finishedQuery.isFetching} onClick={() => handleLoadMore("finished")} type="button">
                {finishedQuery.isFetching ? "加载中..." : "更多归档"}
              </button>
            ) : null}
          </div>
        ) : null}
      </section>

      <AnimatePresence>
        {detailOpen && detailData && requestedTaskId === "__legacy_modal__" ? (
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
              {/* Detached modal rendering is intentionally disabled.
              <TaskDetailPanel
                artifactActionPendingId={artifactOpenMutation.isPending ? artifactOpenMutation.variables?.artifactId ?? null : null}
                artifactErrorMessage={artifactListQuery.isError ? (artifactListQuery.error instanceof Error ? artifactListQuery.error.message : "成果列表请求失败") : null}
                artifactItems={artifactListQuery.data?.items ?? detailData?.detail.artifacts ?? []}
                artifactLoading={artifactListQuery.isPending}
                detailData={detailData}
                detailWarningMessage={detailData.detailWarningMessage ?? null}
                detailErrorMessage={detailErrorMessage}
                eventErrorMessage={taskEventsQuery.isError ? (taskEventsQuery.error instanceof Error ? taskEventsQuery.error.message : "运行时事件请求失败") : null}
                eventFilters={taskEventFilters}
                eventItems={taskEventsQuery.data?.items ?? []}
                eventLoading={taskEventsQuery.isPending}
                detailState={detailState}
                deliveryActionPending={deliveryOpenMutation.isPending}
                feedback={feedback}
                onAction={handlePrimaryAction}
                onClose={() => setDetailOpen(false)}
                onOpenArtifact={handleOpenArtifact}
                onOpenLatestDelivery={handleOpenLatestDelivery}
                onApplyEventFilters={handleApplyTaskEventFilters}
                onResetEventFilters={handleResetTaskEventFilters}
                onRetryDetail={taskDetailQuery.isError ? () => void taskDetailQuery.refetch() : null}
                onSteerTask={handleSteerTask}
                steeringPending={taskSteerMutation.isPending}
              />
              */}
            </motion.div>
          </>
        ) : null}
      </AnimatePresence>

      <AnimatePresence>
        {pendingConfirmation || feedback || bucketErrors.length > 0 ? (
          <motion.aside
            animate={{ opacity: 1, y: 0 }}
            className="task-tower__floating-card"
            exit={{ opacity: 0, y: 12 }}
            initial={{ opacity: 0, y: 18 }}
            transition={{ bounce: 0.18, damping: 24, stiffness: 250, type: "spring" }}
          >
            <div className="task-tower__floating-icon">
              <AlertTriangle className="h-4 w-4" />
            </div>
            <div className="task-tower__floating-copy">
              <p className="task-tower__floating-title">{pendingConfirmation ? "工作区外路径确认" : feedback ? "柔雾提示" : "任务同步失败"}</p>
              <p className="task-tower__floating-text">
                {pendingConfirmation
                  ? pendingConfirmation.confirmMessage
                  : feedback ??
                    (bucketErrors.length === 1
                      ? `${bucketErrors[0].label}：${bucketErrors[0].error instanceof Error ? bucketErrors[0].error.message : "请求失败"}`
                      : `${bucketErrors.length} 个分组加载失败：${bucketErrors
                          .map((item) => `${item.label}${item.error instanceof Error ? `（${item.error.message}）` : ""}`)
                          .join("；")}`)}
              </p>
            </div>

            {pendingConfirmation ? (
              <div className="task-tower__floating-actions">
                <button
                  className="task-tower__floating-action is-strong"
                  onClick={async () => {
                    const currentConfirmation = pendingConfirmation;
                    if (!currentConfirmation) {
                      return;
                    }

                    setPendingConfirmation(null);
                    await handleOpenPlan(currentConfirmation, true);
                  }}
                  type="button"
                >
                  确认打开
                </button>
                <button
                  className="task-tower__floating-action"
                  onClick={() => {
                    setPendingConfirmation(null);
                    setFeedback(null);
                  }}
                  type="button"
                >
                  取消
                </button>
              </div>
            ) : !feedback ? (
              <button
                className="task-tower__floating-action"
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
          setPendingConfirmation(null);
          setDataMode((current) => (current === "rpc" ? "mock" : "rpc"));
        }}
      />
    </main>
  );
}
