import { useMemo, useState } from "react";
import type { ReactElement } from "react";

const mascotLines = [
  "Ciallo。先从你眼前的任务开始，而不是从一个空白 Prompt 框开始。",
  "把文件拖到悬浮球上，我会先等你确认，再决定接下来怎么做。",
  "高风险动作应该先问你。授权和恢复不是隐藏细节，而是正式故事的一部分。",
  "Dashboard 是你查看进度、信任状态和 artifact 的地方，不会把任务线索打断。",
];

export function MascotIsland(): ReactElement {
  const [index, setIndex] = useState(0);
  const line = useMemo(() => mascotLines[index % mascotLines.length], [index]);

  return (
    <aside className="fixed bottom-5 right-5 z-20 hidden w-[18rem] md:block lg:bottom-8 lg:right-8 lg:w-[20rem]">
      <div className="glass-card rounded-[1.8rem] p-4 shadow-cc-glow">
        <div className="flex items-start gap-4">
          <button
            type="button"
            aria-label="吉祥物提示"
            onClick={() => setIndex((current) => current + 1)}
            className="focus-ring relative flex h-20 w-20 shrink-0 items-center justify-center rounded-[1.6rem] border border-white/12 bg-linear-to-br from-cc-violet/55 via-slate-950/85 to-cc-cyan/55 transition hover:-translate-y-1"
          >
            <span className="absolute inset-x-4 bottom-4 h-3 rounded-full bg-white/16 blur-md"></span>
            <span className="text-3xl">=^.^=</span>
          </button>

          <div>
            <p className="text-xs uppercase tracking-[0.24em] text-cc-peach">吉祥物岛</p>
            <p className="mt-3 text-sm leading-7 text-white/72">{line}</p>
            <button
              type="button"
              onClick={() => setIndex((current) => current + 1)}
              className="focus-ring mt-4 inline-flex rounded-full border border-white/10 bg-white/6 px-3 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-white/76 transition hover:bg-white/10 hover:text-white"
            >
              下一句
            </button>
          </div>
        </div>
      </div>
    </aside>
  );
}
