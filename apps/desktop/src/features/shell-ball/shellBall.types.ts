export const shellBallVisualStates = [
  "idle",
  "hover_input",
  "confirming_intent",
  "processing",
  "waiting_auth",
  "voice_listening",
  "voice_locked",
] as const;

export type ShellBallVisualState = (typeof shellBallVisualStates)[number];

export type ShellBallPanelMode = "hidden" | "peek" | "compact" | "full";

export type ShellBallBadgeTone = "status" | "intent_confirm" | "processing" | "waiting_auth";

export type ShellBallDemoViewModel = {
  badgeTone: ShellBallBadgeTone;
  badgeLabel: string;
  title: string;
  subtitle: string;
  helperText: string;
  panelMode: ShellBallPanelMode;
  showRiskBlock: boolean;
  riskTitle?: string;
  riskText?: string;
  showVoiceHint: boolean;
  voiceHintText?: string;
};

export type ShellBallPanelSection = "badge" | "title" | "subtitle" | "helperText" | "risk" | "voiceHint";

export type ShellBallAccentTone = "slate" | "sky" | "teal" | "amber";

export type ShellBallRingMode = "hidden" | "listening" | "locked";

export type ShellBallWingMode = "rest" | "lift" | "flutter" | "tucked";

export type ShellBallEyeMode = "soft" | "curious" | "focus" | "careful" | "listening" | "locked";

export type ShellBallMotionConfig = {
  accentTone: ShellBallAccentTone;
  wingMode: ShellBallWingMode;
  ringMode: ShellBallRingMode;
  eyeMode: ShellBallEyeMode;
  bodyScale: number;
  bodyTiltDeg: number;
  floatOffsetY: number;
  floatDurationMs: number;
  breatheScale: number;
  breatheDurationMs: number;
  wingLiftDeg: number;
  wingSpreadPx: number;
  wingDurationMs: number;
  tailSwingDeg: number;
  tailDurationMs: number;
  crestLiftPx: number;
  blinkDelayMs: number;
  showAuthMarker: boolean;
};
