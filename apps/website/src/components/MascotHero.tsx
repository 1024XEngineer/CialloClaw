import { motion } from "motion/react";
import bodyImage from "../../../desktop/src/assets/cialloclaw-pet/body.png";
import leftWingImage from "../../../desktop/src/assets/cialloclaw-pet/left_wing.png";
import rightWingImage from "../../../desktop/src/assets/cialloclaw-pet/right_wing.png";
import bubbleImage from "../../../desktop/src/assets/cialloclaw-pet/bubble_safe.png";

// MascotHero composes the existing desktop pet assets into a lightweight website hero illustration.
export function MascotHero() {
  return (
    <div className="relative mx-auto flex h-[28rem] w-full max-w-[30rem] items-center justify-center">
      <div className="absolute inset-6 rounded-full bg-[radial-gradient(circle_at_center,rgba(255,255,255,0.95),rgba(255,251,246,0.3),transparent_72%)] blur-2xl" />
      <div className="absolute right-4 top-6 h-24 w-24 rounded-full bg-[color:var(--cc-module-memory-glow)] blur-2xl" />
      <div className="absolute bottom-8 left-2 h-28 w-28 rounded-full bg-[color:var(--cc-module-task-glow)] blur-2xl" />

      <motion.img
        animate={{ rotate: [-8, 0, -8], y: [0, -6, 0] }}
        transition={{ duration: 5.2, repeat: Number.POSITIVE_INFINITY, ease: "easeInOut" }}
        src={leftWingImage}
        alt=""
        className="absolute left-[11%] top-[34%] w-28 opacity-90"
      />
      <motion.img
        animate={{ rotate: [8, 0, 8], y: [0, -4, 0] }}
        transition={{ duration: 5.2, repeat: Number.POSITIVE_INFINITY, ease: "easeInOut", delay: 0.2 }}
        src={rightWingImage}
        alt=""
        className="absolute right-[10%] top-[36%] w-28 opacity-90"
      />
      <motion.img
        animate={{ y: [0, -10, 0], rotate: [-1.5, 1.5, -1.5] }}
        transition={{ duration: 5.8, repeat: Number.POSITIVE_INFINITY, ease: "easeInOut" }}
        src={bodyImage}
        alt="CialloClaw mascot"
        className="relative z-10 w-[16rem] drop-shadow-[0_28px_55px_rgba(91,66,42,0.18)]"
      />
      <motion.img
        animate={{ y: [0, -8, 0], x: [0, 4, 0] }}
        transition={{ duration: 4.6, repeat: Number.POSITIVE_INFINITY, ease: "easeInOut" }}
        src={bubbleImage}
        alt=""
        className="absolute right-[8%] top-[10%] z-20 w-28"
      />
    </div>
  );
}
