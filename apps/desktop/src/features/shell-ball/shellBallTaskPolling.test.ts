import test from "node:test";
import assert from "node:assert/strict";

import {
  SHELL_BALL_TASK_POLL_MAX_FAILURES,
  getShellBallTaskTerminalStatusText,
  isTerminalShellBallTaskStatus,
  resolveShellBallTaskPollingFailure,
} from "./shellBallTaskPolling";

test("shell-ball task polling terminal status helper classifies terminal states", () => {
  assert.equal(isTerminalShellBallTaskStatus("completed"), true);
  assert.equal(isTerminalShellBallTaskStatus("failed"), true);
  assert.equal(isTerminalShellBallTaskStatus("cancelled"), true);
  assert.equal(isTerminalShellBallTaskStatus("ended_unfinished"), true);
  assert.equal(isTerminalShellBallTaskStatus("processing"), false);
  assert.equal(isTerminalShellBallTaskStatus("waiting_auth"), false);
});

test("shell-ball task polling terminal text helper prefers explicit failure summaries", () => {
  assert.equal(
    getShellBallTaskTerminalStatusText({
      status: "failed",
      failureSummary: "  tool timeout  ",
    }),
    "tool timeout",
  );
  assert.equal(
    getShellBallTaskTerminalStatusText({
      status: "failed",
      failureSummary: "   ",
    }),
    "任务执行失败，请到任务详情查看原因。",
  );
  assert.equal(getShellBallTaskTerminalStatusText({ status: "cancelled" }), "任务已取消。");
  assert.equal(getShellBallTaskTerminalStatusText({ status: "ended_unfinished" }), "任务已结束，但还有未完成项，请到任务详情继续处理。");
  assert.equal(getShellBallTaskTerminalStatusText({ status: "completed" }), "任务已完成。");
});

test("shell-ball task polling failure helper stops after configured threshold", () => {
  const firstFailure = resolveShellBallTaskPollingFailure(0);
  assert.deepEqual(firstFailure, {
    nextCount: 1,
    shouldStopPolling: false,
  });

  const thresholdFailure = resolveShellBallTaskPollingFailure(SHELL_BALL_TASK_POLL_MAX_FAILURES - 1);
  assert.deepEqual(thresholdFailure, {
    nextCount: SHELL_BALL_TASK_POLL_MAX_FAILURES,
    shouldStopPolling: true,
  });

  const negativeFailure = resolveShellBallTaskPollingFailure(-5);
  assert.deepEqual(negativeFailure, {
    nextCount: 1,
    shouldStopPolling: false,
  });
});
