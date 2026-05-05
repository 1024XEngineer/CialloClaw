import { useEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties, ReactNode } from "react";
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
  type FloatingPetBoneLayout,
  type FloatingPetCheekLayout,
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

const FLOATING_PET_BASELINE_BODY_Y = -1.06;
const FLOATING_PET_BODY_UP_Y = -7.5;
const FLOATING_PET_BODY_END_Y = -1;
const HAPPY_FACE_TIMES = [0, 10 / 120, 15 / 120, 45 / 120, 50 / 120, 1] as const;
const BREATH_EYE_TIMES = [0, 30 / 300, 40 / 300, 50 / 300, 1] as const;
const QUICK_CLAP_TIMES = [0, 0.25, 0.5, 0.75, 1] as const;
const LISTEN_EFFECT_TIMES = [0, 10 / 120, 0.5, 1] as const;
const EFFECT_LOOP_TIMES = [0, 10 / 120, 0.5, 1] as const;
const HAPPY_EFFECT_TIMES = [0, 10 / 120, 0.5, 1] as const;
const EFFECT_EXIT_TIMES = [0, 1] as const;

const MODE_TO_EFFECT: Partial<Record<FloatingPetMode, FloatingPetEffectName>> = {
  alert: "bubbleAlert",
  happy: "sparkle",
  listen: "bubbleListening",
  safe: "bubbleSafe",
  think: "bubbleThinking",
};

const EFFECT_TO_ASSET: Record<FloatingPetEffectName, string> = {
  bubbleAlert: floatingPetAssets.bubbleAlert,
  bubbleListening: floatingPetAssets.bubbleListening,
  bubbleSafe: floatingPetAssets.bubbleSafe,
  bubbleThinking: floatingPetAssets.bubbleThinking,
  sparkle: floatingPetAssets.sparkle,
};

const EFFECT_TO_LAYOUT: Record<FloatingPetEffectName, FloatingPetLayerTransform> = {
  bubbleAlert: floatingPetInitialLayout.effects.bubbleAlert,
  bubbleListening: floatingPetInitialLayout.effects.bubbleListening,
  bubbleSafe: floatingPetInitialLayout.effects.bubbleSafe,
  bubbleThinking: floatingPetInitialLayout.effects.bubbleThinking,
  sparkle: floatingPetInitialLayout.effects.sparkle,
};

function toStagePercent(value: number) {
  return `${(value / FLOATING_PET_STAGE_SIZE) * 100}%`;
}

function toScalePercent(value: number) {
  return `${value}%`;
}

function toCenterOffset(value: number) {
  if (value === 0) {
    return "50%";
  }

  const offset = toStagePercent(Math.abs(value));
  return value > 0 ? `calc(50% + ${offset})` : `calc(50% - ${offset})`;
}

function resolveStageLayerStyle(layout: FloatingPetLayerTransform): CSSProperties {
  return {
    left: toCenterOffset(layout.position.x),
    opacity: layout.opacity,
    top: toCenterOffset(layout.position.y),
  };
}

function resolveAssetLayerStyle(assetName: FloatingPetAssetName, layout: FloatingPetLayerTransform): CSSProperties {
  const asset = floatingPetAssetDimensions[assetName];

  return {
    ...resolveStageLayerStyle(layout),
    height: toStagePercent(asset.height * (layout.scale.y / 100)),
    width: toStagePercent(asset.width * (layout.scale.x / 100)),
  };
}

function resolveCheekStyle(layout: FloatingPetCheekLayout): CSSProperties {
  return {
    background: layout.fill,
    height: toStagePercent(layout.size.h * (layout.scale.y / 100)),
    left: toCenterOffset(layout.position.x),
    opacity: layout.fillOpacity,
    top: toCenterOffset(layout.position.y),
    width: toStagePercent(layout.size.w * (layout.scale.x / 100)),
  };
}

function resolveBoneTransformOrigin(assetName: FloatingPetAssetName, bone: FloatingPetBoneLayout, layer: FloatingPetLayerTransform) {
  const asset = floatingPetAssetDimensions[assetName];
  const width = asset.width * (layer.scale.x / 100);
  const height = asset.height * (layer.scale.y / 100);
  const originX = ((bone.position.x - layer.position.x) / width + 0.5) * 100;
  const originY = ((bone.position.y - layer.position.y) / height + 0.5) * 100;

  return `${originX}% ${originY}%`;
}

function resolveBodyMotion(mode: FloatingPetMode, listenLocked: boolean) {
  const restY = toStagePercent(FLOATING_PET_BASELINE_BODY_Y);

  if (mode === "happy") {
    return {
      animate: {
        rotate: [0, -4.295, 0],
        y: restY,
      },
      transition: {
        duration: FLOATING_PET_LOOP_DURATION_S,
        ease: "easeInOut" as const,
        times: [0, 0.5, 1],
      },
    };
  }

  if (mode === "listen") {
    return {
      animate: {
        rotate: listenLocked ? 0 : -4.295,
        y: restY,
      },
      transition: {
        duration: 0.45,
        ease: "easeInOut" as const,
      },
    };
  }

  return {
    animate: {
      rotate: 0,
      y: [restY, toStagePercent(FLOATING_PET_BODY_UP_Y), toStagePercent(FLOATING_PET_BODY_END_Y)],
    },
    transition: {
      duration: FLOATING_PET_LOOP_DURATION_S,
      ease: "easeInOut" as const,
      repeat: Number.POSITIVE_INFINITY,
      times: [0, 0.5, 1],
    },
  };
}

function resolveWingMotion(mode: FloatingPetMode, listenLocked: boolean) {
  if (mode === "happy") {
    return {
      left: {
        animate: [104.149, 113.022, 104.149, 115.787, 104.149],
        transition: {
          duration: FLOATING_PET_LOOP_DURATION_S,
          ease: "easeInOut" as const,
          times: QUICK_CLAP_TIMES,
        },
      },
      right: {
        animate: [63.113, 48.691, 58.465, 48.212, 63.113],
        transition: {
          duration: FLOATING_PET_LOOP_DURATION_S,
          ease: "easeInOut" as const,
          times: QUICK_CLAP_TIMES,
        },
      },
    };
  }

  if (mode === "alert" || mode === "safe" || (mode === "listen" && !listenLocked)) {
    return {
      left: {
        animate: [104.149, 113.022, 104.149, 115.787, 104.149],
        transition: {
          duration: FLOATING_PET_LOOP_DURATION_S,
          ease: "easeInOut" as const,
          repeat: Number.POSITIVE_INFINITY,
          times: QUICK_CLAP_TIMES,
        },
      },
      right: {
        animate: [63.113, 48.691, 58.465, 48.212, 63.113],
        transition: {
          duration: FLOATING_PET_LOOP_DURATION_S,
          ease: "easeInOut" as const,
          repeat: Number.POSITIVE_INFINITY,
          times: QUICK_CLAP_TIMES,
        },
      },
    };
  }

  return {
    left: {
      animate: [104.149, 106.587, 104.149],
      transition: {
        duration: FLOATING_PET_LOOP_DURATION_S,
        ease: "easeInOut" as const,
        repeat: Number.POSITIVE_INFINITY,
        times: [0, 0.5, 1],
      },
    },
    right: {
      animate: [63.113, 60.62, 63.113],
      transition: {
        duration: FLOATING_PET_LOOP_DURATION_S,
        ease: "easeInOut" as const,
        repeat: Number.POSITIVE_INFINITY,
        times: [0, 0.5, 1],
      },
    },
  };
}

function resolveTailMotion(mode: FloatingPetMode, listenLocked: boolean) {
  if (mode === "alert" || mode === "safe" || mode === "happy" || (mode === "listen" && !listenLocked)) {
    return {
      animate: [28.402, 22.019, 28.402],
      transition: {
        duration: FLOATING_PET_QUICK_TAIL_DURATION_S,
        ease: "easeInOut" as const,
        repeat: Number.POSITIVE_INFINITY,
        times: [0, 0.5, 1],
      },
    };
  }

  return {
    animate: [28.402, 24.101, 28.402],
    transition: {
      duration: FLOATING_PET_LOOP_DURATION_S,
      ease: "easeInOut" as const,
      repeat: Number.POSITIVE_INFINITY,
      times: [0, 0.5, 1],
    },
  };
}

function resolveOpenEyeAnimation(eyesClosed: boolean, mode: FloatingPetMode) {
  if (eyesClosed) {
    return {
      animate: {
        opacity: 0,
        scaleY: 1,
      },
      transition: { duration: 0.18, ease: "easeInOut" as const },
    };
  }

  if (mode === "happy") {
    return {
      animate: {
        opacity: [1, 1, 0, 0, 1, 1],
        scaleY: [1, 2 / 14.9, 2 / 14.9, 2 / 14.9, 2 / 14.9, 1],
      },
      transition: {
        duration: FLOATING_PET_LOOP_DURATION_S,
        ease: "easeInOut" as const,
        times: HAPPY_FACE_TIMES,
      },
    };
  }

  return {
    animate: {
      opacity: 1,
      scaleY: [1, 1, 5 / 14.9, 1, 1],
    },
    transition: {
      duration: FLOATING_PET_EYE_BREATH_DURATION_S,
      ease: "easeInOut" as const,
      repeat: Number.POSITIVE_INFINITY,
      times: BREATH_EYE_TIMES,
    },
  };
}

function resolveClosedEyeAnimation(eyesClosed: boolean, mode: FloatingPetMode) {
  if (eyesClosed) {
    return {
      animate: { opacity: 1, rotate: 180 },
      transition: { duration: 0.18, ease: "easeInOut" as const },
    };
  }

  if (mode === "happy") {
    return {
      animate: {
        opacity: [0, 0, 1, 1, 0, 0],
        rotate: [180, 180, 180, 180, 180, 180],
      },
      transition: {
        duration: FLOATING_PET_LOOP_DURATION_S,
        ease: "easeInOut" as const,
        times: HAPPY_FACE_TIMES,
      },
    };
  }

  return {
    animate: { opacity: 0, rotate: 180 },
    transition: { duration: 0.18, ease: "easeInOut" as const },
  };
}

function resolveBeakClosedAnimation(mode: FloatingPetMode) {
  if (mode === "happy") {
    return {
      animate: { opacity: [1, 1, 0, 0, 1, 1] },
      transition: {
        duration: FLOATING_PET_LOOP_DURATION_S,
        ease: "easeInOut" as const,
        times: HAPPY_FACE_TIMES,
      },
    };
  }

  return {
    animate: { opacity: 1 },
    transition: { duration: 0.18, ease: "easeInOut" as const },
  };
}

function resolveBeakOpenAnimation(mode: FloatingPetMode) {
  if (mode === "happy") {
    return {
      animate: { opacity: [0, 0, 1, 1, 0, 0] },
      transition: {
        duration: FLOATING_PET_LOOP_DURATION_S,
        ease: "easeInOut" as const,
        times: HAPPY_FACE_TIMES,
      },
    };
  }

  return {
    animate: { opacity: 0 },
    transition: { duration: 0.18, ease: "easeInOut" as const },
  };
}

function renderAnimatedEffect(effectName: FloatingPetEffectName, phase: "active" | "exit") {
  const layout = EFFECT_TO_LAYOUT[effectName];
  const sharedStyle = resolveStageLayerStyle(layout);

  if (effectName === "sparkle") {
    return {
      animate:
        phase === "active"
          ? {
              opacity: [0, 1, 1, 1],
              scaleX: [1, 22 / 19, 24 / 19, 22 / 19],
              scaleY: [1, 22 / 19, 24 / 19, 22 / 19],
            }
          : {
              opacity: [1, 0],
              scaleX: [22 / 19, 1],
              scaleY: [22 / 19, 1],
            },
      style: sharedStyle,
      transition:
        phase === "active"
          ? {
              duration: FLOATING_PET_LOOP_DURATION_S,
              ease: "easeInOut" as const,
              times: HAPPY_EFFECT_TIMES,
            }
          : {
              duration: FLOATING_PET_HAPPY_END_DURATION_S,
              ease: "easeInOut" as const,
              times: EFFECT_EXIT_TIMES,
            },
    };
  }

  if (effectName === "bubbleListening") {
    return {
      animate:
        phase === "active"
          ? {
              opacity: [0, 1, 1, 1],
              scaleX: [1, 1, 18 / 16.9, 1],
              scaleY: [1, 1, 18 / 16.9, 1],
            }
          : {
              opacity: [1, 0],
              scaleX: [1, 1],
              scaleY: [1, 1],
            },
      style: sharedStyle,
      transition:
        phase === "active"
          ? {
              duration: FLOATING_PET_LOOP_DURATION_S,
              ease: "easeInOut" as const,
              repeat: Number.POSITIVE_INFINITY,
              times: LISTEN_EFFECT_TIMES,
            }
          : {
              duration: FLOATING_PET_EFFECT_END_DURATION_S,
              ease: "easeInOut" as const,
              times: EFFECT_EXIT_TIMES,
            },
    };
  }

  if (effectName === "bubbleSafe") {
    return {
      animate:
        phase === "active"
          ? {
              opacity: [0, 1, 1, 1],
              scaleX: [1, 12.4 / 12.3, 14 / 12.3, 12.4 / 12.3],
              scaleY: [1, 12.4 / 12, 14 / 12, 12.4 / 12],
            }
          : {
              opacity: [1, 0],
              scaleX: [12.4 / 12.3, 12.4 / 12.3],
              scaleY: [12.4 / 12, 12.4 / 12],
            },
      style: sharedStyle,
      transition:
        phase === "active"
          ? {
              duration: FLOATING_PET_LOOP_DURATION_S,
              ease: "easeInOut" as const,
              repeat: Number.POSITIVE_INFINITY,
              times: EFFECT_LOOP_TIMES,
            }
          : {
              duration: FLOATING_PET_EFFECT_END_DURATION_S,
              ease: "easeInOut" as const,
              times: EFFECT_EXIT_TIMES,
            },
    };
  }

  if (effectName === "bubbleThinking") {
    return {
      animate:
        phase === "active"
          ? {
              opacity: [0, 1, 1, 1],
              scaleX: [1, 21.7 / 21.6, 24 / 21.6, 21.7 / 21.6],
              scaleY: [1, 21.7 / 21.6, 24 / 21.6, 21.7 / 21.6],
            }
          : {
              opacity: [1, 0],
              scaleX: [1, 1],
              scaleY: [1, 1],
            },
      style: sharedStyle,
      transition:
        phase === "active"
          ? {
              duration: FLOATING_PET_LOOP_DURATION_S,
              ease: "easeInOut" as const,
              repeat: Number.POSITIVE_INFINITY,
              times: EFFECT_LOOP_TIMES,
            }
          : {
              duration: FLOATING_PET_EFFECT_END_DURATION_S,
              ease: "easeInOut" as const,
              times: EFFECT_EXIT_TIMES,
            },
    };
  }

  return {
    animate:
      phase === "active"
        ? {
            opacity: [0, 1, 1, 1],
            scaleX: [1, 17 / 16.6, 19 / 16.6, 17 / 16.6],
            scaleY: [1, 17 / 16.6, 19 / 16.6, 17 / 16.6],
          }
        : {
            opacity: [1, 0],
            scaleX: [17 / 16.6, 17 / 16.6],
            scaleY: [17 / 16.6, 17 / 16.6],
          },
    style: sharedStyle,
    transition:
      phase === "active"
        ? {
            duration: FLOATING_PET_LOOP_DURATION_S,
            ease: "easeInOut" as const,
            repeat: Number.POSITIVE_INFINITY,
            times: EFFECT_LOOP_TIMES,
          }
        : {
            duration: FLOATING_PET_EFFECT_END_DURATION_S,
            ease: "easeInOut" as const,
            times: EFFECT_EXIT_TIMES,
          },
  };
}

function EffectLayer({ effectName, phase }: { effectName: FloatingPetEffectName; phase: "active" | "exit" }) {
  const animation = renderAnimatedEffect(effectName, phase);
  const assetName: FloatingPetAssetName = effectName === "sparkle" ? "sparkle" : effectName;

  return (
    <motion.div
      animate={animation.animate}
      className={cn(styles.layerShell, styles.effectLayer)}
      initial={false}
      style={resolveAssetLayerStyle(assetName, EFFECT_TO_LAYOUT[effectName])}
      transition={animation.transition}
    >
      <img alt="" aria-hidden="true" className={styles.asset} draggable={false} src={EFFECT_TO_ASSET[effectName]} />
    </motion.div>
  );
}

function RenderIf({ children, when }: { children: ReactNode; when: boolean }) {
  return when ? <>{children}</> : null;
}

/**
 * Recreates the desktop floating pet with the PNG layer stack and motion
 * timings extracted from the original Rive layout and timelines.
 *
 * @param props Size overrides plus the current pet mode and optional eye override.
 * @returns The animated floating pet stage.
 */
export function FloatingPet({ className, size = "100%", mode = "idle", listenLocked = false, eyesClosed = false }: FloatingPetProps) {
  const rootStyle: CSSProperties = {
    height: size,
    width: size,
  };
  const wingMotion = useMemo(() => resolveWingMotion(mode, listenLocked), [listenLocked, mode]);
  const tailMotion = useMemo(() => resolveTailMotion(mode, listenLocked), [listenLocked, mode]);
  const bodyMotion = useMemo(() => resolveBodyMotion(mode, listenLocked), [listenLocked, mode]);
  const openEyeAnimation = useMemo(() => resolveOpenEyeAnimation(eyesClosed, mode), [eyesClosed, mode]);
  const closedEyeAnimation = useMemo(() => resolveClosedEyeAnimation(eyesClosed, mode), [eyesClosed, mode]);
  const beakClosedAnimation = useMemo(() => resolveBeakClosedAnimation(mode), [mode]);
  const beakOpenAnimation = useMemo(() => resolveBeakOpenAnimation(mode), [mode]);
  const previousModeRef = useRef<FloatingPetMode>(mode);
  const exitTimeoutRef = useRef<number | null>(null);
  const [exitEffect, setExitEffect] = useState<FloatingPetEffectName | null>(null);
  const activeEffect = MODE_TO_EFFECT[mode] ?? null;
  const leftWingOrigin = resolveBoneTransformOrigin("leftWing", floatingPetInitialLayout.rootBody.leftBone, floatingPetInitialLayout.rootBody.leftWing);
  const rightWingOrigin = resolveBoneTransformOrigin("rightWing", floatingPetInitialLayout.rootBody.rightBone, floatingPetInitialLayout.rootBody.rightWing);
  const tailOrigin = resolveBoneTransformOrigin("tail", floatingPetInitialLayout.rootBody.tailBone, floatingPetInitialLayout.rootBody.tail);

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
      setExitEffect((currentEffect) => (currentEffect === previousEffect ? null : currentEffect));
      exitTimeoutRef.current = null;
    }, timeoutMs);
  }, [activeEffect, mode]);

  useEffect(() => {
    return () => {
      if (exitTimeoutRef.current !== null) {
        window.clearTimeout(exitTimeoutRef.current);
      }
    };
  }, []);

  return (
    <div className={cn(styles.root, className)} style={rootStyle}>
      <div className={styles.stage}>
        <AnimatePresence initial={false} mode="sync">
          <RenderIf when={activeEffect !== null}>
            <EffectLayer effectName={activeEffect!} phase="active" />
          </RenderIf>
          <RenderIf when={exitEffect !== null && exitEffect !== activeEffect}>
            <EffectLayer effectName={exitEffect!} phase="exit" />
          </RenderIf>
        </AnimatePresence>

        <motion.div animate={bodyMotion.animate} className={styles.rootBody} initial={false} transition={bodyMotion.transition}>
          <motion.div
            animate={{ rotate: tailMotion.animate }}
            className={cn(styles.layerShell, styles.tailLayer)}
            initial={false}
            style={{ ...resolveAssetLayerStyle("tail", floatingPetInitialLayout.rootBody.tail), transformOrigin: tailOrigin }}
            transition={tailMotion.transition}
          >
            <img alt="" aria-hidden="true" className={styles.asset} draggable={false} src={floatingPetAssets.tail} style={{ transform: `rotate(${floatingPetInitialLayout.rootBody.tail.rotation}deg)` }} />
          </motion.div>

          <motion.div
            animate={{ rotate: wingMotion.left.animate }}
            className={cn(styles.layerShell, styles.wingLayer)}
            initial={false}
            style={{ ...resolveAssetLayerStyle("leftWing", floatingPetInitialLayout.rootBody.leftWing), transformOrigin: leftWingOrigin }}
            transition={wingMotion.left.transition}
          >
            <img alt="" aria-hidden="true" className={styles.asset} draggable={false} src={floatingPetAssets.leftWing} style={{ transform: `rotate(${floatingPetInitialLayout.rootBody.leftWing.rotation}deg)` }} />
          </motion.div>

          <motion.div
            animate={{ rotate: wingMotion.right.animate }}
            className={cn(styles.layerShell, styles.wingLayer)}
            initial={false}
            style={{ ...resolveAssetLayerStyle("rightWing", floatingPetInitialLayout.rootBody.rightWing), transformOrigin: rightWingOrigin }}
            transition={wingMotion.right.transition}
          >
            <img alt="" aria-hidden="true" className={styles.asset} draggable={false} src={floatingPetAssets.rightWing} style={{ transform: `rotate(${floatingPetInitialLayout.rootBody.rightWing.rotation}deg)` }} />
          </motion.div>

          <div className={cn(styles.layerShell, styles.bodyLayer)} style={resolveAssetLayerStyle("body", floatingPetInitialLayout.rootBody.body)}>
            <img alt="" aria-hidden="true" className={styles.asset} draggable={false} src={floatingPetAssets.body} />
          </div>

          <div className={cn(styles.cheek, styles.cheekLeft)} style={resolveCheekStyle(floatingPetInitialLayout.rootBody.cheekLeft)} />
          <div className={cn(styles.cheek, styles.cheekRight)} style={resolveCheekStyle(floatingPetInitialLayout.rootBody.cheekRight)} />

          <motion.div
            animate={openEyeAnimation.animate}
            className={cn(styles.layerShell, styles.eyeLayer)}
            initial={false}
            style={resolveAssetLayerStyle("eyeOpen", floatingPetInitialLayout.rootBody.eyeOpenLeft)}
            transition={openEyeAnimation.transition}
          >
            <img alt="" aria-hidden="true" className={styles.asset} draggable={false} src={floatingPetAssets.eyeOpen} />
          </motion.div>
          <motion.div
            animate={openEyeAnimation.animate}
            className={cn(styles.layerShell, styles.eyeLayer)}
            initial={false}
            style={resolveAssetLayerStyle("eyeOpen", floatingPetInitialLayout.rootBody.eyeOpenRight)}
            transition={openEyeAnimation.transition}
          >
            <img alt="" aria-hidden="true" className={styles.asset} draggable={false} src={floatingPetAssets.eyeOpen} />
          </motion.div>

          <motion.div
            animate={closedEyeAnimation.animate}
            className={cn(styles.layerShell, styles.eyeLayer)}
            initial={false}
            style={resolveAssetLayerStyle("eyeClosed", floatingPetInitialLayout.rootBody.eyeClosedLeft)}
            transition={closedEyeAnimation.transition}
          >
            <img alt="" aria-hidden="true" className={styles.asset} draggable={false} src={floatingPetAssets.eyeClosed} />
          </motion.div>
          <motion.div
            animate={closedEyeAnimation.animate}
            className={cn(styles.layerShell, styles.eyeLayer)}
            initial={false}
            style={resolveAssetLayerStyle("eyeClosed", floatingPetInitialLayout.rootBody.eyeClosedRight)}
            transition={closedEyeAnimation.transition}
          >
            <img alt="" aria-hidden="true" className={styles.asset} draggable={false} src={floatingPetAssets.eyeClosed} />
          </motion.div>

          <motion.div
            animate={beakClosedAnimation.animate}
            className={cn(styles.layerShell, styles.beakLayer)}
            initial={false}
            style={resolveAssetLayerStyle("beakClosed", floatingPetInitialLayout.rootBody.beakClosed)}
            transition={beakClosedAnimation.transition}
          >
            <img alt="" aria-hidden="true" className={styles.asset} draggable={false} src={floatingPetAssets.beakClosed} />
          </motion.div>
          <motion.div
            animate={beakOpenAnimation.animate}
            className={cn(styles.layerShell, styles.beakLayer)}
            initial={false}
            style={resolveAssetLayerStyle("beakOpen", floatingPetInitialLayout.rootBody.beakOpen)}
            transition={beakOpenAnimation.transition}
          >
            <img alt="" aria-hidden="true" className={styles.asset} draggable={false} src={floatingPetAssets.beakOpen} />
          </motion.div>
        </motion.div>
      </div>
    </div>
  );
}
