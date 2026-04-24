import { useEffect, useState } from "react";
import type { PointerEvent as ReactPointerEvent, ReactElement } from "react";
import { motion } from "motion/react";

interface StickerPosition {
  x: number;
  y: number;
}

const STORAGE_KEY = "cc-site-sticker-position";

function loadPosition(): StickerPosition {
  if (typeof window === "undefined") {
    return { x: 0, y: 0 };
  }

  const saved = window.localStorage.getItem(STORAGE_KEY);

  if (!saved) {
    return { x: 0, y: 0 };
  }

  try {
    const parsed = JSON.parse(saved) as Partial<StickerPosition>;
    return {
      x: typeof parsed.x === "number" ? parsed.x : 0,
      y: typeof parsed.y === "number" ? parsed.y : 0,
    };
  } catch {
    return { x: 0, y: 0 };
  }
}

export function DraggableSticker(): ReactElement {
  const [position, setPosition] = useState<StickerPosition>({ x: 0, y: 0 });

  useEffect(() => {
    setPosition(loadPosition());
  }, []);

  function handleDragEnd(_event: MouseEvent | TouchEvent | PointerEvent, info: { offset: StickerPosition }): void {
    const nextPosition = {
      x: position.x + info.offset.x,
      y: position.y + info.offset.y,
    };

    setPosition(nextPosition);

    if (typeof window !== "undefined") {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(nextPosition));
    }
  }

  function resetPosition(_event: ReactPointerEvent<HTMLButtonElement>): void {
    const nextPosition = { x: 0, y: 0 };
    setPosition(nextPosition);

    if (typeof window !== "undefined") {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(nextPosition));
    }
  }

  return (
    <motion.div
      drag
      dragMomentum={false}
      onDragEnd={handleDragEnd}
      className="glass-card absolute right-4 top-30 z-20 hidden w-[18rem] cursor-grab rounded-[1.8rem] p-4 active:cursor-grabbing lg:block"
      animate={{ x: position.x, y: position.y }}
      whileDrag={{ scale: 1.03, rotate: -3 }}
      transition={{ type: "spring", stiffness: 280, damping: 24 }}
    >
      <p className="text-xs uppercase tracking-[0.24em] text-cc-cyan">Draggable sticker</p>
      <h3 className="mt-3 font-display text-2xl font-semibold text-white">Task chip</h3>
      <p className="mt-3 text-sm leading-7 text-white/68">Move this little widget around to echo the playful desktop-near interaction language requested in issue #332.</p>
      <div className="mt-4 flex flex-wrap gap-2">
        <a className="focus-ring rounded-full border border-white/10 bg-white/6 px-3 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-white/76" href="#download">
          Download
        </a>
        <a className="focus-ring rounded-full border border-white/10 bg-white/6 px-3 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-white/76" href="/docs">
          Docs
        </a>
        <button
          type="button"
          onPointerDown={resetPosition}
          className="focus-ring rounded-full border border-white/10 bg-white/6 px-3 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-white/76"
        >
          Reset
        </button>
      </div>
    </motion.div>
  );
}
