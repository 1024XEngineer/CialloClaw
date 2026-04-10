import type { CSSProperties, PointerEvent as ReactPointerEvent } from "react";
import { useState } from "react";
import { motion } from "motion/react";
import { useNavigate } from "react-router-dom";
import ClickSpark from "@/components/ClickSpark";
import branchImage from "@/assets/lily-of-the-valley/branch.png";
import leaf1Image from "@/assets/lily-of-the-valley/leaf1.png";
import leaf2Image from "@/assets/lily-of-the-valley/leaf2.png";
import leaf3Image from "@/assets/lily-of-the-valley/leaf3.png";
import { dashboardModules } from "@/features/dashboard/shared/dashboardRoutes";
import { useDashboardStore } from "@/stores/dashboardStore";
import { cn } from "@/utils/cn";

type DragState = {
  pointerId: number;
  startX: number;
  startY: number;
};

const branchHitboxes = [
  { className: "dashboard-home__branch-hitbox dashboard-home__branch-hitbox--base" },
  { className: "dashboard-home__branch-hitbox dashboard-home__branch-hitbox--mid-left" },
  { className: "dashboard-home__branch-hitbox dashboard-home__branch-hitbox--mid" },
  { className: "dashboard-home__branch-hitbox dashboard-home__branch-hitbox--mid-right" },
  { className: "dashboard-home__branch-hitbox dashboard-home__branch-hitbox--tip" },
] as const;

function clamp(value: number, minimum: number, maximum: number) {
  return Math.min(maximum, Math.max(minimum, value));
}

export function DashboardHome() {
  const navigate = useNavigate();
  const hoveredModule = useDashboardStore((state) => state.hoveredModule);
  const setHoveredModule = useDashboardStore((state) => state.setHoveredModule);
  const [dragState, setDragState] = useState<DragState | null>(null);
  const [dragOffset, setDragOffset] = useState({ x: 0, y: 0 });
  const canopyStyle = {
    "--plant-shift-x": `${dragOffset.x * 0.12}px`,
    "--plant-shift-y": `${dragOffset.y * 0.08}px`,
    "--plant-rotate": `${dragOffset.x * 0.055}deg`,
    "--plant-lift": `${Math.abs(dragOffset.x) * 0.03}px`,
  } as CSSProperties;

  function handlePointerDown(event: ReactPointerEvent<HTMLDivElement>) {
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    setDragState({
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
    });
  }

  function handlePointerMove(event: ReactPointerEvent<HTMLDivElement>) {
    if (!dragState || dragState.pointerId !== event.pointerId) {
      return;
    }

    // Keep the response restrained so the PNG branch feels elastic instead of rigid.
    setDragOffset({
      x: clamp(event.clientX - dragState.startX, -140, 140),
      y: clamp(event.clientY - dragState.startY, -90, 54),
    });
  }

  function handlePointerEnd(event: ReactPointerEvent<HTMLDivElement>) {
    if (!dragState || dragState.pointerId !== event.pointerId) {
      return;
    }

    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }

    setDragState(null);
    setDragOffset({ x: 0, y: 0 });
  }

  return (
    <ClickSpark
      className="dashboard-home"
      duration={360}
      extraScale={1.15}
      sparkColor="#8ca47a"
      sparkCount={10}
      sparkRadius={18}
      sparkSize={12}
    >
      <motion.h1
        animate={{ opacity: 1, y: 0 }}
        className="dashboard-home__title"
        initial={{ opacity: 0, y: 24 }}
        transition={{ duration: 0.55, delay: 0.08, ease: [0.22, 1, 0.36, 1] }}
      >
        仪表盘
      </motion.h1>

      <section className="dashboard-home__stage">
        <img alt="" className="dashboard-home__leaf dashboard-home__leaf--back" src={leaf3Image} />
        <img alt="" className="dashboard-home__leaf dashboard-home__leaf--tall" src={leaf1Image} />
        <img alt="" className="dashboard-home__leaf dashboard-home__leaf--front" src={leaf2Image} />

        <div className={cn("dashboard-home__canopy", dragState && "is-dragging")} style={canopyStyle}>
          <img alt="" className="dashboard-home__branch-image" draggable={false} src={branchImage} />

          {branchHitboxes.map((hitbox) => (
            <div
              key={hitbox.className}
              className={hitbox.className}
              onLostPointerCapture={handlePointerEnd}
              onPointerCancel={handlePointerEnd}
              onPointerDown={handlePointerDown}
              onPointerMove={handlePointerMove}
              onPointerUp={handlePointerEnd}
              role="presentation"
            />
          ))}

          {dashboardModules.map((module) => {
            const isHovered = hoveredModule === module.route;

            return (
              <div
                key={module.route}
                className="dashboard-home__flower-shell"
                style={
                  {
                    "--flower-left": module.flowerPosition.left,
                    "--flower-top": module.flowerPosition.top,
                    "--flower-image-width": module.flowerPosition.imageWidth,
                    "--flower-duration": module.flowerPosition.swayDuration,
                    "--flower-delay": module.flowerPosition.swayDelay,
                    "--flower-accent": module.accent,
                  } as CSSProperties
                }
              >
                <button
                  aria-label={`进入${module.title}`}
                  className={cn("dashboard-home__flower", isHovered && "is-hovered")}
                  onBlur={() => setHoveredModule(null)}
                  onClick={() => navigate(module.path)}
                  onFocus={() => setHoveredModule(module.route)}
                  onMouseEnter={() => setHoveredModule(module.route)}
                  onMouseLeave={() => setHoveredModule(null)}
                  type="button"
                >
                  <span className="dashboard-home__flower-halo" />
                  <img alt="" className="dashboard-home__flower-image" draggable={false} src={module.flowerImage} />
                  <span aria-hidden="true" className="dashboard-home__flower-label">
                    {module.title.split("").map((character, index) => (
                      <span key={`${module.route}-${index}`}>{character}</span>
                    ))}
                  </span>
                </button>
              </div>
            );
          })}
        </div>
      </section>

      <motion.p
        animate={{ opacity: 1, y: 0 }}
        className="dashboard-home__prompt"
        initial={{ opacity: 0, y: 20 }}
        transition={{ duration: 0.55, delay: 0.16, ease: [0.22, 1, 0.36, 1] }}
      >
        轻点一朵铃兰，进入对应模块。
      </motion.p>
    </ClickSpark>
  );
}
