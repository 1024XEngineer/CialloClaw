import { ArrowUpRight, CircleAlert, Waypoints } from "lucide-react";
import { motion } from "motion/react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/utils/cn";
import { describeTaskPreview, getFinishedTaskGroups, getTaskPrimaryActions, getTaskStatusBadgeClass, sortTasksByLatest } from "../taskPage.mapper";
import type { AssistantCardKey, TaskDetailData, TaskListItem, TaskTabsValue } from "../taskPage.types";
import { TaskTabsPanel } from "./TaskTabsPanel";

type TaskMainPanelProps = {
  activeTab: TaskTabsValue;
  detailData: TaskDetailData;
  feedback: string | null;
  finishedTasks: TaskListItem[];
  noteDraft: string;
  onHighlightAssistantCard: (card: AssistantCardKey) => void;
  onNoteDraftChange: (value: string) => void;
  onOpenFiles: () => void;
  onPrimaryAction: (action: "pause" | "resume" | "cancel" | "restart" | "open-safety") => void;
  onSelectTask: (taskId: string) => void;
  onTabChange: (value: TaskTabsValue) => void;
  onToggleFinished: () => void;
  showMoreFinished: boolean;
  unfinishedTasks: TaskListItem[];
};

export function TaskMainPanel({
  activeTab,
  detailData,
  feedback,
  finishedTasks,
  noteDraft,
  onHighlightAssistantCard,
  onNoteDraftChange,
  onOpenFiles,
  onPrimaryAction,
  onSelectTask,
  onTabChange,
  onToggleFinished,
  showMoreFinished,
  unfinishedTasks,
}: TaskMainPanelProps) {
  const { detail, experience, task } = detailData;
  const finishedGroups = getFinishedTaskGroups(sortTasksByLatest(finishedTasks), showMoreFinished);
  const primaryActions = getTaskPrimaryActions(task, detail);

  return (
    <motion.section animate={{ scale: [1, 1.004, 1] }} className="task-capsule-main flex min-h-0 flex-col gap-4" transition={{ duration: 8, ease: "easeInOut", repeat: Infinity }}>
      <Card className="task-capsule-card rounded-[32px] border-0">
        <CardContent className="space-y-5 p-5">
          <div className="grid gap-4 lg:grid-cols-[minmax(0,1.12fr)_minmax(260px,0.88fr)]">
            <div className="space-y-4">
              <div className="inline-flex items-center gap-2 rounded-full bg-white/70 px-3 py-1 text-[0.72rem] uppercase tracking-[0.18em] text-slate-500 ring-1 ring-white/80">
                <Waypoints className="h-3.5 w-3.5" />
                当前目标区
              </div>
              <div>
                <h2 className="text-[1.5rem] font-semibold tracking-[-0.04em] text-slate-800">{experience.goal}</h2>
                <p className="mt-3 max-w-2xl text-sm leading-7 text-slate-600">{experience.phase}</p>
              </div>
              <div className="rounded-[26px] bg-white/68 p-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.7)]">
                <p className="text-xs uppercase tracking-[0.22em] text-slate-500">下一步建议动作</p>
                <p className="mt-3 text-sm leading-7 text-slate-700">{experience.nextAction}</p>
              </div>
            </div>

            <div className="space-y-4 rounded-[28px] border border-white/70 bg-white/58 p-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.72)]">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-xs uppercase tracking-[0.22em] text-slate-500">任务预览</p>
                  <p className="mt-2 text-sm text-slate-600">先聚焦当前任务，同时保留周边任务轨迹。</p>
                </div>
                <Badge className="border-0 bg-white/70 px-3 py-1 text-[0.72rem] text-slate-600 ring-1 ring-white/80">{unfinishedTasks.length} 个未完成</Badge>
              </div>

              <div className="space-y-3">
                {unfinishedTasks.map((item) => {
                  const isActive = item.task.task_id === task.task_id;

                  return (
                    <button
                      key={item.task.task_id}
                      className={cn(
                        "w-full rounded-[24px] border px-4 py-3 text-left transition-all",
                        isActive
                          ? "border-white/90 bg-[linear-gradient(135deg,rgba(255,255,255,0.92),rgba(239,245,252,0.85))] shadow-[0_24px_46px_-34px_rgba(67,85,106,0.42)]"
                          : "border-white/55 bg-white/52 hover:border-white/80 hover:bg-white/72",
                      )}
                      onClick={() => onSelectTask(item.task.task_id)}
                      type="button"
                    >
                      <div className="flex items-start justify-between gap-4">
                        <div>
                          <p className="font-medium text-slate-800">{item.task.title}</p>
                          <p className="mt-2 text-sm leading-6 text-slate-500">{describeTaskPreview(item.task, item.task.current_step)}</p>
                        </div>
                        <Badge className={cn("border-0 px-3 py-1 text-[0.72rem] ring-1", getTaskStatusBadgeClass(item.task.status))}>
                          {item.experience.priority === "critical" ? "重点" : item.experience.priority === "high" ? "推进中" : "跟进中"}
                        </Badge>
                      </div>
                    </button>
                  );
                })}
              </div>

              <div className="rounded-[24px] bg-white/62 p-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <p className="text-xs uppercase tracking-[0.22em] text-slate-500">已结束任务</p>
                    <p className="mt-2 text-sm text-slate-600">按时间倒序回看最近已完成与已取消记录。</p>
                  </div>
                  <Button className="task-capsule-soft-button h-9 rounded-full px-4 text-sm" onClick={onToggleFinished} variant="ghost">
                    {showMoreFinished ? "收起" : "更多"}
                  </Button>
                </div>

                <div className="mt-4 space-y-4">
                  {finishedGroups.map((group) => (
                    <div key={group.key} className="space-y-2">
                      <div>
                        <p className="text-xs uppercase tracking-[0.2em] text-slate-500">{group.title}</p>
                        <p className="mt-1 text-sm text-slate-500">{group.description}</p>
                      </div>
                      <div className="space-y-2">
                        {group.items.map((item) => (
                          <button
                            key={item.task.task_id}
                            className={cn(
                              "w-full rounded-[20px] border px-4 py-3 text-left transition-all",
                              item.task.task_id === task.task_id ? "border-white/90 bg-white/84" : "border-white/55 bg-white/48 hover:bg-white/72",
                            )}
                            onClick={() => onSelectTask(item.task.task_id)}
                            type="button"
                          >
                            <div className="flex items-center justify-between gap-3">
                              <div>
                                <p className="font-medium text-slate-800">{item.task.title}</p>
                                <p className="mt-1 text-sm text-slate-500">{describeTaskPreview(item.task, item.task.current_step)}</p>
                              </div>
                              <Badge className={cn("border-0 px-3 py-1 text-[0.72rem] ring-1", getTaskStatusBadgeClass(item.task.status))}>
                                {item.task.status === "completed" ? "已完成" : "已取消"}
                              </Badge>
                            </div>
                          </button>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>

          <div className="task-capsule-action-zone flex flex-wrap items-center gap-3 rounded-[28px] border border-white/70 bg-white/58 px-4 py-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.68)]">
            {primaryActions.map((item) => {
              const isSafety = item.action === "open-safety";

              if (isSafety) {
                return (
                  <Tooltip key={item.label}>
                    <TooltipTrigger>
                      <Button className="task-capsule-soft-button h-10 rounded-full px-4 text-sm" onClick={() => onPrimaryAction(item.action)} variant="ghost">
                        {item.label}
                        <ArrowUpRight className="h-4 w-4" />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent className="rounded-full bg-slate-900/90 px-3 py-1.5 text-[0.72rem] text-white">{item.tooltip}</TooltipContent>
                  </Tooltip>
                );
              }

              return (
                <Tooltip key={item.label}>
                  <TooltipTrigger>
                    <Button
                      className="task-capsule-soft-button h-10 rounded-full px-4 text-sm"
                      onClick={() => onPrimaryAction(item.action)}
                      variant="ghost"
                    >
                      {item.label}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent className="rounded-full bg-slate-900/90 px-3 py-1.5 text-[0.72rem] text-white">{item.tooltip}</TooltipContent>
                </Tooltip>
              );
            })}

            <Button className="task-capsule-soft-button h-10 rounded-full px-4 text-sm" onClick={onOpenFiles} variant="ghost">
              查看文件舱门
            </Button>

            {feedback ? (
              <div className="ml-auto inline-flex items-center gap-2 rounded-full bg-orange-50 px-4 py-2 text-sm text-orange-700 ring-1 ring-orange-200/80">
                <CircleAlert className="h-4 w-4" />
                {feedback}
              </div>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <TaskTabsPanel
        activeTab={activeTab}
        detailData={detailData}
        noteDraft={noteDraft}
        onHighlightAssistantCard={onHighlightAssistantCard}
        onNoteDraftChange={onNoteDraftChange}
        onTabChange={onTabChange}
      />
    </motion.section>
  );
}
