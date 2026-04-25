import type { ReactElement } from "react";
import { motion } from "motion/react";

export function MascotIsland(): ReactElement {
  return (
    <div className="site-soft-card rounded-[2rem] p-6">
      <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
        <div className="relative flex h-[11.5rem] w-full shrink-0 items-center justify-center overflow-hidden rounded-[1.8rem] bg-[radial-gradient(circle_at_30%_20%,rgba(255,154,194,0.24),transparent_34%),radial-gradient(circle_at_75%_72%,rgba(145,230,255,0.18),transparent_35%)] sm:w-[12rem]">
          <motion.div
            className="relative h-[8.5rem] w-[8.5rem]"
            animate={{ y: [0, -8, 0], rotate: [0, 2, 0, -2, 0] }}
            transition={{ duration: 5.2, repeat: Infinity, ease: "easeInOut" }}
          >
            <div className="absolute inset-x-4 top-4 h-3 rounded-full bg-cc-peach/70" />
            <div className="absolute left-1/2 top-6 h-20 w-3 -translate-x-1/2 rounded-full bg-[#7a4b30]" />
            <div className="absolute left-1/2 top-10 h-6 w-10 -translate-x-1/2 rounded-[0.7rem] border border-[#7a4b30] bg-[#ffd06c]" />
            <div className="absolute left-1/2 top-18 h-7 w-9 -translate-x-1/2 rounded-[0.7rem] border border-[#7a4b30] bg-[#f3ebdc]" />
            <div className="absolute left-[18%] top-[28%] h-[2px] w-16 rotate-[-24deg] bg-[#fff4de]" />
            <div className="absolute right-2 top-4 text-lg text-[#ff9ac2]">♪</div>
            <div className="absolute right-1 top-10 text-sm text-cc-cyan">♫</div>
          </motion.div>
        </div>

        <div className="min-w-0 flex-1">
          <p className="text-xs uppercase tracking-[0.24em] text-cc-peach">音乐像素主题</p>
          <h3 className="mt-3 font-display text-2xl font-semibold text-white">首页这一块只是风格化装饰，不代表真实产品里存在一个网站吉祥物角色。</h3>
          <p className="mt-3 text-sm leading-7 text-white/72">实际设计重点仍然是悬浮球、任务看板和控制面板。这一块只用来补足可爱和音乐感，让官网更轻巧，而不是新增一个产品内并不存在的正式对象。</p>
        </div>
      </div>

      <div className="mt-5 rounded-[1.45rem] border border-white/8 bg-[rgba(255,255,255,0.04)] p-4 text-sm leading-7 text-white/62">
        这里保留的是风格化视觉挂件和音乐主题，不再把它写成产品里的实际吉祥物设定。
      </div>
    </div>
  );
}
