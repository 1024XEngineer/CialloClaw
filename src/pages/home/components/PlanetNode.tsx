import { useState, useEffect, useRef, useCallback } from 'react';
import type { ModuleKey } from '@/mocks/agentStates';

export interface PlanetConfig {
  key: ModuleKey;
  label: string;
  icon: string;
  color: string;
  glow: string;
  orbitRadius: number;
  orbitSpeed: number;
  orbitOffset: number;
  size: number;
  description: string;
}

// Visual weight tiers for the consciousness field
export type VisualWeight = 'focus' | 'candidate' | 'background' | 'dimmed';

interface PlanetNodeProps {
  config: PlanetConfig;
  angle: number;
  isActive: boolean;
  isAnyActive: boolean;
  onClick: () => void;
  pulse: number;
  orbDragOffset?: { x: number; y: number };
  onPositionUpdate?: (key: string, x: number, y: number) => void;
  containerRef?: React.RefObject<HTMLDivElement | null>;
  onOrbitAngleChange?: (key: string, newAngle: number) => void;
  onDragStart?: (key: string) => void;
  onDragEnd?: (key: string) => void;
  visualWeight?: VisualWeight;
}

// Per-tier visual parameters
const TIER_PARAMS: Record<VisualWeight, {
  opacity: number;
  glowMultiplier: number;
  labelVisible: boolean;
  ringVisible: boolean;
  iconOpacity: number;
}> = {
  focus: {
    opacity: 1,
    glowMultiplier: 1.4,
    labelVisible: true,
    ringVisible: true,
    iconOpacity: 0.95,
  },
  candidate: {
    opacity: 0.82,        // was 0.62 — more visible
    glowMultiplier: 1.05, // was 0.75 — stronger glow
    labelVisible: true,
    ringVisible: false,
    iconOpacity: 0.88,    // was 0.7
  },
  background: {
    opacity: 0.52,        // was 0.28 — much more visible
    glowMultiplier: 0.7,  // was 0.4
    labelVisible: false,
    ringVisible: false,
    iconOpacity: 0.65,    // was 0.45
  },
  dimmed: {
    opacity: 0.1,
    glowMultiplier: 0.15,
    labelVisible: false,
    ringVisible: false,
    iconOpacity: 0.2,
  },
};

export default function PlanetNode({
  config,
  angle,
  isActive,
  isAnyActive,
  onClick,
  pulse,
  orbDragOffset = { x: 0, y: 0 },
  onPositionUpdate,
  containerRef,
  onOrbitAngleChange,
  onDragStart,
  onDragEnd,
  visualWeight = 'candidate',
}: PlanetNodeProps) {
  const [hovered, setHovered] = useState(false);
  const [showLabel, setShowLabel] = useState(false);
  const [isDragging, setIsDragging] = useState(false);
  const [dragPos, setDragPos] = useState<{ x: number; y: number } | null>(null);
  const [isSnapping, setIsSnapping] = useState(false);

  const dragStartRef = useRef({ x: 0, y: 0 });
  const isDraggingRef = useRef(false);
  const movedRef = useRef(false);

  // Orbit base position
  const rad = (angle * Math.PI) / 180;
  const lagFactor = 0.3;
  const orbitX = Math.cos(rad) * config.orbitRadius + orbDragOffset.x * lagFactor;
  const orbitY = Math.sin(rad) * config.orbitRadius + orbDragOffset.y * lagFactor;

  const finalX = dragPos ? dragPos.x : orbitX;
  const finalY = dragPos ? dragPos.y : orbitY;

  // Report screen position to parent
  useEffect(() => {
    if (!onPositionUpdate || !containerRef?.current) return;
    const rect = containerRef.current.getBoundingClientRect();
    onPositionUpdate(config.key, rect.left + rect.width / 2 + finalX, rect.top + rect.height / 2 + finalY);
  }, [finalX, finalY, config.key, onPositionUpdate, containerRef]);

  useEffect(() => {
    if (hovered) {
      const t = setTimeout(() => setShowLabel(true), 80);
      return () => clearTimeout(t);
    }
    setShowLabel(false);
  }, [hovered]);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    isDraggingRef.current = true;
    movedRef.current = false;
    dragStartRef.current = { x: e.clientX, y: e.clientY };
  }, []);

  useEffect(() => {
    const handleMove = (e: MouseEvent) => {
      if (!isDraggingRef.current) return;
      const dx = e.clientX - dragStartRef.current.x;
      const dy = e.clientY - dragStartRef.current.y;
      if (!movedRef.current && Math.sqrt(dx * dx + dy * dy) > 6) {
        movedRef.current = true;
        setIsDragging(true);
        onDragStart?.(config.key);
      }
      if (movedRef.current && containerRef?.current) {
        const rect = containerRef.current.getBoundingClientRect();
        setDragPos({
          x: e.clientX - (rect.left + rect.width / 2),
          y: e.clientY - (rect.top + rect.height / 2),
        });
      }
    };

    const handleUp = (e: MouseEvent) => {
      if (!isDraggingRef.current) return;
      isDraggingRef.current = false;

      if (!movedRef.current) {
        setIsDragging(false);
        setDragPos(null);
        onClick();
        return;
      }

      setIsDragging(false);
      onDragEnd?.(config.key);

      if (containerRef?.current) {
        const rect = containerRef.current.getBoundingClientRect();
        const relX = e.clientX - (rect.left + rect.width / 2);
        const relY = e.clientY - (rect.top + rect.height / 2);
        const dropAngle = (Math.atan2(relY, relX) * 180) / Math.PI;
        const normalizedAngle = ((dropAngle % 360) + 360) % 360;
        setIsSnapping(true);
        setDragPos(null);
        onOrbitAngleChange?.(config.key, normalizedAngle);
        setTimeout(() => setIsSnapping(false), 500);
      } else {
        setDragPos(null);
      }
    };

    window.addEventListener('mousemove', handleMove);
    window.addEventListener('mouseup', handleUp);
    return () => {
      window.removeEventListener('mousemove', handleMove);
      window.removeEventListener('mouseup', handleUp);
    };
  }, [onClick, containerRef, onOrbitAngleChange, config.key, onDragStart, onDragEnd]);

  // Resolve visual weight — active always overrides to focus
  const effectiveWeight: VisualWeight = isActive ? 'focus' : visualWeight;
  const tierParams = TIER_PARAMS[effectiveWeight];

  // Opacity: active overrides everything
  const baseOpacity = isActive ? 1 : tierParams.opacity;
  // Hover boosts opacity slightly for non-focus tiers
  const opacity = hovered && !isActive ? Math.min(1, baseOpacity + 0.25) : baseOpacity;

  const scale = isActive ? 1.28 : hovered ? 1.1 : isDragging ? 1.15 : 1;
  const planetSize = config.size;

  // Glow intensity scales with tier
  const glowBase = 12 + pulse * 6;
  const glowIntensity = glowBase * tierParams.glowMultiplier;

  return (
    <div
      className="absolute flex items-center justify-center"
      style={{
        left: '50%',
        top: '50%',
        width: planetSize + 40,
        height: planetSize + 40,
        transform: `translate(calc(-50% + ${finalX}px), calc(-50% + ${finalY}px))`,
        zIndex: isDragging ? 40 : isActive ? 20 : hovered ? 15 : effectiveWeight === 'focus' ? 12 : effectiveWeight === 'candidate' ? 10 : 8,
        transition: isDragging
          ? 'none'
          : isSnapping
          ? 'transform 0.45s cubic-bezier(0.34,1.56,0.64,1)'
          : 'opacity 0.6s ease, transform 0.3s ease',
        opacity,
        cursor: isDragging ? 'grabbing' : 'pointer',
        pointerEvents: 'auto',
        willChange: 'transform',
      }}
      onMouseDown={handleMouseDown}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => { setHovered(false); setShowLabel(false); }}
    >
      {/* Hover label — only for focus/candidate tiers */}
      {showLabel && !isActive && !isDragging && tierParams.labelVisible && (
        <div
          className="absolute whitespace-nowrap"
          style={{
            bottom: '100%',
            left: '50%',
            transform: 'translateX(-50%)',
            marginBottom: 6,
            background: 'rgba(4,6,14,0.95)',
            border: `1px solid ${config.color}30`,
            borderRadius: 5,
            padding: '3px 9px',
            fontSize: 10,
            color: config.color,
            letterSpacing: '0.12em',
            backdropFilter: 'blur(12px)',
            animation: 'labelFadeIn 0.18s ease',
            pointerEvents: 'none',
            zIndex: 50,
          }}
        >
          {config.label}
        </div>
      )}

      {/* Outer glow — scales with tier */}
      <div
        className="absolute rounded-full"
        style={{
          width: planetSize + 18 + pulse * (effectiveWeight === 'focus' ? 10 : 5),
          height: planetSize + 18 + pulse * (effectiveWeight === 'focus' ? 10 : 5),
          background: `radial-gradient(circle, ${config.glow} 0%, transparent 65%)`,
          opacity: isActive ? 0.9 : isDragging ? 0.75 : 0.35 + pulse * 0.18 * tierParams.glowMultiplier,
          pointerEvents: 'none',
          left: '50%',
          top: '50%',
          transform: 'translate(-50%, -50%)',
        }}
      />

      {/* Active ring — only for focus tier */}
      {isActive && tierParams.ringVisible && (
        <div
          className="absolute rounded-full"
          style={{
            width: planetSize + 28,
            height: planetSize + 28,
            border: `1.5px solid ${config.color}50`,
            animation: 'activeRingPulse 2s ease-in-out infinite',
            pointerEvents: 'none',
            left: '50%',
            top: '50%',
            transform: 'translate(-50%, -50%)',
          }}
        />
      )}

      {/* Active ring for focus tier (always show when active) */}
      {isActive && (
        <div
          className="absolute rounded-full"
          style={{
            width: planetSize + 28,
            height: planetSize + 28,
            border: `1.5px solid ${config.color}50`,
            animation: 'activeRingPulse 2s ease-in-out infinite',
            pointerEvents: 'none',
            left: '50%',
            top: '50%',
            transform: 'translate(-50%, -50%)',
          }}
        />
      )}

      {/* Snap indicator */}
      {isSnapping && (
        <div
          className="absolute rounded-full"
          style={{
            width: planetSize + 40,
            height: planetSize + 40,
            border: `1px solid ${config.color}55`,
            animation: 'snapRing 0.5s ease-out forwards',
            pointerEvents: 'none',
            left: '50%',
            top: '50%',
            transform: 'translate(-50%, -50%)',
          }}
        />
      )}

      {/* Planet body */}
      <div
        className="absolute rounded-full flex items-center justify-center"
        style={{
          width: planetSize,
          height: planetSize,
          left: '50%',
          top: '50%',
          transform: `translate(-50%, -50%) scale(${scale})`,
          background: effectiveWeight === 'dimmed'
            ? `radial-gradient(circle at 35% 30%, ${config.color}33 0%, ${config.color}10 100%)`
            : effectiveWeight === 'background'
            ? `radial-gradient(circle at 35% 30%, ${config.color}99 0%, ${config.color}44 50%, ${config.color}18 100%)`
            : `radial-gradient(circle at 35% 30%, ${config.color}cc 0%, ${config.color}55 50%, ${config.color}22 100%)`,
          border: `1px solid ${config.color}${isActive ? '80' : isDragging ? '70' : effectiveWeight === 'focus' ? '55' : effectiveWeight === 'candidate' ? '45' : effectiveWeight === 'background' ? '35' : '12'}`,
          boxShadow: isActive
            ? `0 0 36px ${config.glow}, 0 0 14px ${config.glow} inset, 0 0 70px ${config.glow}`
            : isDragging
            ? `0 0 28px ${config.glow}, 0 0 10px ${config.glow} inset`
            : `0 0 ${glowIntensity}px ${config.glow}, 0 0 4px ${config.glow} inset`,
          transition: isDragging
            ? 'transform 0.05s ease'
            : 'transform 0.35s cubic-bezier(0.34,1.56,0.64,1), box-shadow 0.4s ease',
          userSelect: 'none',
          pointerEvents: 'none',
        }}
      >
        {/* Inner shimmer — only for focus/candidate */}
        {(effectiveWeight === 'focus' || effectiveWeight === 'candidate') && (
          <div
            className="absolute inset-0 rounded-full"
            style={{
              background: `radial-gradient(circle at 30% 25%, rgba(255,255,255,${0.18 + pulse * 0.08}) 0%, transparent 55%)`,
              pointerEvents: 'none',
            }}
          />
        )}
        <i
          className={config.icon}
          style={{
            fontSize: planetSize * 0.38,
            color: `rgba(255,255,255,${tierParams.iconOpacity})`,
            position: 'relative',
            zIndex: 1,
            pointerEvents: 'none',
          }}
        />
      </div>

      {/* Active label */}
      {isActive && !isDragging && (
        <div
          className="absolute whitespace-nowrap"
          style={{
            top: '100%',
            left: '50%',
            transform: 'translateX(-50%)',
            marginTop: 4,
            fontSize: 9.5,
            color: config.color,
            letterSpacing: '0.2em',
            fontWeight: 600,
            animation: 'labelFadeIn 0.3s ease',
            pointerEvents: 'none',
          }}
        >
          {config.label.toUpperCase()}
        </div>
      )}

      {/* Drag label */}
      {isDragging && (
        <div
          className="absolute whitespace-nowrap"
          style={{
            top: '100%',
            left: '50%',
            transform: 'translateX(-50%)',
            marginTop: 4,
            fontSize: 9,
            color: config.color,
            letterSpacing: '0.18em',
            opacity: 0.7,
            pointerEvents: 'none',
          }}
        >
          松手吸附轨道
        </div>
      )}
    </div>
  );
}
