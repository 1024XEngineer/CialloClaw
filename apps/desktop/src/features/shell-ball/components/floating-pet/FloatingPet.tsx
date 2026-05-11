import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { AnimatePresence, motion } from "motion/react";
import { cn } from "@/utils/cn";
import styles from "./FloatingPet.module.css";
import {
  floatingPetAssetDimensions,
  FLOATING_PET_EFFECT_END_DURATION_S,
  FLOATING_PET_EYE_BREATH_DURATION_S,
  FLOATING_PET_HAPPY_END_DURATION_S,
  FLOATING_PET_LOOP_DURATION_S,
  FLOATING_PET_QUICK_TAIL_DURATION_S,
  FLOATING_PET_STAGE_SIZE,
  floatingPetAssets,
  floatingPetInitialLayout,
  type FloatingPetAssetName,
  type FloatingPetLayerTransform,
  type FloatingPetMode,
} from "./petAssets";

type FloatingPetProps = {
  className?: string;
  size?: number | string;
  mode?: FloatingPetMode;
  listenLocked?: boolean;
  eyesClosed?: boolean;
};

type FloatingPetEffectName = "sparkle" | "bubbleAlert" | "bubbleSafe" | "bubbleThinking" | "bubbleListening";
type FloatingPetBubbleAnimation = {
  initial?: {
    opacity: number;
    scale?: number;
    scaleX?: number;
    scaleY?: number;
  };
  animate:
    | {
        opacity: number[];
        scale: number[];
      }
    | {
        opacity: number[];
        scaleX: number[];
        scaleY: number[];
      };
  assetName: FloatingPetAssetName;
  duration: number;
  layout: FloatingPetLayerTransform;
  repeat: number;
  repeatType?: "loop" | "reverse" | "mirror";
  times: readonly number[];
};

type FloatingPetBubbleAnimation = {
  animate:
    | { opacity: number[]; scale: number[] }
    | { opacity: number[]; scaleX: number[]; scaleY: number[] };
  assetName: FloatingPetAssetName;
  duration: number;
  initial?:
    | { opacity: number; scale: number }
    | { opacity: number; scaleX: number; scaleY: number };
  layout: FloatingPetLayerTransform;
  repeat: number;
  repeatType: undefined;
  times: readonly number[];
};

const HAPPY_FACE_TIMES = [0, 10 / 120, 15 / 120, 105 / 120, 110 / 120, 1] as const;
const HAPPY_FACE_CLOSED_EYE_ROTATE = [180, 180, 180, 180, 180, 180] as const;
const BREATH_EYE_TIMES = [0, 30 / 300, 40 / 300, 50 / 300, 1] as const;
const QUICK_CLAP_TIMES = [0, 0.25, 0.5, 0.75, 1] as const;
const EFFECT_LOOP_TIMES = [0, 10 / 120, 0.5, 1] as const;
const LISTEN_EFFECT_TIMES = [0, 10 / 120, 0.5, 1] as const;
const ROOT_BODY_UPDOWN_Y = [248.94, 242.5, 249];
const ROOT_BODY_BASE_X = floatingPetInitialLayout.rootBody.position.x;
const ROOT_BODY_BASE_Y = floatingPetInitialLayout.rootBody.position.y;
const MODE_TO_EFFECT: Partial<Record<FloatingPetMode, FloatingPetEffectName>> = {
  alert: "bubbleAlert",
  happy: "sparkle",
  listen: "bubbleListening",
  safe: "bubbleSafe",
  think: "bubbleThinking",
};

const EFFECT_TO_ASSET: Record<FloatingPetEffectName, FloatingPetAssetName> = {
  bubbleAlert: "bubbleAlert",
  bubbleListening: "bubbleListening",
  bubbleSafe: "bubbleSafe",
  bubbleThinking: "bubbleThinking",
  sparkle: "sparkle",
};

function resolveAssetSize(assetName: FloatingPetAssetName, scale: FloatingPetLayerTransform["scale"]) {
  const asset = floatingPetAssetDimensions[assetName];

  return {
    height: asset.height * (scale.y / 100),
    width: asset.width * (scale.x / 100),
  };
}

function renderCenteredImage(assetName: FloatingPetAssetName, layout: FloatingPetLayerTransform, rotationDeg = 0) {
  const size = resolveAssetSize(assetName, layout.scale);
  const centerX = layout.position.x;
  const centerY = layout.position.y;
  const transform = rotationDeg === 0 ? undefined : `rotate(${rotationDeg} ${centerX} ${centerY})`;

  return (
    <image
      className={styles.svgAsset}
      height={size.height}
      href={floatingPetAssets[assetName]}
      preserveAspectRatio="xMidYMid meet"
      transform={transform}
      width={size.width}
      x={centerX - size.width / 2}
      y={centerY - size.height / 2}
    />
  );
}

function renderAnimatedCenteredImage(
  assetName: FloatingPetAssetName,
  layout: FloatingPetLayerTransform,
  animate: {
    opacity?: number | number[];
    rotate?: number | number[];
    scaleY?: number | number[];
  },
  transition: {
    duration: number;
    ease: "linear" | "easeInOut";
    repeat?: number;
    times?: readonly number[];
  },
) {
  const size = resolveAssetSize(assetName, layout.scale);
  const transform = layout.rotation === 0 ? undefined : `rotate(${layout.rotation} 0 0)`;

  return (
    <g transform={`translate(${layout.position.x} ${layout.position.y})`}>
      <motion.g animate={animate} initial={false} transition={transition}>
        <image
          className={styles.svgAsset}
          height={size.height}
          href={floatingPetAssets[assetName]}
          preserveAspectRatio="xMidYMid meet"
          transform={transform}
          width={size.width}
          x={-size.width / 2}
          y={-size.height / 2}
        />
      </motion.g>
    </g>
  );
}

function toPingPongKeyframes(values: readonly number[], times: readonly number[]) {
  return {
    times: [...times.map((time) => time / 2), ...times.slice(0, -1).reverse().map((time) => 1 - time / 2)],
    values: [...values, ...values.slice(0, -1).reverse()],
  };
}

function renderBubbleAnimation(effectName: FloatingPetEffectName, phase: "active" | "exit"): FloatingPetBubbleAnimation {
  const layout = effectName === "sparkle" ? floatingPetInitialLayout.sparkle : floatingPetInitialLayout.bubble.effects[effectName];
  const assetName = EFFECT_TO_ASSET[effectName];
  const alertOpacity = toPingPongKeyframes([0, 1, 1, 1], EFFECT_LOOP_TIMES);
  const alertScale = toPingPongKeyframes([1, 17 / 16.6, 19 / 16.6, 17 / 16.6], EFFECT_LOOP_TIMES);
  const listenOpacity = toPingPongKeyframes([0, 1, 1, 1], LISTEN_EFFECT_TIMES);
  const listenScale = toPingPongKeyframes([1, 1, 18 / 16.9, 1], LISTEN_EFFECT_TIMES);
  const safeOpacity = toPingPongKeyframes([0, 1, 1, 1], EFFECT_LOOP_TIMES);
  const safeScaleX = toPingPongKeyframes([1, 12.4 / 12.3, 14 / 12.3, 12.4 / 12.3], EFFECT_LOOP_TIMES);
  const safeScaleY = toPingPongKeyframes([1, 12.4 / 12, 14 / 12, 12.4 / 12], EFFECT_LOOP_TIMES);
  const thinkOpacity = toPingPongKeyframes([0, 1, 1, 1], EFFECT_LOOP_TIMES);
  const thinkScale = toPingPongKeyframes([1, 21.7 / 21.6, 24 / 21.6, 21.7 / 21.6], EFFECT_LOOP_TIMES);

  if (effectName === "sparkle") {
    return {
      initial: phase === "active" ? { opacity: 0, scale: 1 } : undefined,
      animate:
        phase === "active"
          ? { opacity: [0, 1, 1, 1], scale: [1, 22 / 19, 24 / 19, 22 / 19] }
          : { opacity: [1, 0], scale: [22 / 19, 1] },
      assetName,
      duration: phase === "active" ? FLOATING_PET_LOOP_DURATION_S : FLOATING_PET_HAPPY_END_DURATION_S,
      layout,
      repeat: phase === "active" ? 0 : 0,
      repeatType: undefined,
      times: phase === "active" ? EFFECT_LOOP_TIMES : [0, 1],
    };
  }

  if (effectName === "bubbleListening") {
    return {
      initial: phase === "active" ? { opacity: 0, scale: 1 } : undefined,
      animate:
        phase === "active"
          ? { opacity: listenOpacity.values, scale: listenScale.values }
          : { opacity: [1, 0], scale: [1, 1] },
      assetName,
      duration: phase === "active" ? FLOATING_PET_LOOP_DURATION_S * 2 : FLOATING_PET_EFFECT_END_DURATION_S,
      layout,
      repeat: phase === "active" ? Number.POSITIVE_INFINITY : 0,
      repeatType: undefined,
      times: phase === "active" ? listenOpacity.times : [0, 1],
    };
  }

  if (effectName === "bubbleSafe") {
    return {
      initial: phase === "active" ? { opacity: 0, scaleX: 1, scaleY: 1 } : undefined,
      animate:
        phase === "active"
          ? { opacity: safeOpacity.values, scaleX: safeScaleX.values, scaleY: safeScaleY.values }
          : { opacity: [1, 0], scaleX: [1, 1], scaleY: [1, 1] },
      assetName,
      duration: phase === "active" ? FLOATING_PET_LOOP_DURATION_S * 2 : FLOATING_PET_EFFECT_END_DURATION_S,
      layout,
      repeat: phase === "active" ? Number.POSITIVE_INFINITY : 0,
      repeatType: undefined,
      times: phase === "active" ? safeOpacity.times : [0, 1],
    };
  }

  if (effectName === "bubbleThinking") {
    return {
      initial: phase === "active" ? { opacity: 0, scale: 1 } : undefined,
      animate:
        phase === "active"
          ? { opacity: thinkOpacity.values, scale: thinkScale.values }
          : { opacity: [1, 0], scale: [1, 1] },
      assetName,
      duration: phase === "active" ? FLOATING_PET_LOOP_DURATION_S * 2 : FLOATING_PET_EFFECT_END_DURATION_S,
      layout,
      repeat: phase === "active" ? Number.POSITIVE_INFINITY : 0,
      repeatType: undefined,
      times: phase === "active" ? thinkOpacity.times : [0, 1],
    };
  }

  return {
    initial: phase === "active" ? { opacity: 0, scale: 1 } : undefined,
    animate:
      phase === "active"
        ? { opacity: alertOpacity.values, scale: alertScale.values }
        : { opacity: [1, 0], scale: [17 / 16.6, 17 / 16.6] },
    assetName,
    duration: phase === "active" ? FLOATING_PET_LOOP_DURATION_S * 2 : FLOATING_PET_EFFECT_END_DURATION_S,
    layout,
    repeat: phase === "active" ? Number.POSITIVE_INFINITY : 0,
    repeatType: undefined,
    times: phase === "active" ? alertOpacity.times : [0, 1],
  };
}

function FloatingPetEffectLayer({ effectName, phase }: { effectName: FloatingPetEffectName; phase: "active" | "exit" }) {
  const animation = renderBubbleAnimation(effectName, phase);
  const groupPosition = effectName === "sparkle" ? null : floatingPetInitialLayout.bubble.position;

  return (
    <motion.g
      animate={animation.animate}
      initial={animation.initial ?? false}
      transition={{
        duration: animation.duration,
        ease: "easeInOut",
        repeat: animation.repeat,
        repeatType: animation.repeatType,
        times: animation.times,
      }}
    >
      {groupPosition !== null ? (
        <g transform={`translate(${groupPosition.x} ${groupPosition.y})`}>
          {renderCenteredImage(animation.assetName, animation.layout)}
        </g>
      ) : (
        renderCenteredImage(animation.assetName, animation.layout)
      )}
    </motion.g>
  );
}

type NumericKeyframeMotion = {
  durationMs: number;
  repeat: boolean;
  times: readonly number[];
  values: readonly number[];
};

type RootBodyMotion = {
  animate: {
    rotate: number | number[];
    x: number;
    y: number | number[];
  };
  initial: {
    rotate: number;
    x: number;
    y: number;
  };
  transition: {
    duration: number;
    ease: "easeInOut";
    repeat?: number;
    times?: readonly number[];
  };
};

function resolveRootBodyMotionConfig(mode: FloatingPetMode, listenLocked: boolean, playListenTiltEnd: boolean): RootBodyMotion {
  const initial = { rotate: 0, x: ROOT_BODY_BASE_X, y: ROOT_BODY_UPDOWN_Y[0] };

  if (playListenTiltEnd) {
    return {
      animate: { rotate: [-4.295, 0], x: ROOT_BODY_BASE_X, y: ROOT_BODY_BASE_Y },
      initial,
      transition: { duration: 1, ease: "easeInOut", times: [0, 1] },
    };
  }

  if (mode === "happy") {
    return {
      animate: { rotate: [0, -4.295, 0], x: ROOT_BODY_BASE_X, y: ROOT_BODY_BASE_Y },
      initial,
      transition: { duration: FLOATING_PET_LOOP_DURATION_S, ease: "easeInOut", times: [0, 0.5, 1] },
    };
  }

  if (mode === "listen" && !listenLocked) {
    return {
      animate: { rotate: -4.295, x: ROOT_BODY_BASE_X, y: ROOT_BODY_BASE_Y },
      initial,
      transition: { duration: 0.45, ease: "easeInOut" },
    };
  }

  return {
    animate: { rotate: 0, x: ROOT_BODY_BASE_X, y: ROOT_BODY_UPDOWN_Y },
    initial,
    transition: { duration: FLOATING_PET_LOOP_DURATION_S, ease: "easeInOut", repeat: Number.POSITIVE_INFINITY, times: [0, 0.5, 1] },
  };
}

function resolveLeftWingMotion(mode: FloatingPetMode, listenLocked: boolean): NumericKeyframeMotion {
  if (mode === "happy") {
    return {
      durationMs: FLOATING_PET_LOOP_DURATION_S * 1000,
      repeat: false,
      times: QUICK_CLAP_TIMES,
      values: [0, 8.873, 0, 11.638, 0],
    };
  }

  if (mode === "alert" || mode === "safe" || (mode === "listen" && !listenLocked)) {
    return {
      durationMs: FLOATING_PET_LOOP_DURATION_S * 1000,
      repeat: true,
      times: QUICK_CLAP_TIMES,
      values: [0, 8.873, 0, 11.638, 0],
    };
  }

  return {
    durationMs: FLOATING_PET_LOOP_DURATION_S * 1000,
    repeat: true,
    times: [0, 0.5, 1],
    values: [0, 2.438, 0],
  };
}

function resolveRightWingMotion(mode: FloatingPetMode, listenLocked: boolean): NumericKeyframeMotion {
  if (mode === "happy") {
    return {
      durationMs: FLOATING_PET_LOOP_DURATION_S * 1000,
      repeat: false,
      times: QUICK_CLAP_TIMES,
      values: [0, -14.422, -4.648, -14.901, 0],
    };
  }

  if (mode === "alert" || mode === "safe" || (mode === "listen" && !listenLocked)) {
    return {
      durationMs: FLOATING_PET_LOOP_DURATION_S * 1000,
      repeat: true,
      times: QUICK_CLAP_TIMES,
      values: [0, -14.422, -4.648, -14.901, 0],
    };
  }

  return {
    durationMs: FLOATING_PET_LOOP_DURATION_S * 1000,
    repeat: true,
    times: [0, 0.5, 1],
    values: [0, -2.493, 0],
  };
}

function resolveTailMotion(mode: FloatingPetMode, listenLocked: boolean): NumericKeyframeMotion {
  if (mode === "alert" || mode === "safe" || mode === "happy" || (mode === "listen" && !listenLocked)) {
    return {
      durationMs: FLOATING_PET_QUICK_TAIL_DURATION_S * 1000,
      repeat: true,
      times: [0, 0.5, 1],
      values: [0, -6.383, 0],
    };
  }

  return {
    durationMs: FLOATING_PET_LOOP_DURATION_S * 1000,
    repeat: true,
    times: [0, 0.5, 1],
    values: [0, -4.301, 0],
  };
}

function interpolateKeyframeValue(values: number[], times: number[], progress: number) {
  if (values.length === 0) {
    return 0;
  }

  if (values.length === 1 || times.length <= 1) {
    return values[0] ?? 0;
  }

  if (progress <= times[0]) {
    return values[0] ?? 0;
  }

  for (let index = 1; index < times.length; index += 1) {
    const endTime = times[index] ?? 1;

    if (progress <= endTime) {
      const startTime = times[index - 1] ?? 0;
      const startValue = values[index - 1] ?? 0;
      const endValue = values[index] ?? startValue;
      const segmentProgress = endTime === startTime ? 0 : (progress - startTime) / (endTime - startTime);

      return startValue + (endValue - startValue) * segmentProgress;
    }
  }

  return values[values.length - 1] ?? 0;
}

function useKeyframedMotionValue(motion: NumericKeyframeMotion) {
  const [value, setValue] = useState(motion.values[0] ?? 0);

  useEffect(() => {
    setValue(motion.values[0] ?? 0);

    if (motion.values.length <= 1) {
      return;
    }

    let frame = 0;
    const startTime = performance.now();

    const tick = (time: number) => {
      const elapsed = time - startTime;
      const progress = motion.repeat ? (elapsed % motion.durationMs) / motion.durationMs : Math.min(elapsed / motion.durationMs, 1);

      setValue(interpolateKeyframeValue([...motion.values], [...motion.times], progress));

      if (motion.repeat || progress < 1) {
        frame = window.requestAnimationFrame(tick);
      }
    };

    frame = window.requestAnimationFrame(tick);

    return () => {
      window.cancelAnimationFrame(frame);
    };
  }, [motion.durationMs, motion.repeat, motion.times, motion.values]);

  return value;
}

function useRootBodyMotion(mode: FloatingPetMode, listenLocked: boolean) {
  const previousModeRef = useRef<FloatingPetMode>(mode);
  const previousListenLockedRef = useRef(listenLocked);
  const timeoutRef = useRef<number | null>(null);
  const [playListenTiltEnd, setPlayListenTiltEnd] = useState(false);

  useLayoutEffect(() => {
    const previousMode = previousModeRef.current;
    const previousListenLocked = previousListenLockedRef.current;

    previousModeRef.current = mode;
    previousListenLockedRef.current = listenLocked;

    if (timeoutRef.current !== null) {
      window.clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }

    setPlayListenTiltEnd(false);

    // Body decides locally whether leaving the unlocked listen pose requires a
    // single tilt_end before returning to the shared bobbing motion.
    if (previousMode === "listen" && previousListenLocked === false && (mode === "idle" || (mode === "listen" && listenLocked))) {
      setPlayListenTiltEnd(true);
      timeoutRef.current = window.setTimeout(() => {
        setPlayListenTiltEnd(false);
        timeoutRef.current = null;
      }, 1_000);
    }
  }, [listenLocked, mode]);

  useEffect(() => () => {
    if (timeoutRef.current !== null) {
      window.clearTimeout(timeoutRef.current);
    }
  }, []);

  return useMemo(() => resolveRootBodyMotionConfig(mode, listenLocked, playListenTiltEnd), [listenLocked, mode, playListenTiltEnd]);
}

function useFloatingPetEffectState(mode: FloatingPetMode) {
  const activeEffectRef = useRef<FloatingPetEffectName | null>(MODE_TO_EFFECT[mode] ?? null);
  const timeoutRef = useRef<number | null>(null);
  const [activeEffect, setActiveEffect] = useState<FloatingPetEffectName | null>(MODE_TO_EFFECT[mode] ?? null);
  const [exitEffect, setExitEffect] = useState<FloatingPetEffectName | null>(null);
  const requestedEffect = MODE_TO_EFFECT[mode] ?? null;

  useEffect(() => {
    if (timeoutRef.current !== null) {
      window.clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }

    const previousEffect = activeEffectRef.current;
    if (previousEffect === requestedEffect) {
      setActiveEffect(requestedEffect);
      setExitEffect(null);
      return;
    }

    if (previousEffect === null) {
      activeEffectRef.current = requestedEffect;
      setActiveEffect(requestedEffect);
      setExitEffect(null);
      return;
    }

    setActiveEffect(null);
    setExitEffect(previousEffect);
    const timeoutMs = previousEffect === "sparkle" ? FLOATING_PET_HAPPY_END_DURATION_S * 1000 : FLOATING_PET_EFFECT_END_DURATION_S * 1000;
    timeoutRef.current = window.setTimeout(() => {
      setExitEffect((current) => (current === previousEffect ? null : current));
      activeEffectRef.current = requestedEffect;
      setActiveEffect(requestedEffect);
      timeoutRef.current = null;
    }, timeoutMs);
  }, [requestedEffect]);

  useEffect(() => () => {
    if (timeoutRef.current !== null) {
      window.clearTimeout(timeoutRef.current);
    }
  }, []);

  return { activeEffect, exitEffect };
}

function useLeftWingAngle(mode: FloatingPetMode, listenLocked: boolean) {
  const motion = useMemo(() => resolveLeftWingMotion(mode, listenLocked), [listenLocked, mode]);
  return useKeyframedMotionValue(motion);
}

function useRightWingAngle(mode: FloatingPetMode, listenLocked: boolean) {
  const motion = useMemo(() => resolveRightWingMotion(mode, listenLocked), [listenLocked, mode]);
  return useKeyframedMotionValue(motion);
}

function useTailAngle(mode: FloatingPetMode, listenLocked: boolean) {
  const motion = useMemo(() => resolveTailMotion(mode, listenLocked), [listenLocked, mode]);
  return useKeyframedMotionValue(motion);
}


function resolveOpenEyeAnimation(eyesClosed: boolean, mode: FloatingPetMode) {
  if (eyesClosed) {
    return { opacity: 0, scaleY: 1, times: [0, 1], duration: 0.18, repeat: 0 };
  }

  if (mode === "happy") {
    return { opacity: [1, 1, 0, 0, 1, 1], scaleY: [1, 2 / 14.9, 2 / 14.9, 2 / 14.9, 2 / 14.9, 1], times: HAPPY_FACE_TIMES, duration: FLOATING_PET_LOOP_DURATION_S, repeat: 0 };
  }

  return { opacity: 1, scaleY: [1, 1, 5 / 14.9, 1, 1], times: BREATH_EYE_TIMES, duration: FLOATING_PET_EYE_BREATH_DURATION_S, repeat: Number.POSITIVE_INFINITY };
}

function resolveClosedEyeAnimation(eyesClosed: boolean, mode: FloatingPetMode) {
  if (eyesClosed) {
    return { opacity: 1, rotate: 180, times: [0, 1], duration: 0.18, repeat: 0 };
  }

  if (mode === "happy") {
    return { opacity: [0, 0, 1, 1, 0, 0], rotate: 180, times: HAPPY_FACE_TIMES, duration: FLOATING_PET_LOOP_DURATION_S, repeat: 0 };
  }

  return { opacity: 0, rotate: 180, times: [0, 1], duration: 0.18, repeat: 0 };
}

function resolveBeakAnimation(mode: FloatingPetMode) {
  if (mode === "happy") {
    return {
      closed: { opacity: [1, 1, 0, 0, 1, 1], duration: FLOATING_PET_LOOP_DURATION_S, times: HAPPY_FACE_TIMES },
      open: { opacity: [0, 0, 1, 1, 0, 0], duration: FLOATING_PET_LOOP_DURATION_S, times: HAPPY_FACE_TIMES },
    };
  }

  return {
    closed: { opacity: 1, duration: 0.18, times: [0, 1] },
    open: { opacity: 0, duration: 0.18, times: [0, 1] },
  };
}

/**
 * Recreates the floating pet with nested SVG groups so every local x/y uses the
 * same parent-space rules as the original Rive hierarchy.
 */
export function FloatingPet({ className, size = "100%", mode = "idle", listenLocked = false, eyesClosed = false }: FloatingPetProps) {
  const rootStyle: CSSProperties = { height: size, width: size };
  const { activeEffect, exitEffect } = useFloatingPetEffectState(mode);
  const rootBodyMotion = useRootBodyMotion(mode, listenLocked);
  const leftWingAngle = useLeftWingAngle(mode, listenLocked);
  const rightWingAngle = useRightWingAngle(mode, listenLocked);
  const tailAngle = useTailAngle(mode, listenLocked);
  const openEyeAnimation = useMemo(() => resolveOpenEyeAnimation(eyesClosed, mode), [eyesClosed, mode]);
  const closedEyeAnimation = useMemo(() => resolveClosedEyeAnimation(eyesClosed, mode), [eyesClosed, mode]);
  const beakAnimation = useMemo(() => resolveBeakAnimation(mode), [mode]);

  return (
    <div className={cn(styles.root, className)} style={rootStyle}>
      <motion.svg className={styles.svgStage} initial={false} viewBox={`0 0 ${FLOATING_PET_STAGE_SIZE} ${FLOATING_PET_STAGE_SIZE}`} xmlns="http://www.w3.org/2000/svg">
        <AnimatePresence initial={false} mode="sync">
          {activeEffect ? <FloatingPetEffectLayer effectName={activeEffect} key={`active-${activeEffect}`} phase="active" /> : null}
          {exitEffect && exitEffect !== activeEffect ? <FloatingPetEffectLayer effectName={exitEffect} key={`exit-${exitEffect}`} phase="exit" /> : null}
        </AnimatePresence>

        <motion.g animate={rootBodyMotion.animate} initial={rootBodyMotion.initial} transition={rootBodyMotion.transition}>
          <g transform={`rotate(${tailAngle} ${floatingPetInitialLayout.rootBody.tailBone.position.x} ${floatingPetInitialLayout.rootBody.tailBone.position.y})`}>
            {renderCenteredImage(
              "tail",
              floatingPetInitialLayout.rootBody.tail,
              //floatingPetInitialLayout.rootBody.tailBone.rotation + floatingPetInitialLayout.rootBody.tail.rotation
              0
            )}
          </g>

          <g transform={`rotate(${leftWingAngle} ${floatingPetInitialLayout.rootBody.leftBone.position.x} ${floatingPetInitialLayout.rootBody.leftBone.position.y})`}>
            {renderCenteredImage(
              "leftWing",
              floatingPetInitialLayout.rootBody.leftWing,
              //floatingPetInitialLayout.rootBody.leftBone.rotation + floatingPetInitialLayout.rootBody.leftWing.rotation
              2
            )}
          </g>

          {renderCenteredImage("body", floatingPetInitialLayout.rootBody.body)}

          <g transform={`rotate(${rightWingAngle} ${floatingPetInitialLayout.rootBody.rightBone.position.x} ${floatingPetInitialLayout.rootBody.rightBone.position.y})`}>
            {renderCenteredImage(
              "rightWing",
              floatingPetInitialLayout.rootBody.rightWing,
              floatingPetInitialLayout.rootBody.rightBone.rotation + floatingPetInitialLayout.rootBody.rightWing.rotation
              //0
            )}
          </g>

          <g transform={`translate(${floatingPetInitialLayout.rootBody.face.position.x} ${floatingPetInitialLayout.rootBody.face.position.y})`}>
            <g transform={`translate(${floatingPetInitialLayout.rootBody.cheek.position.x} ${floatingPetInitialLayout.rootBody.cheek.position.y})`}>
              <ellipse
                className={styles.cheek}
                cx={floatingPetInitialLayout.rootBody.cheek.cheekLeft.position.x}
                cy={floatingPetInitialLayout.rootBody.cheek.cheekLeft.position.y}
                fill={floatingPetInitialLayout.rootBody.cheek.cheekLeft.fill}
                fillOpacity={floatingPetInitialLayout.rootBody.cheek.cheekLeft.fillOpacity}
                rx={(floatingPetInitialLayout.rootBody.cheek.cheekLeft.size.w * (floatingPetInitialLayout.rootBody.cheek.cheekLeft.scale.x / 100)) / 2}
                ry={(floatingPetInitialLayout.rootBody.cheek.cheekLeft.size.h * (floatingPetInitialLayout.rootBody.cheek.cheekLeft.scale.y / 100)) / 2}
              />
              <ellipse
                className={styles.cheek}
                cx={floatingPetInitialLayout.rootBody.cheek.cheekRight.position.x}
                cy={floatingPetInitialLayout.rootBody.cheek.cheekRight.position.y}
                fill={floatingPetInitialLayout.rootBody.cheek.cheekRight.fill}
                fillOpacity={floatingPetInitialLayout.rootBody.cheek.cheekRight.fillOpacity}
                rx={(floatingPetInitialLayout.rootBody.cheek.cheekRight.size.w * (floatingPetInitialLayout.rootBody.cheek.cheekRight.scale.x / 100)) / 2}
                ry={(floatingPetInitialLayout.rootBody.cheek.cheekRight.size.h * (floatingPetInitialLayout.rootBody.cheek.cheekRight.scale.y / 100)) / 2}
              />
            </g>

            <g transform={`translate(${floatingPetInitialLayout.rootBody.eyes.position.x} ${floatingPetInitialLayout.rootBody.eyes.position.y})`}>
              {mode === "happy" ? (
                <>
                  {renderAnimatedCenteredImage(
                    "eyeOpen",
                    floatingPetInitialLayout.rootBody.eyes.eyeOpenLeft,
                    { opacity: openEyeAnimation.opacity, scaleY: openEyeAnimation.scaleY },
                    { duration: openEyeAnimation.duration, ease: "linear", repeat: openEyeAnimation.repeat, times: openEyeAnimation.times },
                  )}
                  {renderAnimatedCenteredImage(
                    "eyeOpen",
                    floatingPetInitialLayout.rootBody.eyes.eyeOpenRight,
                    { opacity: openEyeAnimation.opacity, scaleY: openEyeAnimation.scaleY },
                    { duration: openEyeAnimation.duration, ease: "linear", repeat: openEyeAnimation.repeat, times: openEyeAnimation.times },
                  )}
                  {renderAnimatedCenteredImage(
                    "eyeClosed",
                    floatingPetInitialLayout.rootBody.eyes.eyeClosedLeft,
                    { opacity: closedEyeAnimation.opacity, rotate: [...HAPPY_FACE_CLOSED_EYE_ROTATE] },
                    { duration: closedEyeAnimation.duration, ease: "linear", repeat: closedEyeAnimation.repeat, times: closedEyeAnimation.times },
                  )}
                  {renderAnimatedCenteredImage(
                    "eyeClosed",
                    floatingPetInitialLayout.rootBody.eyes.eyeClosedRight,
                    { opacity: closedEyeAnimation.opacity, rotate: [...HAPPY_FACE_CLOSED_EYE_ROTATE] },
                    { duration: closedEyeAnimation.duration, ease: "linear", repeat: closedEyeAnimation.repeat, times: closedEyeAnimation.times },
                  )}
                </>
              ) : (
                <>
                  <motion.g animate={{ opacity: openEyeAnimation.opacity, scaleY: openEyeAnimation.scaleY }} initial={false} transition={{ duration: openEyeAnimation.duration, ease: "easeInOut", repeat: openEyeAnimation.repeat, times: openEyeAnimation.times }}>
                    {renderCenteredImage("eyeOpen", floatingPetInitialLayout.rootBody.eyes.eyeOpenLeft)}
                    {renderCenteredImage("eyeOpen", floatingPetInitialLayout.rootBody.eyes.eyeOpenRight)}
                  </motion.g>
                  <motion.g animate={{ opacity: closedEyeAnimation.opacity, rotate: closedEyeAnimation.rotate }} initial={false} transition={{ duration: closedEyeAnimation.duration, ease: "easeInOut", repeat: closedEyeAnimation.repeat, times: closedEyeAnimation.times }}>
                    {renderCenteredImage("eyeClosed", floatingPetInitialLayout.rootBody.eyes.eyeClosedLeft)}
                    {renderCenteredImage("eyeClosed", floatingPetInitialLayout.rootBody.eyes.eyeClosedRight)}
                  </motion.g>
                </>
              )}
            </g>

            <g transform={`translate(${floatingPetInitialLayout.rootBody.beak.position.x} ${floatingPetInitialLayout.rootBody.beak.position.y})`}>
              {mode === "happy" ? (
                <>
                  {renderAnimatedCenteredImage(
                    "beakClosed",
                    floatingPetInitialLayout.rootBody.beak.beakClosed,
                    { opacity: beakAnimation.closed.opacity },
                    { duration: beakAnimation.closed.duration, ease: "linear", times: beakAnimation.closed.times },
                  )}
                  {renderAnimatedCenteredImage(
                    "beakOpen",
                    floatingPetInitialLayout.rootBody.beak.beakOpen,
                    { opacity: beakAnimation.open.opacity },
                    { duration: beakAnimation.open.duration, ease: "linear", times: beakAnimation.open.times },
                  )}
                </>
              ) : (
                <>
                  <motion.g animate={{ opacity: beakAnimation.closed.opacity }} initial={false} transition={{ duration: beakAnimation.closed.duration, ease: "easeInOut", times: beakAnimation.closed.times }}>
                    {renderCenteredImage("beakClosed", floatingPetInitialLayout.rootBody.beak.beakClosed)}
                  </motion.g>
                  <motion.g animate={{ opacity: beakAnimation.open.opacity }} initial={false} transition={{ duration: beakAnimation.open.duration, ease: "easeInOut", times: beakAnimation.open.times }}>
                    {renderCenteredImage("beakOpen", floatingPetInitialLayout.rootBody.beak.beakOpen)}
                  </motion.g>
                </>
              )}
            </g>
          </g>
        </motion.g>
      </motion.svg>
    </div>
  );
}
