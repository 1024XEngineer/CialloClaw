import { useEffect, useRef, useState } from 'react';
import type { AgentStateData } from '@/mocks/agentStates';

interface OrbV2Props {
  stateData: AgentStateData;
  isOpen: boolean;
  onOpen: () => void;
  onClose: () => void;
}

export default function OrbV2({ stateData, isOpen, onOpen, onClose }: OrbV2Props) {
  const [hovered, setHovered] = useState(false);
  const [pulse, setPulse] = useState(0);
  const [rotation, setRotation] = useState(0);
  const animRef = useRef<number>(0);
  const startRef = useRef(Date.now());

  useEffect(() => {
    const animate = () => {
      const elapsed = (Date.now() - startRef.current) / 1000;
      setPulse(Math.sin(elapsed * stateData.breathSpeed) * 0.5 + 0.5);
      setRotation(elapsed * 18);
      animRef.current = requestAnimationFrame(animate);
    };
    animRef.current = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(animRef.current);
  }, [stateData.breathSpeed]);

  const isAnomaly = stateData.key.startsWith('error');
  const isHighlight = stateData.key === 'highlight';
  const hasNotif = isAnomaly || isHighlight || stateData.key === 'completing';
  const orbSize = 56;
  const glowSize = hovered ? 52 : 28 + pulse * 18;

  return (
    <div className="relative flex flex-col items-center" style={{ userSelect: 'none' }}>
      {/* Hover tooltip */}
      {hovered && !isOpen && (
        <div
          className="absolute whitespace-nowrap rounded-xl px-3 py-1.5 pointer-events-none"
          style={{
            bottom: orbSize + 14,
            left: '50%',
            transform: 'translateX(-50%)',
            background: 'rgba(8,12,20,0.92)',
            border: '1px solid rgba(255,255,255,0.08)',
            fontSize: 11,
            color: 'rgba(148,163,184,0.9)',
            backdropFilter: 'blur(12px)',
            animation: 'fadeInUp 0.2s ease',
          }}
        >
          点击查看 Agent 状态
          <div
            className="absolute left-1/2 -translate-x-1/2"
            style={{
              bottom: -4,
              width: 6,
              height: 6,
              background: 'rgba(8,12,20,0.92)',
              border: '1px solid rgba(255,255,255,0.08)',
              borderTop: 'none',
              borderLeft: 'none',
              transform: 'translateX(-50%) rotate(45deg)',
            }}
          />
        </div>
      )}

      {/* Orb container */}
      <div
        className="relative flex items-center justify-center cursor-pointer"
        style={{ width: orbSize + 32, height: orbSize + 32 }}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
        onClick={() => (isOpen ? onClose() : onOpen())}
      >
        {/* Ambient glow */}
        <div
          className="absolute rounded-full pointer-events-none"
          style={{
            width: orbSize + glowSize * 2,
            height: orbSize + glowSize * 2,
            background: `radial-gradient(circle, ${stateData.orbGlow} 0%, transparent 65%)`,
            opacity: 0.7 + pulse * 0.3,
            transition: 'width 0.4s ease, height 0.4s ease',
          }}
        />

        {/* Orbital ring - only when active */}
        {stateData.key !== 'standby' && (
          <div
            className="absolute rounded-full pointer-events-none"
            style={{
              width: orbSize + 22,
              height: orbSize + 22,
              border: `1px solid ${stateData.accentColor}28`,
              transform: `rotate(${rotation}deg)`,
            }}
          >
            <div
              className="absolute rounded-full"
              style={{
                width: 4,
                height: 4,
                background: stateData.accentColor,
                top: -2,
                left: '50%',
                transform: 'translateX(-50%)',
                boxShadow: `0 0 6px ${stateData.accentColor}`,
                opacity: 0.7 + pulse * 0.3,
              }}
            />
          </div>
        )}

        {/* Core orb */}
        <div
          className="relative rounded-full flex items-center justify-center"
          style={{
            width: orbSize,
            height: orbSize,
            background: `radial-gradient(circle at 38% 32%, ${stateData.accentColor}cc 0%, ${stateData.orbColor} 60%)`,
            boxShadow: `0 0 ${20 + pulse * 14}px ${stateData.orbGlow}, 0 0 ${8 + pulse * 6}px ${stateData.orbGlow} inset`,
            transform: `scale(${hovered ? 1.1 : 1 + pulse * 0.03})`,
            transition: 'transform 0.3s ease',
          }}
        >
          {/* Inner shimmer */}
          <div
            className="absolute inset-0 rounded-full pointer-events-none"
            style={{
              background: `radial-gradient(circle at 35% 28%, rgba(255,255,255,${0.22 + pulse * 0.12}) 0%, transparent 55%)`,
            }}
          />
          {/* Logo */}
          <img
            src="https://public.readdy.ai/ai/img_res/8b676746-42b7-4954-94f1-f8b31a0d2d1e.png"
            alt="Agent"
            className="rounded-full object-cover relative z-10"
            style={{ width: 26, height: 26, opacity: 0.85 }}
          />
        </div>

        {/* Notification badge */}
        {hasNotif && (
          <div
            className="absolute rounded-full"
            style={{
              width: 10,
              height: 10,
              background: isAnomaly ? '#fb7185' : stateData.accentColor,
              border: '2px solid #080c14',
              top: 10,
              right: 10,
              boxShadow: `0 0 8px ${isAnomaly ? '#fb7185' : stateData.accentColor}`,
              animation: 'notifPulse 1.8s ease-in-out infinite',
            }}
          />
        )}
      </div>

      {/* State label below orb */}
      <div
        className="flex items-center gap-1.5"
        style={{
          opacity: hovered || isOpen ? 1 : 0,
          transform: hovered || isOpen ? 'translateY(0)' : 'translateY(4px)',
          transition: 'opacity 0.25s ease, transform 0.25s ease',
          marginTop: -4,
        }}
      >
        <div
          className="rounded-full"
          style={{
            width: 5,
            height: 5,
            background: stateData.accentColor,
            boxShadow: `0 0 5px ${stateData.accentColor}`,
          }}
        />
        <span style={{ fontSize: 10, color: 'rgba(148,163,184,0.65)', letterSpacing: '0.14em' }}>
          {stateData.label.toUpperCase()}
        </span>
      </div>
    </div>
  );
}
