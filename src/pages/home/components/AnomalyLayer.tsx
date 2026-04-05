import { useEffect, useState } from 'react';
import type { AgentStateData } from '@/mocks/agentStates';

interface AnomalyLayerProps {
  stateData: AgentStateData;
  visible: boolean;
  onAction: () => void;
  onDismiss: () => void;
}

const severityConfig = {
  error: {
    bg: 'rgba(251,113,133,0.07)',
    border: 'rgba(251,113,133,0.2)',
    iconBg: 'rgba(251,113,133,0.12)',
    iconColor: '#fb7185',
    icon: 'ri-error-warning-line',
    actionBg: 'rgba(251,113,133,0.15)',
    actionColor: '#fb7185',
    actionHover: 'rgba(251,113,133,0.25)',
  },
  warn: {
    bg: 'rgba(251,146,60,0.06)',
    border: 'rgba(251,146,60,0.18)',
    iconBg: 'rgba(251,146,60,0.12)',
    iconColor: '#fb923c',
    icon: 'ri-alert-line',
    actionBg: 'rgba(251,146,60,0.12)',
    actionColor: '#fb923c',
    actionHover: 'rgba(251,146,60,0.22)',
  },
  info: {
    bg: 'rgba(45,212,191,0.05)',
    border: 'rgba(45,212,191,0.15)',
    iconBg: 'rgba(45,212,191,0.1)',
    iconColor: '#2dd4bf',
    icon: 'ri-information-line',
    actionBg: 'rgba(45,212,191,0.12)',
    actionColor: '#2dd4bf',
    actionHover: 'rgba(45,212,191,0.22)',
  },
};

export default function AnomalyLayer({ stateData, visible, onAction, onDismiss }: AnomalyLayerProps) {
  const [show, setShow] = useState(false);
  const [actionHovered, setActionHovered] = useState(false);
  const anomaly = stateData.anomaly;

  useEffect(() => {
    if (visible && anomaly) {
      const t = setTimeout(() => setShow(true), 320);
      return () => clearTimeout(t);
    }
    setShow(false);
  }, [visible, stateData.key, anomaly]);

  if (!anomaly) return null;

  const cfg = severityConfig[anomaly.severity];

  return (
    <div
      className="rounded-2xl overflow-hidden"
      style={{
        background: cfg.bg,
        border: `1px solid ${cfg.border}`,
        opacity: show ? 1 : 0,
        transform: show ? 'translateY(0) scale(1)' : 'translateY(8px) scale(0.98)',
        transition: 'opacity 0.4s ease, transform 0.4s ease',
      }}
    >
      {/* Top accent line */}
      <div
        style={{
          height: 1,
          background: `linear-gradient(to right, ${cfg.iconColor}60, transparent)`,
        }}
      />

      <div className="px-4 py-3.5">
        {/* Header */}
        <div className="flex items-start gap-3 mb-2.5">
          <div
            className="flex items-center justify-center rounded-full flex-shrink-0 mt-0.5"
            style={{ width: 24, height: 24, background: cfg.iconBg }}
          >
            <i
              className={cfg.icon}
              style={{
                fontSize: 12,
                color: cfg.iconColor,
                animation: anomaly.severity === 'error' ? 'notifPulse 1.5s ease-in-out infinite' : 'none',
              }}
            />
          </div>
          <div>
            <div style={{ fontSize: 12.5, color: 'rgba(226,232,240,0.9)', fontWeight: 500, lineHeight: 1.4 }}>
              {anomaly.title}
            </div>
            <p style={{ fontSize: 11.5, color: 'rgba(148,163,184,0.65)', lineHeight: 1.65, marginTop: 3 }}>
              {anomaly.desc}
            </p>
          </div>
        </div>

        {/* Actions */}
        <div className="flex items-center gap-2 mt-3">
          <button
            onClick={onAction}
            className="flex-1 py-2 rounded-xl text-center cursor-pointer transition-all duration-200 whitespace-nowrap"
            style={{
              background: actionHovered ? cfg.actionHover : cfg.actionBg,
              fontSize: 11.5,
              color: cfg.actionColor,
              fontWeight: 500,
              border: `1px solid ${cfg.border}`,
              letterSpacing: '0.04em',
            }}
            onMouseEnter={() => setActionHovered(true)}
            onMouseLeave={() => setActionHovered(false)}
          >
            {anomaly.actionLabel}
          </button>
          <button
            onClick={onDismiss}
            className="px-4 py-2 rounded-xl cursor-pointer transition-all duration-200 whitespace-nowrap"
            style={{
              background: 'rgba(255,255,255,0.04)',
              fontSize: 11.5,
              color: 'rgba(100,116,139,0.7)',
              border: '1px solid rgba(255,255,255,0.06)',
            }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.07)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.04)'; }}
          >
            {anomaly.dismissLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
