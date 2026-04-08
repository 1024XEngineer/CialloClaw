// 该文件维护任务主对象的本地示例状态。 
import { create } from "zustand";
import type { Task } from "@cialloclaw/protocol";

const seededTask: Task = {
  task_id: "task_demo_001",
  title: "整理拖入的规划笔记并输出重点摘要",
  source_type: "dragged_file",
  status: "confirming_intent",
  intent: {
    name: "summarize",
    arguments: {
      style: "key_points",
    },
  },
  current_step: "intent_confirmation",
  risk_level: "green",
  started_at: "2026-04-07T10:00:00Z",
  updated_at: "2026-04-07T10:00:05Z",
  finished_at: null,
};

type TaskState = {
  tasks: Task[];
  activeTaskId: string | null;
  setTasks: (tasks: Task[]) => void;
  setActiveTaskId: (taskId: string | null) => void;
};

export const useTaskStore = create<TaskState>((set) => ({
  tasks: [seededTask],
  activeTaskId: seededTask.task_id,
  setTasks: (tasks) => set({ tasks }),
  setActiveTaskId: (activeTaskId) => set({ activeTaskId }),
}));
