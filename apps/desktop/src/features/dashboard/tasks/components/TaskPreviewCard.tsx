import { motion } from "motion/react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/utils/cn";
import { formatTimestamp } from "@/utils/formatters";
import { describeCurrentStep, getTaskPreviewStatusLabel, getTaskStatusBadgeClass, isTaskEnded } from "../taskPage.mapper";
import type { TaskListItem } from "../taskPage.types";

type TaskPreviewCardProps = {
  isActive: boolean;
  item: TaskListItem;
  onSelect: (taskId: string) => void;
};

export function TaskPreviewCard({ isActive, item, onSelect }: TaskPreviewCardProps) {
  const ended = isTaskEnded(item.task);

  return (
    <motion.button
      className={cn(
        "task-preview-card",
        ended && "task-preview-card--ended",
        isActive && "task-preview-card--active",
      )}
      onClick={() => onSelect(item.task.task_id)}
      type="button"
      whileHover={{ y: -2 }}
      whileTap={{ scale: 0.995 }}
    >
      <div className="task-preview-card__top">
        <div>
          <h3 className="task-preview-card__title">{item.task.title}</h3>
          {!ended ? <p className="task-preview-card__step">{describeCurrentStep(item.task, item.experience)}</p> : null}
        </div>

        <Badge className={cn("border-0 px-3 py-1 text-[0.72rem] ring-1", getTaskStatusBadgeClass(item.task.status))}>
          {getTaskPreviewStatusLabel(item.task.status)}
        </Badge>
      </div>

      <div className="task-preview-card__meta">
        {ended ? <span>{formatTimestamp(item.task.finished_at)}</span> : <span>{item.experience.progressHint}</span>}
        <span>{item.experience.priority === "critical" ? "高优先级" : item.experience.priority === "high" ? "重点任务" : "常规推进"}</span>
      </div>
    </motion.button>
  );
}
