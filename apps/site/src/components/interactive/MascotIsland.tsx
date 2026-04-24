import { useMemo, useState } from "react";
import type { ReactElement } from "react";

const mascotLines = [
  "Ciallo. Start from the task in front of you, not from a blank prompt box.",
  "Drag a file to the floating ball and I will wait for confirmation before anything starts.",
  "Risky actions should ask first. Approval and recovery are part of the story, not hidden details.",
  "Dashboard is where you check progress, trust, and artifacts without losing the thread.",
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
            aria-label="Mascot prompt"
            onClick={() => setIndex((current) => current + 1)}
            className="focus-ring relative flex h-20 w-20 shrink-0 items-center justify-center rounded-[1.6rem] border border-white/12 bg-linear-to-br from-cc-violet/55 via-slate-950/85 to-cc-cyan/55 transition hover:-translate-y-1"
          >
            <span className="absolute inset-x-4 bottom-4 h-3 rounded-full bg-white/16 blur-md"></span>
            <span className="text-3xl">=^.^=</span>
          </button>

          <div>
            <p className="text-xs uppercase tracking-[0.24em] text-cc-peach">Mascot island</p>
            <p className="mt-3 text-sm leading-7 text-white/72">{line}</p>
            <button
              type="button"
              onClick={() => setIndex((current) => current + 1)}
              className="focus-ring mt-4 inline-flex rounded-full border border-white/10 bg-white/6 px-3 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-white/76 transition hover:bg-white/10 hover:text-white"
            >
              Next line
            </button>
          </div>
        </div>
      </div>
    </aside>
  );
}
