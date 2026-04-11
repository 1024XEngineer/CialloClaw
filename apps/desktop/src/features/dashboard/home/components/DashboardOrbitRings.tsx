import { useMemo } from "react";
import type { CSSProperties } from "react";

type DashboardOrbitRingsProps = {
  offset: { x: number; y: number };
};

export function DashboardOrbitRings({ offset }: DashboardOrbitRingsProps) {
  const particles = useMemo(
    () =>
      Array.from({ length: 8 }).map((_, index) => ({
        angle: (index / 8) * Math.PI * 2,
        radius: 210 + (index % 3) * 28,
        size: index % 3 === 0 ? 6 : index % 2 === 0 ? 4 : 3,
      })),
    [],
  );

  const fieldStyle = {
    transform: `translate(calc(-50% + ${offset.x * 0.05}px), calc(-50% + ${offset.y * 0.05}px))`,
  } as CSSProperties;

  return (
    <div className="dashboard-orbit-rings" style={fieldStyle}>
      <div className="dashboard-orbit-rings__halo" />
      <div className="dashboard-orbit-rings__ring dashboard-orbit-rings__ring--entrance" />
      <div className="dashboard-orbit-rings__ring dashboard-orbit-rings__ring--event" />
      <div className="dashboard-orbit-rings__ring dashboard-orbit-rings__ring--decor" />

      {particles.map((particle, index) => {
        const x = Math.cos(particle.angle) * particle.radius;
        const y = Math.sin(particle.angle) * particle.radius;

        return (
          <span
            key={`${particle.angle}-${particle.radius}`}
            className="dashboard-orbit-rings__particle"
            style={{
              animationDelay: `${index * 0.35}s`,
              height: `${particle.size}px`,
              left: `calc(50% + ${x}px)`,
              top: `calc(50% + ${y}px)`,
              width: `${particle.size}px`,
            }}
          />
        );
      })}
    </div>
  );
}
