import { useEffect } from "react";
import { subscribeTask } from "@/rpc/subscriptions";

export function useTaskStream(taskId: string | null) {
  useEffect(() => {
    if (!taskId) {
      return;
    }

    return subscribeTask(taskId, () => {});
  }, [taskId]);
}
