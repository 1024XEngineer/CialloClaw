// 该文件封装前端任务服务调用。 
import type { RequestMeta, Task } from "@cialloclaw/protocol";
import { startTask } from "@/rpc/methods";
import { useTaskStore } from "@/stores/taskStore";

export async function bootstrapTask(title: string) {
  const requestMeta: RequestMeta = {
    trace_id: `trace_task_${Date.now()}`,
    client_time: new Date().toISOString(),
  };

  const taskResult = await startTask({
    request_meta: requestMeta,
    source: "floating_ball",
    trigger: "hover_text_input",
    input: {
      type: "text",
      text: title,
      page_context: {
        title: "Quick Input",
        url: "local://shell-ball",
        app_name: "desktop",
      },
    },
    intent: {
      name: "summarize",
      arguments: {
        style: "key_points",
      },
    },
    delivery: {
      preferred: "bubble",
      fallback: "workspace_document",
    },
  });

  return taskResult.task;
}

export function listActiveTasks(): Task[] {
  return useTaskStore.getState().tasks;
}
