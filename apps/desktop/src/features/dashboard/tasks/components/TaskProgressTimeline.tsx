import { cn } from "@/utils/cn";
import { formatStatusLabel } from "@/utils/formatters";
import type { TaskStep } from "@cialloclaw/protocol";

type TaskProgressTimelineProps = {
  timeline: TaskStep[];
};

export function TaskProgressTimeline({ timeline }: TaskProgressTimelineProps) {
  if (timeline.length === 0) {
    return <div className="task-detail-card__empty">无</div>;
  }

  return (
    <div className="task-detail-timeline">
      {timeline.map((step, index) => (
        <div key={step.step_id} className="task-detail-timeline__item">
          <div className="task-detail-timeline__rail">
            <span className={cn("task-detail-timeline__dot", `is-${step.status}`)} />
            {index < timeline.length - 1 ? <span className="task-detail-timeline__line" /> : null}
          </div>

          <div className={cn("task-detail-timeline__card", step.status === "running" && "is-running")}>
            <div className="task-detail-timeline__row">
              <div>
                <p className="task-detail-timeline__order">步骤 {step.order_index}</p>
                <h4 className="task-detail-timeline__title">{step.name}</h4>
              </div>
              <span className="task-detail-timeline__status">{formatStatusLabel(step.status)}</span>
            </div>

            {step.input_summary ? <p className="task-detail-timeline__text">{step.input_summary}</p> : null}
            {step.output_summary ? <p className="task-detail-timeline__text task-detail-timeline__text--muted">{step.output_summary}</p> : null}
          </div>
        </div>
      ))}
    </div>
  );
}
