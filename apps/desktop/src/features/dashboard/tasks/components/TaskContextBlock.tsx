import { useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { resolveDashboardModuleRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import type { TaskDetailData } from "../taskPage.types";

type TaskContextBlockProps = {
  detailData: TaskDetailData;
};

export function TaskContextBlock({ detailData }: TaskContextBlockProps) {
  const navigate = useNavigate();
  const { detail, experience } = detailData;
  const openMirrorReference = useCallback(
    (memoryId: string) => {
      navigate(resolveDashboardModuleRoutePath("memory"), {
        state: {
          activeDetailKey: "memory",
          focusMemoryId: memoryId,
        },
      });
    },
    [navigate],
  );
  const openMirrorHistory = useCallback(() => {
    navigate(resolveDashboardModuleRoutePath("memory"), {
      state: {
        activeDetailKey: "history",
      },
    });
  }, [navigate]);

  return (
    <div className="task-detail-context-grid">
      <section className="task-detail-card">
        <div className="task-detail-card__header">
          <h3 className="task-detail-card__title">上下文</h3>
        </div>
        <div className="task-detail-context-list">
          {detail.mirror_references.length > 0
            ? detail.mirror_references.map((reference) => (
                <article key={reference.memory_id} className="task-detail-context-item">
                  <p className="task-detail-context-item__label">{reference.memory_id}</p>
                  <p className="task-detail-context-item__text">{reference.reason}</p>
                  <p className="task-detail-context-item__meta">{reference.summary}</p>
                  <div className="task-detail-context-item__actions">
                    <button className="task-detail-card__action task-detail-context-item__action" onClick={() => openMirrorReference(reference.memory_id)} type="button">
                      镜子
                    </button>
                  </div>
                </article>
              ))
            : null}
          {experience.quickContext.map((item) => (
            <article key={item.id} className="task-detail-context-item">
              <p className="task-detail-context-item__label">{item.label}</p>
              <p className="task-detail-context-item__text">{item.content}</p>
            </article>
          ))}
          {detail.mirror_references.length === 0 && experience.quickContext.length === 0 ? <p className="task-detail-card__empty">无</p> : null}
        </div>
      </section>

      <section className="task-detail-card">
        <div className={`task-detail-card__header${experience.recentConversation.length > 0 ? " task-detail-card__header--actionable" : ""}`}>
          <div>
            <h3 className="task-detail-card__title">对话</h3>
          </div>
          {experience.recentConversation.length > 0 ? (
            <button className="task-detail-card__action" onClick={openMirrorHistory} type="button">
              记录
            </button>
          ) : null}
        </div>
        <ul className="task-detail-conversation-list">
          {experience.recentConversation.length > 0 ? experience.recentConversation.map((item) => <li key={item}>{item}</li>) : <li className="task-detail-card__empty">无</li>}
        </ul>
      </section>
    </div>
  );
}
