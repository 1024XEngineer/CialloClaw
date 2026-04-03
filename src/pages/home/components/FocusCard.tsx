import { useEffect, useState } from 'react';
import type { AgentStateData } from '@/mocks/agentStates';

interface FocusCardProps {
  stateData: AgentStateData;
  visible: boolean;
}

const tagStyles: Record<string, { bg: string; color: string }> = {
  normal:    { bg: 'rgba(100,116,139,0.12)', color: '#94a3b8' },
  active:    { bg: 'rgba(52,211,153,0.12)',  color: '#34d399' },
  highlight: { bg: 'rgba(251,191,36,0.12)',  color: '#fbbf24' },
  warn:      { bg: 'rgba(251,146,60,0.12)',  color: '#fb923c' },
  error:     { bg: 'rgba(251,113,133,0.12)', color: '#fb7185' },
  done:      { bg: 'rgba(45,212,191,0.1)',   color: '#2dd4bf' },
  mirror:    { bg: 'rgba(196,181,253,0.12)', color: '#c4b5fd' },
  sense:     { bg: 'rgba(56,189,248,0.1)',   color: '#38bdf8' },
};

const moduleIcons: Record<string, string> = {
  task:    'ri-robot-line',
  notepad: 'ri-sticky-note-line',
  mirror:  'ri-eye-2-line',
  sense:   'ri-pulse-line',
};

export default function FocusCard({ stateData, visible }: FocusCardProps) {
  const [show, setShow] = useState(false);
  const [textShow, setTextShow] = useState(false);
  const ts = tagStyles[stateData.tagType] || tagStyles.normal;

  useEffect(() => {
    if (visible) {
      const t1 = setTimeout(() => setShow(true), 60);
      const t2 = setTimeout(() => setTextShow(true), 280);
      return () => { clearTimeout(t1); clearTimeout(t2); };
    }
    setShow(false);
    setTextShow(false);
  }, [visible, stateData.key]);

  return (
    <div
      style={{
        opacity: show ? 1 : 0,
        transform: show ? 'translateY(0)' : 'translateY(10px)',
        transition: 'opacity 0.45s ease, transform 0.45s ease',
      }}
    >
      {/* Module + Tag row */}
      <div className="flex items-center gap-2 mb-3">
        {/* Module indicator */}
        <div
          className="flex items-center justify-center rounded-full"
          style={{
            width: 18,
            height: 18,
            background: `${ts.color}15`,
          }}
        >
          <i
            className={moduleIcons[stateData.module] || 'ri-robot-line'}
            style={{ fontSize: 9, color: ts.color }}
          />
        </div>

        {/* Tag pill */}
        <div
          className="px-2.5 py-0.5 rounded-full flex items-center gap-1.5"
          style={{ background: ts.bg }}
        >
          <div
            className="rounded-full"
            style={{
              width: 4,
              height: 4,
              background: ts.color,
              boxShadow: `0 0 4px ${ts.color}`,
              animation: ['active', 'error', 'highlight', 'sense'].includes(stateData.tagType)
                ? 'notifPulse 1.8s ease-in-out infinite' : 'none',
            }}
          />
          <span style={{ fontSize: 9.5, color: ts.color, letterSpacing: '0.18em', fontWeight: 600 }}>
            {stateData.tag.toUpperCase()}
          </span>
        </div>
      </div>

      {/* Headline */}
      <h2
        style={{
          fontSize: 18,
          fontWeight: 300,
          color: 'rgba(241,245,249,0.95)',
          lineHeight: 1.45,
          letterSpacing: '-0.01em',
          marginBottom: 10,
          opacity: textShow ? 1 : 0,
          transform: textShow ? 'translateY(0)' : 'translateY(6px)',
          transition: 'opacity 0.4s ease 0.08s, transform 0.4s ease 0.08s',
        }}
      >
        {stateData.headline}
      </h2>

      {/* Subline */}
      <p
        style={{
          fontSize: 12.5,
          color: 'rgba(203,213,225,0.78)',
          lineHeight: 1.75,
          opacity: textShow ? 1 : 0,
          transform: textShow ? 'translateY(0)' : 'translateY(5px)',
          transition: 'opacity 0.4s ease 0.18s, transform 0.4s ease 0.18s',
        }}
      >
        {stateData.subline}
      </p>

      {/* Accent underline */}
      <div
        style={{
          height: 1,
          marginTop: 14,
          background: `linear-gradient(to right, ${stateData.accentColor}50, transparent)`,
          width: textShow ? '100%' : '0%',
          transition: 'width 0.7s ease 0.35s',
        }}
      />
    </div>
  );
}
