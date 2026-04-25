import { useEffect, useRef, useState } from "react";
import type { ReactElement } from "react";

interface SparkPoint {
  id: number;
  x: number;
  y: number;
  size: number;
  color: string;
  symbol: "." | "square" | "note" | "beam";
}

function shouldEnableCursor(): boolean {
  if (typeof window === "undefined") {
    return false;
  }

  return window.matchMedia("(pointer: fine)").matches && !window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

function CursorArtwork({ hoveringAction }: { hoveringAction: boolean }): ReactElement {
  return (
    <svg
      viewBox="0 0 72 88"
      aria-hidden="true"
      className={`h-[58px] w-[46px] ${hoveringAction ? "rotate-[4deg] scale-105" : "rotate-0 scale-100"}`}
      style={{ filter: "drop-shadow(0 14px 24px rgba(0, 0, 0, 0.35))" }}
    >
      <g transform="translate(10 6)">
        <rect x="20" y="0" width="4" height="58" rx="2" fill="#7a4b30" />
        <rect x="28" y="10" width="3" height="46" rx="1.5" fill="#c78c64" opacity="0.8" />
        <rect x="24" y="12" width="16" height="18" rx="4" fill="#ffcf6a" stroke="#7a4b30" strokeWidth="2" />
        <rect x="24" y="32" width="14" height="17" rx="4" fill="#f5e9d3" stroke="#7a4b30" strokeWidth="2" />
        <path d="M7 24L44 6" stroke="#fff4de" strokeWidth="2.4" strokeLinecap="round" />
        <path d="M6 29L45 11" stroke="#ffd38a" strokeWidth="1.6" strokeLinecap="round" opacity="0.85" />
        <circle cx="47" cy="11" r="4" fill="#ff9ac2" />
        <path d="M47 7V1" stroke="#ff9ac2" strokeWidth="2.2" strokeLinecap="round" />
        <path d="M47 1C50 2 53 5 53 8" fill="none" stroke="#ff9ac2" strokeWidth="2.2" strokeLinecap="round" />
        <path d="M22 58L17 73" stroke="#7a4b30" strokeWidth="4" strokeLinecap="round" />
        <path d="M22 58L27 73" stroke="#7a4b30" strokeWidth="4" strokeLinecap="round" />
        <rect x="19" y="54" width="6" height="8" rx="2" fill="#c78c64" />
        <circle cx="8" cy="18" r="2.4" fill="#8fe8ff" opacity="0.8" />
        <circle cx="12" cy="12" r="1.8" fill="#fff4de" opacity="0.9" />
        <path d="M3 41C7 39 11 40 13 43" fill="none" stroke="#8fe8ff" strokeWidth="2" strokeLinecap="round" />
      </g>
    </svg>
  );
}

export function PixelErhuCursor(): ReactElement | null {
  const [enabled, setEnabled] = useState(false);
  const [hoveringAction, setHoveringAction] = useState(false);
  const [sparks, setSparks] = useState<SparkPoint[]>([]);
  const cursorRef = useRef<HTMLDivElement | null>(null);
  const lastTrailRef = useRef(0);

  function spawnSpark(x: number, y: number, lifetime: number): void {
    const palette = ["#8fe8ff", "#ffd36e", "#ff9ac2", "#fff4de"];
    const types: SparkPoint["symbol"][] = ["square", "note", ".", "beam"];
    const spark: SparkPoint = {
      id: Date.now() + Math.random(),
      x,
      y,
      size: 3 + Math.floor(Math.random() * 3),
      color: palette[Math.floor(Math.random() * palette.length)]!,
      symbol: types[Math.floor(Math.random() * types.length)]!,
    };

    setSparks((current) => [...current, spark]);
    window.setTimeout(() => {
      setSparks((current) => current.filter((item) => item.id !== spark.id));
    }, lifetime);
  }

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
      if (cursorRef.current) {
        cursorRef.current.style.transform = `translate3d(${event.clientX + 10}px, ${event.clientY - 12}px, 0)`;
      }

      if (event.timeStamp - lastTrailRef.current > 22) {
        lastTrailRef.current = event.timeStamp;
        spawnSpark(event.clientX - 6 + Math.random() * 12, event.clientY - 2 + Math.random() * 10, 360);
      }

      const target = event.target;
      if (target instanceof HTMLElement) {
        setHoveringAction(Boolean(target.closest("a, button, summary")));
      } else {
        setHoveringAction(false);
      }
    }

    function handleDown(event: PointerEvent): void {
      spawnSpark(event.clientX, event.clientY, 420);
      spawnSpark(event.clientX + 8, event.clientY - 6, 420);
      spawnSpark(event.clientX - 10, event.clientY + 4, 420);
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
        <CursorArtwork hoveringAction={hoveringAction} />
      </div>

      {sparks.map((spark) => (
        <div
          key={spark.id}
          aria-hidden="true"
          className="pointer-events-none fixed left-0 top-0 z-50 hidden h-7 w-7 md:block"
          style={{
            transform: `translate(${spark.x}px, ${spark.y}px) translate(-50%, -50%)`,
            animation: "site-spark 320ms ease-out forwards",
          }}
        >
          {spark.symbol === "note" ? (
            <span className="absolute left-1/2 top-0 -translate-x-1/2 text-[12px]" style={{ color: spark.color }}>
              ♪
            </span>
          ) : spark.symbol === "." ? (
            <span
              className="absolute left-1/2 top-1/2 rounded-full"
              style={{
                width: `${spark.size}px`,
                height: `${spark.size}px`,
                transform: "translate(-50%, -50%)",
                backgroundColor: spark.color,
              }}
            />
          ) : spark.symbol === "beam" ? (
            <span className="absolute left-1/2 top-1/2 h-[2px] w-[10px] -translate-x-1/2 -translate-y-1/2" style={{ backgroundColor: spark.color }} />
          ) : (
            <span
              className="absolute left-1/2 top-1/2"
              style={{
                width: `${spark.size + 1}px`,
                height: `${spark.size + 1}px`,
                transform: "translate(-50%, -50%)",
                backgroundColor: spark.color,
              }}
            />
          )}
        </div>
      ))}
    </>
  );
}
