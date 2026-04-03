import { useEffect, useRef, useState, useCallback } from 'react';
import type { AgentStateData } from '@/mocks/agentStates';

const LONG_PRESS_DURATION = 650; // ms

interface CenterOrbProps {
  stateData: AgentStateData;
  activePlanetColor: string | null;
  pulse: number;
  onDragOffset?: (dx: number, dy: number) => void;
  onLongPress?: () => void;
  focusMode?: boolean;
}

// Agent status messages — what the agent "says" proactively
const agentMessages: Record<string, string[]> = {
  working: ['正在整理关键信息', '已扫描 47 个来源', '专注推进中'],
  highlight: ['发现 2 个值得关注的风险点', '竞品定价出现重大调整', '已标记为优先项'],
  completing: ['草稿已就绪', '等待你确认', '随时可以发送'],
  done: ['任务已完成', '共处理 3 个议题', '随时待命'],
  standby: ['随时准备好了', '持续监听中', '等待你的指令'],
  idle_present: ['我在这里', '持续关注中', '上次任务 23 分钟前完成'],
  error_permission: ['需要你的介入', '缺少访问权限', '等待授权后继续'],
  error_blocked: ['当前步骤被阻塞', '需要你决定', '跳过还是等待？'],
  error_missing_info: ['还差一项关键信息', '需要你补充', '补充后立即继续'],
  notepad_processing: ['正在处理你的便签', '发现 2 项可执行任务', '1 项需要确认优先级'],
  notepad_reminder: ['你每周一都会整理周报', '数据摘要已就绪', '可以直接开始'],
  scheduled_task: ['每日巡检已完成', '发现 1 封重要邮件', '已标记为优先处理'],
  mirror_summary: ['我帮你总结了近两周', '高效时段 10-12 点', '周四是你的深度工作日'],
  mirror_habit: ['你形成了新的工作节奏', '周四下午深度工作', '已调整为低打扰模式'],
  sense_alert: ['系统负荷过高', 'CPU 持续高负载 8 分钟', '建议暂缓大文件处理'],
  sense_suggestion: ['系统状态良好', 'CPU 空闲率 78%', '适合启动大型任务'],
};

export default function CenterOrb({ stateData, activePlanetColor, pulse, onDragOffset, onLongPress, focusMode = false }: CenterOrbProps) {
  const [rotation, setRotation] = useState(0);
  const [breathScale, setBreathScale] = useState(1);
  const [messageIdx, setMessageIdx] = useState(0);
  const [messageVisible, setMessageVisible] = useState(true);
  const [fieldRotation, setFieldRotation] = useState(0);

  // Long press progress ring
  const [longPressProgress, setLongPressProgress] = useState(0); // 0–1
  const [longPressActive, setLongPressActive] = useState(false);
  const longPressAnimRef = useRef<number>(0);
  const longPressStartRef = useRef<number>(0);

  // Drag state
  const [offset, setOffset] = useState({ x: 0, y: 0 });
  const [isDragging, setIsDragging] = useState(false);
  const [isReturning, setIsReturning] = useState(false);

  const animRef = useRef<number>(0);
  const startRef = useRef(Date.now());
  const dragStartRef = useRef({ x: 0, y: 0 });
  const velRef = useRef({ x: 0, y: 0 });
  const offsetRef = useRef({ x: 0, y: 0 });
  const returnAnimRef = useRef<number>(0);
  const longPressRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const isDraggingRef = useRef(false);

  const messages = agentMessages[stateData.key] ?? agentMessages['standby'];

  // Cycle messages
  useEffect(() => {
    const interval = setInterval(() => {
      setMessageVisible(false);
      setTimeout(() => {
        setMessageIdx(i => (i + 1) % messages.length);
        setMessageVisible(true);
      }, 400);
    }, 3200);
    return () => clearInterval(interval);
  }, [messages.length, stateData.key]);

  useEffect(() => {
    setMessageIdx(0);
    setMessageVisible(true);
  }, [stateData.key]);

  // Main animation loop
  useEffect(() => {
    const animate = () => {
      const elapsed = (Date.now() - startRef.current) / 1000;
      const speed = stateData.breathSpeed ?? 1;
      setRotation(elapsed * 14);
      setFieldRotation(elapsed * -5);
      // Breathing: gentle scale oscillation
      setBreathScale(1 + Math.sin(elapsed * speed * 0.8) * 0.025);
      animRef.current = requestAnimationFrame(animate);
    };
    animRef.current = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(animRef.current);
  }, [stateData.breathSpeed]);

  // Spring return
  const startReturn = useCallback(() => {
    cancelAnimationFrame(returnAnimRef.current);
    setIsReturning(true);
    const spring = () => {
      const stiffness = 0.16;
      const damping = 0.70;
      velRef.current.x += -offsetRef.current.x * stiffness;
      velRef.current.y += -offsetRef.current.y * stiffness;
      velRef.current.x *= damping;
      velRef.current.y *= damping;
      offsetRef.current.x += velRef.current.x;
      offsetRef.current.y += velRef.current.y;
      const dist = Math.sqrt(offsetRef.current.x ** 2 + offsetRef.current.y ** 2);
      setOffset({ x: offsetRef.current.x, y: offsetRef.current.y });
      onDragOffset?.(offsetRef.current.x, offsetRef.current.y);
      if (dist > 0.3 || Math.abs(velRef.current.x) > 0.1) {
        returnAnimRef.current = requestAnimationFrame(spring);
      } else {
        offsetRef.current = { x: 0, y: 0 };
        velRef.current = { x: 0, y: 0 };
        setOffset({ x: 0, y: 0 });
        onDragOffset?.(0, 0);
        setIsReturning(false);
      }
    };
    returnAnimRef.current = requestAnimationFrame(spring);
  }, [onDragOffset]);

  // Long press progress animation
  const startLongPressAnim = useCallback(() => {
    longPressStartRef.current = Date.now();
    setLongPressProgress(0);
    setLongPressActive(true);
    const tick = () => {
      const elapsed = Date.now() - longPressStartRef.current;
      const progress = Math.min(elapsed / LONG_PRESS_DURATION, 1);
      setLongPressProgress(progress);
      if (progress < 1) {
        longPressAnimRef.current = requestAnimationFrame(tick);
      }
    };
    longPressAnimRef.current = requestAnimationFrame(tick);
  }, []);

  const cancelLongPressAnim = useCallback(() => {
    cancelAnimationFrame(longPressAnimRef.current);
    setLongPressActive(false);
    setLongPressProgress(0);
  }, []);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    cancelAnimationFrame(returnAnimRef.current);
    isDraggingRef.current = true;
    setIsDragging(true);
    setIsReturning(false);
    dragStartRef.current = { x: e.clientX - offsetRef.current.x, y: e.clientY - offsetRef.current.y };
    velRef.current = { x: 0, y: 0 };
    // Start long press progress ring
    startLongPressAnim();
    longPressRef.current = setTimeout(() => {
      cancelLongPressAnim();
      onLongPress?.();
    }, LONG_PRESS_DURATION);
  }, [onLongPress, startLongPressAnim, cancelLongPressAnim]);

  useEffect(() => {
    if (!isDragging) return;
    const handleMove = (e: MouseEvent) => {
      if (!isDraggingRef.current) return;
      if (longPressRef.current) { clearTimeout(longPressRef.current); longPressRef.current = null; }
      const rawX = e.clientX - dragStartRef.current.x;
      const rawY = e.clientY - dragStartRef.current.y;
      const maxR = 100;
      const dist = Math.sqrt(rawX ** 2 + rawY ** 2);
      const clamped = Math.min(dist, maxR);
      const a = Math.atan2(rawY, rawX);
      const nx = Math.cos(a) * clamped;
      const ny = Math.sin(a) * clamped;
      velRef.current.x = nx - offsetRef.current.x;
      velRef.current.y = ny - offsetRef.current.y;
      offsetRef.current = { x: nx, y: ny };
      setOffset({ x: nx, y: ny });
      onDragOffset?.(nx, ny);
    };
    const handleUp = () => {
      if (longPressRef.current) { clearTimeout(longPressRef.current); longPressRef.current = null; }
      cancelLongPressAnim();
      isDraggingRef.current = false;
      setIsDragging(false);
      startReturn();
    };
    window.addEventListener('mousemove', handleMove);
    window.addEventListener('mouseup', handleUp);
    return () => { window.removeEventListener('mousemove', handleMove); window.removeEventListener('mouseup', handleUp); };
  }, [isDragging, startReturn, onDragOffset, cancelLongPressAnim]);

  const isAnomaly = stateData.key.startsWith('error');
  const orbColor = activePlanetColor ?? stateData.accentColor;
  const orbGlow = stateData.orbGlow;
  const orbSize = 100;
  // Focus mode boosts
  const focusScaleBoost = focusMode ? 1.12 : 1;
  const focusGlowBoost = focusMode ? 1.8 : 1;
  const tiltX = offset.y * 0.12;
  const tiltY = -offset.x * 0.12;
  const dragDist = Math.sqrt(offset.x ** 2 + offset.y ** 2);
  const dragRatio = Math.min(dragDist / 100, 1);

  // Status color
  const statusColor = isAnomaly ? '#fb7185' : orbColor;

  return (
    <div
      className="absolute flex items-center justify-center"
      style={{
        left: '50%', top: '50%',
        transform: `translate(calc(-50% + ${offset.x}px), calc(-50% + ${offset.y}px))`,
        zIndex: isDragging ? 50 : 30,
        transition: isDragging ? 'none' : isReturning ? 'none' : 'none',
      }}
    >
      {/* ── Outermost information field — very subtle, large ── */}
      <div
        className="absolute rounded-full pointer-events-none"
        style={{
          width: (orbSize + 160 + pulse * 16) * (focusMode ? 1.3 : 1),
          height: (orbSize + 160 + pulse * 16) * (focusMode ? 1.3 : 1),
          background: `radial-gradient(circle, ${orbGlow} 0%, transparent 55%)`,
          opacity: (0.18 + pulse * 0.08) * focusGlowBoost,
          transform: `rotate(${fieldRotation}deg)`,
          transition: 'width 0.9s cubic-bezier(0.34,1.56,0.64,1), height 0.9s cubic-bezier(0.34,1.56,0.64,1), opacity 0.9s ease',
        }}
      />

      {/* ── Breathing field — medium ── */}
      <div
        className="absolute rounded-full pointer-events-none"
        style={{
          width: orbSize + 80,
          height: orbSize + 80,
          background: `radial-gradient(circle, ${orbGlow} 0%, transparent 60%)`,
          opacity: (0.28 + pulse * 0.14) * focusGlowBoost,
          transform: `scale(${breathScale * focusScaleBoost})`,
          transition: 'opacity 0.9s ease, transform 0.9s cubic-bezier(0.34,1.56,0.64,1)',
        }}
      />

      {/* ── Focus mode extra ring — only in focus mode ── */}
      {focusMode && (
        <div
          className="absolute rounded-full pointer-events-none"
          style={{
            width: orbSize + 130,
            height: orbSize + 130,
            border: `1px solid ${orbColor}18`,
            animation: 'focusModeRing 3s ease-in-out infinite',
          }}
        />
      )}

      {/* ── Outer orbit ring — thin, slow rotation ── */}
      <div
        className="absolute rounded-full pointer-events-none"
        style={{
          width: orbSize + 52,
          height: orbSize + 52,
          border: `1px solid ${orbColor}${isDragging ? '35' : '18'}`,
          transform: `rotate(${rotation}deg)`,
          opacity: 1 - dragRatio * 0.5,
        }}
      >
        {/* Orbiting dot */}
        <div className="absolute rounded-full" style={{
          width: 4, height: 4,
          background: statusColor,
          boxShadow: `0 0 8px ${statusColor}`,
          top: -2, left: '50%',
          transform: 'translateX(-50%)',
          opacity: 0.85 + pulse * 0.15,
        }} />
        {/* Second dot — opposite side */}
        <div className="absolute rounded-full" style={{
          width: 3, height: 3,
          background: orbColor,
          bottom: -1.5, left: '50%',
          transform: 'translateX(-50%)',
          opacity: 0.4,
        }} />
      </div>

      {/* ── Inner ring — counter-rotation, dashed ── */}
      <div
        className="absolute rounded-full pointer-events-none"
        style={{
          width: orbSize + 24,
          height: orbSize + 24,
          border: `1px dashed ${orbColor}12`,
          transform: `rotate(${-rotation * 0.6}deg)`,
        }}
      />

      {/* ── Long press progress ring ── */}
      {longPressActive && (
        <svg
          className="absolute pointer-events-none"
          style={{
            width: orbSize + 16,
            height: orbSize + 16,
            left: '50%',
            top: '50%',
            transform: 'translate(-50%, -50%)',
            overflow: 'visible',
          }}
        >
          {(() => {
            const r = (orbSize + 16) / 2 - 3;
            const cx = (orbSize + 16) / 2;
            const cy = (orbSize + 16) / 2;
            const circumference = 2 * Math.PI * r;
            const dashOffset = circumference * (1 - longPressProgress);
            return (
              <>
                {/* Track */}
                <circle
                  cx={cx} cy={cy} r={r}
                  fill="none"
                  stroke={`${orbColor}18`}
                  strokeWidth="2"
                />
                {/* Progress arc */}
                <circle
                  cx={cx} cy={cy} r={r}
                  fill="none"
                  stroke={orbColor}
                  strokeWidth="2.5"
                  strokeLinecap="round"
                  strokeDasharray={circumference}
                  strokeDashoffset={dashOffset}
                  transform={`rotate(-90 ${cx} ${cy})`}
                  style={{
                    filter: `drop-shadow(0 0 4px ${orbColor})`,
                    transition: 'stroke-dashoffset 0.04s linear',
                  }}
                />
              </>
            );
          })()}
        </svg>
      )}

      {/* ── Core orb ── */}
      <div
        className="relative rounded-full flex items-center justify-center"
        style={{
          width: orbSize,
          height: orbSize,
          background: `radial-gradient(circle at 38% 32%, ${orbColor}bb 0%, ${stateData.orbColor} 65%)`,
          boxShadow: `
            0 0 ${(35 + pulse * 22 + dragRatio * 18) * focusGlowBoost}px ${orbGlow},
            0 0 ${(14 + pulse * 8) * focusGlowBoost}px ${orbGlow} inset,
            0 0 ${focusMode ? '120px' : isAnomaly ? '70px' : '50px'} ${orbGlow}
          `,
          transform: `perspective(300px) rotateX(${tiltX}deg) rotateY(${tiltY}deg) scale(${isDragging ? 1.06 : breathScale * focusScaleBoost})`,
          transition: isDragging ? 'transform 0.05s ease' : 'transform 0.9s cubic-bezier(0.34,1.56,0.64,1), box-shadow 0.9s ease',
          cursor: isDragging ? 'grabbing' : 'grab',
          animation: isAnomaly && !isDragging ? 'anomalyPulse 1.2s ease-in-out infinite' : 'none',
          userSelect: 'none',
        }}
        onMouseDown={handleMouseDown}
      >
        {/* Inner shimmer */}
        <div className="absolute inset-0 rounded-full pointer-events-none" style={{
          background: `radial-gradient(circle at ${32 + tiltY * 0.5}% ${26 - tiltX * 0.5}%, rgba(255,255,255,${0.22 + pulse * 0.1 + dragRatio * 0.08}) 0%, transparent 55%)`,
        }} />

        {/* Agent icon — abstract, not a logo */}
        <div className="relative flex items-center justify-center" style={{ zIndex: 1, pointerEvents: 'none' }}>
          <div style={{
            width: 32, height: 32,
            borderRadius: '50%',
            background: `radial-gradient(circle at 40% 35%, rgba(255,255,255,0.35) 0%, rgba(255,255,255,0.08) 100%)`,
            border: '1px solid rgba(255,255,255,0.2)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>
            <i className="ri-brain-line" style={{ fontSize: 14, color: 'rgba(255,255,255,0.85)' }} />
          </div>
        </div>

        {/* Anomaly indicator */}
        {isAnomaly && (
          <div className="absolute rounded-full" style={{
            width: 10, height: 10,
            background: '#fb7185',
            border: '2px solid #03050c',
            top: 10, right: 10,
            boxShadow: '0 0 10px #fb7185',
            animation: 'notifPulse 1.2s ease-in-out infinite',
          }} />
        )}
      </div>

      {/* ── Agent message — floats below, cycles ── */}
      <div
        className="absolute pointer-events-none"
        style={{
          top: orbSize / 2 + 22,
          left: '50%',
          transform: 'translateX(-50%)',
          width: 220,
          textAlign: 'center',
        }}
      >
        {/* Status dot + label */}
        <div className="flex items-center justify-center gap-1.5 mb-1.5">
          <div className="rounded-full" style={{
            width: 4, height: 4,
            background: statusColor,
            boxShadow: `0 0 5px ${statusColor}`,
            animation: 'notifPulse 2s ease-in-out infinite',
          }} />
          <span style={{
            fontSize: 8.5,
            color: 'rgba(148,163,184,0.45)',
            letterSpacing: '0.22em',
          }}>
            {isAnomaly ? 'ATTENTION' : 'AGENT'}
          </span>
        </div>

        {/* Cycling message */}
        <div style={{
          fontSize: 11,
          color: messageVisible ? (isAnomaly ? 'rgba(251,113,133,0.75)' : `${orbColor}cc`) : 'transparent',
          letterSpacing: '0.06em',
          lineHeight: 1.5,
          transition: 'color 0.35s ease',
          whiteSpace: 'nowrap',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
        }}>
          {messages[messageIdx]}
        </div>

        {/* Long press hint */}
        <div style={{
          fontSize: 8,
          color: 'rgba(71,85,105,0.35)',
          letterSpacing: '0.16em',
          marginTop: 6,
        }}>
          长按语音对话
        </div>
      </div>

      {/* ── Drag hint ring ── */}
      {isDragging && (
        <div className="absolute rounded-full pointer-events-none" style={{
          width: 220, height: 220,
          border: `1px dashed ${orbColor}18`,
          animation: 'dragHintSpin 4s linear infinite',
        }} />
      )}

      <style>{`
        @keyframes dragHintSpin {
          from { transform: translate(-50%,-50%) rotate(0deg); }
          to   { transform: translate(-50%,-50%) rotate(360deg); }
        }
        @keyframes focusModeRing {
          0%, 100% { opacity: 0.18; transform: scale(1); }
          50% { opacity: 0.06; transform: scale(1.06); }
        }
      `}</style>
    </div>
  );
}
