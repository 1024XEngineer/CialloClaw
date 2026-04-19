import { motion } from "motion/react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/utils/cn";
import { formatTimestamp } from "@/utils/formatters";
import {
  buildTaskTowerCode,
  describeCurrentStep,
  formatTaskSourceLabel,
  getTaskPreviewStatusLabel,
  getTaskPriorityLabel,
  getTaskRunwayTone,
  getTaskStatusBadgeClass,
  isTaskEnded,
} from "../taskPage.mapper";
import type { TaskListItem } from "../taskPage.types";

type TaskPreviewCardProps = {
  isActive: boolean;
  isPeeked?: boolean;
  item: TaskListItem;
  onSelect: (taskId: string) => void;
  runwayLabel: string;
};

/**
 * Renders a soft focus card for each task cluster in the dashboard scene.
 */
export function TaskPreviewCard({ isActive, isPeeked = false, item, onSelect, runwayLabel }: TaskPreviewCardProps) {
  const ended = isTaskEnded(item.task);
  const towerCode = buildTaskTowerCode(item.task.task_id);
  const tone = getTaskRunwayTone(item.task.status);
  const progressCopy = ended ? item.experience.endedSummary ?? getTaskPreviewStatusLabel(item.task.status) : describeCurrentStep(item.task, item.experience);
  const focusCopy = ended ? formatTimestamp(item.task.finished_at) : item.experience.nextAction;

  return (
    <motion.button
      className={cn("task-preview-card", `is-${tone}`, ended && "task-preview-card--ended", isActive && "task-preview-card--active", isPeeked && "task-preview-card--peeked")}
      layout
      onClick={() => onSelect(item.task.task_id)}
      type="button"
      transition={{ bounce: 0.18, damping: 24, stiffness: 260, type: "spring" }}
      whileHover={{ scale: 1.01, y: -6 }}
      whileTap={{ scale: 0.985 }}
    >
      <div className="task-preview-card__signal">
        <div className="task-preview-card__signal-left">
          <motion.span className="task-preview-card__signal-orb" layoutId={`task-cloud-signal-${item.task.task_id}`} />
          <span className="task-preview-card__runway">{runwayLabel}</span>
        </div>
        <motion.span className="task-preview-card__flight-code" layoutId={`task-cloud-code-${item.task.task_id}`}>
          {towerCode}
        </motion.span>
      </div>

      <div className="task-preview-card__body">
        <div className="task-preview-card__top">
          <div>
            <p className="task-preview-card__kicker">{formatTaskSourceLabel(item.task.source_type)}</p>
            <h3 className="task-preview-card__title">{item.task.title}</h3>
          </div>

          <Badge className={cn("task-preview-card__status border-0 px-3 py-1 text-[0.72rem] ring-1", getTaskStatusBadgeClass(item.task.status))}>
            {getTaskPreviewStatusLabel(item.task.status)}
          </Badge>
        </div>

        <p className="task-preview-card__step">{progressCopy}</p>

        <div className="task-preview-card__focus">
          <p className="task-preview-card__focus-label">{ended ? "最近收束" : "下一步"}</p>
          <p className="task-preview-card__focus-copy">{focusCopy}</p>
        </div>

        <div className="task-preview-card__meta">
          {ended ? <span className="task-preview-card__meta-chip">{formatTimestamp(item.task.finished_at)}</span> : <span className="task-preview-card__meta-chip">{item.experience.progressHint}</span>}
          <span className="task-preview-card__meta-chip task-preview-card__meta-chip--priority">{getTaskPriorityLabel(item.experience.priority)}</span>
        </div>
      </div>
    </motion.button>
  );
}
