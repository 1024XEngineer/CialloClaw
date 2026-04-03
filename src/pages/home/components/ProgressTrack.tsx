import { useEffect, useState } from 'react';
import type { AgentStateData } from '@/mocks/agentStates';

interface ProgressTrackProps {
  stateData: AgentStateData;
  visible: boolean;
}

const stepStatusStyle: Record<string, { dot: string; line: string; label: string }> = {
  done:    { dot: '#34d399', line: '#34d39940', label: 'rgba(148,163,184,0.6)' },
  active:  { dot: '#ffffff', line: 'transparent', label: 'rgba(241,245,249,0.9)' },
  pending: { dot: '#1e293b', line: 'transparent', label: 'rgba(71,85,105,0.5)' },
  error:   { dot: '#fb7185', line: '#fb718540', label: 'rgba(251,113,133,0.85)' },
};

export default function ProgressTrack({ stateData, visible }: ProgressTrackProps) {
  const [show, setShow] = useState(false);
  const [barWidth, setBarWidth] = useState(0);
  const steps = stateData.progressSteps ?? [];
  const progress = stateData.progress ?? 0;

  useEffect(() => {
    if (visible) {
      const t1 = setTimeout(() => setShow(true), 160);
      const t2 = setTimeout(() => setBarWidth(progress), 400);
      return () => { clearTimeout(t1); clearTimeout(t2); };
    }
    setShow(false);
    setBarWidth(0);
  }, [visible, stateData.key, progress]);

  if (!steps.length) return null;

  return (
    <div
      style={{
        opacity: show ? 1 : 0,
        transform: show ? 'translateY(0)' : 'translateY(6px)',
        transition: 'opacity 0.4s ease, transform 0.4s ease',
      }}
    >
      {/* Progress bar */}
      {progress > 0 && (
        <div className="mb-3">
          <div className="flex items-center justify-between mb-1.5">
            <span style={{ fontSize: 9.5, color: 'rgba(71,85,105,0.7)', letterSpacing: '0.14em' }}>
              {stateData.progressLabel?.toUpperCase()}
            </span>
            <span style={{ fontSize: 9.5, color: stateData.accentColor, fontWeight: 500 }}>
              {progress}%
            </span>
          </div>
          <div
            className="rounded-full overflow-hidden"
            style={{ height: 2, background: 'rgba(255,255,255,0.06)' }}
          >
            <div
              className="h-full rounded-full"
              style={{
                width: `${barWidth}%`,
                background: `linear-gradient(to right, ${stateData.accentColor}60, ${stateData.accentColor})`,
                transition: 'width 0.9s cubic-bezier(0.16,1,0.3,1)',
                boxShadow: `0 0 8px ${stateData.accentColor}50`,
              }}
            />
          </div>
        </div>
      )}

      {/* Steps */}
      <div className="flex items-center gap-0">
        {steps.map((step, i) => {
          const style = stepStatusStyle[step.status] || stepStatusStyle.pending;
          const isLast = i === steps.length - 1;
          return (
            <div key={i} className="flex items-center" style={{ flex: isLast ? '0 0 auto' : 1 }}>
              {/* Step node */}
              <div className="flex flex-col items-center gap-1">
                <div
                  className="rounded-full flex items-center justify-center"
                  style={{
                    width: step.status === 'active' ? 8 : 6,
                    height: step.status === 'active' ? 8 : 6,
                    background: style.dot,
                    boxShadow: step.status === 'active'
                      ? `0 0 8px ${stateData.accentColor}, 0 0 16px ${stateData.accentColor}50`
                      : step.status === 'error'
                      ? `0 0 6px #fb7185`
                      : 'none',
                    border: step.status === 'pending' ? '1px solid rgba(255,255,255,0.12)' : 'none',
                    animation: step.status === 'active' ? 'notifPulse 2s ease-in-out infinite' : 'none',
                    transition: 'all 0.3s ease',
                  }}
                />
                <span
                  style={{
                    fontSize: 9,
                    color: style.label,
                    letterSpacing: '0.04em',
                    whiteSpace: 'nowrap',
                    maxWidth: 60,
                    textAlign: 'center',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                  }}
                >
                  {step.label}
                </span>
              </div>

              {/* Connector line */}
              {!isLast && (
                <div
                  className="flex-1 mx-1"
                  style={{
                    height: 1,
                    background: step.status === 'done'
                      ? `linear-gradient(to right, ${stateData.accentColor}40, ${stateData.accentColor}20)`
                      : 'rgba(255,255,255,0.06)',
                    marginBottom: 14,
                    transition: 'background 0.4s ease',
                  }}
                />
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
