import { useEffect, useState } from 'react';
import type { AgentStateData } from '@/mocks/agentStates';

interface ContextStreamProps {
  stateData: AgentStateData;
  visible: boolean;
}

const typeStyle: Record<string, { color: string; iconColor: string }> = {
  normal: { color: 'rgba(203,213,225,0.82)',  iconColor: 'rgba(148,163,184,0.7)' },
  active: { color: 'rgba(241,245,249,0.92)', iconColor: '#34d399' },
  warn:   { color: 'rgba(251,146,60,0.92)',  iconColor: '#fb923c' },
  error:  { color: 'rgba(251,113,133,0.92)', iconColor: '#fb7185' },
  hint:   { color: 'rgba(148,163,184,0.72)', iconColor: 'rgba(100,116,139,0.6)' },
};

export default function ContextStream({ stateData, visible }: ContextStreamProps) {
  const [show, setShow] = useState(false);
  const items = stateData.context;

  useEffect(() => {
    if (visible) {
      const t = setTimeout(() => setShow(true), 200);
      return () => clearTimeout(t);
    }
    setShow(false);
  }, [visible, stateData.key]);

  if (!items.length) return null;

  return (
    <div
      style={{
        opacity: show ? 1 : 0,
        transform: show ? 'translateY(0)' : 'translateY(6px)',
        transition: 'opacity 0.4s ease, transform 0.4s ease',
      }}
    >
      {/* Section label */}
      <div className="flex items-center gap-2 mb-2.5">
        <div
          className="w-3 h-px"
          style={{ background: 'rgba(255,255,255,0.1)' }}
        />
        <span style={{ fontSize: 9, color: 'rgba(148,163,184,0.5)', letterSpacing: '0.18em' }}>
          上下文
        </span>
      </div>

      {/* Items */}
      <div className="flex flex-col gap-1.5">
        {items.map((item, i) => {
          const ts = typeStyle[item.type || 'normal'];
          return (
            <div
              key={i}
              className="flex items-center gap-2.5"
              style={{
                opacity: show ? 1 : 0,
                transform: show ? 'translateX(0)' : 'translateX(-6px)',
                transition: `opacity 0.35s ease ${i * 0.08}s, transform 0.35s ease ${i * 0.08}s`,
              }}
            >
              {/* Icon */}
              <div
                className="flex items-center justify-center flex-shrink-0"
                style={{ width: 16, height: 16 }}
              >
                <i
                  className={item.icon}
                  style={{
                    fontSize: 11,
                    color: ts.iconColor,
                    animation: item.type === 'active' ? 'spin 1.5s linear infinite' : 'none',
                  }}
                />
              </div>

              {/* Text */}
              <span
                style={{
                  fontSize: 11.5,
                  color: ts.color,
                  flex: 1,
                  lineHeight: 1.5,
                }}
              >
                {item.text}
              </span>

              {/* Time */}
              {item.time && (
                <span
                  style={{
                    fontSize: 9.5,
                    color: 'rgba(148,163,184,0.55)',
                    flexShrink: 0,
                    letterSpacing: '0.04em',
                  }}
                >
                  {item.time}
                </span>
              )}
            </div>
          );
        })}
      </div>

      <style>{`
        @keyframes spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
}
