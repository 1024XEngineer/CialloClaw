import type { CSSProperties, PointerEvent, ReactNode } from "react";
import { useCallback, useEffect, useRef } from "react";
import { cn } from "@/utils/cn";

type SparkEasing = "linear" | "ease-in" | "ease-in-out" | "ease-out";

type Spark = {
  angle: number;
  startTime: number;
  x: number;
  y: number;
};

type ClickSparkProps = {
  children: ReactNode;
  className?: string;
  style?: CSSProperties;
  sparkColor?: string;
  sparkSize?: number;
  sparkRadius?: number;
  sparkCount?: number;
  duration?: number;
  easing?: SparkEasing;
  extraScale?: number;
};

function easeSpark(progress: number, easing: SparkEasing) {
  switch (easing) {
    case "linear":
      return progress;
    case "ease-in":
      return progress * progress;
    case "ease-in-out":
      return progress < 0.5 ? 2 * progress * progress : -1 + (4 - 2 * progress) * progress;
    default:
      return progress * (2 - progress);
  }
}

export default function ClickSpark({
  children,
  className,
  style,
  sparkColor = "#fff",
  sparkSize = 10,
  sparkRadius = 15,
  sparkCount = 8,
  duration = 400,
  easing = "ease-out",
  extraScale = 1,
}: ClickSparkProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const sparksRef = useRef<Spark[]>([]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const parent = canvas?.parentElement;
    if (!canvas || !parent) {
      return;
    }

    let resizeTimeout: ReturnType<typeof setTimeout> | null = null;

    const resizeCanvas = () => {
      const { height, width } = parent.getBoundingClientRect();
      if (canvas.width !== width || canvas.height !== height) {
        canvas.width = width;
        canvas.height = height;
      }
    };

    const observer = new ResizeObserver(() => {
      if (resizeTimeout) {
        clearTimeout(resizeTimeout);
      }

      resizeTimeout = setTimeout(resizeCanvas, 100);
    });

    observer.observe(parent);
    resizeCanvas();

    return () => {
      observer.disconnect();
      if (resizeTimeout) {
        clearTimeout(resizeTimeout);
      }
    };
  }, []);

  useEffect(() => {
    const canvas = canvasRef.current;
    const context = canvas?.getContext("2d");
    if (!canvas || !context) {
      return;
    }

    let animationFrameId = 0;

    const draw = (timestamp: number) => {
      context.clearRect(0, 0, canvas.width, canvas.height);

      sparksRef.current = sparksRef.current.filter((spark) => {
        const elapsed = timestamp - spark.startTime;
        if (elapsed >= duration) {
          return false;
        }

        const progress = elapsed / duration;
        const eased = easeSpark(progress, easing);
        const distance = eased * sparkRadius * extraScale;
        const lineLength = sparkSize * (1 - eased);
        const x1 = spark.x + distance * Math.cos(spark.angle);
        const y1 = spark.y + distance * Math.sin(spark.angle);
        const x2 = spark.x + (distance + lineLength) * Math.cos(spark.angle);
        const y2 = spark.y + (distance + lineLength) * Math.sin(spark.angle);

        context.strokeStyle = sparkColor;
        context.lineWidth = 2;
        context.beginPath();
        context.moveTo(x1, y1);
        context.lineTo(x2, y2);
        context.stroke();

        return true;
      });

      animationFrameId = window.requestAnimationFrame(draw);
    };

    animationFrameId = window.requestAnimationFrame(draw);
    return () => window.cancelAnimationFrame(animationFrameId);
  }, [duration, easing, extraScale, sparkColor, sparkRadius, sparkSize]);

  const emitSparks = useCallback(
    (clientX: number, clientY: number) => {
      const canvas = canvasRef.current;
      if (!canvas) {
        return;
      }

      const rect = canvas.getBoundingClientRect();
      const x = clientX - rect.left;
      const y = clientY - rect.top;
      const now = performance.now();
      const newSparks = Array.from({ length: sparkCount }, (_, index) => ({
        angle: (2 * Math.PI * index) / sparkCount,
        startTime: now,
        x,
        y,
      }));

      sparksRef.current.push(...newSparks);
    },
    [sparkCount],
  );

  const handlePointerDown = useCallback(
    (event: PointerEvent<HTMLDivElement>) => {
      emitSparks(event.clientX, event.clientY);
    },
    [emitSparks],
  );

  return (
    <div className={cn("relative h-full w-full", className)} onPointerDown={handlePointerDown} style={style}>
      <canvas
        className="pointer-events-none absolute left-0 top-0 block h-full w-full select-none"
        ref={canvasRef}
      />
      {children}
    </div>
  );
}
