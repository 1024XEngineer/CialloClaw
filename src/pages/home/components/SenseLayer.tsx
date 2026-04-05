import { useEffect, useState, useRef } from 'react';
import type { AgentStateData, SenseSignal } from '@/mocks/agentStates';

interface SenseLayerProps {
  stateData: AgentStateData;
  visible: boolean;
}

const levelConfig: Record<SenseSignal['level'], { color: string; barColor: string }> = {
  normal:   { color: '#34d399', barColor: '#34d399' },
  warn:     { color: '#fb923c', barColor: '#fb923c' },
  critical: { color: '#fb7185', barColor: '#fb7185' },
};

function SignalBar({ signal, show, delay }: { signal: SenseSignal; show: boolean; delay: number }) {
  const cfg = levelConfig[signal.level];
  const [animated, setAnimated] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (show) {
      timerRef.current = setTimeout(() => setAnimated(true), delay + 200);
    } else {
      setAnimated(false);
    }
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [show, delay]);

  // Parse numeric value for bar width
  const numericVal = parseFloat(signal.value);
  const barWidth = isNaN(numericVal) ? 50 : Math.min(numericVal, 100);

  return (
    <div
      className="flex items-center gap-3"
      style={{
        opacity: show ? 1 : 0,
        transform: show ? 'translateX(0)' : 'translateX(-6px)',
        transition: `opacity 0.35s ease ${delay}ms, transform 0.35s ease ${delay}ms`,
      }}
    >
      {/* Icon */}
      <div
        className="flex items-center justify-center rounded-full flex-shrink-0"
        style={{
          width: 22,
          height: 22,
          background: `${cfg.color}15`,
          border: `1px solid ${cfg.color}25`,
        }}
      >
        <i className={signal.icon} style={{ fontSize: 10, color: cfg.color }} />
      </div>

      {/* Label + bar */}
      <div className="flex-1">
        <div className="flex items-center justify-between mb-1">
          <span style={{ fontSize: 10.5, color: 'rgba(148,163,184,0.7)', letterSpacing: '0.06em' }}>
            {signal.label}
          </span>
          <span style={{ fontSize: 10.5, color: cfg.color, fontWeight: 500 }}>
            {signal.value}
          </span>
        </div>
        {/* Bar track */}
        <div
          className="rounded-full overflow-hidden"
          style={{ height: 3, background: 'rgba(255,255,255,0.06)' }}
        >
          <div
            className="h-full rounded-full"
            style={{
              width: animated ? `${barWidth}%` : '0%',
              background: `linear-gradient(to right, ${cfg.barColor}80, ${cfg.barColor})`,
              transition: 'width 0.7s cubic-bezier(0.16,1,0.3,1)',
              boxShadow: signal.level !== 'normal' ? `0 0 6px ${cfg.barColor}60` : 'none',
            }}
          />
        </div>
        {/* Translation */}
        {signal.translation && (
          <div style={{ fontSize: 9.5, color: 'rgba(71,85,105,0.65)', marginTop: 2 }}>
            {signal.translation}
          </div>
        )}
      </div>
    </div>
  );
}

export default function SenseLayer({ stateData, visible }: SenseLayerProps) {
  const [show, setShow] = useState(false);
  const signals = stateData.senseSignals ?? [];

  useEffect(() => {
    if (visible) {
      const t = setTimeout(() => setShow(true), 100);
      return () => clearTimeout(t);
    }
    setShow(false);
  }, [visible, stateData.key]);

  if (!signals.length) return null;

  return (
    <div
      style={{
        opacity: show ? 1 : 0,
        transform: show ? 'translateY(0)' : 'translateY(8px)',
        transition: 'opacity 0.4s ease, transform 0.4s ease',
      }}
    >
      {/* Section label */}
      <div className="flex items-center gap-2 mb-3">
        <div
          className="w-4 h-px"
          style={{ background: `linear-gradient(to right, ${stateData.accentColor}60, transparent)` }}
        />
        <span
          style={{
            fontSize: 9.5,
            color: stateData.accentColor,
            letterSpacing: '0.2em',
            fontWeight: 600,
            opacity: 0.8,
          }}
        >
          硬件感知 · 系统生命体征
        </span>
      </div>

      {/* Signal bars */}
      <div className="flex flex-col gap-3">
        {signals.map((signal, i) => (
          <SignalBar
            key={signal.label}
            signal={signal}
            show={show}
            delay={i * 80}
          />
        ))}
      </div>

      {/* Agent translation */}
      {stateData.senseAction && (
        <div
          className="mt-3 px-3 py-2.5 rounded-xl flex items-start gap-2.5"
          style={{
            background: `${stateData.accentColor}0a`,
            border: `1px solid ${stateData.accentColor}18`,
            opacity: show ? 1 : 0,
            transition: 'opacity 0.4s ease 0.4s',
          }}
        >
          <i
            className="ri-translate-2"
            style={{ fontSize: 11, color: stateData.accentColor, marginTop: 1, flexShrink: 0 }}
          />
          <p style={{ fontSize: 11.5, color: 'rgba(148,163,184,0.7)', lineHeight: 1.65 }}>
            {stateData.senseAction}
          </p>
        </div>
      )}
    </div>
  );
}
