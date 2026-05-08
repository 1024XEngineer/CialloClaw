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

export type ShellBallVoiceHintMode = "hidden" | "lock" | "cancel";

export type ShellBallInteractionEvent =
  | "pointer_enter_hotspot"
  | "pointer_leave_region"
  | "submit_text"
  | "attach_file"
  | "press_start"
  | "voice_lock"
  | "voice_cancel"
  | "voice_finish"
  | "primary_click_locked_voice_end"
  | "auto_advance";

export type ShellBallInputBarMode = "hidden" | "interactive" | "readonly" | "voice";

export type ShellBallTransitionResult =
  | {
      next: ShellBallVisualState;
      autoAdvanceTo: ShellBallVisualState;
      autoAdvanceMs: number;
    }
  | {
      next: ShellBallVisualState;
      autoAdvanceTo?: never;
      autoAdvanceMs?: never;
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
