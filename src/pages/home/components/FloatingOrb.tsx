import { useState, useEffect, useRef } from 'react';

interface FloatingOrbProps {
  onRightClick: () => void;
  agentState: 'idle' | 'working' | 'highlight' | 'completing' | 'done' | 'error';
  hasNotification?: boolean;
}

const stateColors: Record<string, { core: string; glow: string; ring: string }> = {
  idle: {
    core: 'from-slate-400/60 via-slate-300/40 to-slate-500/60',
    glow: 'rgba(148,163,184,0.25)',
    ring: 'rgba(148,163,184,0.15)',
  },
  working: {
    core: 'from-emerald-400/70 via-teal-300/50 to-emerald-500/70',
    glow: 'rgba(52,211,153,0.35)',
    ring: 'rgba(52,211,153,0.18)',
  },
  highlight: {
    core: 'from-amber-300/80 via-yellow-200/60 to-amber-400/80',
    glow: 'rgba(251,191,36,0.45)',
    ring: 'rgba(251,191,36,0.22)',
  },
  completing: {
    core: 'from-teal-300/80 via-cyan-200/60 to-teal-400/80',
    glow: 'rgba(45,212,191,0.4)',
    ring: 'rgba(45,212,191,0.2)',
  },
  done: {
    core: 'from-slate-300/70 via-white/40 to-slate-400/70',
    glow: 'rgba(226,232,240,0.3)',
    ring: 'rgba(226,232,240,0.15)',
  },
  error: {
    core: 'from-rose-400/80 via-red-300/60 to-rose-500/80',
    glow: 'rgba(251,113,133,0.45)',
    ring: 'rgba(251,113,133,0.22)',
  },
};

export default function FloatingOrb({ onRightClick, agentState, hasNotification }: FloatingOrbProps) {
  const [hovered, setHovered] = useState(false);
  const [pulse, setPulse] = useState(0);
  const animRef = useRef<number>(0);
  const startRef = useRef<number>(Date.now());

  useEffect(() => {
    const animate = () => {
      const elapsed = (Date.now() - startRef.current) / 1000;
      setPulse(Math.sin(elapsed * 1.2) * 0.5 + 0.5);
      animRef.current = requestAnimationFrame(animate);
    };
    animRef.current = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(animRef.current);
  }, []);

  const colors = stateColors[agentState] || stateColors.idle;
  const scale = hovered ? 1.12 : 1 + pulse * 0.04;
  const glowSize = hovered ? 48 : 32 + pulse * 12;

  return (
    <div
      className="relative flex items-center justify-center cursor-pointer select-none"
      style={{ width: 64, height: 64 }}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      onContextMenu={(e) => { e.preventDefault(); onRightClick(); }}
      title="右键进入 Agent 仪表盘"
    >
      {/* Outer glow ring */}
      <div
        className="absolute rounded-full transition-all duration-700"
        style={{
          width: 64 + glowSize,
          height: 64 + glowSize,
          left: -(glowSize / 2),
          top: -(glowSize / 2),
          background: `radial-gradient(circle, ${colors.ring} 0%, transparent 70%)`,
          opacity: 0.6 + pulse * 0.4,
        }}
      />
      {/* Core orb */}
      <div
        className={`relative rounded-full bg-gradient-to-br ${colors.core} transition-all duration-300`}
        style={{
          width: 52,
          height: 52,
          transform: `scale(${scale})`,
          boxShadow: `0 0 ${glowSize}px ${colors.glow}, 0 0 ${glowSize * 0.5}px ${colors.glow} inset`,
          backdropFilter: 'blur(8px)',
        }}
      >
        {/* Inner shimmer */}
        <div
          className="absolute inset-0 rounded-full"
          style={{
            background: `radial-gradient(circle at 35% 30%, rgba(255,255,255,${0.3 + pulse * 0.2}) 0%, transparent 60%)`,
          }}
        />
      </div>
      {/* Notification dot */}
      {hasNotification && (
        <div
          className="absolute top-0 right-0 w-3 h-3 rounded-full bg-amber-400 border-2 border-slate-900"
          style={{ animation: 'ping 1.5s ease-in-out infinite' }}
        />
      )}
    </div>
  );
}
