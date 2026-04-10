type TaskEmptyStateProps = {
  eyebrow?: string;
  title?: string;
  copy?: string;
};

export function TaskEmptyState({
  eyebrow = "暂无任务",
  title = "这里还没有可展示的任务",
  copy = "等任务进入执行后，会先出现在预览区。点击某条任务后，会在放大的详情弹窗中查看完整信息。",
}: TaskEmptyStateProps) {
  return (
    <div className="task-empty-state">
      <p className="task-empty-state__eyebrow">{eyebrow}</p>
      <h2 className="task-empty-state__title">{title}</h2>
      <p className="task-empty-state__copy">{copy}</p>
    </div>
  );
}
