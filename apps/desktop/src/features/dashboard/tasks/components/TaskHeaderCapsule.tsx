import { CalendarClock, Flag, MoreHorizontal, Sparkles } from "lucide-react";
import { motion } from "motion/react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/utils/cn";
import { formatTimestamp } from "@/utils/formatters";
import { formatTaskSourceLabel, getPriorityBadgeClass, getTaskStatusBadgeClass } from "../taskPage.mapper";
import type { TaskDetailData, TaskProgressState, TaskStateVoice } from "../taskPage.types";

type TaskHeaderCapsuleProps = {
  detailData: TaskDetailData;
  onMoreAction: () => void;
  progress: TaskProgressState;
  stateVoice: TaskStateVoice;
};

const priorityLabels = {
  critical: "P0 · 紧急",
  high: "P1 · 重点",
  steady: "P2 · 平稳",
} as const;

export function TaskHeaderCapsule({ detailData, onMoreAction, progress, stateVoice }: TaskHeaderCapsuleProps) {
  const { experience, task } = detailData;

  return (
    <motion.div animate={{ opacity: 1, y: 0 }} initial={{ opacity: 0, y: 16 }} transition={{ duration: 0.44, ease: [0.22, 1, 0.36, 1] }}>
      <Card className="task-capsule-card overflow-visible rounded-[32px] border-0">
        <CardContent className="grid gap-5 p-6 lg:grid-cols-[minmax(0,1.4fr)_minmax(260px,0.72fr)] lg:items-start">
          <div className="space-y-4">
            <div className="flex flex-wrap items-center gap-2 text-[0.68rem] uppercase tracking-[0.28em] text-slate-500">
              <span>task cabin</span>
              <span className="h-1 w-1 rounded-full bg-slate-300" />
              <span>{formatTaskSourceLabel(task.source_type)}</span>
            </div>

            <div className="space-y-3">
              <h1 className="max-w-4xl text-[clamp(1.9rem,3.1vw,3.3rem)] font-semibold leading-[0.95] tracking-[-0.045em] text-slate-800">
                {task.title}
              </h1>
              <p className="max-w-3xl text-sm leading-7 text-slate-600">{stateVoice.body}</p>
            </div>

            <div className="flex flex-wrap items-center gap-3">
              <Badge className={cn("border-0 px-3 py-1 text-[0.78rem] font-medium ring-1", getTaskStatusBadgeClass(task.status))}>
                {stateVoice.title}
              </Badge>
              <Badge className={cn("border-0 px-3 py-1 text-[0.78rem] font-medium ring-1", getPriorityBadgeClass(experience.priority))}>
                <Flag className="mr-1 h-3.5 w-3.5" />
                {priorityLabels[experience.priority]}
              </Badge>
              <Badge className="border-0 bg-white/70 px-3 py-1 text-[0.78rem] font-medium text-slate-600 ring-1 ring-white/80">
                <CalendarClock className="mr-1 h-3.5 w-3.5" />
                截止 {experience.dueAt ? formatTimestamp(experience.dueAt) : "未设置"}
              </Badge>
            </div>
          </div>

          <div className="space-y-4 rounded-[28px] border border-white/70 bg-white/60 p-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.65)]">
            <div className="flex items-start justify-between gap-3">
              <div>
                <p className="text-[0.72rem] uppercase tracking-[0.2em] text-slate-500">轻量进度</p>
                <p className="mt-2 text-2xl font-semibold tracking-[-0.04em] text-slate-800">
                  {progress.completedCount}/{progress.total}
                </p>
                <p className="mt-1 text-sm text-slate-600">当前在 {progress.currentLabel}</p>
              </div>

              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      className="rounded-full border border-white/80 bg-white/75 text-slate-700 shadow-[0_14px_30px_-24px_rgba(73,88,111,0.48)] hover:-translate-y-0.5 hover:bg-white"
                      onClick={onMoreAction}
                      size="icon-sm"
                      variant="ghost"
                    />
                  }
                >
                  <MoreHorizontal className="h-4 w-4" />
                  <span className="sr-only">更多操作</span>
                </TooltipTrigger>
                <TooltipContent className="rounded-full bg-slate-900/90 px-3 py-1.5 text-[0.72rem] text-white">更多高级操作</TooltipContent>
              </Tooltip>
            </div>

            <div className="space-y-2">
              <div className="h-2 overflow-hidden rounded-full bg-slate-200/80">
                <motion.div className="h-full rounded-full bg-[linear-gradient(90deg,#7a95be_0%,#9fb4d3_48%,#d7e4f0_100%)]" initial={{ width: 0 }} animate={{ width: `${Math.max(progress.percent, 8)}%` }} transition={{ duration: 0.5, ease: [0.22, 1, 0.36, 1] }} />
              </div>
              <div className="flex items-center justify-between text-sm text-slate-500">
                <span>{detailData.experience.progressHint}</span>
                <span className="inline-flex items-center gap-1 rounded-full bg-blue-50 px-2.5 py-1 text-[0.72rem] text-blue-700">
                  <Sparkles className="h-3.5 w-3.5" />
                  {progress.percent}%
                </span>
              </div>
            </div>

            <Button className="task-capsule-soft-button h-10 rounded-full px-4 text-sm font-medium" onClick={onMoreAction} variant="ghost">
              打开更多操作说明
            </Button>
          </div>
        </CardContent>
      </Card>
    </motion.div>
  );
}
