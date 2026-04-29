export const SHELL_BALL_TASK_POLL_MAX_FAILURES = 5;

/**
 * Returns whether the formal task no longer needs shell-ball polling.
 *
 * @param status Current formal task status from `agent.task.detail.get`.
 * @returns `true` when the task has reached a terminal state.
 */
export function isTerminalShellBallTaskStatus(status: string) {
  return status === "completed" || status === "failed" || status === "cancelled" || status === "ended_unfinished";
}

/**
 * Maps a terminal task status into the fallback shell-ball bubble text.
 *
 * @param input Terminal status plus any runtime failure summary returned by the backend.
 * @returns The localized fallback bubble text shown when no delivery result exists.
 */
export function getShellBallTaskTerminalStatusText(input: {
  status: string;
  failureSummary?: string | null;
}) {
  if (input.status === "failed") {
    const failureSummary = input.failureSummary?.trim();
    return failureSummary && failureSummary !== "" ? failureSummary : "任务执行失败，请到任务详情查看原因。";
  }
  if (input.status === "cancelled") {
    return "任务已取消。";
  }
  if (input.status === "ended_unfinished") {
    return "任务已结束，但还有未完成项，请到任务详情继续处理。";
  }
  return "任务已完成。";
}

/**
 * Tracks consecutive polling failures and decides when shell-ball should stop
 * retrying a task detail query.
 *
 * @param previousCount The prior consecutive failure count for the task.
 * @returns The next count and whether polling should stop.
 */
export function resolveShellBallTaskPollingFailure(previousCount: number) {
  const nextCount = Math.max(0, previousCount) + 1;

  return {
    nextCount,
    shouldStopPolling: nextCount >= SHELL_BALL_TASK_POLL_MAX_FAILURES,
  };
}
