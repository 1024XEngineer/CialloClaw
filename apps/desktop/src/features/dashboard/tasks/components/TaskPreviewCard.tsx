import { useEffect, useRef } from "react";
import type { MouseEvent } from "react";
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
  hideStep?: boolean;
  isActive: boolean;
  isPeeked?: boolean;
  item: TaskListItem;
  onOpenDetail: (taskId: string) => void;
  onStage: (taskId: string) => void;
};

/**
 * Renders a soft focus card for each task cluster in the dashboard scene.
 */
export function TaskPreviewCard({ hideStep = false, isActive, isPeeked = false, item, onOpenDetail, onStage }: TaskPreviewCardProps) {
  const ended = isTaskEnded(item.task);
  const towerCode = buildTaskTowerCode(item.task.task_id);
  const tone = getTaskRunwayTone(item.task.status);
  const summaryCopy = ended ? item.experience.endedSummary ?? getTaskPreviewStatusLabel(item.task.status) : describeCurrentStep(item.task, item.experience);
  const footerCopy = ended ? formatTimestamp(item.task.finished_at) : item.experience.nextAction;
  const clickTimeoutRef = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      if (clickTimeoutRef.current) {
        window.clearTimeout(clickTimeoutRef.current);
      }
    };
  }, []);

  function clearPendingStagePlacement() {
    if (clickTimeoutRef.current) {
      window.clearTimeout(clickTimeoutRef.current);
      clickTimeoutRef.current = null;
    }
  }

  function handleClick(event: MouseEvent<HTMLButtonElement>) {
    if (event.detail === 0) {
      clearPendingStagePlacement();
      onStage(item.task.task_id);
      return;
    }

    if (event.detail !== 1) {
      return;
    }

    clearPendingStagePlacement();

    // Delay single-click placement slightly so a double click can take over and
    // open detail without briefly re-rendering the stage in between.
    clickTimeoutRef.current = window.setTimeout(() => {
      onStage(item.task.task_id);
      clickTimeoutRef.current = null;
    }, 180);
  }

  function handleDoubleClick() {
    clearPendingStagePlacement();
    onOpenDetail(item.task.task_id);
  }

  return (
    <motion.button
      className={cn("task-preview-card", `is-${tone}`, ended && "task-preview-card--ended", isActive && "task-preview-card--active", isPeeked && "task-preview-card--peeked")}
      layout
      onClick={handleClick}
      onDoubleClick={handleDoubleClick}
      type="button"
      transition={{ bounce: 0.18, damping: 24, stiffness: 260, type: "spring" }}
      whileHover={{ scale: 1.01, y: -6 }}
      whileTap={{ scale: 0.985 }}
    >
      <div className="task-preview-card__signal">
        <div className="task-preview-card__signal-left">
          <motion.span className="task-preview-card__signal-orb" layoutId={`task-cloud-signal-${item.task.task_id}`} />
          <p className="task-preview-card__kicker">{formatTaskSourceLabel(item.task.source_type)}</p>
        </div>
        <motion.span className="task-preview-card__flight-code" layoutId={`task-cloud-code-${item.task.task_id}`}>
          {towerCode}
        </motion.span>
      </div>

      <div className="task-preview-card__body">
        <div className="task-preview-card__top">
          <h3 className="task-preview-card__title">{item.task.title}</h3>
          <Badge className={cn("task-preview-card__status border-0 px-3 py-1 text-[0.72rem] ring-1", getTaskStatusBadgeClass(item.task.status))}>
            {getTaskPreviewStatusLabel(item.task.status)}
          </Badge>
        </div>

        {hideStep ? null : <p className="task-preview-card__step">{summaryCopy}</p>}

        <div className="task-preview-card__meta">
          <span className="task-preview-card__meta-chip task-preview-card__meta-chip--priority">{getTaskPriorityLabel(item.experience.priority)}</span>
          <span className="task-preview-card__footer-copy">{ended ? "收束于" : "下一步"} · {footerCopy}</span>
        </div>
      </div>
    </motion.button>
  );
}
