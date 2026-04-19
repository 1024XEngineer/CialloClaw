import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/utils/cn";
import type { FinishedTaskGroup, TaskListItem } from "../taskPage.types";
import { TaskPreviewCard } from "./TaskPreviewCard";

type TaskPreviewGroupProps = {
  activeTaskId: string | null;
  finishedGroups: FinishedTaskGroup[];
  onSelect: (taskId: string) => void;
  onToggleFinished: () => void;
  showMoreFinished: boolean;
  unfinishedTasks: TaskListItem[];
};

/**
 * Preserves the legacy two-column preview layout for callers that still
 * consume the old grouping component outside the main dispatch page.
 */
export function TaskPreviewGroup({ activeTaskId, finishedGroups, onSelect, onToggleFinished, showMoreFinished, unfinishedTasks }: TaskPreviewGroupProps) {
  return (
    <div className="task-preview-layout">
      <section className="task-preview-section task-preview-section--unfinished">
        <div className="task-preview-section__header">
          <div>
            <p className="task-preview-section__eyebrow">未完成</p>
            <h2 className="task-preview-section__title">先看正在推进的任务</h2>
          </div>
          <span className="task-preview-section__count">{unfinishedTasks.length}</span>
        </div>

        <ScrollArea className="task-preview-section__scroll">
          <div className="task-preview-section__list">
            {unfinishedTasks.map((item) => (
              <TaskPreviewCard key={item.task.task_id} isActive={item.task.task_id === activeTaskId} item={item} onSelect={onSelect} runwayLabel="进行中" />
            ))}
          </div>
        </ScrollArea>
      </section>

      <section className="task-preview-section task-preview-section--finished">
        <div className="task-preview-section__header">
          <div>
            <p className="task-preview-section__eyebrow">已结束</p>
            <h2 className="task-preview-section__title">回看最近完成与取消的任务</h2>
          </div>
          <Button className="task-preview-section__toggle" onClick={onToggleFinished} variant="ghost">
            {showMoreFinished ? "收起" : "更多"}
          </Button>
        </div>

        <div className="task-preview-section__finished-groups">
          {finishedGroups.map((group) => (
            <div key={group.key} className={cn("task-preview-group", group.key === "older" && !showMoreFinished && "hidden")}>
              <div className="task-preview-group__meta">
                <p className="task-preview-group__name">{group.title}</p>
                <p className="task-preview-group__description">{group.description}</p>
              </div>

              <div className="task-preview-group__list">
                {group.items.map((item) => (
                  <TaskPreviewCard key={item.task.task_id} isActive={item.task.task_id === activeTaskId} item={item} onSelect={onSelect} runwayLabel="已结束" />
                ))}
              </div>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
