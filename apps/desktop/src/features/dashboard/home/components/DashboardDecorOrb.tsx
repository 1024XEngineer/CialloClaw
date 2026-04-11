import { useEffect, useState } from "react";
import type { CSSProperties } from "react";
import { motion } from "motion/react";
import type { DashboardDecorOrbConfig } from "../dashboardHome.types";

type DashboardDecorOrbProps = {
  config: DashboardDecorOrbConfig;
  dimmed: boolean;
  offset: { x: number; y: number };
};

export function DashboardDecorOrb({ config, dimmed, offset }: DashboardDecorOrbProps) {
  const [rotationAngle, setRotationAngle] = useState(config.orbitOffset);

  useEffect(() => {
    let frame = 0;
    let last = 0;

    const animate = (timestamp: number) => {
      const dt = last ? (timestamp - last) / 1000 : 0;
      last = timestamp;

      if (dt > 0 && dt < 0.1) {
        setRotationAngle((current) => (current + config.orbitSpeed * dt) % 360);
      }

      frame = window.requestAnimationFrame(animate);
    };

    frame = window.requestAnimationFrame(animate);
    return () => window.cancelAnimationFrame(frame);
  }, [config.orbitSpeed]);

  const rad = (rotationAngle * Math.PI) / 180;
  const x = Math.cos(rad) * config.orbitRadius + offset.x * 0.08;
  const y = Math.sin(rad) * config.orbitRadius + offset.y * 0.08;
  const style = {
    background: config.color,
    boxShadow: `0 0 18px ${config.glow}`,
    height: `${config.size}px`,
    left: `calc(50% + ${x}px)`,
    top: `calc(50% + ${y}px)`,
    width: `${config.size}px`,
  } as CSSProperties;

  return <motion.span animate={{ opacity: dimmed ? 0.12 : 0.46, scale: dimmed ? 0.9 : 1 }} className="dashboard-orbit-decor" style={style} transition={{ duration: 0.3 }} />;
}
