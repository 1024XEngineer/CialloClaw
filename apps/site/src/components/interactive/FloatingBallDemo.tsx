import type { ReactElement } from "react";
import { motion } from "motion/react";

const signals = ["选中文本", "拖入文件", "当前报错", "按住说话"];

export function FloatingBallDemo(): ReactElement {
  return (
    <div className="site-soft-card rounded-[2rem] p-6">
      <p className="text-xs uppercase tracking-[0.24em] text-cc-cyan">悬浮球入口</p>
      <div className="mt-6 flex flex-col gap-6 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex justify-center lg:w-[44%]">
          <motion.div
            animate={{ y: [-5, 8, -5], scale: [1, 1.02, 1] }}
            transition={{ duration: 4.8, repeat: Infinity, ease: "easeInOut" }}
            className="relative flex h-[8.5rem] w-[8.5rem] items-center justify-center rounded-full border border-white/18 bg-linear-to-br from-[#8fe8ff] via-[#fff7e8] to-[#ff9ac2] shadow-[0_26px_70px_-36px_rgba(255,132,179,0.9)]"
          >
            <div className="absolute inset-4 rounded-full border border-white/36"></div>
            <div className="absolute inset-9 rounded-full border border-white/16"></div>
            <span className="text-sm font-semibold uppercase tracking-[0.24em] text-slate-950">task</span>
          </motion.div>
        </div>

        <div className="lg:w-[56%]">
          <h3 className="font-display text-3xl font-semibold text-white">从身边开始，不从空白聊天页开始。</h3>
          <p className="mt-4 text-sm leading-7 text-white/68">
            这个演示只负责说明产品入口语言：离当前现场更近、确认更轻、打断更少。
            它不会假装网页可以直接接管本地 runtime。
          </p>
          <div className="mt-5 flex flex-wrap gap-2">
            {signals.map((signal) => (
              <span key={signal} className="site-chip px-3 py-1.5 text-xs font-semibold text-white/78">
                {signal}
              </span>
            ))}
          </div>
          <div className="mt-5 rounded-[1.35rem] border border-white/8 bg-white/6 p-4 text-sm leading-7 text-white/62">
            首页只展示入口感受；真正的任务进度、风险和结果，会回到各自的正式界面里。
          </div>
        </div>
      </div>
    </div>
  );
}
