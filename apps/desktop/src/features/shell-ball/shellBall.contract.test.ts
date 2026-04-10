import assert from "node:assert/strict";
import test from "node:test";
import { getShellBallDemoViewModel } from "./shellBall.demo";
import {
  createShellBallInteractionController,
  getShellBallInputBarMode,
  getShellBallProcessingReturnState,
  resolveShellBallTransition,
  SHELL_BALL_CANCEL_DELTA_PX,
  SHELL_BALL_CONFIRMING_MS,
  SHELL_BALL_LOCK_DELTA_PX,
  SHELL_BALL_LONG_PRESS_MS,
  SHELL_BALL_PROCESSING_MS,
  SHELL_BALL_WAITING_AUTH_MS,
} from "./shellBall.interaction";
import { getShellBallMotionConfig } from "./shellBall.motion";
import type { ShellBallTransitionResult } from "./shellBall.types";
import { shellBallVisualStates } from "./shellBall.types";
import { syncShellBallInteractionController, useShellBallInteraction } from "./useShellBallInteraction";
import { useShellBallStore } from "../../stores/shellBallStore";

function createFakeScheduler() {
  let nextId = 0;
  const queue = new Map<number, () => void>();

  return {
    schedule(callback: () => void, _ms: number) {
      const handle = ++nextId;
      queue.set(handle, callback);
      return handle;
    },
    cancel(handle: unknown) {
      if (typeof handle === "number") {
        queue.delete(handle);
      }
    },
    flush() {
      while (queue.size > 0) {
        const [handle, callback] = queue.entries().next().value as [number, () => void];
        queue.delete(handle);
        callback();
      }
    },
    get size() {
      return queue.size;
    },
  };
}

const validTransitionResult: ShellBallTransitionResult = {
  next: "processing",
  autoAdvanceTo: "idle",
  autoAdvanceMs: 1,
};

assert.equal(validTransitionResult.autoAdvanceTo, "idle");

// @ts-expect-error auto-advance fields must be defined together
const invalidTransitionResultMissingMs: ShellBallTransitionResult = {
  next: "processing",
  autoAdvanceTo: "idle",
};

// @ts-expect-error auto-advance fields must be defined together
const invalidTransitionResultMissingTarget: ShellBallTransitionResult = {
  next: "processing",
  autoAdvanceMs: 1,
};

test("shell-ball demo fixtures preserve the frozen seven-state contract", () => {
  assert.deepEqual(shellBallVisualStates, [
    "idle",
    "hover_input",
    "confirming_intent",
    "processing",
    "waiting_auth",
    "voice_listening",
    "voice_locked",
  ]);

  assert.deepEqual(getShellBallDemoViewModel("idle"), {
    badgeTone: "status",
    badgeLabel: "待机",
    title: "小胖啾正在桌面待命",
    subtitle: "轻量承接入口已就绪",
    helperText: "悬停后可进入输入承接态",
    panelMode: "hidden",
    showRiskBlock: false,
    showVoiceHint: false,
  });

  assert.deepEqual(getShellBallDemoViewModel("waiting_auth"), {
    badgeTone: "waiting_auth",
    badgeLabel: "等待授权",
    title: "此操作需要进一步确认",
    subtitle: "检测到潜在影响范围，正在等待授权",
    helperText: "确认后才会继续执行后续动作",
    panelMode: "full",
    showRiskBlock: true,
    riskTitle: "潜在影响范围",
    riskText: "本次操作可能修改当前工作区内容，需要你明确允许后继续。",
    showVoiceHint: false,
  });

  assert.deepEqual(getShellBallDemoViewModel("voice_locked"), {
    badgeTone: "processing",
    badgeLabel: "持续收音",
    title: "持续收音已锁定",
    subtitle: "语音输入会保持开启直到结束",
    helperText: "说完后可主动结束本次语音输入",
    panelMode: "compact",
    showRiskBlock: false,
    showVoiceHint: true,
    voiceHintText: "持续收音中，结束前不会自动退出。",
  });
});

test("shell-ball interaction contract auto-advances text submission into processing", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "hover_input",
      event: "submit_text",
      regionActive: true,
    }),
    {
      next: "confirming_intent",
      autoAdvanceTo: "processing",
      autoAdvanceMs: 600,
    },
  );
});

test("shell-ball interaction contract enters hover mode on hotspot entry", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "idle",
      event: "pointer_enter_hotspot",
      regionActive: true,
    }),
    { next: "hover_input" },
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "processing",
      event: "pointer_enter_hotspot",
      regionActive: true,
    }),
    { next: "processing" },
  );
});

test("shell-ball interaction contract leaves the region only from hoverable resting states", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "hover_input",
      event: "pointer_leave_region",
      regionActive: false,
    }),
    { next: "idle" },
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "processing",
      event: "pointer_leave_region",
      regionActive: false,
    }),
    { next: "processing" },
  );
});

test("shell-ball interaction contract auto-advances file attach through auth waiting", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "hover_input",
      event: "attach_file",
      regionActive: true,
    }),
    {
      next: "waiting_auth",
      autoAdvanceTo: "processing",
      autoAdvanceMs: 700,
    },
  );
});

test("shell-ball interaction contract starts voice listening only from resting input states", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "idle",
      event: "press_start",
      regionActive: true,
    }),
    { next: "voice_listening" },
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "hover_input",
      event: "press_start",
      regionActive: true,
    }),
    { next: "voice_listening" },
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "processing",
      event: "press_start",
      regionActive: true,
    }),
    { next: "processing" },
  );
});

test("shell-ball interaction contract supports voice lock and locked voice completion", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "voice_listening",
      event: "voice_lock",
      regionActive: true,
    }),
    { next: "voice_locked" },
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "voice_locked",
      event: "primary_click_locked_voice_end",
      regionActive: true,
    }),
    { next: "processing" },
  );
});

test("shell-ball interaction contract supports voice cancel and voice finish", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "voice_listening",
      event: "voice_cancel",
      regionActive: true,
    }),
    { next: "idle" },
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "voice_listening",
      event: "voice_finish",
      regionActive: true,
    }),
    { next: "processing" },
  );
});

test("shell-ball interaction contract auto-advances waiting auth and processing states", () => {
  assert.deepEqual(
    resolveShellBallTransition({
      current: "waiting_auth",
      event: "auto_advance",
      regionActive: true,
    }),
    { next: "processing" },
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "processing",
      event: "auto_advance",
      regionActive: true,
    }),
    { next: "hover_input" },
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "processing",
      event: "auto_advance",
      regionActive: false,
    }),
    { next: "idle" },
  );
});

test("shell-ball controller schedules confirm, auth, and processing auto-advances", () => {
  const confirmingScheduler = createFakeScheduler();
  const confirmingController = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: confirmingScheduler.schedule,
    cancel: confirmingScheduler.cancel,
  });

  confirmingController.dispatch("submit_text", { regionActive: true });
  assert.equal(confirmingController.getState(), "confirming_intent");
  assert.equal(confirmingScheduler.size, 1);

  confirmingScheduler.flush();
  assert.equal(confirmingController.getState(), "hover_input");
  confirmingController.dispose();

  const authScheduler = createFakeScheduler();
  const authController = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: authScheduler.schedule,
    cancel: authScheduler.cancel,
  });

  authController.dispatch("attach_file", { regionActive: false });
  assert.equal(authController.getState(), "waiting_auth");
  assert.equal(authScheduler.size, 1);

  authScheduler.flush();
  assert.equal(authController.getState(), "idle");
  authController.dispose();
});

test("shell-ball controller cancels stale auto-advance on forceState and replacement flows", () => {
  const forceScheduler = createFakeScheduler();
  const forceController = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: forceScheduler.schedule,
    cancel: forceScheduler.cancel,
  });

  forceController.dispatch("submit_text", { regionActive: true });
  forceController.forceState("idle");
  forceScheduler.flush();
  assert.equal(forceController.getState(), "idle");
  forceController.dispose();

  const replacementScheduler = createFakeScheduler();
  const replacementController = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: replacementScheduler.schedule,
    cancel: replacementScheduler.cancel,
  });

  replacementController.dispatch("submit_text", { regionActive: true });
  replacementController.forceState("hover_input");
  replacementController.dispatch("attach_file", { regionActive: false });
  replacementScheduler.flush();
  assert.equal(replacementController.getState(), "idle");
  replacementController.dispose();
});

test("shell-ball controller keeps locked voice active until explicit end", () => {
  const scheduler = createFakeScheduler();
  const controller = createShellBallInteractionController({
    initialState: "voice_locked",
    schedule: scheduler.schedule,
    cancel: scheduler.cancel,
  });

  controller.dispatch("pointer_leave_region", { regionActive: false });
  controller.dispatch("voice_finish", { regionActive: false });
  controller.dispatch("auto_advance", { regionActive: false });

  assert.equal(controller.getState(), "voice_locked");

  controller.dispatch("primary_click_locked_voice_end", { regionActive: true });
  assert.equal(controller.getState(), "processing");
  assert.equal(scheduler.size, 1);

  scheduler.flush();
  assert.equal(controller.getState(), "hover_input");
  controller.dispose();
});

test("shell-ball processing return follows the latest region activity when the timer completes", () => {
  const scheduler = createFakeScheduler();
  const controller = createShellBallInteractionController({
    initialState: "voice_locked",
    schedule: scheduler.schedule,
    cancel: scheduler.cancel,
  });

  controller.dispatch("primary_click_locked_voice_end", { regionActive: true });
  controller.dispatch("pointer_leave_region", { regionActive: false });

  scheduler.flush();
  assert.equal(controller.getState(), "idle");
  controller.dispose();
});

test("shell-ball interaction sync helper re-aligns an externally changed visual state", () => {
  const scheduler = createFakeScheduler();
  const controller = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: scheduler.schedule,
    cancel: scheduler.cancel,
  });

  controller.dispatch("submit_text", { regionActive: true });
  assert.equal(controller.getState(), "confirming_intent");

  syncShellBallInteractionController({
    controller,
    visualState: "voice_locked",
    regionActive: true,
  });

  scheduler.flush();
  assert.equal(controller.getState(), "voice_locked");
  controller.dispose();
});

test("shell-ball processing completion returns to the region-aware resting state", () => {
  assert.equal(getShellBallProcessingReturnState(true), "hover_input");
  assert.equal(getShellBallProcessingReturnState(false), "idle");
});

test("shell-ball input bar mode stays aligned with visual states", () => {
  assert.equal(getShellBallInputBarMode("idle"), "hidden");
  assert.equal(getShellBallInputBarMode("hover_input"), "interactive");
  assert.equal(getShellBallInputBarMode("confirming_intent"), "readonly");
  assert.equal(getShellBallInputBarMode("waiting_auth"), "readonly");
  assert.equal(getShellBallInputBarMode("processing"), "readonly");
  assert.equal(getShellBallInputBarMode("voice_listening"), "voice");
  assert.equal(getShellBallInputBarMode("voice_locked"), "voice");
});

test("shell-ball interaction timing constants stay frozen", () => {
  assert.equal(SHELL_BALL_LONG_PRESS_MS, 300);
  assert.equal(SHELL_BALL_LOCK_DELTA_PX, 24);
  assert.equal(SHELL_BALL_CANCEL_DELTA_PX, 24);
  assert.equal(SHELL_BALL_CONFIRMING_MS, 600);
  assert.equal(SHELL_BALL_WAITING_AUTH_MS, 700);
  assert.equal(SHELL_BALL_PROCESSING_MS, 1200);
});

test("shell-ball motion mapping exposes state-specific accents and animations", () => {
  assert.equal(getShellBallMotionConfig("processing").wingMode, "flutter");
  assert.equal(getShellBallMotionConfig("waiting_auth").accentTone, "amber");
  assert.equal(getShellBallMotionConfig("voice_listening").ringMode, "listening");
  assert.equal(getShellBallMotionConfig("voice_locked").ringMode, "locked");
});

test("shell-ball store defaults to idle and only exposes the visual-state API", () => {
  useShellBallStore.setState({ visualState: "idle" });

  assert.equal(useShellBallStore.getState().visualState, "idle");

  useShellBallStore.getState().setVisualState("processing");

  assert.equal(useShellBallStore.getState().visualState, "processing");
  assert.deepEqual(Object.keys(useShellBallStore.getState()).sort(), ["setVisualState", "visualState"]);

  useShellBallStore.setState({ visualState: "idle" });
});

test("shell-ball interaction hook module exports the thin adapter", () => {
  assert.equal(typeof useShellBallInteraction, "function");
  assert.equal(typeof syncShellBallInteractionController, "function");
});
