import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { DashboardBackHomeLink } from "@/features/dashboard/shared/DashboardBackHomeLink";
import { subscribeTask } from "@/rpc/subscriptions";
import { getTaskProgress, getTaskStateVoice, sortTasksByLatest } from "./taskPage.mapper";
import { controlTaskByAction, loadTaskBuckets, loadTaskDetailData } from "./taskPage.service";
import type { AssistantCardKey, TaskActionShortcut, TaskTabsValue } from "./taskPage.types";
import { TaskAssistantPanel } from "./components/TaskAssistantPanel";
import { TaskBottomActions } from "./components/TaskBottomActions";
import { TaskFilesSheet } from "./components/TaskFilesSheet";
import { TaskHeaderCapsule } from "./components/TaskHeaderCapsule";
import { TaskMainPanel } from "./components/TaskMainPanel";
import "./taskPage.css";

const taskShortcutActions: TaskActionShortcut[] = [
  { id: "split", label: "拆分任务", tooltip: "把当前任务拆成更细的子任务卡。" },
  { id: "continue", label: "继续生成", tooltip: "继续围绕当前任务推进下一步内容。" },
  { id: "summarize", label: "总结进度", tooltip: "收束当前进度，生成一段简短总结。" },
  { id: "note", label: "添加备注", tooltip: "切到笔记记录舱，补充临时想法。" },
  { id: "attach", label: "关联文件", tooltip: "打开文件舱门查看或关联当前文件。" },
];

export function TaskPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<TaskTabsValue>("details");
  const [filesSheetOpen, setFilesSheetOpen] = useState(false);
  const [showMoreFinished, setShowMoreFinished] = useState(false);
  const [highlightedAssistantCard, setHighlightedAssistantCard] = useState<AssistantCardKey | null>(null);
  const [noteDraft, setNoteDraft] = useState("");
  const [feedback, setFeedback] = useState<string | null>(null);
  const feedbackTimeoutRef = useRef<number | null>(null);

  const taskBucketsQuery = useQuery({
    queryKey: ["dashboard", "tasks", "buckets"],
    queryFn: loadTaskBuckets,
  });

  const unfinishedTasks = sortTasksByLatest(taskBucketsQuery.data?.unfinished ?? []);
  const finishedTasks = sortTasksByLatest(taskBucketsQuery.data?.finished ?? []);

  useEffect(() => {
    const allTasks = [...unfinishedTasks, ...finishedTasks];
    if (allTasks.length === 0) {
      return;
    }

    const selectedExists = selectedTaskId ? allTasks.some((item) => item.task.task_id === selectedTaskId) : false;
    if (selectedExists) {
      return;
    }

    const nextTask = unfinishedTasks.find((item) => item.task.status === "processing") ?? unfinishedTasks[0] ?? finishedTasks[0];
    setSelectedTaskId(nextTask.task.task_id);
  }, [finishedTasks, selectedTaskId, unfinishedTasks]);

  const taskDetailQuery = useQuery({
    enabled: Boolean(selectedTaskId),
    queryKey: ["dashboard", "tasks", "detail", selectedTaskId],
    queryFn: () => loadTaskDetailData(selectedTaskId!),
  });

  useEffect(() => {
    if (!taskDetailQuery.data) {
      return;
    }

    setNoteDraft(taskDetailQuery.data.experience.noteDraft);
  }, [taskDetailQuery.data]);

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

  function showFeedback(message: string) {
    setFeedback(message);
    if (feedbackTimeoutRef.current) {
      window.clearTimeout(feedbackTimeoutRef.current);
    }
    feedbackTimeoutRef.current = window.setTimeout(() => setFeedback(null), 2600);
  }

  function handlePrimaryAction(action: "pause" | "resume" | "cancel" | "restart" | "edit" | "open-safety") {
    if (!taskDetailQuery.data) {
      return;
    }

    if (action === "edit") {
      showFeedback("修改任务能力会在后续补齐，当前先保持这条轨迹稳定。");
      return;
    }

    if (action === "open-safety") {
      navigate("/safety");
      return;
    }

    taskControlMutation.mutate({ action, taskId: taskDetailQuery.data.task.task_id });
  }

  function handleShortcutAction(actionId: string) {
    if (actionId === "split") {
      setActiveTab("subtasks");
      setHighlightedAssistantCard("agent");
      showFeedback("已切到子任务舱，可以继续拆分当前任务。");
      return;
    }

    if (actionId === "continue") {
      if (selectedTaskId) {
        taskControlMutation.mutate({ action: "resume", taskId: selectedTaskId });
      }
      setActiveTab("outputs");
      return;
    }

    if (actionId === "summarize") {
      setActiveTab("details");
      setHighlightedAssistantCard("context");
      showFeedback("已把视线切回详情舱，方便快速总结当前进度。");
      return;
    }

    if (actionId === "note") {
      setActiveTab("notes");
      showFeedback("已打开笔记记录舱，可以继续补充决策点。");
      return;
    }

    setFilesSheetOpen(true);
  }

  if (taskBucketsQuery.isLoading && !taskBucketsQuery.data) {
    return (
      <main className="app-shell task-capsule-page">
        <div className="task-capsule-page__frame">
          <div className="task-capsule-card h-[10rem] animate-pulse rounded-[32px] border-0 bg-white/70" />
          <div className="grid min-h-0 flex-1 gap-4 lg:grid-cols-[minmax(0,1.4fr)_minmax(280px,0.76fr)]">
            <div className="task-capsule-card min-h-[30rem] animate-pulse rounded-[32px] border-0 bg-white/70" />
            <div className="grid gap-4">
              <div className="task-capsule-card h-[12rem] animate-pulse rounded-[32px] border-0 bg-white/70" />
              <div className="task-capsule-card h-[12rem] animate-pulse rounded-[32px] border-0 bg-white/70" />
              <div className="task-capsule-card h-[12rem] animate-pulse rounded-[32px] border-0 bg-white/70" />
            </div>
          </div>
        </div>
      </main>
    );
  }

  if (!taskDetailQuery.data) {
    return null;
  }

  const detailData = taskDetailQuery.data;
  const progress = getTaskProgress(detailData.detail.timeline);
  const stateVoice = getTaskStateVoice(detailData.task, detailData.experience, detailData.detail.timeline);

  return (
    <main className="app-shell task-capsule-page">
      <DashboardBackHomeLink />

      <div className="task-capsule-page__frame">
        <TaskHeaderCapsule detailData={detailData} onMoreAction={() => showFeedback("高频操作已经放到底部，更多高级动作会在后续补齐。")} progress={progress} stateVoice={stateVoice} />

        <div className="task-capsule-page__layout min-h-0 gap-4 lg:grid lg:grid-cols-[minmax(0,1.48fr)_minmax(300px,0.82fr)]">
          <TaskMainPanel
            activeTab={activeTab}
            detailData={detailData}
            feedback={feedback}
            finishedTasks={finishedTasks}
            noteDraft={noteDraft}
            onHighlightAssistantCard={setHighlightedAssistantCard}
            onNoteDraftChange={setNoteDraft}
            onOpenFiles={() => setFilesSheetOpen(true)}
            onPrimaryAction={handlePrimaryAction}
            onSelectTask={(taskId) => {
              setSelectedTaskId(taskId);
              setHighlightedAssistantCard(null);
            }}
            onTabChange={setActiveTab}
            onToggleFinished={() => setShowMoreFinished((current) => !current)}
            showMoreFinished={showMoreFinished}
            unfinishedTasks={unfinishedTasks}
          />

          <TaskAssistantPanel detailData={detailData} highlightedCard={highlightedAssistantCard} onOpenFiles={() => setFilesSheetOpen(true)} />
        </div>

        <TaskBottomActions actions={taskShortcutActions} onAction={handleShortcutAction} />
      </div>

      <TaskFilesSheet detailData={detailData} onOpenChange={setFilesSheetOpen} open={filesSheetOpen} />
    </main>
  );
}
