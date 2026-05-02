/**
 * Dashboard entrance orbs provide draggable module shortcuts around the center
 * dashboard scene without changing the underlying module routing contract.
 */
import { useCallback, useEffect, useRef, useState } from "react";
import { useEventListener, useUnmount } from "ahooks";
import { motion } from "motion/react";
import type { CSSProperties } from "react";
import { BadgeCheck, FileText, NotebookPen, ShieldCheck } from "lucide-react";
import type { DashboardEntranceOrbConfig } from "../dashboardHome.types";

type DashboardEntranceOrbProps = {
  config: DashboardEntranceOrbConfig;
  dimmed: boolean;
  isHovered: boolean;
  offset: { x: number; y: number };
  onClick: () => void;
  onHoverChange: (hovered: boolean) => void;
};

const entranceIcons = {
  tasks: FileText,
  notes: NotebookPen,
  memory: BadgeCheck,
  safety: ShieldCheck,
} as const;

/**
 * Renders a draggable dashboard entrance orb that can still behave like a
 * button when users click without moving it.
 *
 * @param props Orb configuration, pointer state, and navigation callbacks.
 * @returns The animated entrance orb button.
 */
export function DashboardEntranceOrb({ config, dimmed, isHovered, offset, onClick, onHoverChange }: DashboardEntranceOrbProps) {
  const [dragPos, setDragPos] = useState<{ x: number; y: number } | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [isSnapping, setIsSnapping] = useState(false);
  const [rotationAngle, setRotationAngle] = useState(config.orbitOffset);
  const dragStartRef = useRef({ mouseX: 0, mouseY: 0 });
  const clickHandledRef = useRef(false);
  const movedRef = useRef(false);
  const draggingRef = useRef(false);
  const snapTimerRef = useRef<number | null>(null);
  const windowTarget = typeof window === "undefined" ? undefined : window;
  const Icon = entranceIcons[config.module];

  useEffect(() => {
    let frame = 0;
    let last = 0;

    const animate = (timestamp: number) => {
      const dt = last ? (timestamp - last) / 1000 : 0;
      last = timestamp;

      if (!draggingRef.current && dt > 0 && dt < 0.1) {
        setRotationAngle((current) => (current + config.orbitSpeed * dt) % 360);
      }

      frame = window.requestAnimationFrame(animate);
    };

    frame = window.requestAnimationFrame(animate);
    return () => window.cancelAnimationFrame(frame);
  }, [config.orbitSpeed]);

  const rad = (rotationAngle * Math.PI) / 180;
  const orbitX = Math.cos(rad) * config.orbitRadius + offset.x * 0.16;
  const orbitY = Math.sin(rad) * config.orbitRadius + offset.y * 0.16;
  const x = dragPos ? dragPos.x : orbitX;
  const y = dragPos ? dragPos.y : orbitY;

  const handleMouseDown = useCallback((event: React.MouseEvent<HTMLButtonElement>) => {
    event.preventDefault();
    event.stopPropagation();
    clickHandledRef.current = false;
    draggingRef.current = true;
    movedRef.current = false;
    dragStartRef.current = { mouseX: event.clientX, mouseY: event.clientY };
  }, []);

  const handleMove = useCallback((event: MouseEvent) => {
    if (!draggingRef.current) {
      return;
    }

    const dx = event.clientX - dragStartRef.current.mouseX;
    const dy = event.clientY - dragStartRef.current.mouseY;

    if (!movedRef.current && Math.hypot(dx, dy) > 6) {
      movedRef.current = true;
      setIsDragging(true);
    }

    if (movedRef.current) {
      setDragPos({ x: event.clientX - window.innerWidth / 2, y: event.clientY - window.innerHeight / 2 });
    }
  }, []);

  const handleUp = useCallback((event: MouseEvent) => {
    if (!draggingRef.current) {
      return;
    }

    draggingRef.current = false;

    if (!movedRef.current) {
      setIsDragging(false);
      setDragPos(null);
      clickHandledRef.current = true;
      window.setTimeout(() => {
        onClick();
        clickHandledRef.current = false;
      }, 0);
      return;
    }

    setIsDragging(false);
    const orbitCenterX = window.innerWidth / 2 + offset.x * 0.16;
    const orbitCenterY = window.innerHeight / 2 + offset.y * 0.16;
    const relX = event.clientX - orbitCenterX;
    const relY = event.clientY - orbitCenterY;
    const dropAngle = (Math.atan2(relY, relX) * 180) / Math.PI;
    const normalizedAngle = ((dropAngle % 360) + 360) % 360;
    setDragPos(null);
    setIsSnapping(true);
    setRotationAngle(normalizedAngle);
    if (snapTimerRef.current) {
      window.clearTimeout(snapTimerRef.current);
    }
    snapTimerRef.current = window.setTimeout(() => setIsSnapping(false), 420);
  }, [offset.x, offset.y, onClick]);

  useEventListener("mousemove", handleMove, {
    target: windowTarget,
    enable: windowTarget !== undefined,
  });
  useEventListener("mouseup", handleUp, {
    target: windowTarget,
    enable: windowTarget !== undefined,
  });

  useUnmount(() => {
    if (snapTimerRef.current) {
      window.clearTimeout(snapTimerRef.current);
    }
  });
  const style = {
    left: `calc(50% + ${x}px)`,
    top: `calc(50% + ${y}px)`,
    width: `${config.size}px`,
    height: `${config.size}px`,
  } as CSSProperties;
  // CSS variable colors cannot be alpha-suffixed like hex values, so the ring uses color-mix for transparency.
  const ringColor = `color-mix(in srgb, ${config.color} 30%, transparent)`;

  return (
    <motion.button
      animate={{ opacity: dimmed ? 0.28 : 1, scale: isDragging ? 1.12 : isHovered ? 1.08 : 1 }}
      className="dashboard-orbit-entrance"
      data-hovered={isHovered ? "true" : "false"}
      data-snapping={isSnapping ? "true" : "false"}
      onClick={(event) => {
        event.stopPropagation();
        if (movedRef.current || draggingRef.current || clickHandledRef.current) {
          return;
        }
        onClick();
      }}
      onMouseDown={handleMouseDown}
      onPointerDown={(event) => event.stopPropagation()}
      onFocus={() => onHoverChange(true)}
      onBlur={() => onHoverChange(false)}
      onMouseEnter={() => onHoverChange(true)}
      onMouseLeave={() => onHoverChange(false)}
      style={style}
      transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}
      type="button"
      whileHover={{ y: -4 }}
      whileTap={{ scale: 0.98 }}
    >
      <span className="dashboard-orbit-entrance__halo" style={{ background: `radial-gradient(circle, ${config.glow} 0%, transparent 70%)` }} />
      <span className="dashboard-orbit-entrance__shell" style={{ boxShadow: `0 26px 44px -34px ${config.glow}, 0 0 0 1px ${ringColor} inset` }}>
        <span className="dashboard-orbit-entrance__core" style={{ background: `radial-gradient(circle at 30% 28%, rgba(255,255,255,0.92), ${config.color} 56%, color-mix(in srgb, ${config.color} 74%, #566070) 100%)` }}>
          <Icon className="h-[34%] w-[34%] text-white/90" />
        </span>
      </span>
      <span className="dashboard-orbit-entrance__label">{config.label}</span>
    </motion.button>
  );
}
