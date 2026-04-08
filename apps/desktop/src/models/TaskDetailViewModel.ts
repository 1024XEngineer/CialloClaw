// 该文件负责把共享主模型映射成前端 ViewModel。
import type { Task } from "@cialloclaw/protocol";
import { formatStatusLabel, formatTimestamp } from "@/utils/formatters";

// TaskDetailViewModel 定义当前模块的数据结构。
export type TaskDetailViewModel = {
  id: string;
  title: string;
  statusLabel: string;
  statusTone: Task["status"];
  startedAtLabel: string;
};

// mapTaskToDetailViewModel 处理当前模块的相关逻辑。
export function mapTaskToDetailViewModel(task: Task): TaskDetailViewModel {
  return {
    id: task.task_id,
    title: task.title,
    statusLabel: formatStatusLabel(task.status),
    statusTone: task.status,
    startedAtLabel: formatTimestamp(task.started_at),
  };
}
