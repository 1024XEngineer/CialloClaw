import { useEffect, useState, useRef, useCallback } from 'react';

interface DashboardRingsProps {
  pulse: number;
  offset: { x: number; y: number };
  focusFragmentIndex?: number;
}

const ATTENTION_FRAGMENTS = [
  {
    id: 'f1',
    angle: 22,
    radius: 268,
    text: '刚保存了一份成果到工作区',
    subtext: '竞品分析报告 · 2 分钟前',
    color: 'rgba(52,211,153,0.85)',
    dotColor: '#34d399',
    icon: 'ri-save-line',
    priority: 'low' as const,
  },
  {
    id: 'f2',
    angle: 108,
    radius: 262,
    text: '你有一个 2 小时后到期的任务',
    subtext: '周报整理 · 今日 11:00',
    color: 'rgba(251,191,36,0.9)',
    dotColor: '#fbbf24',
    icon: 'ri-time-line',
    priority: 'high' as const,
  },
  {
    id: 'f3',
    angle: 195,
    radius: 270,
    text: '我总结了你这周反复出现的 3 类工作',
    subtext: '镜子洞察 · 点击查看',
    color: 'rgba(196,181,253,0.8)',
    dotColor: '#c4b5fd',
    icon: 'ri-eye-2-line',
    priority: 'medium' as const,
  },
  {
    id: 'f4',
    angle: 285,
    radius: 265,
    text: '当前网络波动，上传已延后重试',
    subtext: '感知到了 · 自动处理中',
    color: 'rgba(251,146,60,0.85)',
    dotColor: '#fb923c',
    icon: 'ri-wifi-off-line',
    priority: 'medium' as const,
  },
];

interface FragOffset {
  x: number;
  y: number;
}

interface UndoToast {
  id: string;
  label: string;
  timer: ReturnType<typeof setTimeout> | null;
}

export default function DashboardRings({ pulse, offset, focusFragmentIndex }: DashboardRingsProps) {
  const [slowRot, setSlowRot] = useState(0);
  const [microRot, setMicroRot] = useState(0);
  const [fragmentVisibility, setFragmentVisibility] = useState<number[]>([]);
  const [hoveredFrag, setHoveredFrag] = useState<string | null>(null);
  const [dismissedFrags, setDismissedFrags] = useState<Set<string>>(new Set());

  // Per-fragment drag offsets
  const [fragOffsets, setFragOffsets] = useState<Record<string, FragOffset>>(() =>
    Object.fromEntries(ATTENTION_FRAGMENTS.map(f => [f.id, { x: 0, y: 0 }]))
  );

  // Undo toast queue
  const [undoToasts, setUndoToasts] = useState<UndoToast[]>([]);

  // Drag state
  const draggingRef = useRef<{ id: string; startX: number; startY: number; origX: number; origY: number } | null>(null);
  const [draggingId, setDraggingId] = useState<string | null>(null);

  const rafRef = useRef<number>(0);

  // Stagger fragment appearance
  useEffect(() => {
    const timers: ReturnType<typeof setTimeout>[] = [];
    ATTENTION_FRAGMENTS.forEach((_, i) => {
      const t = setTimeout(() => {
        setFragmentVisibility(prev => [...prev, i]);
      }, 800 + i * 600);
      timers.push(t);
    });
    return () => timers.forEach(clearTimeout);
  }, []);

  useEffect(() => {
    const animate = () => {
      const now = Date.now() / 1000;
      setSlowRot(now * 3.5);
      setMicroRot(now * -2.2);
      rafRef.current = requestAnimationFrame(animate);
    };
    rafRef.current = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(rafRef.current);
  }, []);

  // Global mouse move / up for drag
  useEffect(() => {
    const handleMove = (e: MouseEvent) => {
      if (!draggingRef.current) return;
      const { id, startX, startY, origX, origY } = draggingRef.current;
      const dx = e.clientX - startX;
      const dy = e.clientY - startY;
      setFragOffsets(prev => ({ ...prev, [id]: { x: origX + dx, y: origY + dy } }));
    };
    const handleUp = () => {
      if (draggingRef.current) {
        draggingRef.current = null;
        setDraggingId(null);
      }
    };
    window.addEventListener('mousemove', handleMove);
    window.addEventListener('mouseup', handleUp);
    return () => {
      window.removeEventListener('mousemove', handleMove);
      window.removeEventListener('mouseup', handleUp);
    };
  }, []);

  const handleFragMouseDown = useCallback((e: React.MouseEvent, id: string) => {
    e.preventDefault();
    e.stopPropagation();
    const cur = fragOffsets[id] ?? { x: 0, y: 0 };
    draggingRef.current = { id, startX: e.clientX, startY: e.clientY, origX: cur.x, origY: cur.y };
    setDraggingId(id);
    setHoveredFrag(id);
  }, [fragOffsets]);

  const handleDismiss = useCallback((fragId: string, fragText: string) => {
    setDismissedFrags(prev => new Set([...prev, fragId]));
    setHoveredFrag(null);

    // Add undo toast
    const toastId = `toast-${Date.now()}`;
    const timer = setTimeout(() => {
      setUndoToasts(prev => prev.filter(t => t.id !== toastId));
    }, 4000);

    setUndoToasts(prev => [...prev, { id: toastId, label: fragText, timer }]);
  }, []);

  const handleUndo = useCallback((toastId: string, fragId: string) => {
    // Clear timer
    setUndoToasts(prev => {
      const toast = prev.find(t => t.id === toastId);
      if (toast?.timer) clearTimeout(toast.timer);
      return prev.filter(t => t.id !== toastId);
    });
    // Restore fragment
    setDismissedFrags(prev => {
      const next = new Set(prev);
      next.delete(fragId);
      return next;
    });
  }, []);

  return (
    <>
      {/* ── Outermost ambient consciousness field ── */}
      <div
        className="absolute rounded-full pointer-events-none"
        style={{
          width: 700, height: 700,
          left: '50%', top: '50%',
          transform: `translate(calc(-50% + ${offset.x * 0.07}px), calc(-50% + ${offset.y * 0.07}px))`,
          background: 'radial-gradient(circle, rgba(30,40,80,0.06) 0%, transparent 60%)',
          opacity: 0.5 + pulse * 0.25,
        }}
      />

      {/* ── Outer guide ring ── */}
      <div
        className="absolute rounded-full pointer-events-none"
        style={{
          width: 540, height: 540,
          left: '50%', top: '50%',
          transform: `translate(calc(-50% + ${offset.x * 0.1}px), calc(-50% + ${offset.y * 0.1}px)) rotate(${slowRot}deg)`,
          border: '1px solid rgba(255,255,255,0.025)',
        }}
      >
        <div className="absolute rounded-full" style={{
          width: 3, height: 3,
          background: 'rgba(52,211,153,0.45)',
          boxShadow: '0 0 5px rgba(52,211,153,0.35)',
          top: -1.5, left: '50%',
          transform: 'translateX(-50%)',
        }} />
        <div className="absolute rounded-full" style={{
          width: 2, height: 2,
          background: 'rgba(167,139,250,0.35)',
          bottom: -1, left: '50%',
          transform: 'translateX(-50%)',
        }} />
      </div>

      {/* ── Dynamic attention fragments ── */}
      {ATTENTION_FRAGMENTS.map((frag, i) => {
        const rad = (frag.angle * Math.PI) / 180;
        const isDismissed = dismissedFrags.has(frag.id);
        const isVisible = fragmentVisibility.includes(i) && !isDismissed;
        const isFocused = focusFragmentIndex === i;
        const isHovered = hoveredFrag === frag.id;
        const isDragging = draggingId === frag.id;
        const focusRadius = isFocused ? frag.radius - 40 : frag.radius;
        const baseX = Math.cos(rad) * focusRadius + offset.x * 0.05;
        const baseY = Math.sin(rad) * focusRadius + offset.y * 0.05;
        const dragOff = fragOffsets[frag.id] ?? { x: 0, y: 0 };
        const x = baseX + dragOff.x;
        const y = baseY + dragOff.y;

        const baseOpacity = frag.priority === 'high' ? 0.72 : frag.priority === 'medium' ? 0.52 : 0.38;
        const opacity = !isVisible ? 0 : isDismissed ? 0 : isFocused ? 0.95 : isHovered || isDragging ? 0.92 : baseOpacity + pulse * 0.06;

        return (
          <div
            key={frag.id}
            className="absolute"
            style={{
              left: '50%', top: '50%',
              transform: `translate(calc(-50% + ${x}px), calc(-50% + ${y}px)) scale(${isDismissed ? 0.7 : isFocused ? 1.08 : isDragging ? 1.06 : isHovered ? 1.04 : 1})`,
              opacity,
              transition: isDragging ? 'opacity 0.15s ease, transform 0.1s ease' : 'opacity 0.5s cubic-bezier(0.16,1,0.3,1), transform 0.5s cubic-bezier(0.34,1.56,0.64,1)',
              zIndex: isDragging ? 20 : isFocused ? 8 : isHovered ? 7 : 5,
              cursor: isDragging ? 'grabbing' : 'grab',
              pointerEvents: isVisible ? 'auto' : 'none',
              userSelect: 'none',
            }}
            onMouseEnter={() => !isDragging && setHoveredFrag(frag.id)}
            onMouseLeave={() => !isDragging && setHoveredFrag(null)}
            onMouseDown={e => handleFragMouseDown(e, frag.id)}
          >
            <div
              className="relative flex flex-col items-start gap-0.5"
              style={{
                maxWidth: 165,
                padding: '5px 8px',
                paddingRight: isHovered || isDragging ? 22 : 8,
                background: isFocused || isHovered || isDragging ? 'rgba(3,5,12,0.88)' : 'transparent',
                border: isFocused || isHovered || isDragging ? `1px solid ${frag.dotColor}28` : '1px solid transparent',
                borderRadius: 4,
                backdropFilter: isFocused || isHovered || isDragging ? 'blur(12px)' : 'none',
                transition: 'background 0.22s ease, border-color 0.22s ease, padding-right 0.18s ease',
              }}
            >
              {/* Drag handle hint — top-left micro dots */}
              {(isHovered || isDragging) && (
                <div
                  className="absolute flex flex-col gap-0.5"
                  style={{ left: 3, top: '50%', transform: 'translateY(-50%)', opacity: 0.3, pointerEvents: 'none' }}
                >
                  {[0, 1, 2].map(r => (
                    <div key={r} className="flex gap-0.5">
                      {[0, 1].map(c => (
                        <div key={c} className="rounded-full" style={{ width: 2, height: 2, background: frag.dotColor }} />
                      ))}
                    </div>
                  ))}
                </div>
              )}

              {/* Priority indicator */}
              {frag.priority === 'high' && (
                <div className="flex items-center gap-1 mb-0.5">
                  <div className="rounded-full" style={{
                    width: 4, height: 4,
                    background: frag.dotColor,
                    boxShadow: `0 0 5px ${frag.dotColor}`,
                    animation: 'notifPulse 1.5s ease-in-out infinite',
                  }} />
                  <span style={{ fontSize: 7, color: frag.dotColor, letterSpacing: '0.18em', fontWeight: 600 }}>
                    URGENT
                  </span>
                </div>
              )}

              <div className="flex items-start gap-1.5" style={{ paddingLeft: isHovered || isDragging ? 8 : 0, transition: 'padding-left 0.18s ease' }}>
                <i className={frag.icon} style={{ fontSize: 9, color: frag.color, marginTop: 1, flexShrink: 0 }} />
                <span style={{
                  fontSize: 9.5,
                  color: frag.color,
                  letterSpacing: '0.03em',
                  lineHeight: 1.4,
                  fontWeight: frag.priority === 'high' ? 500 : 400,
                }}>
                  {frag.text}
                </span>
              </div>

              <span style={{
                fontSize: 8,
                color: 'rgba(148,163,184,0.4)',
                letterSpacing: '0.06em',
                paddingLeft: isHovered || isDragging ? 22 : 14,
                transition: 'padding-left 0.18s ease',
              }}>
                {frag.subtext}
              </span>

              {/* Dismiss button */}
              {(isHovered || isDragging) && (
                <button
                  onClick={e => {
                    e.stopPropagation();
                    handleDismiss(frag.id, frag.text);
                  }}
                  onMouseDown={e => e.stopPropagation()}
                  className="absolute flex items-center justify-center cursor-pointer"
                  style={{
                    top: 4, right: 4,
                    width: 14, height: 14,
                    borderRadius: '50%',
                    background: 'rgba(255,255,255,0.06)',
                    border: '1px solid rgba(255,255,255,0.1)',
                    color: 'rgba(148,163,184,0.5)',
                    fontSize: 8,
                    lineHeight: 1,
                    transition: 'background 0.15s ease, color 0.15s ease',
                  }}
                  onMouseEnter={e => {
                    (e.currentTarget as HTMLButtonElement).style.background = 'rgba(251,113,133,0.15)';
                    (e.currentTarget as HTMLButtonElement).style.borderColor = 'rgba(251,113,133,0.3)';
                    (e.currentTarget as HTMLButtonElement).style.color = '#fb7185';
                  }}
                  onMouseLeave={e => {
                    (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.06)';
                    (e.currentTarget as HTMLButtonElement).style.borderColor = 'rgba(255,255,255,0.1)';
                    (e.currentTarget as HTMLButtonElement).style.color = 'rgba(148,163,184,0.5)';
                  }}
                >
                  <i className="ri-close-line" style={{ fontSize: 8 }} />
                </button>
              )}
            </div>
          </div>
        );
      })}

      {/* ── Inner micro ring ── */}
      <div
        className="absolute rounded-full pointer-events-none"
        style={{
          width: 430, height: 430,
          left: '50%', top: '50%',
          transform: `translate(calc(-50% + ${offset.x * 0.05}px), calc(-50% + ${offset.y * 0.05}px)) rotate(${microRot}deg)`,
          border: '1px dashed rgba(255,255,255,0.02)',
        }}
      >
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="absolute" style={{
            top: 0, left: '50%',
            width: 1, height: 4,
            background: 'rgba(255,255,255,0.05)',
            transform: `rotate(${i * 60}deg) translateY(-2px)`,
            transformOrigin: 'center 215px',
          }} />
        ))}
      </div>

      {/* ── Ambient particles ── */}
      {Array.from({ length: 4 }).map((_, i) => {
        const a = (i / 4) * Math.PI * 2 + (Date.now() / 8000);
        const r = 200 + Math.sin(i * 2.1) * 20;
        const px = Math.cos(a) * r + offset.x * 0.03;
        const py = Math.sin(a) * r + offset.y * 0.03;
        return (
          <div key={i} className="absolute rounded-full pointer-events-none" style={{
            width: 2, height: 2,
            left: `calc(50% + ${px}px)`,
            top: `calc(50% + ${py}px)`,
            transform: 'translate(-50%, -50%)',
            background: i % 2 === 0 ? 'rgba(52,211,153,0.4)' : 'rgba(167,139,250,0.4)',
            opacity: 0.15 + pulse * 0.12,
            animation: `particlePulse ${2.8 + i * 0.5}s ease-in-out infinite`,
            animationDelay: `${i * 0.4}s`,
          }} />
        );
      })}

      {/* ── Undo toast stack — bottom center ── */}
      <div
        className="fixed flex flex-col items-center gap-2 pointer-events-none"
        style={{ bottom: 32, left: '50%', transform: 'translateX(-50%)', zIndex: 100 }}
      >
        {undoToasts.map((toast, idx) => (
          <div
            key={toast.id}
            className="flex items-center gap-3 pointer-events-auto"
            style={{
              padding: '7px 14px 7px 12px',
              background: 'rgba(15,20,35,0.92)',
              border: '1px solid rgba(255,255,255,0.1)',
              borderRadius: 20,
              backdropFilter: 'blur(16px)',
              animation: 'toastIn 0.35s cubic-bezier(0.16,1,0.3,1)',
              opacity: 1 - idx * 0.15,
              transform: `translateY(${-idx * 4}px) scale(${1 - idx * 0.03})`,
              transition: 'opacity 0.3s ease, transform 0.3s ease',
              whiteSpace: 'nowrap',
            }}
          >
            <i className="ri-eye-off-line" style={{ fontSize: 11, color: 'rgba(148,163,184,0.5)' }} />
            <span style={{ fontSize: 11, color: 'rgba(148,163,184,0.65)', letterSpacing: '0.04em' }}>
              已隐藏
            </span>
            <span
              style={{
                fontSize: 10,
                color: 'rgba(148,163,184,0.35)',
                maxWidth: 120,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              }}
            >
              · {toast.label.length > 14 ? toast.label.slice(0, 14) + '…' : toast.label}
            </span>
            <div style={{ width: 1, height: 12, background: 'rgba(255,255,255,0.08)' }} />
            <button
              onClick={() => {
                const frag = ATTENTION_FRAGMENTS.find(f => f.text === toast.label);
                if (frag) handleUndo(toast.id, frag.id);
              }}
              className="cursor-pointer transition-all duration-150 whitespace-nowrap"
              style={{
                fontSize: 11,
                color: 'rgba(148,163,184,0.7)',
                background: 'none',
                border: 'none',
                padding: 0,
                letterSpacing: '0.06em',
              }}
              onMouseEnter={e => { (e.currentTarget as HTMLButtonElement).style.color = '#e2e8f0'; }}
              onMouseLeave={e => { (e.currentTarget as HTMLButtonElement).style.color = 'rgba(148,163,184,0.7)'; }}
            >
              撤销
            </button>
          </div>
        ))}
      </div>

      <style>{`
        @keyframes particlePulse {
          0%, 100% { opacity: 0.12; transform: translate(-50%, -50%) scale(1); }
          50% { opacity: 0.35; transform: translate(-50%, -50%) scale(1.5); }
        }
        @keyframes toastIn {
          from { opacity: 0; transform: translateY(10px) scale(0.94); }
          to { opacity: 1; transform: translateY(0) scale(1); }
        }
      `}</style>
    </>
  );
}
