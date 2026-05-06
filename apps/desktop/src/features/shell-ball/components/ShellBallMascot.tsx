import { useEffect, useRef, useState } from "react";
import type { CSSProperties, MouseEvent, PointerEvent } from "react";
import { motion, useReducedMotion } from "motion/react";
import { AudioLines, Mic, ShieldAlert } from "lucide-react";
import { cn } from "../../../utils/cn";
import { SHELL_BALL_PRESS_DRIFT_TOLERANCE_PX, type ShellBallVoicePreview } from "../shellBall.interaction";
import type { ShellBallMotionConfig, ShellBallVisualState } from "../shellBall.types";
import { FLOATING_PET_HAPPY_DURATION_MS, type FloatingPetMode } from "./floating-pet/petAssets";
import type { ShellBallEdgeDockSide } from "../useShellBallWindowMetrics";

type ShellBallMascotProps = {
  dockTarget?: ShellBallEdgeDockSide | null;
  edgeDockRevealed?: boolean;
  edgeDockSide?: ShellBallEdgeDockSide | null;
  hasAlertOpportunity?: boolean;
  hasPendingAgentLoading?: boolean;
  hasPendingApproval?: boolean;
  isDragging?: boolean;
  isSettling?: boolean;
  visualState: ShellBallVisualState;
  voicePreview?: ShellBallVoicePreview;
  showVoiceHints?: boolean;
  selectionIndicatorVisible?: boolean;
  voiceHoldProgress?: number;
  motionConfig: ShellBallMotionConfig;
  onPrimaryClick?: () => void;
  onDoubleClick?: () => void;
  onHotspotEnter?: () => void;
  onHotspotLeave?: () => void;
  onHotspotDragStart?: (event: PointerEvent<HTMLButtonElement>) => void;
  onHotspotDragMove?: (event: PointerEvent<HTMLButtonElement>) => void;
  onHotspotDragEnd?: (event: PointerEvent<HTMLButtonElement>) => void;
  onHotspotDragCancel?: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressStart?: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressMove?: (event: PointerEvent<HTMLButtonElement>) => void;
  onPressEnd?: (event: PointerEvent<HTMLButtonElement>) => boolean;
  onPressCancel?: (event: PointerEvent<HTMLButtonElement>) => void;
};

type MotionStyle = CSSProperties & Record<string, string>;

type ShellBallMascotHotspotGesture = "single_click" | "double_click";

type ShellBallMascotHotspotGestureAction = "noop" | "primary_click" | "double_click";

type ShellBallMascotPointerPhase = "pointer_down" | "pointer_up" | "pointer_cancel";

type ShellBallMascotPointerPhaseAction = "noop" | "start_press" | "finish_press" | "suppress_gestures" | "cleanup_only";

type ShellBallMascotPetState = {
  listenLocked: boolean;
  mode: FloatingPetMode;
};

function canTriggerShellBallMascotHappyPulse(visualState: ShellBallVisualState) {
  return visualState === "idle" || visualState === "hover_input";
}

type ShellBallAmbientLoopProfile = {
  durationMs: number;
  repeatDelayMs: number;
  faceX: number[];
  faceY: number[];
  faceRotate: number[];
  crestY: number[];
  crestRotate: number[];
  crestScale: number[];
  tailY: number[];
  tailRotate: number[];
  backOrbitalScale: number[];
  backOrbitalOpacity: number[];
  frontOrbitalScale: number[];
  frontOrbitalOpacity: number[];
  shadowOffsetY: number[];
  shadowScaleX: number[];
  shadowScaleY: number[];
};
function canTriggerShellBallMascotSecondaryGestures(visualState: ShellBallVisualState) {
  return visualState !== "voice_listening" && visualState !== "voice_locked";
}

export function getShellBallMascotHotspotGestureAction(input: {
  visualState: ShellBallVisualState;
  gesture: ShellBallMascotHotspotGesture;
  suppressed: boolean;
  alertOpportunityAvailable?: boolean;
  selectionIndicatorVisible?: boolean;
}): ShellBallMascotHotspotGestureAction {
  if (input.suppressed) {
    return "noop";
  }

  if (input.gesture === "single_click") {
    return input.selectionIndicatorVisible || input.alertOpportunityAvailable ? "primary_click" : "noop";
  }

  if (canTriggerShellBallMascotSecondaryGestures(input.visualState)) {
    return "double_click";
  }

  return "noop";
}

export function getShellBallMascotPointerPhaseAction(input: {
  phase: ShellBallMascotPointerPhase;
  button: number;
  isPrimary: boolean;
  pressHandled: boolean;
}): ShellBallMascotPointerPhaseAction {
  if (input.phase === "pointer_cancel") {
    return input.isPrimary ? "cleanup_only" : "noop";
  }

  const isPrimaryButtonSequence = input.button === 0 && input.isPrimary;

  if (!isPrimaryButtonSequence) {
    return "noop";
  }

  if (input.phase === "pointer_down") {
    return "start_press";
  }

  return input.pressHandled ? "suppress_gestures" : "finish_press";
}

export function shouldSuppressShellBallMascotHotspotGestures(input: {
  startX: number | null;
  startY: number | null;
  pointerX: number;
  pointerY: number;
}) {
  if (input.startX === null || input.startY === null) {
    return false;
  }

  return Math.hypot(input.pointerX - input.startX, input.pointerY - input.startY) > SHELL_BALL_PRESS_DRIFT_TOLERANCE_PX;
}

export function getShellBallMascotPetState(input: {
  hasAlertOpportunity?: boolean;
  hasPendingAgentLoading?: boolean;
  hasPendingApproval?: boolean;
  happyActive: boolean;
  selectionIndicatorVisible?: boolean;
  visualState: ShellBallVisualState;
}): ShellBallMascotPetState {
  if (input.hasPendingAgentLoading || input.visualState === "processing") {
    return {
      listenLocked: false,
      mode: "think",
    };
  }

  if (input.visualState === "voice_locked") {
    return {
      listenLocked: true,
      mode: "listen",
    };
  }

  if (input.visualState === "voice_listening") {
    return {
      listenLocked: false,
      mode: "listen",
    };
  }

  if (input.visualState === "confirming_intent") {
    return {
      listenLocked: false,
      mode: "alert",
    };
  }

  if (input.happyActive) {
    return {
      listenLocked: false,
      mode: "happy",
    };
  }

  if (input.hasPendingApproval || input.visualState === "waiting_auth") {
    return {
      listenLocked: false,
      mode: "safe",
    };
  }

  if (input.selectionIndicatorVisible || input.hasAlertOpportunity) {
    return {
      listenLocked: false,
      mode: "alert",
    };
  }

  return {
    listenLocked: false,
    mode: "idle",
  };
}

function getShellBallDockAttitude(input: {
  edgeDockSide: ShellBallEdgeDockSide | null;
  edgeDockRevealed: boolean;
}) {
  if (input.edgeDockSide === null) {
    return {
      shiftX: 0,
      shiftY: 0,
      tiltDeg: 0,
    };
  }

  if (input.edgeDockSide === "left") {
    return {
      shiftX: input.edgeDockRevealed ? 2 : 6,
      shiftY: 0,
      tiltDeg: input.edgeDockRevealed ? 4 : 12,
    };
  }

  if (input.edgeDockSide === "right") {
    return {
      shiftX: input.edgeDockRevealed ? -2 : -6,
      shiftY: 0,
      tiltDeg: input.edgeDockRevealed ? -4 : -12,
    };
  }

  if (input.edgeDockSide === "top") {
    return {
      shiftX: 0,
      shiftY: input.edgeDockRevealed ? 0 : -8,
      tiltDeg: 0,
    };
  }

  if (input.edgeDockSide === "top_left") {
    return {
      shiftX: input.edgeDockRevealed ? 1 : 5,
      shiftY: input.edgeDockRevealed ? 0 : -8,
      tiltDeg: input.edgeDockRevealed ? 1 : 8,
    };
  }

  if (input.edgeDockSide === "top_right") {
    return {
      shiftX: input.edgeDockRevealed ? -1 : -5,
      shiftY: input.edgeDockRevealed ? 0 : -8,
      tiltDeg: input.edgeDockRevealed ? -1 : -8,
    };
  }

  if (input.edgeDockSide === "bottom_left") {
    return {
      shiftX: input.edgeDockRevealed ? 1 : 4,
      shiftY: input.edgeDockRevealed ? 0 : 4,
      tiltDeg: input.edgeDockRevealed ? 1 : 6,
    };
  }

  if (input.edgeDockSide === "bottom_right") {
    return {
      shiftX: input.edgeDockRevealed ? -1 : -4,
      shiftY: input.edgeDockRevealed ? 0 : 4,
      tiltDeg: input.edgeDockRevealed ? -1 : -6,
    };
  }

  return {
    shiftX: 0,
    shiftY: input.edgeDockRevealed ? 0 : 4,
    tiltDeg: 0,
  };
}

function scaleShellBallKeyframes(values: number[], factor: number) {
  return values.map((value) => Number((value * factor).toFixed(3)));
}

/**
 * Ambient mascot choreography keeps the bird feeling alive between explicit
 * interaction states. Docked parked states damp the loop so top and bottom
 * parking never jiggles the face outside the visible area.
 */
function getShellBallAmbientLoopProfile(input: {
  visualState: ShellBallVisualState;
  edgeDockSide: ShellBallEdgeDockSide | null;
  edgeDockRevealed: boolean;
}): ShellBallAmbientLoopProfile {
  const baseProfile: Record<ShellBallVisualState, ShellBallAmbientLoopProfile> = {
    idle: {
      durationMs: 5800,
      repeatDelayMs: 320,
      faceX: [0, -1.2, 1.2, 0],
      faceY: [0, -0.7, 0.4, 0],
      faceRotate: [0, -1.2, 1.1, 0],
      crestY: [0, -0.8, 0.5, 0],
      crestRotate: [0, -3, 2, 0],
      crestScale: [1, 1.02, 0.985, 1],
      tailY: [0, -0.45, 0.2, 0],
      tailRotate: [0, 3, -2, 0],
      backOrbitalScale: [1, 1.03, 0.99, 1],
      backOrbitalOpacity: [0.74, 0.8, 0.72, 0.74],
      frontOrbitalScale: [1, 1.025, 0.995, 1],
      frontOrbitalOpacity: [0.72, 0.77, 0.69, 0.72],
      shadowOffsetY: [0, 1, -0.4, 0],
      shadowScaleX: [1, 1.04, 0.98, 1],
      shadowScaleY: [1, 0.95, 1.02, 1],
    },
    hover_input: {
      durationMs: 3200,
      repeatDelayMs: 120,
      faceX: [0, 2.2, -1.3, 0],
      faceY: [0, -1.1, -0.2, 0],
      faceRotate: [0, 2.3, -1.2, 0],
      crestY: [0, -1.3, 0.8, 0],
      crestRotate: [0, 4, -2, 0],
      crestScale: [1, 1.05, 0.98, 1],
      tailY: [0, -0.8, 0.3, 0],
      tailRotate: [0, 4.8, -3, 0],
      backOrbitalScale: [1, 1.05, 1, 1],
      backOrbitalOpacity: [0.74, 0.85, 0.76, 0.74],
      frontOrbitalScale: [1, 1.04, 1, 1],
      frontOrbitalOpacity: [0.72, 0.82, 0.74, 0.72],
      shadowOffsetY: [0, 1.2, -0.5, 0],
      shadowScaleX: [1, 1.06, 0.99, 1],
      shadowScaleY: [1, 0.94, 1.03, 1],
    },
    confirming_intent: {
      durationMs: 2700,
      repeatDelayMs: 80,
      faceX: [0, 1.5, -0.7, 0],
      faceY: [0, -1.2, 0.25, 0],
      faceRotate: [0, 1.6, -0.8, 0],
      crestY: [0, -1.1, 0.7, 0],
      crestRotate: [0, 3.8, -2.2, 0],
      crestScale: [1, 1.04, 0.985, 1],
      tailY: [0, -0.7, 0.25, 0],
      tailRotate: [0, 4, -2.6, 0],
      backOrbitalScale: [1, 1.04, 1, 1],
      backOrbitalOpacity: [0.74, 0.83, 0.75, 0.74],
      frontOrbitalScale: [1, 1.03, 0.998, 1],
      frontOrbitalOpacity: [0.72, 0.8, 0.74, 0.72],
      shadowOffsetY: [0, 1, -0.45, 0],
      shadowScaleX: [1, 1.05, 0.985, 1],
      shadowScaleY: [1, 0.95, 1.02, 1],
    },
    processing: {
      durationMs: 1800,
      repeatDelayMs: 0,
      faceX: [0, 1.6, -1.6, 0],
      faceY: [0, -1.4, 0.55, 0],
      faceRotate: [0, 2.8, -2.6, 0],
      crestY: [0, -1.6, 1.1, 0],
      crestRotate: [0, 5, -4, 0],
      crestScale: [1, 1.07, 0.97, 1],
      tailY: [0, -1, 0.45, 0],
      tailRotate: [0, 6, -4.5, 0],
      backOrbitalScale: [1, 1.07, 0.985, 1],
      backOrbitalOpacity: [0.74, 0.92, 0.76, 0.74],
      frontOrbitalScale: [1, 1.05, 0.99, 1],
      frontOrbitalOpacity: [0.72, 0.88, 0.74, 0.72],
      shadowOffsetY: [0, 1.4, -0.6, 0],
      shadowScaleX: [1, 1.08, 0.98, 1],
      shadowScaleY: [1, 0.92, 1.04, 1],
    },
    waiting_auth: {
      durationMs: 4600,
      repeatDelayMs: 260,
      faceX: [0, -0.6, 0.6, 0],
      faceY: [0, 0.25, -0.15, 0],
      faceRotate: [0, -0.8, 0.8, 0],
      crestY: [0, -0.35, 0.18, 0],
      crestRotate: [0, -2, 1.5, 0],
      crestScale: [1, 1.01, 0.995, 1],
      tailY: [0, -0.2, 0.1, 0],
      tailRotate: [0, 1.6, -1.2, 0],
      backOrbitalScale: [1, 1.02, 0.995, 1],
      backOrbitalOpacity: [0.74, 0.77, 0.72, 0.74],
      frontOrbitalScale: [1, 1.015, 0.995, 1],
      frontOrbitalOpacity: [0.72, 0.75, 0.7, 0.72],
      shadowOffsetY: [0, 0.5, -0.2, 0],
      shadowScaleX: [1, 1.02, 0.99, 1],
      shadowScaleY: [1, 0.98, 1.01, 1],
    },
    voice_listening: {
      durationMs: 2200,
      repeatDelayMs: 40,
      faceX: [0, -1.4, 1.3, 0],
      faceY: [0, -1.05, 0.1, 0],
      faceRotate: [0, -1.8, 1.2, 0],
      crestY: [0, -1.4, 0.9, 0],
      crestRotate: [0, -4.2, 2.8, 0],
      crestScale: [1, 1.05, 0.98, 1],
      tailY: [0, -0.7, 0.25, 0],
      tailRotate: [0, 4.2, -2.5, 0],
      backOrbitalScale: [1, 1.05, 0.995, 1],
      backOrbitalOpacity: [0.74, 0.87, 0.76, 0.74],
      frontOrbitalScale: [1, 1.04, 0.995, 1],
      frontOrbitalOpacity: [0.72, 0.84, 0.74, 0.72],
      shadowOffsetY: [0, 1.1, -0.45, 0],
      shadowScaleX: [1, 1.05, 0.985, 1],
      shadowScaleY: [1, 0.94, 1.03, 1],
    },
    voice_locked: {
      durationMs: 1900,
      repeatDelayMs: 20,
      faceX: [0, 0.8, -0.8, 0],
      faceY: [0, -0.6, 0, 0],
      faceRotate: [0, 1, -1, 0],
      crestY: [0, -0.8, 0.5, 0],
      crestRotate: [0, 2.6, -2, 0],
      crestScale: [1, 1.03, 0.99, 1],
      tailY: [0, -0.4, 0.18, 0],
      tailRotate: [0, 2.4, -1.8, 0],
      backOrbitalScale: [1, 1.04, 0.995, 1],
      backOrbitalOpacity: [0.74, 0.83, 0.76, 0.74],
      frontOrbitalScale: [1, 1.03, 0.995, 1],
      frontOrbitalOpacity: [0.72, 0.8, 0.74, 0.72],
      shadowOffsetY: [0, 0.8, -0.35, 0],
      shadowScaleX: [1, 1.04, 0.988, 1],
      shadowScaleY: [1, 0.95, 1.02, 1],
    },
  };
  const base = baseProfile[input.visualState];

  if (input.edgeDockSide === null || input.edgeDockRevealed) {
    return base;
  }

  const parkedDamping =
    input.edgeDockSide === "top"
    || input.edgeDockSide === "bottom"
    || input.edgeDockSide === "top_left"
    || input.edgeDockSide === "top_right"
    || input.edgeDockSide === "bottom_left"
    || input.edgeDockSide === "bottom_right"
      ? 0.58
      : 0.82;

  return {
    ...base,
    faceX: scaleShellBallKeyframes(base.faceX, parkedDamping),
    faceY: scaleShellBallKeyframes(base.faceY, parkedDamping),
    faceRotate: scaleShellBallKeyframes(base.faceRotate, parkedDamping),
    crestY: scaleShellBallKeyframes(base.crestY, parkedDamping),
    crestRotate: scaleShellBallKeyframes(base.crestRotate, parkedDamping),
    tailY: scaleShellBallKeyframes(base.tailY, parkedDamping),
    tailRotate: scaleShellBallKeyframes(base.tailRotate, parkedDamping),
    shadowOffsetY: scaleShellBallKeyframes(base.shadowOffsetY, parkedDamping),
    shadowScaleX: scaleShellBallKeyframes(base.shadowScaleX, parkedDamping),
    shadowScaleY: scaleShellBallKeyframes(base.shadowScaleY, parkedDamping),
  };
}

export function ShellBallMascot({
  dockTarget = null,
  edgeDockRevealed = false,
  edgeDockSide = null,
  hasAlertOpportunity = false,
  hasPendingAgentLoading = false,
  hasPendingApproval = false,
  isDragging = false,
  isSettling = false,
  visualState,
  voicePreview = null,
  showVoiceHints = true,
  selectionIndicatorVisible = false,
  voiceHoldProgress = 0,
  motionConfig,
  onPrimaryClick = () => {},
  onDoubleClick = () => {},
  onHotspotEnter = () => {},
  onHotspotLeave = () => {},
  onHotspotDragStart = () => {},
  onHotspotDragMove = () => {},
  onHotspotDragEnd = () => {},
  onHotspotDragCancel = () => {},
  onPressStart = () => {},
  onPressMove = () => {},
  onPressEnd = () => false,
  onPressCancel = () => {},
}: ShellBallMascotProps) {
  const prefersReducedMotion = useReducedMotion();
  const activeSequenceRef = useRef(false);
  const draggingSequenceRef = useRef(false);
  const pointerStartXRef = useRef<number | null>(null);
  const pointerStartYRef = useRef<number | null>(null);
  const suppressGestureRef = useRef(false);
  const happyTimeoutRef = useRef<number | null>(null);
  const [happyActive, setHappyActive] = useState(false);

  const floatStyle: MotionStyle = {
    "--shell-ball-float-distance": `${motionConfig.floatOffsetY}px`,
    "--shell-ball-float-duration": `${motionConfig.floatDurationMs}ms`,
  };
  const bodyShellStyle: MotionStyle = {
    "--shell-ball-breathe-scale": String(motionConfig.breatheScale),
    "--shell-ball-breathe-duration": `${motionConfig.breatheDurationMs}ms`,
  };
  const dockAttitude = getShellBallDockAttitude({
    edgeDockSide,
    edgeDockRevealed,
  });
  const wingStyle: MotionStyle = {
    "--shell-ball-wing-lift": `${motionConfig.wingLiftDeg}deg`,
    "--shell-ball-wing-duration": `${motionConfig.wingDurationMs}ms`,
    "--shell-ball-wing-spread": `${motionConfig.wingSpreadPx}px`,
  };
  const tailStyle: MotionStyle = {
    "--shell-ball-tail-swing": `${motionConfig.tailSwingDeg}deg`,
    "--shell-ball-tail-duration": `${motionConfig.tailDurationMs}ms`,
  };
  const eyeStyle: CSSProperties = {
    animationDelay: `${motionConfig.blinkDelayMs}ms`,
  };
  const holdRingCircumference = 2 * Math.PI * 84;
  const holdRingDashOffset = holdRingCircumference * (1 - voiceHoldProgress);
  const showVoiceHoldRing = voiceHoldProgress > 0 && visualState !== "voice_listening" && visualState !== "voice_locked";
  const shouldRenderVoiceHints = showVoiceHints && (visualState === "voice_listening" || visualState === "voice_locked");
  const showVoiceMarker = visualState === "voice_listening" || visualState === "voice_locked";
  const showSelectionMarker = (selectionIndicatorVisible || hasAlertOpportunity) && !showVoiceMarker;
  const shouldRouteHotspotDrag = visualState !== "voice_listening" && visualState !== "voice_locked";
  const petState = getShellBallMascotPetState({
    hasAlertOpportunity,
    hasPendingAgentLoading,
    hasPendingApproval,
    happyActive,
    selectionIndicatorVisible,
    visualState,
  });

  useEffect(() => {
    return () => {
      if (happyTimeoutRef.current !== null) {
        window.clearTimeout(happyTimeoutRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (canTriggerShellBallMascotHappyPulse(visualState)) {
      return;
    }

    if (happyTimeoutRef.current !== null) {
      window.clearTimeout(happyTimeoutRef.current);
      happyTimeoutRef.current = null;
    }

    setHappyActive(false);
  }, [visualState]);

  function triggerHappyPulse() {
    if (!canTriggerShellBallMascotHappyPulse(visualState)) {
      return;
    }

    if (happyTimeoutRef.current !== null) {
      return;
    }

    setHappyActive(true);
    happyTimeoutRef.current = window.setTimeout(() => {
      happyTimeoutRef.current = null;
      setHappyActive(false);
    }, FLOATING_PET_HAPPY_DURATION_MS);
  }

  const ambientLoopEnabled = !prefersReducedMotion && !isDragging && !isSettling;
  const ambientLoopProfile = getShellBallAmbientLoopProfile({
    visualState,
    edgeDockSide,
    edgeDockRevealed,
  });
  const rootScale = isDragging ? 1.03 : isSettling ? 1.01 : happyActive ? 1.02 : 1;
  const rootBrightness = isDragging ? 1.06 : isSettling ? 1.08 : happyActive ? 1.03 : 1;
  const attitudeLift = isDragging ? -4 : isSettling ? -2 : 0;
  const attitudeScale = motionConfig.bodyScale * (isDragging ? 1.06 : isSettling ? 1.02 : happyActive ? 1.01 : 1);
  const ambientLoopTransition = ambientLoopEnabled
    ? {
        duration: ambientLoopProfile.durationMs / 1000,
        ease: "easeInOut" as const,
        repeat: Number.POSITIVE_INFINITY,
        repeatDelay: ambientLoopProfile.repeatDelayMs / 1000,
        times: [0, 0.28, 0.72, 1] as number[],
      }
    : { duration: 0.28, ease: "easeOut" as const };
  const showAuthMarker = motionConfig.showAuthMarker || petState.mode === "safe";

  function resetPointerSequence() {
    activeSequenceRef.current = false;
    draggingSequenceRef.current = false;
    pointerStartXRef.current = null;
    pointerStartYRef.current = null;
  }

  function handlePointerDown(event: PointerEvent<HTMLButtonElement>) {
    if (
      getShellBallMascotPointerPhaseAction({
        phase: "pointer_down",
        button: event.button,
        isPrimary: event.isPrimary,
        pressHandled: false,
      }) !== "start_press"
    ) {
      return;
    }

    // Prevent pointer drag from leaving a focus ring on the hotspot button.
    event.preventDefault();
    event.currentTarget.blur();
    suppressGestureRef.current = false;
    activeSequenceRef.current = true;
    draggingSequenceRef.current = false;
    pointerStartXRef.current = event.screenX;
    pointerStartYRef.current = event.screenY;
    event.currentTarget.setPointerCapture(event.pointerId);
    onPressStart(event);
    onHotspotDragStart(event);
  }

  function handlePointerMove(event: PointerEvent<HTMLButtonElement>) {
    if (!activeSequenceRef.current) {
      return;
    }

    if (shouldRouteHotspotDrag) {
      onHotspotDragMove(event);
    }

    if (
      !draggingSequenceRef.current &&
      shouldSuppressShellBallMascotHotspotGestures({
        startX: pointerStartXRef.current,
        startY: pointerStartYRef.current,
        pointerX: event.screenX,
        pointerY: event.screenY,
      })
    ) {
      draggingSequenceRef.current = true;
      suppressGestureRef.current = true;
    }

    onPressMove(event);
  }

  function handlePointerEnd(event: PointerEvent<HTMLButtonElement>) {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }

    if (!activeSequenceRef.current) {
      return;
    }

    const pointerAction = getShellBallMascotPointerPhaseAction({
      phase: "pointer_up",
      button: event.button,
      isPrimary: event.isPrimary,
      pressHandled: false,
    });

    if (pointerAction === "noop") {
      return;
    }

    const dragSuppressed = draggingSequenceRef.current;
    const handled = onPressEnd(event);

    if (!handled) {
      onHotspotDragEnd(event);
    }

    resetPointerSequence();
    const action = getShellBallMascotPointerPhaseAction({
      phase: "pointer_up",
      button: event.button,
      isPrimary: event.isPrimary,
      pressHandled: handled,
    });

    if (action !== "suppress_gestures" && !dragSuppressed) {
      return;
    }

    suppressGestureRef.current = true;
  }

  function handlePointerCancel(event: PointerEvent<HTMLButtonElement>) {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }

    const action = getShellBallMascotPointerPhaseAction({
      phase: "pointer_cancel",
      button: event.button,
      isPrimary: event.isPrimary,
      pressHandled: false,
    });

    if (action !== "cleanup_only") {
      return;
    }

    suppressGestureRef.current = false;
    const shouldNotifyCancel = activeSequenceRef.current;
    if (shouldNotifyCancel && shouldRouteHotspotDrag) {
      onHotspotDragCancel(event);
    }
    resetPointerSequence();
    if (shouldNotifyCancel) {
      onPressCancel(event);
    }
  }

  function handleClick(_event: MouseEvent<HTMLButtonElement>) {
    const action = getShellBallMascotHotspotGestureAction({
      alertOpportunityAvailable: hasAlertOpportunity,
      visualState,
      gesture: "single_click",
      suppressed: suppressGestureRef.current,
      selectionIndicatorVisible,
    });

    if (_event.detail === 1 && !suppressGestureRef.current) {
      triggerHappyPulse();
    }

    if (action === "primary_click") {
      onPrimaryClick();
    }
  }

  function handleDoubleClick(_event: MouseEvent<HTMLButtonElement>) {
    const action = getShellBallMascotHotspotGestureAction({
      visualState,
      gesture: "double_click",
      suppressed: suppressGestureRef.current,
    });

    if (action !== "double_click") {
      return;
    }

    onDoubleClick();
  }

  return (
    <motion.div
      className={cn("shell-ball-mascot", voicePreview !== null && `shell-ball-mascot--preview-${voicePreview}`)}
      animate={{
        "--shell-ball-mascot-visual-scale": rootScale,
        filter: `brightness(${rootBrightness}) saturate(${isDragging ? 1.1 : isSettling ? 1.06 : 1})`,
      }}
      data-shell-ball-pet-listen-locked={petState.listenLocked ? "true" : "false"}
      data-shell-ball-pet-mode={petState.mode}
      data-state={visualState}
      data-dock-target={dockTarget ?? "none"}
      data-edge-dock-revealed={edgeDockRevealed ? "true" : "false"}
      data-edge-dock-side={edgeDockSide ?? "none"}
      data-shell-ball-dragging={isDragging ? "true" : "false"}
      data-shell-ball-settling={isSettling ? "true" : "false"}
      data-tone={motionConfig.accentTone}
      data-voice-hints={shouldRenderVoiceHints ? "true" : "false"}
      data-voice-preview={voicePreview ?? undefined}
      transition={
        isDragging
          ? { damping: 22, mass: 0.52, stiffness: 340, type: "spring" }
          : { damping: 24, mass: 0.64, stiffness: 280, type: "spring" }
      }
    >
      <motion.div
        animate={{
          opacity: isDragging
            ? 0.94
            : isSettling
              ? 0.9
              : ambientLoopEnabled
                ? ambientLoopProfile.backOrbitalOpacity
                : 0.74,
          "--shell-ball-orbital-scale": isDragging
            ? 1.12
            : isSettling
              ? 1.08
              : ambientLoopEnabled
                ? ambientLoopProfile.backOrbitalScale
                : 1,
        }}
        className="shell-ball-mascot__orbital shell-ball-mascot__orbital--back"
        transition={ambientLoopEnabled ? ambientLoopTransition : { damping: 24, mass: 0.62, stiffness: 260, type: "spring" }}
      />
      <motion.div
        animate={{
          opacity: isDragging ? 0.34 : isSettling ? 0.3 : 0.24,
          "--shell-ball-shadow-offset-y": isDragging
            ? "2px"
            : ambientLoopEnabled
              ? ambientLoopProfile.shadowOffsetY.map((value) => `${value}px`)
              : "0px",
          "--shell-ball-shadow-scale-x": isDragging
            ? 1.18
            : isSettling
              ? 1.08
              : ambientLoopEnabled
                ? ambientLoopProfile.shadowScaleX
                : 1,
          "--shell-ball-shadow-scale-y": isDragging
            ? 1.12
            : isSettling
              ? 1.04
              : ambientLoopEnabled
                ? ambientLoopProfile.shadowScaleY
                : 1,
        }}
        className="shell-ball-mascot__shadow"
        transition={ambientLoopEnabled ? ambientLoopTransition : { damping: 26, mass: 0.68, stiffness: 240, type: "spring" }}
      />

      {showVoiceHoldRing ? (
        <svg aria-hidden="true" className="shell-ball-mascot__hold-ring" viewBox="0 0 190 190">
          <circle cx="95" cy="95" fill="none" r="84" stroke="rgba(255,255,255,0.28)" strokeWidth="4" />
          <circle
            cx="95"
            cy="95"
            fill="none"
            r="84"
            stroke="rgba(106,145,200,0.78)"
            strokeDasharray={holdRingCircumference}
            strokeDashoffset={holdRingDashOffset}
            strokeLinecap="round"
            strokeWidth="5"
            transform="rotate(-90 95 95)"
          />
        </svg>
      ) : null}

      {motionConfig.ringMode === "hidden" ? null : (
        <div className="shell-ball-mascot__rings" data-ring={motionConfig.ringMode}>
          <span className="shell-ball-mascot__ring shell-ball-mascot__ring--outer" />
          <span className="shell-ball-mascot__ring shell-ball-mascot__ring--inner" />
          <span className="shell-ball-mascot__ring-core">
            <AudioLines className="shell-ball-mascot__ring-icon" />
          </span>
        </div>
      )}

      <div className="shell-ball-mascot__float" style={floatStyle}>
        <motion.div
          animate={{
            rotate: motionConfig.bodyTiltDeg + dockAttitude.tiltDeg,
            scale: attitudeScale,
            x: dockAttitude.shiftX,
            y: dockAttitude.shiftY + attitudeLift,
          }}
          className="shell-ball-mascot__attitude"
          transition={
            isSettling
              ? { damping: 18, mass: 0.48, stiffness: 360, type: "spring" }
              : { damping: 24, mass: 0.64, stiffness: 260, type: "spring" }
          }
        >
          <motion.div
            animate={{
              rotate: ambientLoopEnabled ? ambientLoopProfile.tailRotate : 0,
              y: ambientLoopEnabled ? ambientLoopProfile.tailY : 0,
            }}
            className="shell-ball-mascot__tail-shell"
            style={tailStyle}
            transition={ambientLoopEnabled ? ambientLoopTransition : { damping: 24, mass: 0.62, stiffness: 250, type: "spring" }}
          >
            <div className="shell-ball-mascot__tail" />
          </motion.div>

          <div className="shell-ball-mascot__wing-shell shell-ball-mascot__wing-shell--left" style={wingStyle}>
            <div className="shell-ball-mascot__wing" data-mode={motionConfig.wingMode} data-side="left" />
          </div>
          <div className="shell-ball-mascot__wing-shell shell-ball-mascot__wing-shell--right" style={wingStyle}>
            <div className="shell-ball-mascot__wing" data-mode={motionConfig.wingMode} data-side="right" />
          </div>

          <div className="shell-ball-mascot__body-shell" style={bodyShellStyle}>
            <div className="shell-ball-mascot__crest-anchor">
              <motion.div
                animate={{
                  rotate: ambientLoopEnabled ? ambientLoopProfile.crestRotate : 0,
                  scale: ambientLoopEnabled ? ambientLoopProfile.crestScale : 1,
                  y: ambientLoopEnabled
                    ? ambientLoopProfile.crestY.map((value) => value - motionConfig.crestLiftPx)
                    : -motionConfig.crestLiftPx,
                }}
                className="shell-ball-mascot__crest"
                transition={ambientLoopEnabled ? ambientLoopTransition : { damping: 24, mass: 0.62, stiffness: 250, type: "spring" }}
              >
                <span className="shell-ball-mascot__crest-feather shell-ball-mascot__crest-feather--left" />
                <span className="shell-ball-mascot__crest-feather shell-ball-mascot__crest-feather--center" />
                <span className="shell-ball-mascot__crest-feather shell-ball-mascot__crest-feather--right" />
              </motion.div>
            </div>

            <div className="shell-ball-mascot__body">
              <div className="shell-ball-mascot__belly" />
              <div className="shell-ball-mascot__cheek shell-ball-mascot__cheek--left" />
              <div className="shell-ball-mascot__cheek shell-ball-mascot__cheek--right" />

              <div className="shell-ball-mascot__face-anchor">
                <motion.div
                  animate={{
                    rotate: ambientLoopEnabled ? ambientLoopProfile.faceRotate : 0,
                    x: ambientLoopEnabled ? ambientLoopProfile.faceX : 0,
                    y: ambientLoopEnabled ? ambientLoopProfile.faceY : 0,
                  }}
                  className="shell-ball-mascot__face"
                  transition={ambientLoopEnabled ? ambientLoopTransition : { damping: 24, mass: 0.62, stiffness: 250, type: "spring" }}
                >
                  <div className="shell-ball-mascot__eyes" data-eye={motionConfig.eyeMode} style={eyeStyle}>
                    <span className="shell-ball-mascot__eye" />
                    <span className="shell-ball-mascot__eye" />
                  </div>
                  <div className="shell-ball-mascot__beak" />
                </motion.div>
              </div>
            </div>
          </div>

          {showSelectionMarker ? (
            <div className="shell-ball-mascot__selection-marker" aria-hidden="true">
              <span className="shell-ball-mascot__selection-marker-glyph">!</span>
            </div>
          ) : null}

          {showVoiceMarker ? (
            <div className={cn("shell-ball-mascot__voice-marker", visualState === "voice_locked" && "is-locked")} aria-hidden="true">
              <Mic className="shell-ball-mascot__voice-marker-icon" />
            </div>
          ) : null}

          {showAuthMarker ? (
            <div className="shell-ball-mascot__auth-marker" aria-hidden="true">
              <ShieldAlert className="shell-ball-mascot__auth-icon" />
            </div>
          ) : null}
        </motion.div>
      </div>

      <motion.div
        animate={{
          opacity: isDragging
            ? 0.84
            : isSettling
              ? 0.8
              : ambientLoopEnabled
                ? ambientLoopProfile.frontOrbitalOpacity
                : 0.72,
          "--shell-ball-orbital-scale": isDragging
            ? 1.08
            : isSettling
              ? 1.05
              : ambientLoopEnabled
                ? ambientLoopProfile.frontOrbitalScale
                : 1,
        }}
        className="shell-ball-mascot__orbital shell-ball-mascot__orbital--front"
        transition={ambientLoopEnabled ? ambientLoopTransition : { damping: 24, mass: 0.62, stiffness: 250, type: "spring" }}
      />
      <button
        type="button"
        className="shell-ball-mascot__hotspot"
        aria-label="Shell-ball mascot"
        data-shell-ball-interactive="true"
        data-shell-ball-zone="voice-hotspot"
        onClick={handleClick}
        onDoubleClick={handleDoubleClick}
        onPointerEnter={onHotspotEnter}
        onPointerLeave={onHotspotLeave}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerEnd}
        onPointerCancel={handlePointerCancel}
      />
    </motion.div>
  );
}
