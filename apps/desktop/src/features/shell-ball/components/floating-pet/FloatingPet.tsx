import { useEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { animate } from "motion";
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

const HAPPY_FACE_TIMES = [0, 10 / 120, 15 / 120, 45 / 120, 50 / 120, 1] as const;
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

function renderBubbleAnimation(effectName: FloatingPetEffectName, phase: "active" | "exit") {
  const layout = effectName === "sparkle" ? floatingPetInitialLayout.sparkle : floatingPetInitialLayout.bubble.effects[effectName];
  const assetName = EFFECT_TO_ASSET[effectName];

  if (effectName === "sparkle") {
    return {
      animate:
        phase === "active"
          ? { opacity: [0, 1, 1, 1], scale: [1, 22 / 19, 24 / 19, 22 / 19] }
          : { opacity: [1, 0], scale: [22 / 19, 1] },
      assetName,
      duration: phase === "active" ? FLOATING_PET_LOOP_DURATION_S : FLOATING_PET_HAPPY_END_DURATION_S,
      layout,
      repeat: phase === "active" ? 0 : 0,
      times: EFFECT_LOOP_TIMES,
    };
  }

  if (effectName === "bubbleListening") {
    return {
      animate:
        phase === "active"
          ? { opacity: [0, 1, 1, 1], scale: [1, 1, 18 / 16.9, 1] }
          : { opacity: [1, 0], scale: [1, 1] },
      assetName,
      duration: phase === "active" ? FLOATING_PET_LOOP_DURATION_S : FLOATING_PET_EFFECT_END_DURATION_S,
      layout,
      repeat: phase === "active" ? Number.POSITIVE_INFINITY : 0,
      times: LISTEN_EFFECT_TIMES,
    };
  }

  if (effectName === "bubbleSafe") {
    return {
      animate:
        phase === "active"
          ? { opacity: [0, 1, 1, 1], scaleX: [1, 12.4 / 12.3, 14 / 12.3, 12.4 / 12.3], scaleY: [1, 12.4 / 12, 14 / 12, 12.4 / 12] }
          : { opacity: [1, 0], scaleX: [1, 1], scaleY: [1, 1] },
      assetName,
      duration: phase === "active" ? FLOATING_PET_LOOP_DURATION_S : FLOATING_PET_EFFECT_END_DURATION_S,
      layout,
      repeat: phase === "active" ? Number.POSITIVE_INFINITY : 0,
      times: EFFECT_LOOP_TIMES,
    };
  }

  if (effectName === "bubbleThinking") {
    return {
      animate:
        phase === "active"
          ? { opacity: [0, 1, 1, 1], scale: [1, 21.7 / 21.6, 24 / 21.6, 21.7 / 21.6] }
          : { opacity: [1, 0], scale: [1, 1] },
      assetName,
      duration: phase === "active" ? FLOATING_PET_LOOP_DURATION_S : FLOATING_PET_EFFECT_END_DURATION_S,
      layout,
      repeat: phase === "active" ? Number.POSITIVE_INFINITY : 0,
      times: EFFECT_LOOP_TIMES,
    };
  }

  return {
    animate:
      phase === "active"
        ? { opacity: [0, 1, 1, 1], scale: [1, 17 / 16.6, 19 / 16.6, 17 / 16.6] }
        : { opacity: [1, 0], scale: [17 / 16.6, 17 / 16.6] },
    assetName,
    duration: phase === "active" ? FLOATING_PET_LOOP_DURATION_S : FLOATING_PET_EFFECT_END_DURATION_S,
    layout,
    repeat: phase === "active" ? Number.POSITIVE_INFINITY : 0,
    times: EFFECT_LOOP_TIMES,
  };
}

function FloatingPetEffectLayer({ effectName, phase }: { effectName: FloatingPetEffectName; phase: "active" | "exit" }) {
  const animation = renderBubbleAnimation(effectName, phase);
  const groupPosition = effectName === "sparkle" ? null : floatingPetInitialLayout.bubble.position;

  return (
    <motion.g
      animate={animation.animate}
      initial={false}
      transition={{
        duration: animation.duration,
        ease: "easeInOut",
        repeat: animation.repeat,
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

function resolveRootBodyMotion(mode: FloatingPetMode, listenLocked: boolean) {
  if (mode === "happy") {
    return {
      animate: { rotate: [0, -4.295, 0], x: ROOT_BODY_BASE_X, y: ROOT_BODY_BASE_Y },
      transition: { duration: FLOATING_PET_LOOP_DURATION_S, ease: "easeInOut", times: [0, 0.5, 1] },
    };
  }

  if (mode === "listen") {
    return {
      animate: { rotate: listenLocked ? 0 : -4.295, x: ROOT_BODY_BASE_X, y: ROOT_BODY_BASE_Y },
      transition: { duration: 0.45, ease: "easeInOut" },
    };
  }

  return {
    animate: { rotate: 0, x: ROOT_BODY_BASE_X, y: ROOT_BODY_UPDOWN_Y },
    transition: { duration: FLOATING_PET_LOOP_DURATION_S, ease: "easeInOut", repeat: Number.POSITIVE_INFINITY, times: [0, 0.5, 1] },
  };
}

function resolveWingMotion(mode: FloatingPetMode, listenLocked: boolean) {
  if (mode === "happy") {
    return {
      left: { rotate: [104.149, 113.022, 104.149, 115.787, 104.149], repeat: 0 },
      right: { rotate: [63.113, 48.691, 58.465, 48.212, 63.113], repeat: 0 },
    };
  }

  if (mode === "alert" || mode === "safe" || (mode === "listen" && !listenLocked)) {
    return {
      left: { rotate: [104.149, 113.022, 104.149, 115.787, 104.149], repeat: Number.POSITIVE_INFINITY },
      right: { rotate: [63.113, 48.691, 58.465, 48.212, 63.113], repeat: Number.POSITIVE_INFINITY },
    };
  }

  return {
    left: { rotate: [104.149, 106.587, 104.149], repeat: Number.POSITIVE_INFINITY },
    right: { rotate: [63.113, 60.62, 63.113], repeat: Number.POSITIVE_INFINITY },
  };
}

function resolveTailMotion(mode: FloatingPetMode, listenLocked: boolean) {
  if (mode === "alert" || mode === "safe" || mode === "happy" || (mode === "listen" && !listenLocked)) {
    return {
      duration: FLOATING_PET_QUICK_TAIL_DURATION_S,
      repeat: Number.POSITIVE_INFINITY,
      rotate: [28.402, 22.019, 28.402],
      times: [0, 0.5, 1],
    };
  }

  return {
    duration: FLOATING_PET_LOOP_DURATION_S,
    repeat: Number.POSITIVE_INFINITY,
    rotate: [28.402, 24.101, 28.402],
    times: [0, 0.5, 1],
  };
}

function renderCenteredImageAtOrigin(assetName: FloatingPetAssetName, scale: FloatingPetLayerTransform["scale"], rotationDeg = 0) {
  const size = resolveAssetSize(assetName, scale);

  return (
    <image
      className={styles.svgAsset}
      height={size.height}
      href={floatingPetAssets[assetName]}
      preserveAspectRatio="xMidYMid meet"
      transform={rotationDeg === 0 ? undefined : `rotate(${rotationDeg} 0 0)`}
      width={size.width}
      x={-size.width / 2}
      y={-size.height / 2}
    />
  );
}

function resolveLocalPivot(centerPosition: FloatingPetLayerTransform["position"], bonePosition: FloatingPetLayerTransform["position"]) {
  return {
    x: bonePosition.x - centerPosition.x,
    y: bonePosition.y - centerPosition.y,
  };
}

type BoneRotatedLayerProps = {
  animatedRotate: number | number[];
  assetName: FloatingPetAssetName;
  baseRotate: number;
  bonePosition: FloatingPetLayerTransform["position"];
  layer: FloatingPetLayerTransform;
  repeat: number;
  times: number[];
  transitionDuration: number;
};

function BoneRotatedLayer({
  animatedRotate,
  assetName,
  baseRotate,
  bonePosition,
  layer,
  repeat,
  times,
  transitionDuration,
}: BoneRotatedLayerProps) {
  const localPivot = resolveLocalPivot(layer.position, bonePosition);
  const rawKeyframes = Array.isArray(animatedRotate) ? animatedRotate : [animatedRotate];
  const keyframes = rawKeyframes.map((value) => value - baseRotate);
  const [currentRotate, setCurrentRotate] = useState<number>(keyframes[0] ?? 0);

  useEffect(() => {
    if (keyframes.length === 0) {
      return;
    }

    setCurrentRotate(keyframes[0]);

    if (keyframes.length === 1) {
      return;
    }

    const controls = animate(keyframes[0], keyframes, {
      duration: transitionDuration,
      ease: "easeInOut",
      onUpdate: setCurrentRotate,
      repeat,
      times,
    });

    return () => {
      controls.stop();
    };
  }, [keyframes, repeat, times, transitionDuration]);

  const transform = [
    `translate(${layer.position.x} ${layer.position.y})`,
    `translate(${localPivot.x} ${localPivot.y})`,
    `rotate(${currentRotate})`,
    `translate(${-localPivot.x} ${-localPivot.y})`,
  ].join(" ");

  return (
    <g transform={transform}>
      {renderCenteredImageAtOrigin(assetName, layer.scale, layer.rotation)}
    </g>
  );
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
  const previousModeRef = useRef<FloatingPetMode>(mode);
  const exitTimeoutRef = useRef<number | null>(null);
  const [exitEffect, setExitEffect] = useState<FloatingPetEffectName | null>(null);
  const activeEffect = MODE_TO_EFFECT[mode] ?? null;
  const rootBodyMotion = useMemo(() => resolveRootBodyMotion(mode, listenLocked), [listenLocked, mode]);
  const wingMotion = useMemo(() => resolveWingMotion(mode, listenLocked), [listenLocked, mode]);
  const tailMotion = useMemo(() => resolveTailMotion(mode, listenLocked), [listenLocked, mode]);
  const openEyeAnimation = useMemo(() => resolveOpenEyeAnimation(eyesClosed, mode), [eyesClosed, mode]);
  const closedEyeAnimation = useMemo(() => resolveClosedEyeAnimation(eyesClosed, mode), [eyesClosed, mode]);
  const beakAnimation = useMemo(() => resolveBeakAnimation(mode), [mode]);

  useEffect(() => {
    const previousMode = previousModeRef.current;
    previousModeRef.current = mode;

    if (exitTimeoutRef.current !== null) {
      window.clearTimeout(exitTimeoutRef.current);
      exitTimeoutRef.current = null;
    }

    const previousEffect = MODE_TO_EFFECT[previousMode] ?? null;
    if (previousEffect === null || previousEffect === activeEffect) {
      setExitEffect(null);
      return;
    }

    setExitEffect(previousEffect);
    const timeoutMs = previousEffect === "sparkle" ? FLOATING_PET_HAPPY_END_DURATION_S * 1000 : FLOATING_PET_EFFECT_END_DURATION_S * 1000;
    exitTimeoutRef.current = window.setTimeout(() => {
      setExitEffect((current) => (current === previousEffect ? null : current));
      exitTimeoutRef.current = null;
    }, timeoutMs);
  }, [activeEffect, mode]);

  useEffect(() => () => {
    if (exitTimeoutRef.current !== null) {
      window.clearTimeout(exitTimeoutRef.current);
    }
  }, []);

  return (
    <div className={cn(styles.root, className)} style={rootStyle}>
      <motion.svg className={styles.svgStage} initial={false} viewBox={`0 0 ${FLOATING_PET_STAGE_SIZE} ${FLOATING_PET_STAGE_SIZE}`} xmlns="http://www.w3.org/2000/svg">
        <AnimatePresence initial={false} mode="sync">
          {activeEffect ? <FloatingPetEffectLayer effectName={activeEffect} key={`active-${activeEffect}`} phase="active" /> : null}
          {exitEffect && exitEffect !== activeEffect ? <FloatingPetEffectLayer effectName={exitEffect} key={`exit-${exitEffect}`} phase="exit" /> : null}
        </AnimatePresence>

        <motion.g animate={rootBodyMotion.animate} initial={false} transition={rootBodyMotion.transition}>
          <BoneRotatedLayer
            animatedRotate={tailMotion.rotate}
            assetName="tail"
            baseRotate={floatingPetInitialLayout.rootBody.tailBone.rotation}
            bonePosition={floatingPetInitialLayout.rootBody.tailBone.position}
            layer={floatingPetInitialLayout.rootBody.tail}
            repeat={tailMotion.repeat}
            times={tailMotion.times}
            transitionDuration={tailMotion.duration}
          />

          <BoneRotatedLayer
            animatedRotate={wingMotion.left.rotate}
            assetName="leftWing"
            baseRotate={floatingPetInitialLayout.rootBody.leftBone.rotation}
            bonePosition={floatingPetInitialLayout.rootBody.leftBone.position}
            layer={floatingPetInitialLayout.rootBody.leftWing}
            repeat={wingMotion.left.repeat}
            times={wingMotion.left.rotate.length === 5 ? [...QUICK_CLAP_TIMES] : [0, 0.5, 1]}
            transitionDuration={FLOATING_PET_LOOP_DURATION_S}
          />

          {renderCenteredImage("body", floatingPetInitialLayout.rootBody.body)}

          <BoneRotatedLayer
            animatedRotate={wingMotion.right.rotate}
            assetName="rightWing"
            baseRotate={floatingPetInitialLayout.rootBody.rightBone.rotation}
            bonePosition={floatingPetInitialLayout.rootBody.rightBone.position}
            layer={floatingPetInitialLayout.rootBody.rightWing}
            repeat={wingMotion.right.repeat}
            times={wingMotion.right.rotate.length === 5 ? [...QUICK_CLAP_TIMES] : [0, 0.5, 1]}
            transitionDuration={FLOATING_PET_LOOP_DURATION_S}
          />

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
              <motion.g animate={{ opacity: openEyeAnimation.opacity, scaleY: openEyeAnimation.scaleY }} initial={false} transition={{ duration: openEyeAnimation.duration, ease: "easeInOut", repeat: openEyeAnimation.repeat, times: openEyeAnimation.times }}>
                {renderCenteredImage("eyeOpen", floatingPetInitialLayout.rootBody.eyes.eyeOpenLeft)}
                {renderCenteredImage("eyeOpen", floatingPetInitialLayout.rootBody.eyes.eyeOpenRight)}
              </motion.g>
              <motion.g animate={{ opacity: closedEyeAnimation.opacity, rotate: closedEyeAnimation.rotate }} initial={false} transition={{ duration: closedEyeAnimation.duration, ease: "easeInOut", repeat: closedEyeAnimation.repeat, times: closedEyeAnimation.times }}>
                {renderCenteredImage("eyeClosed", floatingPetInitialLayout.rootBody.eyes.eyeClosedLeft)}
                {renderCenteredImage("eyeClosed", floatingPetInitialLayout.rootBody.eyes.eyeClosedRight)}
              </motion.g>
            </g>

            <g transform={`translate(${floatingPetInitialLayout.rootBody.beak.position.x} ${floatingPetInitialLayout.rootBody.beak.position.y})`}>
              <motion.g animate={{ opacity: beakAnimation.closed.opacity }} initial={false} transition={{ duration: beakAnimation.closed.duration, ease: "easeInOut", times: beakAnimation.closed.times }}>
                {renderCenteredImage("beakClosed", floatingPetInitialLayout.rootBody.beak.beakClosed)}
              </motion.g>
              <motion.g animate={{ opacity: beakAnimation.open.opacity }} initial={false} transition={{ duration: beakAnimation.open.duration, ease: "easeInOut", times: beakAnimation.open.times }}>
                {renderCenteredImage("beakOpen", floatingPetInitialLayout.rootBody.beak.beakOpen)}
              </motion.g>
            </g>
          </g>
        </motion.g>
      </motion.svg>
    </div>
  );
}
