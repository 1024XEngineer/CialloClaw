import { useEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { Link, NavLink, useLocation, useNavigate } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  Archive,
  ArrowLeft,
  Clock3,
  PlaneTakeoff,
  Radar,
  RefreshCcw,
  X,
} from "lucide-react";
import { AnimatePresence, LayoutGroup, motion } from "motion/react";
import { subscribeDeliveryReady, subscribeTaskRuntime, subscribeTaskUpdated } from "@/rpc/subscriptions";
import { readDashboardTaskDetailRouteState } from "@/features/dashboard/shared/dashboardTaskDetailNavigation";
import { buildDashboardSafetyNavigationState } from "@/features/dashboard/shared/dashboardSafetyNavigation";
import { resolveDashboardRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import { dashboardModules } from "@/features/dashboard/shared/dashboardRoutes";
import { cn } from "@/utils/cn";
import {
  buildTaskTowerCode,
  describeCurrentStep,
  formatTaskSourceLabel,
  getFinishedTaskGroups,
  getTaskPreviewStatusLabel,
  getTaskPriorityLabel,
  getTaskProgress,
  getTaskRunwayTone,
  getTaskStateVoice,
  getTaskStatusBadgeClass,
  isTaskEnded,
  sortTasksByLatest,
} from "./taskPage.mapper";
import {
  buildDashboardTaskArtifactQueryKey,
  buildDashboardTaskBucketQueryKey,
  buildDashboardTaskDetailQueryKey,
  buildDashboardTaskEventQueryKey,
  dashboardTaskEventQueryPrefix,
  getDashboardTaskSecurityRefreshPlan,
  resolveDashboardTaskSafetyOpenPlan,
  shouldEnableDashboardTaskDetailQuery,
} from "./taskPage.query";
import {
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
  performTaskOpenExecution,
  resolveTaskOpenExecutionPlan,
} from "./taskOutput.service";
import { resolveDashboardTaskDeliveryRoutePath } from "./taskDeliveryNavigation";
import { TaskDetailPanel } from "./components/TaskDetailPanel";
import { TaskPreviewCard } from "./components/TaskPreviewCard";
import type { TaskEventFilters, TaskListItem, TaskPrimaryAction } from "./taskPage.types";
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
const TASK_BUCKET_REFRESH_DEBOUNCE_MS = 300;

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
  // Keep the current task focus stable across transient bucket refreshes so
  // the detail modal does not snap back to an auto-picked fallback selection.
  const [requestedTaskId, setRequestedTaskId] = useState<string | null>(null);
  const [stageInitialized, setStageInitialized] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [showMoreFinished, setShowMoreFinished] = useState(false);
  const [expandedClusterKey, setExpandedClusterKey] = useState<TaskClusterKey | null>(null);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [steeringSuccessVersion, setSteeringSuccessVersion] = useState(0);
  const dataMode: TaskPageDataMode = "rpc";
  const [unfinishedLimit, setUnfinishedLimit] = useState(INITIAL_UNFINISHED_LIMIT);
  const [finishedLimit, setFinishedLimit] = useState(INITIAL_FINISHED_LIMIT);
  const [taskEventFilters, setTaskEventFilters] = useState<TaskEventFilters>(DEFAULT_TASK_EVENT_FILTERS);
  const feedbackTimeoutRef = useRef<number | null>(null);
  const securityRefreshPlan = getDashboardTaskSecurityRefreshPlan(dataMode);
  const detailRouteState = readDashboardTaskDetailRouteState(location.state);
  const routeFocusTaskId = detailRouteState?.focusTaskId ?? null;

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
    () => unfinishedTasks.filter((item) => getTaskRunwayTone(item.task.status) === "departure"),
    [unfinishedTasks],
  );
  const holdingTasks = useMemo(
    () => unfinishedTasks.filter((item) => getTaskRunwayTone(item.task.status) === "holding"),
    [unfinishedTasks],
  );
  const irregularTasks = useMemo(() => allTasks.filter((item) => getTaskRunwayTone(item.task.status) === "irregular"), [allTasks]);
  const archiveTasks = useMemo(
    () => finishedTasks.filter((item) => getTaskRunwayTone(item.task.status) === "archive"),
    [finishedTasks],
  );
  const hasArchiveOlderItems = useMemo(() => {
    const now = Date.now();
    return archiveTasks.some((item) => {
      const finishedAt = item.task.finished_at ? new Date(item.task.finished_at).getTime() : now;
      const diffDays = (now - finishedAt) / (1000 * 60 * 60 * 24);
      return diffDays > 7;
    });
  }, [archiveTasks]);
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
    "--task-accent": "var(--cc-module-task)",
    "--task-accent-strong": "var(--cc-module-task-strong)",
    "--task-accent-soft": "var(--cc-module-safety)",
    "--task-alert": "var(--cc-honey)",
    "--task-danger": "var(--cc-rose)",
    "--task-success": "var(--cc-sage)",
    "--task-ink": "var(--cc-ink)",
    "--task-copy": "var(--cc-ink-muted)",
    "--task-line": "var(--cc-line)",
    "--task-panel": "var(--cc-paper)",
    "--task-panel-strong": "var(--cc-paper-strong)",
    "--task-panel-soft": "var(--cc-glass)",
  } as CSSProperties;

  useEffect(() => {
    setTaskEventFilters(DEFAULT_TASK_EVENT_FILTERS);
  }, [selectedTaskId]);

  useEffect(() => {
    /**
     * Routed task-detail opens can legitimately target tasks outside the
     * currently loaded preview buckets. Accept the routed task id first and let
     * the canonical detail query load it instead of blocking on paginated list data.
     */
    if (detailRouteState) {
      setRequestedTaskId(detailRouteState.focusTaskId);
      setSelectedTaskId(detailRouteState.focusTaskId);
      if (detailRouteState.openDetail) {
        setDetailOpen(true);
      }
      navigate(location.pathname, { replace: true, state: null });
      return;
    }

    if (selectedTaskId && detailOpen) {
      return;
    }
  }, [detailOpen, detailRouteState, location.pathname, navigate, selectedTaskId]);
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
  const detailData = taskDetailQuery.data ?? null;
  const selectedTaskPreview = detailData ?? (selectedTaskItem
    ? {
        experience: selectedTaskItem.experience,
        task: selectedTaskItem.task,
      }
    : null);
  const selectedTask = selectedTaskPreview?.task ?? null;
  const selectedTaskControlTargetId = selectedTask?.task_id ?? selectedTaskId;
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
  const selectedProgress = selectedTaskPreview ? getTaskProgress(detailData?.detail.timeline ?? []) : null;
  const selectedStateVoice = selectedTaskPreview ? getTaskStateVoice(selectedTaskPreview.task, selectedTaskPreview.experience, detailData?.detail.timeline ?? []) : null;
  const selectedTaskEnded = selectedTaskPreview ? isTaskEnded(selectedTaskPreview.task) : false;
  const selectedWaitingInputGuidance =
    selectedTaskPreview?.task.status === "waiting_input"
      ? "如需修改或补充当前任务，请直接使用当前任务详情里的“补充新的执行要求”输入框。"
      : null;
  const selectedStageDescription = selectedTaskPreview
    ? selectedTaskEnded
      ? selectedTaskPreview.experience.endedSummary ?? selectedStateVoice?.body ?? describeCurrentStep(selectedTaskPreview.task, selectedTaskPreview.experience)
      : selectedWaitingInputGuidance ?? `${describeCurrentStep(selectedTaskPreview.task, selectedTaskPreview.experience)} 下一步：${selectedTaskPreview.experience.nextAction}`
    : null;
  const selectedUpdateLabel = selectedTaskPreview ? new Date(selectedTaskPreview.task.updated_at).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" }) : "--";
  const fallbackDetailActions: TaskPrimaryAction[] | null = !selectedTaskPreview && selectedTaskControlTargetId
    ? [
        { action: "restart", label: "重启", tooltip: "重新拉起当前任务。" },
        { action: "cancel", label: "取消", tooltip: "结束当前任务，并保留已有轨迹。" },
        { action: "open-safety", label: "安全总览", tooltip: "查看当前任务相关的安全总览。" },
      ]
    : null;
  const fallbackOutputAccess = !selectedTaskPreview && Boolean(selectedTaskId);

  useEffect(() => {
    if (routeFocusTaskId || stageInitialized || selectedTaskId) {
      return;
    }

    // Seed the stage with the current highest-priority task on first entry so
    // the workspace opens with a concrete focal task instead of an empty dock.
    const stageEntryTask = departureTasks[0] ?? holdingTasks[0] ?? irregularTasks[0] ?? archiveTasks[0] ?? allTasks[0] ?? null;
    if (!stageEntryTask) {
      return;
    }

    focusTaskDetail(stageEntryTask.task.task_id, false);
    setStageInitialized(true);
  }, [allTasks, archiveTasks, departureTasks, holdingTasks, irregularTasks, routeFocusTaskId, selectedTaskId, stageInitialized]);

  useEffect(() => {
    if (routeFocusTaskId) {
      return;
    }

    if (!selectedTaskId) {
      setDetailOpen(false);
      return;
    }

    const selectedExists = allTasks.some((item) => item.task.task_id === selectedTaskId);
    const stageTaskStillResolved = selectedDetailTaskId === selectedTaskId;
    if (selectedExists || requestedTaskId === selectedTaskId || stageTaskStillResolved) {
      return;
    }

    setSelectedTaskId(null);
    setRequestedTaskId(null);
    setDetailOpen(false);
  }, [allTasks, requestedTaskId, routeFocusTaskId, selectedDetailTaskId, selectedTaskId]);

  useEffect(() => {
    let bucketQueryRefreshTimeoutId: number | null = null;

    function invalidateTaskDetailQueries(taskId: string) {
      void queryClient.invalidateQueries({ queryKey: buildDashboardTaskDetailQueryKey(dataMode, taskId) });
      void queryClient.invalidateQueries({ queryKey: buildDashboardTaskArtifactQueryKey(dataMode, taskId) });
    }

    function invalidateTaskRuntimeQueries(taskId: string) {
      invalidateTaskDetailQueries(taskId);
      void queryClient.invalidateQueries({ queryKey: [...dashboardTaskEventQueryPrefix, dataMode, taskId] });
    }

    function invalidateTaskQueriesForNotification(taskId: string) {
      // Plain task.updated or delivery.ready notifications can advance runtime
      // state without a matching loop.* event, so the focused task needs both
      // detail and event queries to stay in sync.
      if (taskId === selectedTaskId) {
        invalidateTaskRuntimeQueries(taskId);
        return;
      }

      invalidateTaskDetailQueries(taskId);
    }

    function flushBucketQueryRefresh() {
      bucketQueryRefreshTimeoutId = null;
      void queryClient.invalidateQueries({ queryKey: buildDashboardTaskBucketQueryKey(dataMode, "unfinished", unfinishedLimit) });
      void queryClient.invalidateQueries({ queryKey: buildDashboardTaskBucketQueryKey(dataMode, "finished", finishedLimit) });
    }

    /**
     * Batch list refreshes in local dashboard view state so high-frequency
     * backend task.updated notifications do not refetch the full workspace on
     * every progress tick.
     */
    function scheduleBucketQueryRefresh() {
      if (bucketQueryRefreshTimeoutId !== null) {
        return;
      }

      bucketQueryRefreshTimeoutId = window.setTimeout(() => {
        flushBucketQueryRefresh();
      }, TASK_BUCKET_REFRESH_DEBOUNCE_MS);
    }

    const clearDeliverySubscription = subscribeDeliveryReady((payload) => {
      invalidateTaskQueriesForNotification(payload.task_id);
      scheduleBucketQueryRefresh();
    });

    const clearTaskUpdatedSubscription = subscribeTaskUpdated((payload) => {
      invalidateTaskQueriesForNotification(payload.task_id);
      scheduleBucketQueryRefresh();
    });

    const clearRuntimeSubscription = selectedTaskId
      ? subscribeTaskRuntime(selectedTaskId, () => {
          invalidateTaskRuntimeQueries(selectedTaskId);
        })
      : () => {};

    return () => {
      if (bucketQueryRefreshTimeoutId !== null) {
        window.clearTimeout(bucketQueryRefreshTimeoutId);
      }
      clearDeliverySubscription();
      clearTaskUpdatedSubscription();
      clearRuntimeSubscription();
    };
  }, [dataMode, finishedLimit, queryClient, selectedTaskId, unfinishedLimit]);

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

  /**
   * Routes every task focus change through the same guarded local state so the
   * requested task stays selected while detail queries catch up for the modal.
   */
  function focusTaskDetail(taskId: string, openDetail = true) {
    setRequestedTaskId(taskId);
    setSelectedTaskId(taskId);
    setDetailOpen(openDetail);
  }

  /**
   * The center stage is a local presentation dock only. Clicking or dropping a
   * card here should never mutate formal task state; it only swaps the stage
   * preview while keeping detail opening as a separate action.
   */
  function placeTaskOnStage(taskId: string) {
    focusTaskDetail(taskId, false);
  }

  function openTaskDetail(taskId: string) {
    focusTaskDetail(taskId, true);
  }

  function clearStagePreview() {
    setRequestedTaskId(null);
    setSelectedTaskId(null);
    setStageInitialized(true);
    setDetailOpen(false);
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

  async function handleResolvedOpen(result: Awaited<ReturnType<typeof openTaskArtifactForTask>>) {
    const plan = resolveTaskOpenExecutionPlan(result);
    const sameTaskMessage = describeTaskOpenResultForCurrentTask(plan, selectedTaskId);
    if (sameTaskMessage) {
      setDetailOpen(true);
      showFeedback(sameTaskMessage);
      return;
    }

    showFeedback(await performTaskOpenExecution(plan, {
      onOpenTaskDetail: ({ taskId }) => {
        focusTaskDetail(taskId);
        return plan.feedback;
      },
    }));
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

  const taskSteerMutation = useMutation({
    mutationFn: ({ message, taskId }: { message: string; taskId: string }) => steerTaskByMessage(taskId, message, dataMode),
    onSuccess: (result, variables) => {
      setSteeringSuccessVersion((current) => current + 1);
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
    if (!selectedTaskControlTargetId) {
      return;
    }

    let resolvedDetailData = detailData;
    const safetyOpenPlan = resolveDashboardTaskSafetyOpenPlan(detailState);

    if (safetyOpenPlan.shouldRefetchDetail) {
      const refetchResult = await taskDetailQuery.refetch();

      if (!refetchResult.data || refetchResult.isError) {
        showFeedback("任务详情还在同步，先打开安全总览。");
        navigate(resolveDashboardRoutePath("safety"), {
          state: {
            source: "task-detail",
            taskId: selectedTaskControlTargetId,
          },
        });
        return;
      }

      resolvedDetailData = refetchResult.data;
    }

    if (!resolvedDetailData) {
      navigate(resolveDashboardRoutePath("safety"), {
        state: {
          source: "task-detail",
          taskId: selectedTaskControlTargetId,
        },
      });
      return;
    }

    navigate(resolveDashboardRoutePath("safety"), { state: buildDashboardSafetyNavigationState(resolvedDetailData.detail) });
  }

  function handlePrimaryAction(action: "pause" | "resume" | "cancel" | "restart" | "open-safety") {
    if (!selectedTaskControlTargetId) {
      return;
    }

    if (action === "open-safety") {
      void handleOpenSafety();
      return;
    }

    taskControlMutation.mutate({ action, taskId: selectedTaskControlTargetId });
  }

  function handleOpenArtifact(artifactId: string) {
    if (!selectedTaskControlTargetId) {
      return;
    }

    artifactOpenMutation.mutate({ artifactId, taskId: selectedTaskControlTargetId });
  }

  function handleOpenLatestDelivery() {
    if (!selectedTaskControlTargetId) {
      return;
    }

    navigate(resolveDashboardTaskDeliveryRoutePath(selectedTaskControlTargetId));
  }

  function handleSteerTask(message: string) {
    if (!selectedTaskControlTargetId) {
      return;
    }

    taskSteerMutation.mutate({ message, taskId: selectedTaskControlTargetId });
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
    placeTaskOnStage(taskId);
  }

  function handleOpenTaskDetail(taskId: string) {
    openTaskDetail(taskId);
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
                    onOpenDetail={handleOpenTaskDetail}
                    onStage={handleSelectTask}
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
            onOpenDetail={handleOpenTaskDetail}
            onStage={handleSelectTask}
            runwayLabel={section.code}
          />
        ))}
      </div>
    );
  }

  function renderClusterSection(section: TaskClusterSection) {
    const Icon = clusterIcons[section.key];
    const isExpanded = expandedClusterKey === section.key;
    const shouldShowToggle = section.key === "archive" ? hasArchiveOlderItems : section.items.length > CLUSTER_PREVIEW_LIMIT;
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
        <div className="task-tower__topbar-actions">
          <Link className="dashboard-page__home-link" to={resolveDashboardRoutePath("home")}>
            <ArrowLeft className="h-4 w-4" />
            返回首页
          </Link>
        </div>

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
              {selectedTaskPreview && selectedProgress && selectedStateVoice ? (
                <motion.div className="task-cloud__stage-shell" layout>
                  <header className="task-cloud__stage-header">
                    <div className="task-cloud__stage-lockup">
                      <motion.span className="task-cloud__stage-signal" layoutId={`task-cloud-signal-${selectedTaskPreview.task.task_id}`} />
                      <motion.span className="task-cloud__stage-code" layoutId={`task-cloud-code-${selectedTaskPreview.task.task_id}`}>
                        {buildTaskTowerCode(selectedTaskPreview.task.task_id)}
                      </motion.span>
                      <span className={cn("task-cloud__stage-status", getTaskStatusBadgeClass(selectedTaskPreview.task.status))}>{getTaskPreviewStatusLabel(selectedTaskPreview.task.status)}</span>
                    </div>

                    <div className="task-cloud__stage-actions">
                      <button className="task-runway__toggle task-cloud__stage-toggle" onClick={() => openTaskDetail(selectedTaskPreview.task.task_id)} type="button">
                        打开详情
                      </button>
                      <button aria-label="移除舞台卡片" className="task-cloud__stage-close" onClick={clearStagePreview} type="button">
                        <X className="h-4 w-4" />
                      </button>
                    </div>
                  </header>

                  <section className="task-cloud__stage-dock">
                    <div className="task-cloud__stage-title-row">
                      <div>
                        <p className="task-cloud__stage-kicker">
                          {formatTaskSourceLabel(selectedTaskPreview.task.source_type)} · {getTaskPriorityLabel(selectedTaskPreview.experience.priority)} · 已放到舞台
                        </p>
                        <h2>{selectedTaskPreview.task.title}</h2>
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

                    <div className="task-cloud__stage-dock-meta">
                      <span className="task-cloud__stage-chip">{getTaskPreviewStatusLabel(selectedTaskPreview.task.status)}</span>
                      <span className="task-cloud__stage-chip">{selectedProgress.currentLabel}</span>
                      <span className="task-cloud__stage-chip">{selectedTaskEnded ? "已结束" : `更新于 ${selectedUpdateLabel}`}</span>
                    </div>
                    <p className="task-cloud__stage-description">{selectedStageDescription ?? selectedStateVoice.body}</p>
                  </section>
                </motion.div>
              ) : (
                <div className="task-cloud__stage-empty">
                  <Radar className="h-6 w-6" />
                  <h2>中央舞台现在还是空的</h2>
                  <p>单击任务卡片会把它放到舞台，双击任务卡片会直接打开完整详情。</p>
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
        {detailOpen && (detailData || selectedTaskPreview || selectedTaskId) ? (
          <>
            <motion.button
              animate={{ opacity: 1 }}
              className="task-detail-modal__backdrop"
              exit={{ opacity: 0 }}
              initial={{ opacity: 0 }}
              onClick={() => setDetailOpen(false)}
              type="button"
            />
            <div className="task-detail-modal__frame">
              <motion.div
                animate={{ opacity: 1, scale: 1, y: 0 }}
                className="task-detail-modal"
                exit={{ opacity: 0, scale: 0.98, y: 20 }}
                initial={{ opacity: 0, scale: 0.98, y: 16 }}
                transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
              >
                <TaskDetailPanel
                  artifactActionPendingId={artifactOpenMutation.isPending ? artifactOpenMutation.variables?.artifactId ?? null : null}
                  artifactErrorMessage={artifactListQuery.isError ? (artifactListQuery.error instanceof Error ? artifactListQuery.error.message : "成果列表请求失败") : null}
                  artifactItems={artifactListQuery.data?.items ?? detailData?.detail.artifacts ?? []}
                  artifactLoading={artifactListQuery.isPending}
                  fallbackOutputAccess={fallbackOutputAccess}
                  detailData={detailData}
                  detailWarningMessage={detailData?.detailWarningMessage ?? null}
                  detailErrorMessage={detailErrorMessage}
                  eventErrorMessage={taskEventsQuery.isError ? (taskEventsQuery.error instanceof Error ? taskEventsQuery.error.message : "运行时事件请求失败") : null}
                  eventFilters={taskEventFilters}
                  eventItems={taskEventsQuery.data?.items ?? []}
                  eventLoading={taskEventsQuery.isPending}
                  detailState={detailState}
                  deliveryActionPending={false}
                  feedback={feedback}
                  fallbackActions={fallbackDetailActions}
                  onAction={handlePrimaryAction}
                  onClose={() => setDetailOpen(false)}
                  onOpenArtifact={handleOpenArtifact}
                  onOpenLatestDelivery={handleOpenLatestDelivery}
                  onApplyEventFilters={handleApplyTaskEventFilters}
                  onResetEventFilters={handleResetTaskEventFilters}
                  previewExperience={selectedTaskPreview?.experience ?? null}
                  previewTask={selectedTaskPreview?.task ?? null}
                  onRetryDetail={taskDetailQuery.isError ? () => void taskDetailQuery.refetch() : null}
                  onSteerTask={handleSteerTask}
                  steeringPending={taskSteerMutation.isPending}
                  steeringSuccessVersion={steeringSuccessVersion}
                />
              </motion.div>
            </div>
          </>
        ) : null}
      </AnimatePresence>

      <AnimatePresence>
        {feedback || bucketErrors.length > 0 ? (
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
              <p className="task-tower__floating-title">{feedback ? "柔雾提示" : "任务同步失败"}</p>
              <p className="task-tower__floating-text">
                {feedback ??
                  (bucketErrors.length === 1
                    ? `${bucketErrors[0].label}：${bucketErrors[0].error instanceof Error ? bucketErrors[0].error.message : "请求失败"}`
                    : `${bucketErrors.length} 个分组加载失败：${bucketErrors.map((item) => `${item.label}${item.error instanceof Error ? `（${item.error.message}）` : ""}`).join("；")}`)}
              </p>
            </div>

            {!feedback ? (
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

    </main>
  );
}
