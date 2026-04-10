import assert from "node:assert/strict";
import test from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { getShellBallDemoViewModel } from "./shellBall.demo";
import {
  createShellBallInteractionController,
  getShellBallGestureAxisIntent,
  getShellBallInputBarMode,
  getShellBallProcessingReturnState,
  shouldPreviewShellBallVoiceGesture,
  getShellBallVoicePreview,
  resolveShellBallTransition,
  resolveShellBallVoiceReleaseEvent,
  shouldRetainShellBallHoverInput,
  SHELL_BALL_CANCEL_DELTA_PX,
  SHELL_BALL_CONFIRMING_MS,
  SHELL_BALL_HOVER_INTENT_MS,
  SHELL_BALL_LEAVE_GRACE_MS,
  SHELL_BALL_LOCK_DELTA_PX,
  SHELL_BALL_LONG_PRESS_MS,
  SHELL_BALL_PROCESSING_MS,
  SHELL_BALL_VERTICAL_PRIORITY_RATIO,
  SHELL_BALL_WAITING_AUTH_MS,
} from "./shellBall.interaction";
import { getShellBallMotionConfig } from "./shellBall.motion";
import { ShellBallApp } from "./ShellBallApp";
import { ShellBallDevLayer } from "./ShellBallDevLayer";
import { ShellBallSurface } from "./ShellBallSurface";
import { shouldShowShellBallDemoSwitcher } from "./shellBall.dev";
import { ShellBallInputBar } from "./components/ShellBallInputBar";
import type { ShellBallTransitionResult } from "./shellBall.types";
import { shellBallVisualStates } from "./shellBall.types";
import {
  getShellBallPostSubmitInputReset,
  getShellBallVoicePreviewFromEvent,
  shouldKeepShellBallVoicePreviewOnRegionLeave,
  syncShellBallInteractionController,
  useShellBallInteraction,
} from "./useShellBallInteraction";
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
      const currentHandles = [...queue.keys()];

      for (const handle of currentHandles) {
        const callback = queue.get(handle);
        if (callback === undefined) {
          continue;
        }

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
    {
      next: "idle",
      autoAdvanceTo: "hover_input",
      autoAdvanceMs: SHELL_BALL_HOVER_INTENT_MS,
    },
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
      hoverRetained: false,
    }),
    {
      next: "hover_input",
      autoAdvanceTo: "idle",
      autoAdvanceMs: SHELL_BALL_LEAVE_GRACE_MS,
    },
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "processing",
      event: "pointer_leave_region",
      regionActive: false,
      hoverRetained: false,
    }),
    { next: "processing" },
  );
});

test("shell-ball interaction contract retains hover input while focus or draft is active", () => {
  assert.equal(
    shouldRetainShellBallHoverInput({
      regionActive: false,
      inputFocused: true,
      hasDraft: false,
    }),
    true,
  );

  assert.equal(
    shouldRetainShellBallHoverInput({
      regionActive: false,
      inputFocused: false,
      hasDraft: true,
    }),
    true,
  );

  assert.deepEqual(
    resolveShellBallTransition({
      current: "hover_input",
      event: "pointer_leave_region",
      regionActive: false,
      hoverRetained: true,
    }),
    { next: "hover_input" },
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
  const hoverScheduler = createFakeScheduler();
  const hoverController = createShellBallInteractionController({
    initialState: "idle",
    schedule: hoverScheduler.schedule,
    cancel: hoverScheduler.cancel,
  });

  hoverController.dispatch("pointer_enter_hotspot", { regionActive: true });
  assert.equal(hoverController.getState(), "idle");
  assert.equal(hoverScheduler.size, 1);

  hoverScheduler.flush();
  assert.equal(hoverController.getState(), "hover_input");

  hoverController.dispatch("pointer_leave_region", { regionActive: false });
  assert.equal(hoverController.getState(), "hover_input");
  assert.equal(hoverScheduler.size, 1);

  hoverScheduler.flush();
  assert.equal(hoverController.getState(), "idle");
  hoverController.dispose();

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
  assert.equal(confirmingController.getState(), "processing");
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
  assert.equal(authController.getState(), "processing");
  assert.equal(authScheduler.size, 1);

  authScheduler.flush();
  assert.equal(authController.getState(), "idle");
  authController.dispose();
});

test("shell-ball controller cancels leave grace when the hotspot is re-entered", () => {
  const scheduler = createFakeScheduler();
  const controller = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: scheduler.schedule,
    cancel: scheduler.cancel,
  });

  controller.dispatch("pointer_leave_region", { regionActive: false });
  assert.equal(scheduler.size, 1);

  controller.dispatch("pointer_enter_hotspot", { regionActive: true });
  assert.equal(controller.getState(), "hover_input");
  assert.equal(scheduler.size, 0);

  scheduler.flush();
  assert.equal(controller.getState(), "hover_input");
  controller.dispose();
});

test("shell-ball controller keeps hover input open while retained and closes after retention ends", () => {
  const scheduler = createFakeScheduler();
  const controller = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: scheduler.schedule,
    cancel: scheduler.cancel,
  });

  controller.dispatch("pointer_leave_region", { regionActive: false, hoverRetained: true });
  assert.equal(controller.getState(), "hover_input");
  assert.equal(scheduler.size, 0);

  controller.dispatch("pointer_leave_region", { regionActive: false, hoverRetained: false });
  assert.equal(scheduler.size, 1);

  scheduler.flush();
  assert.equal(controller.getState(), "idle");
  controller.dispose();
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
  assert.equal(replacementController.getState(), "processing");
  replacementScheduler.flush();
  assert.equal(replacementController.getState(), "idle");
  replacementController.dispose();
});

test("shell-ball controller forceState applies processing entry side effects", () => {
  const scheduler = createFakeScheduler();
  const controller = createShellBallInteractionController({
    initialState: "hover_input",
    schedule: scheduler.schedule,
    cancel: scheduler.cancel,
  });

  controller.forceState("processing", { regionActive: true });
  assert.equal(controller.getState(), "processing");
  assert.equal(scheduler.size, 1);

  scheduler.flush();
  assert.equal(controller.getState(), "hover_input");
  controller.dispose();
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
  assert.equal(scheduler.size, 0);
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

test("shell-ball voice preview helpers keep preview and release resolution pure", () => {
  assert.equal(getShellBallVoicePreview({ deltaX: 0, deltaY: -SHELL_BALL_LOCK_DELTA_PX }), "lock");
  assert.equal(getShellBallVoicePreview({ deltaX: 0, deltaY: SHELL_BALL_CANCEL_DELTA_PX }), "cancel");
  assert.equal(
    getShellBallVoicePreview({
      deltaX: SHELL_BALL_CANCEL_DELTA_PX,
      deltaY: SHELL_BALL_CANCEL_DELTA_PX,
    }),
    null,
  );
  assert.equal(getShellBallVoicePreview({ deltaX: SHELL_BALL_LOCK_DELTA_PX, deltaY: 0 }), null);

  assert.equal(resolveShellBallVoiceReleaseEvent("lock"), "voice_lock");
  assert.equal(resolveShellBallVoiceReleaseEvent("cancel"), "voice_cancel");
  assert.equal(resolveShellBallVoiceReleaseEvent(null), "voice_finish");
});

test("shell-ball gesture helpers classify vertical intent explicitly for drag-safe voice previews", () => {
  assert.equal(
    getShellBallGestureAxisIntent({
      deltaX: 8,
      deltaY: -SHELL_BALL_LOCK_DELTA_PX,
    }),
    "vertical",
  );

  assert.equal(
    getShellBallGestureAxisIntent({
      deltaX: SHELL_BALL_CANCEL_DELTA_PX,
      deltaY: SHELL_BALL_CANCEL_DELTA_PX,
    }),
    "horizontal",
  );

  assert.equal(
    getShellBallGestureAxisIntent({
      deltaX: SHELL_BALL_CANCEL_DELTA_PX,
      deltaY: 12,
    }),
    "horizontal",
  );
});

test("shell-ball gesture helpers gate voice preview behind vertical-priority intent", () => {
  assert.equal(
    shouldPreviewShellBallVoiceGesture({
      deltaX: 0,
      deltaY: SHELL_BALL_CANCEL_DELTA_PX,
    }),
    true,
  );

  assert.equal(
    shouldPreviewShellBallVoiceGesture({
      deltaX: SHELL_BALL_CANCEL_DELTA_PX,
      deltaY: SHELL_BALL_CANCEL_DELTA_PX,
    }),
    false,
  );

  assert.equal(
    shouldPreviewShellBallVoiceGesture({
      deltaX: SHELL_BALL_CANCEL_DELTA_PX,
      deltaY: 12,
    }),
    false,
  );
});

test("shell-ball input bar surfaces voice preview guidance to the UI", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallInputBar, {
      mode: "voice",
      voicePreview: "cancel",
      value: "",
      onValueChange: () => {},
      onAttachFile: () => {},
      onSubmit: () => {},
      onFocusChange: () => {},
    }),
  );

  assert.match(markup, /data-voice-preview="cancel"/);
  assert.match(markup, /Release to cancel/);
});

test("shell-ball release preview recomputes from the final pointer position", () => {
  assert.equal(
    getShellBallVoicePreviewFromEvent({
      startX: 100,
      startY: 100,
      clientX: 100,
      clientY: 52,
      fallbackPreview: null,
    }),
    "lock",
  );

  assert.equal(
    getShellBallVoicePreviewFromEvent({
      startX: 100,
      startY: 100,
      clientX: 100,
      clientY: 148,
      fallbackPreview: null,
    }),
    "cancel",
  );
});

test("shell-ball keeps voice preview alive on leave while voice listening is active", () => {
  assert.equal(shouldKeepShellBallVoicePreviewOnRegionLeave("voice_listening"), true);
  assert.equal(shouldKeepShellBallVoicePreviewOnRegionLeave("hover_input"), false);
  assert.equal(shouldKeepShellBallVoicePreviewOnRegionLeave("voice_locked"), false);
});

test("shell-ball submit reset clears draft retention after submit", () => {
  assert.deepEqual(getShellBallPostSubmitInputReset("summarize this"), {
    nextInputValue: "",
    nextFocused: false,
  });

  assert.equal(
    shouldRetainShellBallHoverInput({
      regionActive: false,
      inputFocused: false,
      hasDraft: false,
    }),
    false,
  );
});

test("shell-ball input bar removes keyboard focus stops outside interactive mode", () => {
  const readonlyMarkup = renderToStaticMarkup(
    createElement(ShellBallInputBar, {
      mode: "readonly",
      voicePreview: null,
      value: "submitted",
      onValueChange: () => {},
      onAttachFile: () => {},
      onSubmit: () => {},
      onFocusChange: () => {},
    }),
  );

  const voiceMarkup = renderToStaticMarkup(
    createElement(ShellBallInputBar, {
      mode: "voice",
      voicePreview: null,
      value: "",
      onValueChange: () => {},
      onAttachFile: () => {},
      onSubmit: () => {},
      onFocusChange: () => {},
    }),
  );

  assert.match(readonlyMarkup, /tabindex="-1"/i);
  assert.match(voiceMarkup, /tabindex="-1"/i);
});

test("shell-ball app drops page-shell copy while preserving the floating shell surface", () => {
  const markup = renderToStaticMarkup(createElement(ShellBallApp, { isDev: false }));

  assert.doesNotMatch(markup, /shell-ball phase 1/i);
  assert.doesNotMatch(markup, /小胖啾近场承接/);
  assert.doesNotMatch(markup, /demo-only 第一阶段边界/);
  assert.doesNotMatch(markup, /<main/i);
  assert.match(markup, /shell-ball-surface/);
  assert.match(markup, /shell-ball-bubble-zone/);
  assert.match(markup, /shell-ball-mascot/);
  assert.doesNotMatch(markup, /Shell-ball demo switcher/);
});

test("shell-ball surface renders the floating structure without the demo switcher", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallSurface, {
      visualState: "hover_input",
      voicePreview: null,
      inputBarMode: "interactive",
      inputValue: "draft",
      motionConfig: getShellBallMotionConfig("hover_input"),
      onPrimaryClick: () => {},
      onRegionEnter: () => {},
      onRegionLeave: () => {},
      onInputValueChange: () => {},
      onAttachFile: () => {},
      onSubmitText: () => {},
      onPressStart: () => {},
      onPressMove: () => {},
      onPressEnd: () => false,
      onInputFocusChange: () => {},
    }),
  );

  assert.match(markup, /shell-ball-surface/);
  assert.match(markup, /shell-ball-bubble-zone/);
  assert.match(markup, /shell-ball-mascot/);
  assert.match(markup, /shell-ball-input-bar/);
  assert.doesNotMatch(markup, /Shell-ball demo switcher/);
  assert.doesNotMatch(markup, /shell-ball-surface__switcher-shell/);
});

test("shell-ball surface reserves a host drag zone separate from the interaction zone", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallSurface, {
      visualState: "hover_input",
      voicePreview: null,
      inputBarMode: "interactive",
      inputValue: "draft",
      motionConfig: getShellBallMotionConfig("hover_input"),
      onPrimaryClick: () => {},
      onRegionEnter: () => {},
      onRegionLeave: () => {},
      onInputValueChange: () => {},
      onAttachFile: () => {},
      onSubmitText: () => {},
      onPressStart: () => {},
      onPressMove: () => {},
      onPressEnd: () => false,
      onInputFocusChange: () => {},
    }),
  );

  assert.match(markup, /data-shell-ball-zone="host-drag"/);
  assert.match(markup, /data-shell-ball-zone="interaction"/);
  assert.match(markup, /data-shell-ball-zone="voice-hotspot"/);
  assert.match(markup, /shell-ball-surface__host-drag-zone/);
  assert.match(markup, /shell-ball-surface__interaction-zone/);
});

test("shell-ball demo switcher visibility stays dev-only", () => {
  assert.equal(shouldShowShellBallDemoSwitcher(true), true);
  assert.equal(shouldShowShellBallDemoSwitcher(false), false);
});

test("shell-ball dev layer isolates demo controls from the formal surface", () => {
  const markup = renderToStaticMarkup(
    createElement(ShellBallDevLayer, {
      value: "idle",
      onChange: () => {},
    }),
  );

  assert.match(markup, /Shell-ball demo controls/);
  assert.match(markup, /Shell-ball demo switcher/);
  assert.match(markup, /shell-ball-surface__switcher-shell/);
});

test("shell-ball app keeps the reusable surface as the production structure", () => {
  const markup = renderToStaticMarkup(createElement(ShellBallApp, { isDev: false }));

  assert.match(markup, /Shell-ball floating surface/);
  assert.match(markup, /shell-ball-surface__body/);
  assert.doesNotMatch(markup, /Shell-ball demo switcher/);
  assert.doesNotMatch(markup, /shell-ball-surface__switcher-shell/);
});

test("shell-ball app injects the demo switcher only in dev mode", () => {
  const markup = renderToStaticMarkup(createElement(ShellBallApp, { isDev: true }));

  assert.match(markup, /Shell-ball floating surface/);
  assert.match(markup, /shell-ball-surface__body/);
  assert.match(markup, /Shell-ball demo switcher/);
  assert.match(markup, /shell-ball-surface__switcher-shell/);
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
  assert.equal(SHELL_BALL_HOVER_INTENT_MS, 240);
  assert.equal(SHELL_BALL_LEAVE_GRACE_MS, 180);
  assert.equal(SHELL_BALL_LONG_PRESS_MS, 420);
  assert.equal(SHELL_BALL_LOCK_DELTA_PX, 48);
  assert.equal(SHELL_BALL_CANCEL_DELTA_PX, 48);
  assert.equal(SHELL_BALL_VERTICAL_PRIORITY_RATIO, 1.25);
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
