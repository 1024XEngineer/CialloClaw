import { useEffect, useState } from "react";
import type { PointerEvent as ReactPointerEvent, ReactElement } from "react";

interface CursorPoint {
  x: number;
  y: number;
}

interface SparkPoint extends CursorPoint {
  id: number;
}

function shouldEnableCursor(): boolean {
  if (typeof window === "undefined") {
    return false;
  }

  return window.matchMedia("(pointer: fine)").matches && !window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

export function CuteCursor(): ReactElement | null {
  const [enabled, setEnabled] = useState(false);
  const [point, setPoint] = useState<CursorPoint>({ x: 0, y: 0 });
  const [hoveringAction, setHoveringAction] = useState(false);
  const [sparks, setSparks] = useState<SparkPoint[]>([]);

  useEffect(() => {
    setEnabled(shouldEnableCursor());
  }, []);

  useEffect(() => {
    if (!enabled) {
      document.body.classList.remove("has-cute-cursor");
      return;
    }

    document.body.classList.add("has-cute-cursor");

    function handleMove(event: PointerEvent): void {
      setPoint({ x: event.clientX, y: event.clientY });
      const target = event.target;

      if (target instanceof HTMLElement) {
        setHoveringAction(Boolean(target.closest("a, button, summary")));
      } else {
        setHoveringAction(false);
      }
    }

    function handleDown(event: PointerEvent): void {
      if (!(event.target instanceof HTMLElement)) {
        return;
      }

      const spark: SparkPoint = {
        id: Date.now() + Math.random(),
        x: event.clientX,
        y: event.clientY,
      };

      setSparks((current) => [...current, spark]);
      window.setTimeout(() => {
        setSparks((current) => current.filter((item) => item.id !== spark.id));
      }, 420);
    }

    window.addEventListener("pointermove", handleMove);
    window.addEventListener("pointerdown", handleDown);

    return () => {
      document.body.classList.remove("has-cute-cursor");
      window.removeEventListener("pointermove", handleMove);
      window.removeEventListener("pointerdown", handleDown);
    };
  }, [enabled]);

  if (!enabled) {
    return null;
  }

  return (
    <>
      <div
        aria-hidden="true"
        className={`pointer-events-none fixed left-0 top-0 z-50 hidden -translate-x-1/2 -translate-y-1/2 rounded-full border border-white/20 bg-white/10 backdrop-blur-sm transition-transform duration-150 md:block ${hoveringAction ? "h-14 w-14" : "h-9 w-9"}`}
        style={{ transform: `translate(${point.x}px, ${point.y}px) translate(-50%, -50%)` }}
      >
        <div className="absolute left-1/2 top-1/2 h-2.5 w-2.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-cc-peach"></div>
      </div>

      {sparks.map((spark) => (
        <div
          key={spark.id}
          aria-hidden="true"
          className="pointer-events-none fixed left-0 top-0 z-50 hidden h-10 w-10 -translate-x-1/2 -translate-y-1/2 rounded-full border border-cc-peach/60 md:block"
          style={{
            transform: `translate(${spark.x}px, ${spark.y}px) translate(-50%, -50%)`,
            animation: "site-spark 420ms ease-out forwards",
          }}
        />
      ))}
    </>
  );
}

export function ClickSparkPreview({ onTrigger }: { onTrigger?: () => void }): ReactElement {
  function handlePointerDown(_event: ReactPointerEvent<HTMLButtonElement>): void {
    onTrigger?.();
  }

  return (
    <button
      type="button"
      onPointerDown={handlePointerDown}
      className="focus-ring inline-flex items-center justify-center rounded-full border border-white/10 bg-white/6 px-4 py-2 text-sm font-semibold text-white/80 transition hover:bg-white/10 hover:text-white"
    >
      Tap a spark
    </button>
  );
}
