import assert from "node:assert/strict";
import test from "node:test";
import { getShellBallDemoViewModel, getShellBallPanelSections } from "./shellBall.demo";
import { getShellBallMotionConfig } from "./shellBall.motion";
import { shellBallVisualStates } from "./shellBall.types";
import { useShellBallStore } from "../../stores/shellBallStore";

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

test("shell-ball panel layout stays derived from the frozen view-model contract", () => {
  assert.deepEqual(getShellBallPanelSections(getShellBallDemoViewModel("idle")), []);
  assert.deepEqual(getShellBallPanelSections(getShellBallDemoViewModel("hover_input")), ["badge", "title", "helperText"]);
  assert.deepEqual(getShellBallPanelSections(getShellBallDemoViewModel("waiting_auth")), [
    "badge",
    "title",
    "subtitle",
    "helperText",
    "risk",
  ]);
  assert.deepEqual(getShellBallPanelSections(getShellBallDemoViewModel("voice_locked")), [
    "badge",
    "title",
    "subtitle",
    "helperText",
    "voiceHint",
  ]);
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
