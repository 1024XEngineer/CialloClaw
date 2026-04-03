import { useEffect, useRef, useState } from 'react';

interface AgentPresenceProps {
  agentState: 'idle' | 'working' | 'highlight' | 'completing' | 'done' | 'error';
}

const stateColors: Record<string, string> = {
  idle: '#64748b', working: '#34d399', highlight: '#fbbf24',
  completing: '#2dd4bf', done: '#e2e8f0', error: '#fb7185',
};

export default function AgentPresence({ agentState }: AgentPresenceProps) {
  const [pulse, setPulse] = useState(0);
  const animRef = useRef<number>(0);
  const startRef = useRef(Date.now());
  const color = stateColors[agentState] || stateColors.idle;

  useEffect(() => {
    const animate = () => {
      const elapsed = (Date.now() - startRef.current) / 1000;
      setPulse(Math.sin(elapsed * (agentState === 'working' ? 2 : 1)) * 0.5 + 0.5);
      animRef.current = requestAnimationFrame(animate);
    };
    animRef.current = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(animRef.current);
  }, [agentState]);

  return (
    <div className="flex items-center gap-2">
      <div className="relative w-2 h-2 flex items-center justify-center">
        <div className="absolute rounded-full" style={{ width: 8 + pulse * 6, height: 8 + pulse * 6, background: `${color}22`, borderRadius: '50%' }} />
        <div className="w-1.5 h-1.5 rounded-full" style={{ background: color, boxShadow: `0 0 6px ${color}` }} />
      </div>
      <span style={{ fontSize: 10.5, color: 'rgba(100,116,139,0.7)', letterSpacing: '0.12em' }}>
        {agentState === 'idle' ? 'STANDBY' : agentState === 'working' ? 'ACTIVE' : agentState === 'error' ? 'BLOCKED' : agentState === 'done' ? 'COMPLETE' : 'ACTIVE'}
      </span>
    </div>
  );
}
