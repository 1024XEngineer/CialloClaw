import type { ReactElement } from "react";
import { motion } from "motion/react";

const signals = ["click", "hover", "drag", "voice hold"];

export function FloatingBallDemo(): ReactElement {
  return (
    <div className="glass-card rounded-[2rem] p-6">
      <p className="text-xs uppercase tracking-[0.24em] text-cc-cyan">Floating ball demo</p>
      <div className="mt-6 flex flex-col gap-6 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex justify-center lg:w-1/2">
          <motion.div
            animate={{ y: [-6, 6, -6] }}
            transition={{ duration: 4.6, repeat: Infinity, ease: "easeInOut" }}
            className="relative flex h-36 w-36 items-center justify-center rounded-full border border-white/14 bg-linear-to-br from-cc-cyan/50 via-white/90 to-cc-violet/55 shadow-cc-glow"
          >
            <div className="absolute inset-4 rounded-full border border-white/30"></div>
            <div className="absolute inset-8 rounded-full border border-white/18"></div>
            <span className="text-sm font-semibold uppercase tracking-[0.24em] text-slate-950">task</span>
          </motion.div>
        </div>

        <div className="lg:w-1/2">
          <h3 className="font-display text-3xl font-semibold text-white">A concept demo, not a fake desktop bridge.</h3>
          <p className="mt-4 text-sm leading-7 text-white/68">This website demo illustrates the entry language of the product without pretending to control your local files, runtime, or RPC stack from the browser.</p>
          <div className="mt-5 flex flex-wrap gap-2">
            {signals.map((signal) => (
              <span key={signal} className="rounded-full border border-white/10 bg-slate-950/48 px-3 py-1.5 text-xs font-semibold uppercase tracking-[0.16em] text-white/74">
                {signal}
              </span>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
