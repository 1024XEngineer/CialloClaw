import { useEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { Link, NavLink, useLocation, useNavigate } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, CircleDashed, LayoutList } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { subscribeTask } from "@/rpc/subscriptions";
import { dashboardModules } from "@/features/dashboard/shared/dashboardRoutes";
import { cn } from "@/utils/cn";
import { getFinishedTaskGroups, isTaskEnded, sortTasksByLatest } from "./taskPage.mapper";
import { controlTaskByAction, loadTaskBuckets, loadTaskDetailData } from "./taskPage.service";
import { TaskDetailPanel } from "./components/TaskDetailPanel";
import { TaskEmptyState } from "./components/TaskEmptyState";
import { TaskPreviewCard } from "./components/TaskPreviewCard";
import "./taskPage.css";

export function TaskPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [showMoreFinished, setShowMoreFinished] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);
  const feedbackTimeoutRef = useRef<number | null>(null);

  const taskBucketsQuery = useQuery({
    queryKey: ["dashboard", "tasks", "buckets"],
    queryFn: loadTaskBuckets,
  });

  const unfinishedTasks = sortTasksByLatest(taskBucketsQuery.data?.unfinished ?? []);
  const finishedTasks = sortTasksByLatest(taskBucketsQuery.data?.finished ?? []);
  const finishedGroups = useMemo(() => getFinishedTaskGroups(finishedTasks, showMoreFinished), [finishedTasks, showMoreFinished]);
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
    const allTasks = [...unfinishedTasks, ...finishedTasks];
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
    setSelectedTaskId(nextTask.task.task_id);
  }, [finishedTasks, location.pathname, location.state, navigate, selectedTaskId, unfinishedTasks]);

  const taskDetailQuery = useQuery({
    enabled: Boolean(selectedTaskId),
    queryKey: ["dashboard", "tasks", "detail", selectedTaskId],
    queryFn: () => loadTaskDetailData(selectedTaskId!),
  });

  useEffect(() => {
    if (!selectedTaskId) {
      return;
    }

    return subscribeTask(selectedTaskId, () => {
      void queryClient.invalidateQueries({ queryKey: ["dashboard", "tasks", "buckets"] });
      void queryClient.invalidateQueries({ queryKey: ["dashboard", "tasks", "detail", selectedTaskId] });
    });
  }, [queryClient, selectedTaskId]);

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
    mutationFn: ({ action, taskId }: { action: "pause" | "resume" | "cancel" | "restart"; taskId: string }) => controlTaskByAction(taskId, action),
    onSuccess: (outcome) => {
      showFeedback(outcome.result.bubble_message?.text ?? "任务操作已执行。");
      void queryClient.invalidateQueries({ queryKey: ["dashboard", "tasks", "buckets"] });
      void queryClient.invalidateQueries({ queryKey: ["dashboard", "tasks", "detail", selectedTaskId] });
    },
    onError: () => {
      showFeedback("任务操作暂时没有成功返回，请稍后再试。");
    },
  });

  function handlePrimaryAction(action: "pause" | "resume" | "cancel" | "restart" | "edit" | "open-safety") {
    if (!taskDetailQuery.data) {
      return;
    }

    if (action === "edit") {
      showFeedback("修改任务能力即将支持，当前先保持这条任务轨迹稳定。");
      return;
    }

    if (action === "open-safety") {
      navigate("/safety");
      return;
    }

    taskControlMutation.mutate({ action, taskId: taskDetailQuery.data.task.task_id });
  }

  if (taskBucketsQuery.isLoading && !taskBucketsQuery.data) {
    return (
      <main className="app-shell task-preview-page">
        <div className="task-preview-page__frame">
          <div className="task-preview-page__header task-preview-page__header--loading" />
          <div className="task-preview-page__layout">
            <div className="task-preview-page__column task-preview-page__column--loading" />
            <div className="task-preview-page__detail task-preview-page__detail--loading" />
          </div>
        </div>
      </main>
    );
  }

  if (!taskBucketsQuery.data || (!selectedTaskId && unfinishedTasks.length === 0 && finishedTasks.length === 0)) {
    return (
      <main className="app-shell task-preview-page">
        <div className="task-preview-page__frame">
          <TaskEmptyState />
        </div>
      </main>
    );
  }

  return (
    <main className="dashboard-page task-preview-page" style={pageStyle}>
      <header className="dashboard-page__topbar">
        <Link className="dashboard-page__home-link" to="/">
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
          {taskDetailQuery.data ? (
            <>
              <p className="task-preview-page__hero-title">{taskDetailQuery.data.task.title}</p>
              <div className="dashboard-card__status-row">
                <CircleDashed className="h-4 w-4" />
                <span>{isTaskEnded(taskDetailQuery.data.task) ? "这条任务已经结束，可点开回看结果。" : "当前正在推进，点开可查看完整进展与上下文。"}</span>
              </div>
            </>
          ) : (
            <div className="dashboard-card__status-row">
              <CircleDashed className="h-4 w-4" />
              <span>从左侧任务卡开始浏览，点击后会打开详情弹窗。</span>
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
            <span className="task-preview-card-shell__count">{unfinishedTasks.length}</span>
          </div>
          <div className="task-preview-card-shell__list">
            {unfinishedTasks.map((item) => (
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
            {finishedGroups.map((group) => (
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
            ))}
          </div>
        </article>

        <article className="dashboard-card task-preview-card-shell task-preview-card-shell--hint">
          <p className="dashboard-card__kicker">说明</p>
          <ul className="dashboard-card__list">
            <li>未完成任务会直接展示当前执行步骤，方便判断是否需要立即介入。</li>
            <li>等待授权、暂停、失败等任务会保留停住原因，避免只有一个“失败”状态。</li>
            <li>更多的上下文、成果区与操作按钮，都放在点击任务后的详情弹窗中。</li>
          </ul>
        </article>
      </section>

      <AnimatePresence>
        {detailOpen && taskDetailQuery.data ? (
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
              <TaskDetailPanel detailData={taskDetailQuery.data} feedback={feedback} onAction={handlePrimaryAction} onClose={() => setDetailOpen(false)} />
            </motion.div>
          </>
        ) : null}
      </AnimatePresence>
    </main>
  );
}
