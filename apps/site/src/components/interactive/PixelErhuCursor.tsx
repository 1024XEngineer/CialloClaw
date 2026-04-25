import { useEffect, useRef, useState } from "react";
import type { ReactElement } from "react";

interface SparkPoint {
  id: number;
  x: number;
  y: number;
}

function shouldEnableCursor(): boolean {
  if (typeof window === "undefined") {
    return false;
  }

  return window.matchMedia("(pointer: fine)").matches && !window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

const pixelPattern = [
  "0001000000",
  "0001000000",
  "0011100000",
  "0011100000",
  "0001001100",
  "0001011100",
  "0001011100",
  "0001001100",
  "0001001000",
  "0001001000",
  "0011100000",
  "0110110000",
  "0100010000",
  "0100010000",
];

export function PixelErhuCursor(): ReactElement | null {
  const [enabled, setEnabled] = useState(false);
  const [hoveringAction, setHoveringAction] = useState(false);
  const [sparks, setSparks] = useState<SparkPoint[]>([]);
  const cursorRef = useRef<HTMLDivElement | null>(null);
  const positionRef = useRef({ x: 0, y: 0 });

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
      positionRef.current = { x: event.clientX, y: event.clientY };

      if (cursorRef.current) {
        cursorRef.current.style.transform = `translate3d(${event.clientX}px, ${event.clientY}px, 0)`;
      }

      const target = event.target;
      if (target instanceof HTMLElement) {
        setHoveringAction(Boolean(target.closest("a, button, summary")));
      } else {
        setHoveringAction(false);
      }
    }

    function handleDown(event: PointerEvent): void {
      const spark: SparkPoint = {
        id: Date.now() + Math.random(),
        x: event.clientX,
        y: event.clientY,
      };

      setSparks((current) => [...current, spark]);
      window.setTimeout(() => {
        setSparks((current) => current.filter((item) => item.id !== spark.id));
      }, 280);
    }

    window.addEventListener("pointermove", handleMove, { passive: true });
    window.addEventListener("pointerdown", handleDown, { passive: true });

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
        ref={cursorRef}
        aria-hidden="true"
        className="pointer-events-none fixed left-0 top-0 z-50 hidden md:block"
        style={{ willChange: "transform" }}
      >
        <div className={`origin-top-left scale-[1.05] ${hoveringAction ? "rotate-[-8deg]" : "rotate-0"}`}>
          <div className="grid grid-cols-10 gap-[2px] rounded-lg bg-slate-950/55 p-2 shadow-[0_10px_40px_-20px_rgba(0,0,0,0.9)]">
            {pixelPattern.flatMap((row, rowIndex) =>
              row.split("").map((pixel, pixelIndex) => (
                <span
                  key={`${rowIndex}-${pixelIndex}`}
                  className={`h-[4px] w-[4px] ${pixel === "1" ? "bg-cc-peach" : "bg-transparent"}`}
                />
              )),
            )}
          </div>
        </div>
      </div>

      {sparks.map((spark) => (
        <div
          key={spark.id}
          aria-hidden="true"
          className="pointer-events-none fixed left-0 top-0 z-50 hidden h-5 w-5 md:block"
          style={{
            transform: `translate(${spark.x}px, ${spark.y}px) translate(-50%, -50%)`,
            animation: "site-spark 280ms ease-out forwards",
          }}
        >
          <div className="grid grid-cols-3 gap-[2px]">
            {Array.from({ length: 9 }, (_, index) => (
              <span key={index} className={`h-[3px] w-[3px] ${index === 4 ? "bg-cc-cyan" : "bg-cc-peach"}`} />
            ))}
          </div>
        </div>
      ))}
    </>
  );
}
