import type { TaskDetailData } from "../taskPage.types";

type TaskContextBlockProps = {
  detailData: TaskDetailData;
};

export function TaskContextBlock({ detailData }: TaskContextBlockProps) {
  const { detail, experience } = detailData;

  return (
    <div className="task-detail-context-grid">
      <section className="task-detail-card">
        <div className="task-detail-card__header">
          <p className="task-detail-card__eyebrow">记忆与上下文</p>
          <h3 className="task-detail-card__title">本次任务用到的关键前提</h3>
        </div>
        <div className="task-detail-context-list">
          {detail.mirror_references.length > 0
            ? detail.mirror_references.map((reference) => (
                <article key={reference.memory_id} className="task-detail-context-item">
                  <p className="task-detail-context-item__label">{reference.memory_id}</p>
                  <p className="task-detail-context-item__text">{reference.reason}</p>
                  <p className="task-detail-context-item__meta">{reference.summary}</p>
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
        <div className="task-detail-card__header">
          <p className="task-detail-card__eyebrow">最近对话</p>
          <h3 className="task-detail-card__title">这次任务正在继承的上下文</h3>
        </div>
        <ul className="task-detail-conversation-list">
          {experience.recentConversation.length > 0 ? experience.recentConversation.map((item) => <li key={item}>{item}</li>) : <li className="task-detail-card__empty">无</li>}
        </ul>
      </section>
    </div>
  );
}
