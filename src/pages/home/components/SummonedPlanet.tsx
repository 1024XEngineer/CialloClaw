import { useEffect, useState, useRef, useCallback } from 'react';
import type { PlanetConfig } from './PlanetNode';

export interface SummonEvent {
  id: string;
  planet: PlanetConfig;
  message: string;
  reason: string;
  nextStep?: string;
  priority: 'urgent' | 'normal' | 'low';
  duration?: number;
}

interface SummonedPlanetProps {
  event: SummonEvent;
  onDismiss: (id: string) => void;
  onExpand: (planetKey: string) => void;
}

type Phase = 'dormant' | 'emerging' | 'present' | 'receding' | 'gone';

const SUMMON_RADIUS = 155;
const DORMANT_RADIUS = 260;

export default function SummonedPlanet({ event, onDismiss, onExpand }: SummonedPlanetProps) {
  const [phase, setPhase] = useState<Phase>('dormant');
  const [textVisible, setTextVisible] = useState(false);
  const [hovered, setHovered] = useState(false);
  const [orbitAngle, setOrbitAngle] = useState(0);
  const [dismissed, setDismissed] = useState(false);

  // Drag state — offset from orbit position
  const [dragOffset, setDragOffset] = useState({ x: 0, y: 0 });
  const [isDragging, setIsDragging] = useState(false);
  const isDraggingRef = useRef(false);
  const dragStartRef = useRef({ mouseX: 0, mouseY: 0, offX: 0, offY: 0 });

  const animRef = useRef<number>(0);
  const startAngleRef = useRef(Math.random() * 360);
  const phaseRef = useRef<Phase>('dormant');
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const hoveredRef = useRef(false);

  const { planet, message, reason, nextStep, priority, duration = 5200 } = event;

  // Keep hoveredRef in sync
  useEffect(() => { hoveredRef.current = hovered; }, [hovered]);

  // Orbit animation
  useEffect(() => {
    const speed = 1.8;
    let last = 0;
    const animate = (ts: number) => {
      const dt = last ? (ts - last) / 1000 : 0;
      last = ts;
      if (phaseRef.current === 'present' || phaseRef.current === 'emerging') {
        startAngleRef.current = (startAngleRef.current + speed * dt) % 360;
        setOrbitAngle(startAngleRef.current);
      }
      animRef.current = requestAnimationFrame(animate);
    };
    animRef.current = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(animRef.current);
  }, []);

  const startRecede = useCallback(() => {
    if (phaseRef.current === 'receding' || phaseRef.current === 'gone') return;
    phaseRef.current = 'receding';
    setPhase('receding');
    setTextVisible(false);
    setTimeout(() => {
      phaseRef.current = 'gone';
      setPhase('gone');
      onDismiss(event.id);
    }, 800);
  }, [event.id, onDismiss]);

  // Phase lifecycle
  useEffect(() => {
    const t0 = setTimeout(() => {
      phaseRef.current = 'emerging';
      setPhase('emerging');

      const t1 = setTimeout(() => {
        setTextVisible(true);
        phaseRef.current = 'present';
        setPhase('present');

        timerRef.current = setTimeout(() => {
          if (!hoveredRef.current) startRecede();
        }, duration);
      }, 700);

      return () => clearTimeout(t1);
    }, 80);

    return () => {
      clearTimeout(t0);
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [duration, startRecede]);

  // Global drag handlers
  useEffect(() => {
    const handleMove = (e: MouseEvent) => {
      if (!isDraggingRef.current) return;
      const dx = e.clientX - dragStartRef.current.mouseX;
      const dy = e.clientY - dragStartRef.current.mouseY;
      setDragOffset({ x: dragStartRef.current.offX + dx, y: dragStartRef.current.offY + dy });
    };
    const handleUp = () => {
      if (!isDraggingRef.current) return;
      isDraggingRef.current = false;
      setIsDragging(false);
    };
    window.addEventListener('mousemove', handleMove);
    window.addEventListener('mouseup', handleUp);
    return () => {
      window.removeEventListener('mousemove', handleMove);
      window.removeEventListener('mouseup', handleUp);
    };
  }, []);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    isDraggingRef.current = true;
    setIsDragging(true);
    dragStartRef.current = {
      mouseX: e.clientX,
      mouseY: e.clientY,
      offX: dragOffset.x,
      offY: dragOffset.y,
    };
    // Pause auto-recede while dragging
    if (timerRef.current) { clearTimeout(timerRef.current); timerRef.current = null; }
  }, [dragOffset]);

  const handleMouseUp = useCallback(() => {
    if (isDragging) return; // handled by global
    // Resume recede timer after drag ends
    if (phase === 'present' && !hoveredRef.current) {
      timerRef.current = setTimeout(() => startRecede(), 2500);
    }
  }, [isDragging, phase, startRecede]);

  const handleMouseEnter = useCallback(() => {
    setHovered(true);
    if (timerRef.current) { clearTimeout(timerRef.current); timerRef.current = null; }
  }, []);

  const handleMouseLeave = useCallback(() => {
    setHovered(false);
    if (phase === 'present' && !isDragging) {
      timerRef.current = setTimeout(() => startRecede(), 1800);
    }
  }, [phase, isDragging, startRecede]);

  const handleDismiss = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    setDismissed(true);
    startRecede();
  }, [startRecede]);

  const handleClick = useCallback(() => {
    if (isDragging) return;
    onExpand(planet.key);
    startRecede();
  }, [isDragging, onExpand, planet.key, startRecede]);

  if (phase === 'gone') return null;

  const rad = (orbitAngle * Math.PI) / 180;
  const currentRadius = phase === 'dormant'
    ? DORMANT_RADIUS
    : phase === 'emerging'
    ? SUMMON_RADIUS + (DORMANT_RADIUS - SUMMON_RADIUS) * 0.15
    : phase === 'present'
    ? SUMMON_RADIUS
    : DORMANT_RADIUS * 0.85;

  const baseX = Math.cos(rad) * currentRadius;
  const baseY = Math.sin(rad) * currentRadius;
  const x = baseX + dragOffset.x;
  const y = baseY + dragOffset.y;

  const isVisible = phase !== 'dormant' && phase !== 'gone';
  const opacity = phase === 'dormant' ? 0
    : phase === 'emerging' ? 0.92
    : phase === 'present' ? 1
    : 0;

  const planetScale = phase === 'emerging' ? 1.05
    : phase === 'present' ? (hovered || isDragging ? 1.18 : 1.08)
    : 0.85;

  const priorityColors = {
    urgent: { ring: '#fb7185', dot: '#fb7185', label: 'URGENT' },
    normal: { ring: planet.color, dot: planet.color, label: 'AGENT' },
    low: { ring: `${planet.color}80`, dot: `${planet.color}80`, label: '' },
  };
  const pc = priorityColors[priority];

  return (
    <div
      className="absolute"
      style={{
        left: '50%',
        top: '50%',
        transform: `translate(calc(-50% + ${x}px), calc(-50% + ${y}px))`,
        zIndex: isDragging ? 50 : 25,
        opacity,
        transition: isDragging
          ? 'opacity 0.15s ease'
          : phase === 'emerging'
          ? 'transform 0.7s cubic-bezier(0.16,1,0.3,1), opacity 0.5s ease'
          : phase === 'receding'
          ? 'transform 0.8s cubic-bezier(0.4,0,1,1), opacity 0.6s ease'
          : 'opacity 0.3s ease',
        pointerEvents: isVisible ? 'auto' : 'none',
        cursor: isDragging ? 'grabbing' : 'pointer',
      }}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
      onMouseDown={handleMouseDown}
      onMouseUp={handleMouseUp}
      onClick={handleClick}
    >
      {/* Glow field */}
      <div className="absolute rounded-full pointer-events-none" style={{
        width: planet.size + 80, height: planet.size + 80,
        left: '50%', top: '50%',
        transform: 'translate(-50%, -50%)',
        background: `radial-gradient(circle, ${planet.glow} 0%, transparent 60%)`,
        opacity: phase === 'present' ? (hovered || isDragging ? 1 : 0.75) : 0.4,
        transition: 'opacity 0.4s ease',
      }} />

      {/* Summon ring */}
      {phase === 'present' && (
        <div className="absolute rounded-full pointer-events-none" style={{
          width: planet.size + 36, height: planet.size + 36,
          left: '50%', top: '50%',
          transform: 'translate(-50%, -50%)',
          border: `1px solid ${pc.ring}45`,
          animation: 'summonRingPulse 2.2s ease-in-out infinite',
        }} />
      )}

      {/* Connection line — hidden while dragging */}
      {phase === 'present' && !isDragging && dragOffset.x === 0 && dragOffset.y === 0 && (
        <div className="absolute pointer-events-none" style={{
          left: '50%', top: '50%',
          width: currentRadius,
          height: 1,
          transformOrigin: '0 50%',
          transform: `rotate(${orbitAngle + 180}deg)`,
          background: `linear-gradient(to left, ${planet.color}25, transparent)`,
          opacity: hovered ? 0.6 : 0.25,
          transition: 'opacity 0.3s ease',
        }} />
      )}

      {/* Drag indicator ring */}
      {isDragging && (
        <div className="absolute rounded-full pointer-events-none" style={{
          width: planet.size + 50, height: planet.size + 50,
          left: '50%', top: '50%',
          transform: 'translate(-50%, -50%)',
          border: `1px dashed ${planet.color}40`,
          animation: 'dragSpin 3s linear infinite',
        }} />
      )}

      {/* Planet body */}
      <div className="relative rounded-full flex items-center justify-center" style={{
        width: planet.size + 8, height: planet.size + 8,
        transform: `scale(${planetScale})`,
        background: `radial-gradient(circle at 35% 30%, ${planet.color}dd 0%, ${planet.color}55 50%, ${planet.color}22 100%)`,
        border: `1.5px solid ${planet.color}${hovered || isDragging ? '80' : '55'}`,
        boxShadow: `
          0 0 ${hovered || isDragging ? 50 : 32}px ${planet.glow},
          0 0 ${hovered || isDragging ? 18 : 10}px ${planet.glow} inset,
          0 0 ${hovered || isDragging ? 80 : 55}px ${planet.glow}
        `,
        transition: isDragging ? 'transform 0.05s ease' : 'transform 0.35s cubic-bezier(0.34,1.56,0.64,1), box-shadow 0.3s ease',
      }}>
        <div className="absolute inset-0 rounded-full pointer-events-none" style={{
          background: `radial-gradient(circle at 30% 25%, rgba(255,255,255,0.28) 0%, transparent 55%)`,
        }} />
        <i className={planet.icon} style={{
          fontSize: (planet.size + 8) * 0.38,
          color: 'rgba(255,255,255,0.95)',
          position: 'relative', zIndex: 1,
        }} />
        {priority === 'urgent' && (
          <div className="absolute rounded-full" style={{
            width: 9, height: 9,
            background: '#fb7185',
            border: '2px solid #03050c',
            top: 2, right: 2,
            boxShadow: '0 0 8px #fb7185',
            animation: 'notifPulse 1.2s ease-in-out infinite',
          }} />
        )}
      </div>

      {/* Drag hint label */}
      {isDragging && (
        <div className="absolute whitespace-nowrap pointer-events-none" style={{
          top: '100%', left: '50%',
          transform: 'translateX(-50%)',
          marginTop: 8,
          fontSize: 9,
          color: `${planet.color}99`,
          letterSpacing: '0.12em',
          animation: 'fadeInUp 0.2s ease',
        }}>
          松手固定位置
        </div>
      )}

      {/* Expanded text card */}
      <div style={{
        position: 'absolute',
        left: '50%',
        top: '100%',
        transform: textVisible
          ? 'translateX(-50%) translateY(0px)'
          : 'translateX(-50%) translateY(6px)',
        marginTop: 10,
        width: 200,
        opacity: textVisible && !isDragging ? 1 : 0,
        transition: 'opacity 0.45s cubic-bezier(0.16,1,0.3,1), transform 0.45s cubic-bezier(0.16,1,0.3,1)',
        pointerEvents: 'none',
      }}>
        <div style={{
          background: 'rgba(3,5,12,0.92)',
          border: `1px solid ${planet.color}30`,
          borderRadius: 6,
          padding: '8px 11px',
          backdropFilter: 'blur(16px)',
          boxShadow: `0 8px 32px rgba(0,0,0,0.6), 0 0 20px ${planet.glow}`,
        }}>
          {pc.label && (
            <div className="flex items-center gap-1.5 mb-1.5">
              <div className="rounded-full" style={{
                width: 4, height: 4,
                background: pc.dot,
                boxShadow: `0 0 5px ${pc.dot}`,
                animation: priority === 'urgent' ? 'notifPulse 1.2s ease-in-out infinite' : 'notifPulse 2s ease-in-out infinite',
              }} />
              <span style={{ fontSize: 7.5, color: pc.dot, letterSpacing: '0.2em', fontWeight: 600 }}>
                {pc.label}
              </span>
            </div>
          )}
          <div style={{
            fontSize: 11, color: 'rgba(226,232,240,0.9)',
            lineHeight: 1.5, letterSpacing: '0.02em',
            marginBottom: 4, fontWeight: 500,
          }}>
            {message}
          </div>
          <div style={{
            fontSize: 9.5, color: 'rgba(148,163,184,0.55)',
            lineHeight: 1.4, letterSpacing: '0.02em',
            marginBottom: nextStep ? 6 : 0,
          }}>
            {reason}
          </div>
          {nextStep && (
            <div className="flex items-center gap-1.5 pt-1.5" style={{ borderTop: `1px solid ${planet.color}15` }}>
              <i className="ri-arrow-right-line" style={{ fontSize: 9, color: planet.color }} />
              <span style={{ fontSize: 9, color: planet.color, letterSpacing: '0.06em' }}>{nextStep}</span>
            </div>
          )}
        </div>
        <div className="flex justify-center" style={{ marginTop: -1 }}>
          <div style={{ width: 1, height: 8, background: `linear-gradient(to bottom, ${planet.color}40, transparent)` }} />
        </div>
      </div>

      {/* Dismiss button */}
      {hovered && !dismissed && !isDragging && (
        <button
          onClick={handleDismiss}
          onMouseDown={e => e.stopPropagation()}
          className="absolute flex items-center justify-center rounded-full cursor-pointer"
          style={{
            width: 18, height: 18,
            top: -4, right: -4,
            background: 'rgba(3,5,12,0.9)',
            border: '1px solid rgba(255,255,255,0.12)',
            color: 'rgba(148,163,184,0.6)',
            zIndex: 10,
            animation: 'fadeInScale 0.15s ease',
          }}
          onMouseEnter={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(251,113,133,0.2)'; (e.currentTarget as HTMLButtonElement).style.color = '#fb7185'; }}
          onMouseLeave={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(3,5,12,0.9)'; (e.currentTarget as HTMLButtonElement).style.color = 'rgba(148,163,184,0.6)'; }}
        >
          <i className="ri-close-line" style={{ fontSize: 10 }} />
        </button>
      )}

      <style>{`
        @keyframes summonRingPulse {
          0%, 100% { opacity: 0.5; transform: translate(-50%, -50%) scale(1); }
          50% { opacity: 0.15; transform: translate(-50%, -50%) scale(1.1); }
        }
        @keyframes fadeInScale {
          from { opacity: 0; transform: scale(0.7); }
          to { opacity: 1; transform: scale(1); }
        }
        @keyframes dragSpin {
          from { transform: translate(-50%, -50%) rotate(0deg); }
          to { transform: translate(-50%, -50%) rotate(360deg); }
        }
        @keyframes fadeInUp {
          from { opacity: 0; transform: translateX(-50%) translateY(4px); }
          to { opacity: 1; transform: translateX(-50%) translateY(0); }
        }
      `}</style>
    </div>
  );
}
