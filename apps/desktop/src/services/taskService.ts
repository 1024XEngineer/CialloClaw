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
    request_source: "floating_ball",
    request_trigger: "hover_text_input",
    input_type: "text",
    source_type: "hover_input",
    title,
    payload: {
      text: title,
    },
  });

  return taskResult.task;
}

export function listActiveTasks(): Task[] {
  return useTaskStore.getState().tasks;
}
