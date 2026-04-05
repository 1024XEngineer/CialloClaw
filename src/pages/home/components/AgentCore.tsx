import { useEffect, useRef, useState } from 'react';

interface AgentCoreProps {
  agentState: 'idle' | 'working' | 'highlight' | 'completing' | 'done' | 'error';
  label?: string;
}

const stateConfig = {
  idle: {
    primaryColor: '#94a3b8',
    secondaryColor: '#64748b',
    glowColor: 'rgba(148,163,184,0.3)',
    label: '待命中',
    size: 120,
  },
  working: {
    primaryColor: '#34d399',
    secondaryColor: '#10b981',
    glowColor: 'rgba(52,211,153,0.4)',
    label: '推进中',
    size: 130,
  },
  highlight: {
    primaryColor: '#fbbf24',
    secondaryColor: '#f59e0b',
    glowColor: 'rgba(251,191,36,0.5)',
    label: '有新进展',
    size: 140,
  },
  completing: {
    primaryColor: '#2dd4bf',
    secondaryColor: '#14b8a6',
    glowColor: 'rgba(45,212,191,0.45)',
    label: '接近完成',
    size: 135,
  },
  done: {
    primaryColor: '#e2e8f0',
    secondaryColor: '#cbd5e1',
    glowColor: 'rgba(226,232,240,0.35)',
    label: '已完成',
    size: 125,
  },
  error: {
    primaryColor: '#fb7185',
    secondaryColor: '#f43f5e',
    glowColor: 'rgba(251,113,133,0.5)',
    label: '需要介入',
    size: 130,
  },
};

export default function AgentCore({ agentState }: AgentCoreProps) {
  const [pulse, setPulse] = useState(0);
  const [rotation, setRotation] = useState(0);
  const animRef = useRef<number>(0);
  const startRef = useRef<number>(Date.now());
  const config = stateConfig[agentState] || stateConfig.idle;

  useEffect(() => {
    const animate = () => {
      const elapsed = (Date.now() - startRef.current) / 1000;
      const speed = agentState === 'working' || agentState === 'highlight' ? 1.8 : 0.8;
      setPulse(Math.sin(elapsed * speed) * 0.5 + 0.5);
      setRotation(elapsed * (agentState === 'working' ? 25 : agentState === 'error' ? 40 : 15));
      animRef.current = requestAnimationFrame(animate);
    };
    animRef.current = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(animRef.current);
  }, [agentState]);

  const coreSize = config.size + pulse * 8;
  const glowRadius = coreSize * 0.9 + pulse * 20;

  return (
    <div className="relative flex flex-col items-center justify-center" style={{ width: 220, height: 220 }}>
      {/* Outermost ambient glow */}
      <div
        className="absolute rounded-full"
        style={{
          width: glowRadius * 2.2,
          height: glowRadius * 2.2,
          left: '50%',
          top: '50%',
          transform: 'translate(-50%, -50%)',
          background: `radial-gradient(circle, ${config.glowColor} 0%, transparent 70%)`,
          opacity: 0.5 + pulse * 0.3,
          transition: 'opacity 0.3s ease',
        }}
      />

      {/* Orbital ring 1 */}
      <div
        className="absolute rounded-full border"
        style={{
          width: coreSize * 1.55,
          height: coreSize * 1.55,
          left: '50%',
          top: '50%',
          transform: `translate(-50%, -50%) rotate(${rotation}deg)`,
          borderColor: `${config.primaryColor}22`,
          borderWidth: 1,
        }}
      >
        {/* Orbital dot */}
        <div
          className="absolute rounded-full"
          style={{
            width: 5,
            height: 5,
            background: config.primaryColor,
            top: -2.5,
            left: '50%',
            transform: 'translateX(-50%)',
            boxShadow: `0 0 8px ${config.primaryColor}`,
            opacity: 0.8 + pulse * 0.2,
          }}
        />
      </div>

      {/* Orbital ring 2 */}
      <div
        className="absolute rounded-full border"
        style={{
          width: coreSize * 1.28,
          height: coreSize * 1.28,
          left: '50%',
          top: '50%',
          transform: `translate(-50%, -50%) rotate(${-rotation * 0.7}deg)`,
          borderColor: `${config.secondaryColor}18`,
          borderWidth: 1,
        }}
      >
        <div
          className="absolute rounded-full"
          style={{
            width: 3,
            height: 3,
            background: config.secondaryColor,
            bottom: -1.5,
            left: '50%',
            transform: 'translateX(-50%)',
            boxShadow: `0 0 6px ${config.secondaryColor}`,
            opacity: 0.6 + pulse * 0.3,
          }}
        />
      </div>

      {/* Core sphere */}
      <div
        className="relative rounded-full flex items-center justify-center"
        style={{
          width: coreSize,
          height: coreSize,
          background: `radial-gradient(circle at 38% 32%, ${config.primaryColor}cc 0%, ${config.secondaryColor}88 45%, ${config.secondaryColor}44 100%)`,
          boxShadow: `0 0 ${glowRadius * 0.6}px ${config.glowColor}, 0 0 ${glowRadius * 0.3}px ${config.glowColor} inset`,
          transition: 'width 0.6s ease, height 0.6s ease',
        }}
      >
        {/* Inner highlight */}
        <div
          className="absolute rounded-full"
          style={{
            width: '55%',
            height: '45%',
            top: '15%',
            left: '18%',
            background: `radial-gradient(ellipse, rgba(255,255,255,${0.25 + pulse * 0.15}) 0%, transparent 70%)`,
          }}
        />
        {/* State label */}
        <span
          className="relative z-10 text-xs font-medium tracking-widest uppercase"
          style={{
            color: 'rgba(255,255,255,0.85)',
            letterSpacing: '0.18em',
            fontSize: 10,
            textShadow: `0 0 12px ${config.primaryColor}`,
          }}
        >
          {config.label}
        </span>
      </div>
    </div>
  );
}
